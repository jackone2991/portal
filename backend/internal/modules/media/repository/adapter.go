package mediarepo

// Adapter bridges the sqlc-generated Queries to media.Repository (and the
// embedded worker.Repo). cmd/api and cmd/worker construct one with NewAdapter
// and pass it into media.Deps. All pgtype ↔ domain juggling lives here.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/media"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
)

type Adapter struct {
	q *Queries
}

func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

// Compile-time proof the adapter satisfies the media persistence surface.
var _ media.Repository = (*Adapter)(nil)

func (a *Adapter) CreateAsset(ctx context.Context, in media.CreateAssetInput) (media.Asset, error) {
	row, err := a.q.CreateAsset(ctx, CreateAssetParams{
		ID:               pgUUID(in.ID),
		OwnerID:          pgUUID(in.OwnerID),
		Kind:             in.Kind,
		SourceKey:        in.SourceKey,
		MimeType:         in.MimeType,
		SizeBytes:        in.SizeBytes,
		Title:            toNullStr(in.Title),
		OriginalFilename: toNullStr(in.OriginalFilename),
		Origin:           in.Origin,
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

func (a *Adapter) GetAssetOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.GetAssetOwner(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, media.ErrNotFound
		}
		return uuid.Nil, err
	}
	return uuidFrom(row), nil
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
	return toAssets(rows), nil
}

func (a *Adapter) ListByOwnerCursor(ctx context.Context, in media.ListCursorInput) ([]media.Asset, error) {
	params := ListAssetsByOwnerCursorParams{
		OwnerID:  pgUUID(in.OwnerID),
		Statuses: in.Statuses,
		Lim:      int32(in.Limit),
	}
	if params.Statuses == nil {
		params.Statuses = []string{} // a NULL array would exclude every row
	}
	if in.Kind != "" {
		k := in.Kind
		params.Kind = &k
	}
	if !in.CursorAt.IsZero() {
		params.CursorCreatedAt = pgtype.Timestamptz{Time: in.CursorAt, Valid: true}
		params.CursorID = pgUUID(in.CursorID)
	}
	rows, err := a.q.ListAssetsByOwnerCursor(ctx, params)
	if err != nil {
		return nil, err
	}
	return toAssets(rows), nil
}

func (a *Adapter) ListForPurge(ctx context.Context) ([]media.Asset, error) {
	rows, err := a.q.ListAssetsForPurge(ctx)
	if err != nil {
		return nil, err
	}
	return toAssets(rows), nil
}

func (a *Adapter) ListAbandonedUploads(ctx context.Context) ([]media.Asset, error) {
	rows, err := a.q.ListAbandonedUploads(ctx)
	if err != nil {
		return nil, err
	}
	return toAssets(rows), nil
}

func (a *Adapter) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	return a.q.MarkAssetProcessing(ctx, pgUUID(id))
}

func (a *Adapter) SetStatusDeleting(ctx context.Context, id uuid.UUID) error {
	return a.q.SetAssetStatusDeleting(ctx, pgUUID(id))
}

func (a *Adapter) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteAsset(ctx, pgUUID(id))
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

func (a *Adapter) MarkImageReady(ctx context.Context, id uuid.UUID, width, height int) error {
	w, h := int32(width), int32(height)
	return a.q.MarkAssetImageReady(ctx, MarkAssetImageReadyParams{
		ID:     pgUUID(id),
		Width:  &w,
		Height: &h,
	})
}

func (a *Adapter) MarkFailed(ctx context.Context, id uuid.UUID, msg string) error {
	return a.q.MarkAssetFailed(ctx, MarkAssetFailedParams{ID: pgUUID(id), ErrorMessage: &msg})
}

func (a *Adapter) LoadAssetMeta(ctx context.Context, id uuid.UUID) (worker.AssetMeta, error) {
	row, err := a.q.GetAsset(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return worker.AssetMeta{}, media.ErrNotFound
		}
		return worker.AssetMeta{}, err
	}
	title := derefStr(row.Title)
	if title == "" {
		title = derefStr(row.OriginalFilename)
	}
	return worker.AssetMeta{
		OwnerID: uuidFrom(row.OwnerID),
		Kind:    row.Kind,
		Title:   title,
		Origin:  row.Origin,
	}, nil
}

func (a *Adapter) InsertVariant(ctx context.Context, assetID uuid.UUID, variant, storageKey string, width, height int, sizeBytes int64) error {
	_, err := a.q.InsertVariant(ctx, InsertVariantParams{
		AssetID:    pgUUID(assetID),
		Variant:    variant,
		StorageKey: storageKey,
		Width:      int32(width),
		Height:     int32(height),
		SizeBytes:  sizeBytes,
	})
	return err
}

