// Package music owns the music domain: tracks, albums, artists, playlists.
package music

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	musicapi "github.com/portal/backend/internal/modules/music/api"
)

type Deps struct{}

type Module struct {
	publicAPI musicapi.API
}

func New(_ Deps) (*Module, error) {
	return &Module{publicAPI: musicapi.NewImpl()}, nil
}

func (m *Module) MountHTTP(_ chi.Router)          {}
func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}
func (m *Module) API() musicapi.API               { return m.publicAPI }
