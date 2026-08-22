// Package comic owns the comic vertical (SPEC-02): comics/chapters/pages +
// reading progress over media image assets — the reference media→domain pattern.
// Mutations are owner-or-elevated (RequireOwnerOrPermission); published-or-owner
// reads. It consumes media:asset_deleted to reap dangling references (P0.6).
package comic

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	comicapi "github.com/portal/backend/internal/modules/comic/api"
)

// Deps are the comic module's dependencies. cmd/api fills the HTTP side (incl.
// the owner-or-elevated middlewares built from the account Engine + this module's
// owner extractors); cmd/worker constructs it with Repo (+ the asset-deleted
// consumer). Nil middlewares mount the route without the guard (tests).
type Deps struct {
	Repo   Repository
	Media  MediaAPI
	Events EventPublisher

	// P1.7 zip import. API side sets Storage + Enqueuer; worker side additionally
	// sets RunInTenant (+ a Media whose IngestImage is wired).
	Storage     ObjectStore
	Enqueuer    Enqueuer
	RunInTenant RunInTenant

	// P1.8 external-source sync (API side). Scraper triggers the Python service;
	// InternalSecret guards the scraper→api sync-callback. Nil Scraper ⇒ 503.
	Scraper        ScraperClient
	InternalSecret string
	// SourceAllowlist restricts sync-source hosts (COMIC_SOURCE_ALLOWLIST). Empty
	// allows any public host; internal/private addresses are always rejected.
	SourceAllowlist []string

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)

	WriteComicMW   func(http.Handler) http.Handler // owner or comics:write:any, by comic id
	WriteChapterMW func(http.Handler) http.Handler // owner or comics:write:any, by chapter id
	DeleteComicMW  func(http.Handler) http.Handler // owner or comics:delete:any, by comic id
	DeletePageMW   func(http.Handler) http.Handler // owner or comics:delete:any, by page id
	PublishMW      func(http.Handler) http.Handler // owner or comics:publish:any, by comic id
}

type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("comic: Repo is required")
	}
	svc := &Service{repo: d.Repo, media: d.Media, events: d.Events, store: d.Storage, enqueue: d.Enqueuer, runInTenant: d.RunInTenant, scraper: d.Scraper, sourceAllowlist: d.SourceAllowlist}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/comics", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("comics:read")).Get("/", m.handler.ListComics)
		r.With(m.perm("comics:write:own")).Get("/mine", m.handler.ListMine)
		r.With(m.perm("comics:write:own")).Post("/", m.handler.CreateComic)
		r.With(m.perm("comics:read")).Get("/{id}", m.handler.GetComic)
		r.With(m.guard(m.deps.WriteComicMW)).Patch("/{id}", m.handler.UpdateComic)
		r.With(m.guard(m.deps.DeleteComicMW)).Delete("/{id}", m.handler.DeleteComic)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/publish", m.handler.Publish)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/unpublish", m.handler.Unpublish)
		r.With(m.guard(m.deps.WriteComicMW)).Post("/{id}/chapters", m.handler.CreateChapter)
		r.With(m.guard(m.deps.WriteComicMW)).Put("/{id}/chapters:order", m.handler.ReorderChapters)
		r.With(m.guard(m.deps.WriteComicMW)).Post("/{id}/imports", m.handler.CreateComicImport)     // P1.7: whole-comic (multi-chapter) zip import
		r.With(m.guard(m.deps.WriteComicMW)).Get("/{id}/sync-sources", m.handler.ListSyncSources)   // P1.8
		r.With(m.guard(m.deps.WriteComicMW)).Post("/{id}/sync-sources", m.handler.CreateSyncSource) // P1.8
		r.Put("/{id}/progress", m.handler.SaveProgress)                                             // authenticated; service validates readability
	})
	r.Route("/sync-sources", func(r chi.Router) { // P1.8: sync source ops (owner-checked in handler)
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.Post("/{id}/sync", m.handler.TriggerSync)
		r.Post("/{id}/cancel", m.handler.CancelSync)
		r.Delete("/{id}", m.handler.DeleteSyncSource)
	})
	// Internal: the scraper service calls this when a zip is uploaded (or a scrape
	// failed). Guarded by a shared secret, NOT user auth (P1.8).
	r.Route("/internal/comic", func(r chi.Router) {
		r.Use(m.requireInternalSecret)
		r.Post("/sync-batch", m.handler.SyncBatch)       // allocate a batch import job
		r.Post("/sync-callback", m.handler.SyncCallback) // a batch's zip is uploaded → import it
		r.Post("/sync-progress", m.handler.SyncProgress) // overall chapter progress
		r.Post("/sync-finalize", m.handler.SyncFinalize) // sync done → set source status
	})
	r.Route("/chapters", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("comics:read")).Get("/{id}/pages", m.handler.ReaderPages)
		r.With(m.guard(m.deps.WriteChapterMW)).Patch("/{id}", m.handler.UpdateChapter)
		r.With(m.guard(m.deps.WriteChapterMW)).Delete("/{id}", m.handler.DeleteChapter)
		r.With(m.guard(m.deps.WriteChapterMW)).Post("/{id}/pages", m.handler.CreatePages)
		r.With(m.guard(m.deps.WriteChapterMW)).Put("/{id}/pages:order", m.handler.ReorderPages)
		r.With(m.guard(m.deps.WriteChapterMW)).Post("/{id}/imports", m.handler.CreateImport) // P1.7: create a zip-import job
	})
	r.Route("/pages", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.guard(m.deps.DeletePageMW)).Delete("/{id}", m.handler.DeletePage)
	})
	r.Route("/imports", func(r chi.Router) { // P1.7: zip upload + status poll (owner-checked in handler)
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.Put("/{id}/zip", m.handler.UploadImportZip)
		r.Get("/{id}", m.handler.GetImport)
	})
}

// RegisterTasks registers the media:asset_deleted consumer (P0.6). cmd/worker
// subscribes media:asset_deleted → comic:on_asset_deleted on the shared publisher.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(comicapi.TaskOnAssetDeleted, m.handleAssetDeleted)
	mux.HandleFunc(comicapi.TaskImportZip, m.handleImportZip)
}

// handleImportZip is the comic:import_zip worker task (P1.7).
func (m *Module) handleImportZip(ctx context.Context, t *asynq.Task) error {
	var p comicapi.ImportZipPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil // malformed → skip, don't retry
	}
	importID, err := uuid.Parse(p.ImportID)
	if err != nil {
		return nil
	}
	return m.svc.RunImport(ctx, importID)
}

func (m *Module) handleAssetDeleted(ctx context.Context, t *asynq.Task) error {
	var p comicapi.AssetDeletedPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil // malformed → skip, don't retry
	}
	assetID, err := uuid.Parse(p.AssetID)
	if err != nil {
		return nil
	}
	return m.svc.HandleAssetDeleted(ctx, assetID)
}

// Owner extractors — cmd/api builds the RequireOwnerOrPermission middlewares from
// these (comic must not import account/rbac).
func (m *Module) OwnerByComic(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByComic(ctx, id)
}
func (m *Module) OwnerByChapter(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByChapter(ctx, id)
}
func (m *Module) OwnerByPage(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByPage(ctx, id)
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

// requireInternalSecret gates the scraper→api callback. A blank configured secret
// (or a mismatch) → 404 (don't reveal the endpoint exists).
func (m *Module) requireInternalSecret(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if m.deps.InternalSecret == "" || r.Header.Get("X-Internal-Secret") != m.deps.InternalSecret {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}