func (a *Adapter) ListVariants(ctx context.Context, assetID uuid.UUID) ([]media.Variant, error) {
	rows, err := a.q.ListVariantsByAsset(ctx, pgUUID(assetID))
	if err != nil {
		return nil, err
	}
	out := make([]media.Variant, 0, len(rows))
	for _, r := range rows {
		out = append(out, toVariant(r))
	}
	return out, nil
}

func (a *Adapter) GetVariant(ctx context.Context, assetID uuid.UUID, variant string) (media.Variant, error) {
	row, err := a.q.GetVariant(ctx, GetVariantParams{AssetID: pgUUID(assetID), Variant: variant})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return media.Variant{}, media.ErrNotFound
		}
		return media.Variant{}, err
	}
	return toVariant(row), nil
}

func (a *Adapter) DeleteVariants(ctx context.Context, assetID uuid.UUID) error {
	return a.q.DeleteVariantsByAsset(ctx, pgUUID(assetID))
}

func (a *Adapter) UpsertPlaybackProgress(ctx context.Context, ownerID, assetID uuid.UUID, positionMs int64, completedAt *time.Time) error {
	var cat pgtype.Timestamptz
	if completedAt != nil {
		cat = pgtype.Timestamptz{Time: *completedAt, Valid: true}
	}
	return a.q.UpsertPlaybackProgress(ctx, UpsertPlaybackProgressParams{
		UserID:      pgUUID(ownerID),
		AssetID:     pgUUID(assetID),
		PositionMs:  positionMs,
		CompletedAt: cat,
	})
}

func (a *Adapter) GetPlaybackProgress(ctx context.Context, ownerID, assetID uuid.UUID) (int64, *time.Time, time.Time, error) {
	row, err := a.q.GetPlaybackProgress(ctx, GetPlaybackProgressParams{
		UserID:  pgUUID(ownerID),
		AssetID: pgUUID(assetID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil, time.Time{}, media.ErrNotFound
		}
		return 0, nil, time.Time{}, err
	}
	var completedAt *time.Time
	if row.CompletedAt.Valid {
		completedAt = &row.CompletedAt.Time
	}
	return row.PositionMs, completedAt, row.UpdatedAt.Time, nil
}

func (a *Adapter) GetContinueItems(ctx context.Context, userID uuid.UUID, limit int) ([]mediaapi.ContinueItem, error) {
	rows, err := a.q.GetContinueItems(ctx, GetContinueItemsParams{
		UserID: pgUUID(userID),
		Limit:  int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]mediaapi.ContinueItem, 0, len(rows))
	for _, r := range rows {
		var poster string
		if s, ok := r.PosterUrl.(string); ok {
			poster = s
		}
		var href string
		if s, ok := r.Href.(string); ok {
			href = s
		}
		out = append(out, mediaapi.ContinueItem{
			Module:      r.Module,
			RefID:       uuidFrom(r.RefID),
			Title:       r.Title,
			PosterURL:   poster,
			ProgressPct: int(r.ProgressPct),
			Href:        href,
			UpdatedAt:   timeFrom(r.UpdatedAt),
		})
	}
	return out, nil
}

// ── mapping helpers ─────────────────────────────────────────────────

func toAssets(rows []Asset) []media.Asset {
	out := make([]media.Asset, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAsset(r))
	}
	return out
}

func toAsset(r Asset) media.Asset {
	return media.Asset{
		ID:               uuidFrom(r.ID),
		OwnerID:          uuidFrom(r.OwnerID),
		Kind:             r.Kind,
		Status:           media.Status(r.Status),
		SourceKey:        r.SourceKey,
		OutputPrefix:     derefStr(r.OutputPrefix),
		MimeType:         r.MimeType,
		SizeBytes:        r.SizeBytes,
		DurationMs:       int32ToIntPtr(r.DurationMs),
		Width:            int32ToIntPtr(r.Width),
		Height:           int32ToIntPtr(r.Height),
		ErrorMessage:     derefStr(r.ErrorMessage),
		Title:            derefStr(r.Title),
		OriginalFilename: derefStr(r.OriginalFilename),
		Origin:           r.Origin,
		CreatedAt:        r.CreatedAt.Time,
	}
}

func toVariant(r MediaAssetVariant) media.Variant {
	return media.Variant{
		ID:         uuidFrom(r.ID),
		AssetID:    uuidFrom(r.AssetID),
		Variant:    r.Variant,
		StorageKey: r.StorageKey,
		Width:      int(r.Width),
		Height:     int(r.Height),
		SizeBytes:  r.SizeBytes,
		CreatedAt:  timeFrom(r.CreatedAt),
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func timeFrom(t pgtype.Timestamptz) time.Time {
	if !t.Valid {
		return time.Time{}
	}
	return t.Time
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func toNullStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
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
