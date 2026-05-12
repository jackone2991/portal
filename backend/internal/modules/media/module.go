// Package media owns generic media-asset primitives: the assets table,
// upload pipeline (presigned multipart), and the transcode + thumbnail
// workers. Domain modules (movie, music, story, comic) reference assets
// by ID; they do NOT manage the upload/transcode lifecycle themselves.
package media

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
)

type Deps struct {
	// Storage, jobs client, repository adapter (TBD).
}

type Module struct {
	deps      Deps
	publicAPI mediaapi.API
}

func New(d Deps) (*Module, error) {
	return &Module{
		deps:      d,
		publicAPI: mediaapi.NewImpl(),
	}, nil
}

// MountHTTP — placeholder. Will wire:
//   POST  /assets               create upload session (presigned URLs)
//   GET   /assets/{id}          get asset metadata
//   POST  /assets/{id}/complete finish multipart, enqueue transcode/thumbnail
func (m *Module) MountHTTP(_ chi.Router) {}

// RegisterTasks attaches transcode + thumbnail handlers to the worker mux.
// Handlers are stubs until the FFmpeg pipeline is implemented.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(worker.TaskTypeTranscode, worker.HandleTranscode)
	mux.HandleFunc(worker.TaskTypeThumbnail, worker.HandleThumbnail)
}

func (m *Module) API() mediaapi.API { return m.publicAPI }
