// Package media owns generic media-asset primitives: the assets table, the
// upload pipeline (presigned PUT), the public HLS delivery proxy, and the
// transcode + thumbnail workers. Domain modules (movie, music, story, comic)
// reference assets by ID; they do NOT manage the upload/transcode lifecycle.
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
	Enqueuer    Enqueuer                            // asynq client (api side)
	RequireAuth func(http.Handler) http.Handler     // account middleware (api side)
	CurrentUser func(context.Context) (uuid.UUID, bool)
	PublicBase  string        // public API base for HLS URLs, e.g. https://api.portal.localhost
	UploadTTL   time.Duration // presigned-PUT validity
}

type Module struct {
	deps       Deps
	handler    *Handler
	transcoder *worker.Transcoder
	publicAPI  mediaapi.API
}

func New(d Deps) (*Module, error) {
	if d.Store == nil || d.Repo == nil {
		return nil, errors.New("media: Store and Repo are required")
	}
	ttl := d.UploadTTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	svc := &Service{store: d.Store, repo: d.Repo, enqueue: d.Enqueuer, baseURL: d.PublicBase, uploadTTL: ttl}
	return &Module{
		deps:       d,
		handler:    &Handler{svc: svc, currentUser: d.CurrentUser},
		transcoder: worker.NewTranscoder(d.Store, d.Repo),
		publicAPI:  mediaapi.NewImpl(),
	}, nil
}

// MountHTTP wires the media routes:
//
//	GET  /assets/{id}/hls/*      PUBLIC — HLS manifest + segments (playback)
//	POST /assets                 create upload session (presigned PUT)   [auth]
//	GET  /assets                 list the caller's assets                 [auth]
//	GET  /assets/{id}            asset metadata (+ hls_url when ready)     [auth]
//	PUT  /assets/{id}/source     API-proxied upload of the original (dev)  [auth]
//	POST /assets/{id}/complete   confirm upload → enqueue transcode        [auth]
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/assets", func(r chi.Router) {
		r.Get("/{id}/hls/*", m.handler.HLS) // public playback

		r.Group(func(r chi.Router) {
			if m.deps.RequireAuth != nil {
				r.Use(m.deps.RequireAuth)
			}
			r.Post("/", m.handler.Create)
			r.Get("/", m.handler.List)
			r.Get("/{id}", m.handler.Get)
			r.Put("/{id}/source", m.handler.UploadSource) // dev: API-proxied upload
			r.Post("/{id}/complete", m.handler.Complete)
		})
	})
}

// RegisterTasks attaches the transcode + thumbnail handlers to the worker mux.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(worker.TaskTypeTranscode, m.transcoder.Handle)
	mux.HandleFunc(worker.TaskTypeThumbnail, worker.HandleThumbnail)
}

func (m *Module) API() mediaapi.API { return m.publicAPI }
