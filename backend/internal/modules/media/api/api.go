// Package api is the public surface of the media module. Domain modules
// (movie, music, story, comic) call here to resolve asset metadata + URLs;
// they never read media.assets directly.
package api

import (
	"context"
	"errors"
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

	// AssetStatuses returns id→status for many assets in a single query. The
	// comic zip-import worker polls thousands of assets to ready; use this
	// instead of a GetAsset per asset per poll round. Run inside a tenant ctx.
	AssetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AssetStatus, error)

	// SignedURL returns a short-lived URL for direct delivery (movies/music
	// players use this). Honours tenant boundary; the URL is for the active
	// tenant only.
	SignedURL(ctx context.Context, id uuid.UUID, expires time.Duration) (string, error)

	// Continue returns the active media progress items for the caller.
	Continue(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error)

	// IngestImage ingests raw image bytes as a media asset (same pipeline as a
	// browser upload) and returns the new asset id. Used by the comic zip-import
	// worker; run it inside a tenant-scoped ctx (SPEC-02 P1.7).
	IngestImage(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error)
}

type Impl struct {
	continueFn func(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error)
	getAssetFn func(ctx context.Context, id uuid.UUID) (*Asset, error)
	statusesFn func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AssetStatus, error)
	ingestFn   func(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error)
}

func NewImpl(
	continueFn func(ctx context.Context, userID uuid.UUID, limit int) ([]ContinueItem, error),
	getAssetFn func(ctx context.Context, id uuid.UUID) (*Asset, error),
	statusesFn func(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AssetStatus, error),
	ingestFn func(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error),
) *Impl {
	return &Impl{continueFn: continueFn, getAssetFn: getAssetFn, statusesFn: statusesFn, ingestFn: ingestFn}
}

// IngestImage delegates to the media service (nil-safe: returns an error if the
// module was built without an ingest function).
func (a *Impl) IngestImage(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error) {
	if a.ingestFn == nil {
		return uuid.Nil, errors.New("media: ingest not available")
	}
	return a.ingestFn(ctx, ownerID, filename, contentType, data)
}

// GetAsset returns the safe cross-module projection of an asset, or (nil, nil)
// if it does not exist — domain verticals (comic/movie/...) validate page/cover
// references through this (kind image, status ready, owner match).
func (a *Impl) GetAsset(ctx context.Context, id uuid.UUID) (*Asset, error) {
	if a.getAssetFn != nil {
		return a.getAssetFn(ctx, id)
	}
	return nil, nil
}

// AssetStatuses returns id→status for many assets at once (empty map if the
// module was built without a statuses function).
func (a *Impl) AssetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]AssetStatus, error) {
	if a.statusesFn != nil {
		return a.statusesFn(ctx, ids)
	}
	return map[uuid.UUID]AssetStatus{}, nil
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
