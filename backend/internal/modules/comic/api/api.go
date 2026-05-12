// Package api is the public surface of the comic module.
package api

import (
	"context"

	"github.com/google/uuid"
)

type Comic struct {
	ID    uuid.UUID
	Title string
}

type API interface {
	GetComic(ctx context.Context, id uuid.UUID) (*Comic, error)
}

type Impl struct{}

func NewImpl() *Impl { return &Impl{} }

func (a *Impl) GetComic(_ context.Context, _ uuid.UUID) (*Comic, error) { return nil, nil }
