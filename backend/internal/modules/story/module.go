// Package story owns long-form written content: stories with chapters,
// authors, reading progress. Text is stored inline; cover images and
// audio narration references live as assets via the media module.
package story

import (
	"github.com/go-chi/chi/v5"
	"github.com/hibiken/asynq"

	storyapi "github.com/portal/backend/internal/modules/story/api"
)

type Deps struct{}

type Module struct {
	publicAPI storyapi.API
}

func New(_ Deps) (*Module, error) {
	return &Module{publicAPI: storyapi.NewImpl()}, nil
}

func (m *Module) MountHTTP(_ chi.Router)          {}
func (m *Module) RegisterTasks(_ *asynq.ServeMux) {}
func (m *Module) API() storyapi.API               { return m.publicAPI }
