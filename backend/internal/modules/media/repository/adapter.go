package mediarepo

// Adapter bridges the sqlc-generated Queries to media.Repository (and the
// embedded worker.Repo). cmd/api and cmd/worker construct one with NewAdapter
// and pass it into media.Deps. All pgtype ↔ domain juggling lives here.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/media"
)

type Adapter struct {
	q *Queries
}

func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

// Compile-time proof the adapter satisfies the media persistence surface.
var _ media.Repository = (*Adapter)(nil)

func (a *Adapter) CreateAsset(ctx context.Context, in media.CreateAssetInput) (media.Asset, error) {
	row, err := a.q.CreateAsset(ctx, CreateAssetParams{
		ID:        pgUUID(in.ID),
		OwnerID:   pgUUID(in.OwnerID),
		Kind:      in.Kind,
		SourceKey: in.SourceKey,
		MimeType:  in.MimeType,
		SizeBytes: in.SizeBytes,
	})
	if err != nil {
		return media.Asset{}, err
	}
	return toAsset(row), nil
}

func (a *Adapter) GetAsset(ctx context.Context, id uuid.UUID) (media.Asset, error) {
	row, err := a.q.GetAsset(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return media.Asset{}, media.ErrNotFound
		}
		return media.Asset{}, err
	}
	return toAsset(row), nil
}

func (a *Adapter) ListByOwner(ctx context.Context, ownerID uuid.UUID, limit, offset int) ([]media.Asset, error) {
	rows, err := a.q.ListAssetsByOwner(ctx, ListAssetsByOwnerParams{
		OwnerID: pgUUID(ownerID),
		Limit:   int32(limit),
		Offset:  int32(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]media.Asset, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAsset(r))
	}
	return out, nil
}

func (a *Adapter) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	return a.q.MarkAssetProcessing(ctx, pgUUID(id))
}

func (a *Adapter) MarkReady(ctx context.Context, id uuid.UUID, outputPrefix string, durationMs, width, height *int) error {
	return a.q.MarkAssetReady(ctx, MarkAssetReadyParams{
		ID:           pgUUID(id),
		OutputPrefix: &outputPrefix,
		DurationMs:   intToInt32Ptr(durationMs),
		Width:        intToInt32Ptr(width),
		Height:       intToInt32Ptr(height),
	})
}

func (a *Adapter) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	return a.q.MarkAssetFailed(ctx, MarkAssetFailedParams{ID: pgUUID(id), ErrorMessage: &msg})
}

// ── mapping helpers ─────────────────────────────────────────────────

func toAsset(r Asset) media.Asset {
	return media.Asset{
		ID:           uuidFrom(r.ID),
		OwnerID:      uuidFrom(r.OwnerID),
		Kind:         r.Kind,
		Status:       media.Status(r.Status),
		SourceKey:    r.SourceKey,
		OutputPrefix: derefStr(r.OutputPrefix),
		MimeType:     r.MimeType,
		SizeBytes:    r.SizeBytes,
		DurationMs:   int32ToIntPtr(r.DurationMs),
		Width:        int32ToIntPtr(r.Width),
		Height:       int32ToIntPtr(r.Height),
		ErrorMessage: derefStr(r.ErrorMessage),
		CreatedAt:    r.CreatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intToInt32Ptr(p *int) *int32 {
	if p == nil {
		return nil
	}
	v := int32(*p)
	return &v
}

func int32ToIntPtr(p *int32) *int {
	if p == nil {
		return nil
	}
	v := int(*p)
	return &v
}
