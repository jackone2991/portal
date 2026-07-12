// Package api is the public surface of the media module. Domain modules
// (movie, music, story, comic) call here to resolve asset metadata + URLs;
// they never read media.assets directly.
package api

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// AssetKind mirrors media.assets.kind without leaking the underlying table.
type AssetKind string

const (
	KindVideo AssetKind = "video"
	KindAudio AssetKind = "audio"
	KindImage AssetKind = "image"
)

// AssetStatus mirrors media.assets.status.
type AssetStatus string

const (
	StatusUploaded   AssetStatus = "uploaded"
	StatusProcessing AssetStatus = "processing"
	StatusReady      AssetStatus = "ready"
	StatusFailed     AssetStatus = "failed"
)

// Asset is the projection safe to expose to other modules.
type Asset struct {
	ID         uuid.UUID
	OwnerID    uuid.UUID
	Kind       AssetKind
	Status     AssetStatus
	MimeType   string
	SizeBytes  int64
	DurationMs *int
	Width      *int
	Height     *int
	HLSMaster  *string // nil unless status=ready and kind=video/audio
	CreatedAt  time.Time
}

// ContinueItem is a cross-module progress item returned by the aggregator.
type ContinueItem struct {
	Module      string    `json:"module"`
	RefID       uuid.UUID `json:"ref_id"`
	Title       string    `json:"title"`
	PosterURL   string    `json:"poster_url"`
	ProgressPct int       `json:"progress_pct"` // rounded percentage
	Href        string    `json:"href"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type API interface {
	// GetAsset returns (nil, nil) if the asset is missing or in another tenant.
	GetAsset(ctx context.Context, id uuid.UUID) (*Asset, error)

	// SignedURL returns a short-lived URL for direct delivery (movies/music
	// players use this). Honours tenant boundary; the URL is for the active
	// tenant only.
	SignedURL(ctx context.Context, id uuid.UUID, expires time.Duration) (string, error)

	// Continue returns the active media progress items for the caller.
	Continue(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error)
}

type Impl struct {
	continueFn func(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error)
}

func NewImpl(continueFn func(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error)) *Impl {
	return &Impl{continueFn: continueFn}
}

func (a *Impl) GetAsset(_ context.Context, _ uuid.UUID) (*Asset, error) {
	return nil, nil // placeholder until repository adapter lands
}

func (a *Impl) SignedURL(_ context.Context, _ uuid.UUID, _ time.Duration) (string, error) {
	return "", nil
}

func (a *Impl) Continue(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error) {
	if a.continueFn != nil {
		return a.continueFn(ctx, userID, limit)
	}
	return nil, nil
}
