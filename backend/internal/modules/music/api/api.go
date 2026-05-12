// Package api is the public surface of the music module.
package api

import (
	"context"

	"github.com/google/uuid"
)

type Track struct {
	ID    uuid.UUID
	Title string
}

type API interface {
	GetTrack(ctx context.Context, id uuid.UUID) (*Track, error)
}

type Impl struct{}

func NewImpl() *Impl { return &Impl{} }

func (a *Impl) GetTrack(_ context.Context, _ uuid.UUID) (*Track, error) { return nil, nil }
