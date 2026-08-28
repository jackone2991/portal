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
	"github.com/redis/go-redis/v9"

	musicapi "github.com/portal/backend/internal/modules/music/api"
)

type Deps struct {
	Repo Repository
	// Playlists is the playlist persistence surface (0041). The same adapter
	// implements it; nil leaves the playlist routes unmounted.
	Playlists PlaylistRepository
	Media     MediaAPI
	Events    EventPublisher

	// Zip import (0038). API side: Storage + Enqueuer mount the import routes.
	// Worker side: Storage + a Media whose Ingest is wired run the task.
	Storage  Storage
	Enqueuer Enqueuer
	// RunInTenant is worker-side only: the import runs outside any request, so it
	// opens the owner's tenant scope itself.
	RunInTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error

	// Lookup is the MusicBrainz / Cover Art Archive configuration (0039). The
	// zero value leaves outbound lookups off, which is the intended default.
	Lookup LookupConfig
	// Redis backs the 1-req/s global throttle. Nil degrades to local pacing,
	// which is correct for one worker and honest about not being correct for two.
	Redis *redis.Client

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
		mb: newMBClient(d.Lookup, d.Redis), playlists: d.Playlists,
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
		// Bulk status (0041). No ownership middleware, because the ids arrive in
		// the body rather than the path — the service filters them through the
		// caller's own tracks instead. See BulkSetStatus.
		r.With(m.perm("music:write:own")).Post("/bulk-status", m.handler.BulkStatus)
		// Enrichment is a write to the caller's own track, gated like editing it.
		if m.deps.Enqueuer != nil {
			r.With(m.guard(m.deps.WriteTrackMW)).Post("/{id}/enrich", m.handler.EnrichTrack)
			// Mounted even when lookups are disabled: the handler answers 503 with
			// a reason, which is far more useful to a caller than a 404 that looks
			// like the endpoint was never built.
			r.With(m.guard(m.deps.WriteTrackMW)).Post("/{id}/lookup", m.handler.LookupTrack)
		}

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
				r.Post("/{id}/enrich", m.handler.EnrichImport)
				r.Post("/{id}/lookup", m.handler.LookupImport)
			})
		}
	})

	m.mountPlaylists(r)
}

// mountPlaylists wires the playlist routes (0041). Mounted only when a playlist
// repository is wired, so a binary without one has no dead routes.
//
//	GET/POST /playlists · GET/PATCH/DELETE /playlists/{id}
//	POST /playlists/{id}/tracks · DELETE /playlists/{id}/tracks/{trackId}
//
// Everything is owner-scoped inside the queries, so `music:write:own` is the
// whole gate — a playlist is one person's selection of their own tracks.
func (m *Module) mountPlaylists(r chi.Router) {
	if m.deps.Playlists == nil {
		return
	}
	r.Route("/playlists", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("music:read")).Get("/", m.handler.ListPlaylists)
		r.With(m.perm("music:write:own")).Post("/", m.handler.CreatePlaylist)
		r.With(m.perm("music:read")).Get("/{id}", m.handler.GetPlaylist)
		r.With(m.perm("music:write:own")).Patch("/{id}", m.handler.UpdatePlaylist)
		r.With(m.perm("music:write:own")).Delete("/{id}", m.handler.DeletePlaylist)
		r.With(m.perm("music:write:own")).Post("/{id}/tracks", m.handler.AddPlaylistTracks)
		r.With(m.perm("music:write:own")).Delete("/{id}/tracks/{trackId}", m.handler.RemovePlaylistTrack)
	})
}

func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(musicapi.TaskOnAssetDeleted, m.handleAssetDeleted)
	mux.HandleFunc(musicapi.TaskImportZip, m.handleImportZip)
	mux.HandleFunc(musicapi.TaskEnrichTrack, m.handleEnrichTrack)
	mux.HandleFunc(musicapi.TaskLookupTrack, m.handleLookupTrack)
}

// handleLookupTrack is the music:lookup_track worker task (0039).
func (m *Module) handleLookupTrack(ctx context.Context, t *asynq.Task) error {
	var p musicapi.LookupTrackPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	trackID, err := uuid.Parse(p.TrackID)
	if err != nil {
		return nil
	}
	ownerID, err := uuid.Parse(p.OwnerID)
	if err != nil {
		return nil
	}
	return m.svc.LookupTrack(ctx, trackID, ownerID)
}

// handleEnrichTrack is the music:enrich_track worker task (0038).
func (m *Module) handleEnrichTrack(ctx context.Context, t *asynq.Task) error {
	var p musicapi.EnrichTrackPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil
	}
	trackID, err := uuid.Parse(p.TrackID)
	if err != nil {
		return nil
	}
	ownerID, err := uuid.Parse(p.OwnerID)
	if err != nil {
		return nil
	}
	return m.svc.EnrichTrack(ctx, trackID, ownerID)
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
