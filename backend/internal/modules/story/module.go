// Package story owns the long-form written-content vertical — stories with inline
// markdown chapters over media's image assets (cover only). It copies the comic
// reference pattern (SPEC-02), slimmed: text chapters, no pages, no reading
// progress. Mutations are owner-or-elevated (RequireOwnerOrPermission);
// published-or-owner reads. It consumes media:asset_deleted to reap dangling
// cover references and emits story:published on publish. Other modules import
// only story/api.
package story

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	storyapi "github.com/portal/backend/internal/modules/story/api"
)

// Deps are the story module's dependencies. cmd/api fills the HTTP side (incl.
// the owner-or-elevated middlewares built from the account Engine + this module's
// owner extractors); cmd/worker constructs it with Repo (+ the asset-deleted
// consumer). Nil middlewares mount the route without the guard (tests).
type Deps struct {
	Repo   Repository
	Media  MediaAPI
	Events EventPublisher

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)

	WriteStoryMW   func(http.Handler) http.Handler // owner or stories:write:any, by story id
	WriteChapterMW func(http.Handler) http.Handler // owner or stories:write:any, by chapter id
	DeleteStoryMW  func(http.Handler) http.Handler // owner or stories:delete:any, by story id
	PublishMW      func(http.Handler) http.Handler // owner or stories:publish:any, by story id
}

type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("story: Repo is required")
	}
	svc := &Service{repo: d.Repo, media: d.Media, events: d.Events}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

// MountHTTP wires the story routes (all under RequireAuth). Chapter-level routes
// live under /story-chapters to avoid colliding with comic's /chapters.
//
//	GET    /stories                     list published (cursor)     [stories:read]
//	GET    /stories/mine                list own incl. drafts       [stories:write:own]
//	POST   /stories                     create                      [stories:write:own]
//	GET    /stories/{id}                detail + chapter summaries  [stories:read]
//	PATCH  /stories/{id}                update                      owner or stories:write:any
//	DELETE /stories/{id}                delete                      owner or stories:delete:any
//	POST   /stories/{id}/publish        publish (needs ≥1 body)     owner or stories:publish:any
//	POST   /stories/{id}/unpublish      back to draft               owner or stories:publish:any
//	GET    /stories/{id}/chapters       reader (bodies, pub/own)    [stories:read]
//	POST   /stories/{id}/chapters       add chapter                 owner or stories:write:any
//	PUT    /stories/{id}/chapters:order reorder                     owner or stories:write:any
//	PATCH  /story-chapters/{id}         update chapter              owner or stories:write:any
//	DELETE /story-chapters/{id}         delete chapter              owner or stories:write:any
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/stories", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("stories:read")).Get("/", m.handler.ListStories)
		r.With(m.perm("stories:write:own")).Get("/mine", m.handler.ListMine)
		r.With(m.perm("stories:write:own")).Post("/", m.handler.CreateStory)
		r.With(m.perm("stories:read")).Get("/{id}", m.handler.GetStory)
		r.With(m.guard(m.deps.WriteStoryMW)).Patch("/{id}", m.handler.UpdateStory)
		r.With(m.guard(m.deps.DeleteStoryMW)).Delete("/{id}", m.handler.DeleteStory)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/publish", m.handler.Publish)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/unpublish", m.handler.Unpublish)
		r.With(m.perm("stories:read")).Get("/{id}/chapters", m.handler.Chapters)
		r.With(m.guard(m.deps.WriteStoryMW)).Post("/{id}/chapters", m.handler.CreateChapter)
		r.With(m.guard(m.deps.WriteStoryMW)).Put("/{id}/chapters:order", m.handler.ReorderChapters)
	})
	r.Route("/story-chapters", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.guard(m.deps.WriteChapterMW)).Patch("/{id}", m.handler.UpdateChapter)
		r.With(m.guard(m.deps.WriteChapterMW)).Delete("/{id}", m.handler.DeleteChapter)
	})
}

// RegisterTasks registers the media:asset_deleted consumer. cmd/worker subscribes
// media:asset_deleted → story:on_asset_deleted on the shared publisher.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(storyapi.TaskOnAssetDeleted, m.handleAssetDeleted)
}

func (m *Module) handleAssetDeleted(ctx context.Context, t *asynq.Task) error {
	var p storyapi.AssetDeletedPayload
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
// these (story must not import account/rbac).
func (m *Module) OwnerByStory(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByStory(ctx, id)
}
func (m *Module) OwnerByChapter(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByChapter(ctx, id)
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
