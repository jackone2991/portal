package musicrepo

// Adapter bridges the sqlc-generated Queries to music.Repository. tenant_id is
// populated by the music_tracks.tenant_id column DEFAULT — never written from Go.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/music"
)

type Adapter struct{ q *Queries }

func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

var _ music.Repository = (*Adapter)(nil)

func (a *Adapter) CreateTrack(ctx context.Context, in music.CreateTrackInput) (music.Track, error) {
	row, err := a.q.CreateTrack(ctx, CreateTrackParams{
		OwnerUserID:  pgUUID(in.OwnerID),
		Title:        in.Title,
		Artist:       in.Artist,
		Album:        in.Album,
		Description:  in.Description,
		AudioAssetID: optUUID(in.AudioAssetID),
		CoverAssetID: optUUID(in.CoverAssetID),
	})
	if err != nil {
		return music.Track{}, err
	}
	return toTrack(row), nil
}

func (a *Adapter) GetTrack(ctx context.Context, id uuid.UUID) (music.Track, error) {
	row, err := a.q.GetTrack(ctx, pgUUID(id))
	if err != nil {
		return music.Track{}, mapNotFound(err)
	}
	return toTrack(row), nil
}

func (a *Adapter) ListPublished(ctx context.Context, in music.ListInput) ([]music.Track, error) {
	rows, err := a.q.ListPublishedTracks(ctx, ListPublishedTracksParams{
		CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	return toTracks(rows), nil
}

func (a *Adapter) ListOwn(ctx context.Context, in music.ListInput) ([]music.Track, error) {
	rows, err := a.q.ListOwnTracks(ctx, ListOwnTracksParams{
		OwnerUserID: pgUUID(in.OwnerID), CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	return toTracks(rows), nil
}

func (a *Adapter) UpdateTrack(ctx context.Context, in music.UpdateTrackInput) (music.Track, error) {
	row, err := a.q.UpdateTrack(ctx, UpdateTrackParams{
		Title:        in.Title,
		SetArtist:    in.SetArtist,
		Artist:       in.Artist,
		SetAlbum:     in.SetAlbum,
		Album:        in.Album,
		Description:  in.Description,
		SetAudio:     in.SetAudio,
		AudioAssetID: optUUID(in.AudioAssetID),
		SetCover:     in.SetCover,
		CoverAssetID: optUUID(in.CoverAssetID),
		ID:           pgUUID(in.ID),
	})
	if err != nil {
		return music.Track{}, mapNotFound(err)
	}
	return toTrack(row), nil
}

func (a *Adapter) SetStatus(ctx context.Context, id uuid.UUID, status string) (music.Track, error) {
	row, err := a.q.UpdateTrackStatus(ctx, UpdateTrackStatusParams{ID: pgUUID(id), Status: status})
	if err != nil {
		return music.Track{}, mapNotFound(err)
	}
	return toTrack(row), nil
}

func (a *Adapter) DeleteTrack(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteTrack(ctx, pgUUID(id))
}

func (a *Adapter) OwnerByTrack(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.GetTrackOwner(ctx, pgUUID(id))
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row), nil
}

func (a *Adapter) NullAudioByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullAudioByAsset(ctx, pgUUID(assetID))
}

func (a *Adapter) NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullTrackCoverByAsset(ctx, pgUUID(assetID))
}

// ── mapping helpers ────────────────────────────────────────────────────

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return music.ErrNotFound
	}
	return err
}

func toTracks(rows []MusicTrack) []music.Track {
	out := make([]music.Track, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTrack(r))
	}
	return out
}

func toTrack(r MusicTrack) music.Track {
	return music.Track{
		ID:           uuidFrom(r.ID),
		OwnerID:      uuidFrom(r.OwnerUserID),
		Title:        r.Title,
		Artist:       r.Artist,
		Album:        r.Album,
		Description:  r.Description,
		AudioAssetID: uuidPtr(r.AudioAssetID),
		CoverAssetID: uuidPtr(r.CoverAssetID),
		Status:       r.Status,
		CreatedAt:    r.CreatedAt.Time,
		UpdatedAt:    r.UpdatedAt.Time,
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func optUUID(p *uuid.UUID) pgtype.UUID {
	if p == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *p, Valid: true}
}

func optTS(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
