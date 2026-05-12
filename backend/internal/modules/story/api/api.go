// Package api is the public surface of the story module.
package api

import (
	"context"

	"github.com/google/uuid"
)

type Story struct {
	ID    uuid.UUID
	Title string
}

type API interface {
	GetStory(ctx context.Context, id uuid.UUID) (*Story, error)
}

type Impl struct{}

func NewImpl() *Impl { return &Impl{} }

func (a *Impl) GetStory(_ context.Context, _ uuid.UUID) (*Story, error) { return nil, nil }
