// Package music owns the music vertical — tracks over media's audio assets
// (mirroring the movie vertical). A track is a single audio file + metadata
// (title/artist/album/description/cover). Mutations are owner-or-elevated;
// published-or-owner reads. It consumes media:asset_deleted to reap dangling
// references and emits music:track_published on publish. Other modules import
// only music/api.
package music

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	musicapi "github.com/portal/backend/internal/modules/music/api"
)

type Deps struct {
	Repo   Repository
	Media  MediaAPI
	Events EventPublisher

	// Zip import (0038). API side: Storage + Enqueuer mount the import routes.
	// Worker side: Storage + a Media whose Ingest is wired run the task.
	Storage  Storage
	Enqueuer Enqueuer
	// RunInTenant is worker-side only: the import runs outside any request, so it
	// opens the owner's tenant scope itself.
	RunInTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)

	WriteTrackMW  func(http.Handler) http.Handler // owner or music:write:any, by track id
	DeleteTrackMW func(http.Handler) http.Handler // owner or music:delete:any, by track id
	PublishMW     func(http.Handler) http.Handler // owner or music:publish:any, by track id
}

type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("music: Repo is required")
	}
	svc := &Service{
		repo: d.Repo, media: d.Media, events: d.Events,
		store: d.Storage, enqueue: d.Enqueuer, runInTenant: d.RunInTenant,
	}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

// MountHTTP wires the track routes (all under RequireAuth):
//
//	GET/POST /tracks · GET/PATCH/DELETE /tracks/{id} · POST /tracks/{id}/publish|unpublish
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/tracks", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("music:read")).Get("/", m.handler.ListTracks)
		r.With(m.perm("music:write:own")).Get("/mine", m.handler.ListMine)
		r.With(m.perm("music:write:own")).Post("/", m.handler.CreateTrack)
		r.With(m.perm("music:read")).Get("/{id}", m.handler.GetTrack)
		r.With(m.guard(m.deps.WriteTrackMW)).Patch("/{id}", m.handler.UpdateTrack)
		r.With(m.guard(m.deps.DeleteTrackMW)).Delete("/{id}", m.handler.DeleteTrack)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/publish", m.handler.Publish)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/unpublish", m.handler.Unpublish)

		// Bulk zip import (0038). Mounted only where storage + queue exist, so a
		// binary without them has no dead routes. Everything is owner-scoped in
		// the handler — an import creates tracks for the caller and nobody else,
		// so `music:write:own` is the whole gate.
		if m.deps.Storage != nil && m.deps.Enqueuer != nil {
			r.Route("/imports", func(r chi.Router) {
				r.Use(m.perm("music:write:own"))
				r.Post("/", m.handler.CreateImport)
				r.Get("/", m.handler.ListImports)
				r.Get("/{id}", m.handler.GetImport)
				r.Put("/{id}/upload", m.handler.UploadImportZip)
			})
		}
	})
}

func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(musicapi.TaskOnAssetDeleted, m.handleAssetDeleted)
	mux.HandleFunc(musicapi.TaskImportZip, m.handleImportZip)
}

// handleImportZip is the music:import_zip worker task (0038).
func (m *Module) handleImportZip(ctx context.Context, t *asynq.Task) error {
	var p musicapi.ImportZipPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil // a payload that cannot parse will never parse; retrying is noise
	}
	importID, err := uuid.Parse(p.ImportID)
	if err != nil {
		return nil
	}
	ownerID, err := uuid.Parse(p.OwnerID)
	if err != nil {
		return nil
	}
	return m.svc.RunImport(ctx, importID, ownerID)
}

func (m *Module) handleAssetDeleted(ctx context.Context, t *asynq.Task) error {
	var p musicapi.AssetDeletedPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	assetID, err := uuid.Parse(p.AssetID)
	if err != nil {
		return nil
	}
	return m.svc.HandleAssetDeleted(ctx, assetID)
}

func (m *Module) OwnerByTrack(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByTrack(ctx, id)
}

func (m *Module) perm(code string) func(http.Handler) http.Handler {
	if m.deps.RequirePermission == nil {
		return passthrough
	}
	return m.deps.RequirePermission(code)
}

func (m *Module) guard(mw func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	if mw == nil {
		return passthrough
	}
	return mw
}

func passthrough(next http.Handler) http.Handler { return next }
