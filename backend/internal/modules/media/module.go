// Package media owns generic media-asset primitives: the assets table, the
// upload pipeline (presigned PUT), the public HLS delivery proxy, the image
// (WebP variant) + transcode + poster workers, and the delete/janitor flow.
// Domain modules (movie, music, story, comic) reference assets by ID; they do
// NOT manage the upload/transcode lifecycle.
package media

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/platform/storage"
)

// Deps are the cross-cutting dependencies the media module needs. cmd/api fills
// the HTTP side; cmd/worker fills the worker side (nil for the rest).
type Deps struct {
	Store       storage.Storage
	Repo        Repository
	Enqueuer    Enqueuer                        // asynq client (api + worker: poster follow-up)
	Events      EventPublisher                  // platform/events publisher (optional)
	RequireAuth func(http.Handler) http.Handler // account middleware (api side)
	// DeleteMiddleware guards DELETE /assets/{id} with owner-or-permission. Built
	// by cmd/api from accountMod.Engine() + the asset owner extractor, since media
	// must not import account/rbac. Nil ⇒ the DELETE route is not mounted.
	DeleteMiddleware func(http.Handler) http.Handler
	CurrentUser      func(context.Context) (uuid.UUID, bool)
	PublicBase       string        // public API base for HLS URLs, e.g. https://api.portal.localhost
	UploadTTL        time.Duration // presigned-PUT validity
}

type Module struct {
	deps           Deps
	svc            *Service
	handler        *Handler
	transcoder     *worker.Transcoder
	imageProcessor *worker.ImageProcessor
	thumbnailer    *worker.Thumbnailer
	publicAPI      mediaapi.API
}

func New(d Deps) (*Module, error) {
	if d.Store == nil || d.Repo == nil {
		return nil, errors.New("media: Store and Repo are required")
	}
	ttl := d.UploadTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	svc := &Service{
		store:     d.Store,
		repo:      d.Repo,
		enqueue:   d.Enqueuer,
		events:    d.Events,
		baseURL:   d.PublicBase,
		uploadTTL: ttl,
	}
	return &Module{
		deps:           d,
		svc:            svc,
		handler:        &Handler{svc: svc, currentUser: d.CurrentUser},
		transcoder:     worker.NewTranscoder(d.Store, d.Repo, d.Enqueuer, d.Events),
		imageProcessor: worker.NewImageProcessor(d.Store, d.Repo, d.Events),
		thumbnailer:    worker.NewThumbnailer(d.Store, d.Repo),
		publicAPI:      mediaapi.NewImpl(svc.ContinueItems),
	}, nil
}

// MountHTTP wires the media routes:
//
//	GET    /assets/{id}/hls/*               PUBLIC — HLS manifest + segments
//	GET    /assets/{id}/variants/{variant}  PUBLIC — WebP variant (thumb/medium/poster)
//	POST   /assets                          create upload session (presigned PUT)  [auth]
//	GET    /assets                          list the caller's assets (cursor)       [auth]
//	GET    /assets/{id}                     asset metadata (+ hls_url when ready)    [auth]
//	GET    /assets/{id}/original            download the byte-identical original    [auth, owner]
//	PUT    /assets/{id}/source              API-proxied upload of the original (dev) [auth]
//	POST   /assets/{id}/complete            sniff + confirm upload → enqueue         [auth]
//	DELETE /assets/{id}                     delete asset + all storage objects       [auth, owner-or-perm]
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/assets", func(r chi.Router) {
		r.Get("/{id}/hls/*", m.handler.HLS)                       // public playback
		r.Get("/{id}/variants/{variant}", m.handler.ServeVariant) // public variant proxy

		r.Group(func(r chi.Router) {
			if m.deps.RequireAuth != nil {
				r.Use(m.deps.RequireAuth)
			}
			r.Post("/", m.handler.Create)
			r.Get("/", m.handler.List)
			r.Get("/{id}", m.handler.Get)
			r.Get("/{id}/original", m.handler.DownloadOriginal)
			r.Put("/{id}/source", m.handler.UploadSource) // dev: API-proxied upload
			r.Post("/{id}/complete", m.handler.Complete)
			r.Get("/{id}/progress", m.handler.GetProgress)
			r.Put("/{id}/progress", m.handler.PutProgress)
			if m.deps.DeleteMiddleware != nil {
				r.With(m.deps.DeleteMiddleware).Delete("/{id}", m.handler.Delete)
			}
		})
	})
}

// RegisterHeavyTasks attaches the CPU/memory-heavy handlers (transcode +
// process_image) to the low-concurrency "heavy" server's mux (SPEC-01 P0.1).
func (m *Module) RegisterHeavyTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(worker.TaskTypeTranscode, m.transcoder.Handle)
	mux.HandleFunc(worker.TaskTypeProcessImage, m.imageProcessor.Handle)
}

// RegisterLightTasks attaches the cheap handlers (poster generation) to the
// weighted server's mux so heavy jobs never starve them.
func (m *Module) RegisterLightTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(worker.TaskTypeThumbnail, m.thumbnailer.Handle)
}

// PurgeOrphans is the media:purge_orphans handler body (P0.3). cmd/worker
// registers it on the shared scheduler + the light mux.
func (m *Module) PurgeOrphans(ctx context.Context) error { return m.svc.PurgeOrphans(ctx) }

// AssetOwner resolves an asset's owner (for the RequireOwnerOrPermission extractor).
func (m *Module) AssetOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.AssetOwner(ctx, id)
}

func (m *Module) API() mediaapi.API { return m.publicAPI }
