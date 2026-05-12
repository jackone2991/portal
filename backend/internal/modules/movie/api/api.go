// Package api is the public surface of the movie module.
package api

import (
	"context"

	"github.com/google/uuid"
)

type Movie struct {
	ID    uuid.UUID
	Title string
	// trimmed projection; full record stays inside the module
}

type API interface {
	GetMovie(ctx context.Context, id uuid.UUID) (*Movie, error)
}

type Impl struct{}

func NewImpl() *Impl { return &Impl{} }

func (a *Impl) GetMovie(_ context.Context, _ uuid.UUID) (*Movie, error) { return nil, nil }
