// Package movie owns the movie vertical — films over media's video assets, the
// first domain vertical copying the comic reference pattern (SPEC-02). A movie is
// a single video (title/description/poster/release-year + a video_asset_id).
// Mutations are owner-or-elevated (RequireOwnerOrPermission); published-or-owner
// reads. It consumes media:asset_deleted to reap dangling references and emits
// movie:published on publish. Other modules import only movie/api.
package movie

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"

	movieapi "github.com/portal/backend/internal/modules/movie/api"
)

// Deps are the movie module's dependencies. cmd/api fills the HTTP side (incl.
// the owner-or-elevated middlewares built from the account Engine + this module's
// owner extractor); cmd/worker constructs it with Repo (+ the asset-deleted
// consumer). Nil middlewares mount the route without the guard (tests).
type Deps struct {
	Repo   Repository
	Media  MediaAPI
	Events EventPublisher

	RequireAuth       func(http.Handler) http.Handler
	RequirePermission func(code string) func(http.Handler) http.Handler
	CurrentUser       func(context.Context) (uuid.UUID, bool)

	WriteMovieMW  func(http.Handler) http.Handler // owner or movies:write:any, by movie id
	DeleteMovieMW func(http.Handler) http.Handler // owner or movies:delete:any, by movie id
	PublishMW     func(http.Handler) http.Handler // owner or movies:publish:any, by movie id
}

type Module struct {
	deps    Deps
	svc     *Service
	handler *Handler
}

func New(d Deps) (*Module, error) {
	if d.Repo == nil {
		return nil, errors.New("movie: Repo is required")
	}
	svc := &Service{repo: d.Repo, media: d.Media, events: d.Events}
	return &Module{deps: d, svc: svc, handler: &Handler{svc: svc, currentUser: d.CurrentUser}}, nil
}

// MountHTTP wires the movie routes (all under RequireAuth):
//
//	GET    /movies                list published (cursor)   [movies:read]
//	GET    /movies/mine           list own incl. drafts     [movies:write:own]
//	POST   /movies                create                    [movies:write:own]
//	GET    /movies/{id}           detail (published/own)    [movies:read]
//	PATCH  /movies/{id}           update                    owner or movies:write:any
//	DELETE /movies/{id}           delete                    owner or movies:delete:any
//	POST   /movies/{id}/publish   publish (needs ready video) owner or movies:publish:any
//	POST   /movies/{id}/unpublish back to draft             owner or movies:publish:any
func (m *Module) MountHTTP(r chi.Router) {
	r.Route("/movies", func(r chi.Router) {
		if m.deps.RequireAuth != nil {
			r.Use(m.deps.RequireAuth)
		}
		r.With(m.perm("movies:read")).Get("/", m.handler.ListMovies)
		r.With(m.perm("movies:write:own")).Get("/mine", m.handler.ListMine)
		r.With(m.perm("movies:write:own")).Post("/", m.handler.CreateMovie)
		r.With(m.perm("movies:read")).Get("/{id}", m.handler.GetMovie)
		r.With(m.guard(m.deps.WriteMovieMW)).Patch("/{id}", m.handler.UpdateMovie)
		r.With(m.guard(m.deps.DeleteMovieMW)).Delete("/{id}", m.handler.DeleteMovie)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/publish", m.handler.Publish)
		r.With(m.guard(m.deps.PublishMW)).Post("/{id}/unpublish", m.handler.Unpublish)
	})
}

// RegisterTasks registers the media:asset_deleted consumer. cmd/worker subscribes
// media:asset_deleted → movie:on_asset_deleted on the shared publisher.
func (m *Module) RegisterTasks(mux *asynq.ServeMux) {
	mux.HandleFunc(movieapi.TaskOnAssetDeleted, m.handleAssetDeleted)
}

func (m *Module) handleAssetDeleted(ctx context.Context, t *asynq.Task) error {
	var p movieapi.AssetDeletedPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return nil // malformed → skip, don't retry
	}
	assetID, err := uuid.Parse(p.AssetID)
	if err != nil {
		return nil
	}
	return m.svc.HandleAssetDeleted(ctx, assetID)
}

// OwnerByMovie — cmd/api builds RequireOwnerOrPermission from this (movie must
// not import account/rbac).
func (m *Module) OwnerByMovie(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return m.svc.OwnerByMovie(ctx, id)
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
