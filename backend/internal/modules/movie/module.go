// Package movie owns the movie domain: films, episodes, casts, ratings.
// Assets (video files, posters) belong to the media module — this module
// stores only the references + domain metadata.
package movie

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	movieapi "github.com/portal/backend/internal/modules/movie/api"
)

type Deps struct{}

type Module struct {
	publicAPI movieapi.API
}

func New(_ Deps) (*Module, error) {
	return &Module{publicAPI: movieapi.NewImpl()}, nil
}

func (m *Module) MountHTTP(_ chi.Router)              {}
func (m *Module) RegisterTasks(_ *asynq.ServeMux)     {}
func (m *Module) API() movieapi.API                   { return m.publicAPI }
