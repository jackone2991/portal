// Package comic owns the comic-book/manga vertical: comics, chapters,
// pages. Each page is backed by an image asset stored via the media module.
package comic

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	comicapi "github.com/portal/backend/internal/modules/comic/api"
)

type Deps struct{}

type Module struct {
	publicAPI comicapi.API
}

func New(_ Deps) (*Module, error) {
	return &Module{publicAPI: comicapi.NewImpl()}, nil
}

func (m *Module) MountHTTP(_ chi.Router)          {}
func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}
func (m *Module) API() comicapi.API               { return m.publicAPI }
