package storyrepo

// Adapter bridges the sqlc-generated Queries to story.Repository. ReorderChapters
// runs through an injected RunInTx (the request tenant tx when open, else a fresh
// pool tx), mirroring comic. tenant_id is populated by the column DEFAULT.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/story"
)

type Adapter struct {
	q       *Queries
	runInTx func(context.Context, func(pgx.Tx) error) error
}

// NewAdapter builds the adapter over a DBTX (the context-aware platform/db.Conn)
// plus a RunInTx for the DEFERRABLE-unique chapter reorder.
func NewAdapter(db DBTX, runInTx func(context.Context, func(pgx.Tx) error) error) *Adapter {
	return &Adapter{q: New(db), runInTx: runInTx}
}

var _ story.Repository = (*Adapter)(nil)

// ── stories ─────────────────────────────────────────────────────────────

func (a *Adapter) CreateStory(ctx context.Context, in story.CreateStoryInput) (story.Story, error) {
	row, err := a.q.CreateStory(ctx, CreateStoryParams{
		OwnerUserID: pgUUID(in.OwnerID), Title: in.Title, Description: in.Description, CoverAssetID: optUUID(in.CoverAssetID),
	})
	if err != nil {
		return story.Story{}, err
	}
	return toStory(row), nil
}

func (a *Adapter) GetStory(ctx context.Context, id uuid.UUID) (story.Story, error) {
	row, err := a.q.GetStory(ctx, pgUUID(id))
	if err != nil {
		return story.Story{}, mapNotFound(err)
	}
	return toStory(row), nil
}

func (a *Adapter) ListPublished(ctx context.Context, in story.ListInput) ([]story.Story, error) {
	rows, err := a.q.ListPublishedStories(ctx, ListPublishedStoriesParams{
		CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]story.Story, 0, len(rows))
	for _, r := range rows {
		out = append(out, story.Story{
			ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title, Description: r.Description,
			CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status, ChapterCount: int(r.ChapterCount),
			CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (a *Adapter) ListOwn(ctx context.Context, in story.ListInput) ([]story.Story, error) {
	rows, err := a.q.ListOwnStories(ctx, ListOwnStoriesParams{
		OwnerUserID: pgUUID(in.OwnerID), CursorUpdatedAt: optTS(in.CursorAt), CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]story.Story, 0, len(rows))
	for _, r := range rows {
		out = append(out, story.Story{
			ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title, Description: r.Description,
			CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status, ChapterCount: int(r.ChapterCount),
			CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (a *Adapter) UpdateStory(ctx context.Context, in story.UpdateStoryInput) (story.Story, error) {
	row, err := a.q.UpdateStory(ctx, UpdateStoryParams{
		Title: in.Title, Description: in.Description, SetCover: in.SetCover, CoverAssetID: optUUID(in.CoverAssetID), ID: pgUUID(in.ID),
	})
	if err != nil {
		return story.Story{}, mapNotFound(err)
	}
	return toStory(row), nil
}

func (a *Adapter) SetStatus(ctx context.Context, id uuid.UUID, status string) (story.Story, error) {
	row, err := a.q.UpdateStoryStatus(ctx, UpdateStoryStatusParams{ID: pgUUID(id), Status: status})
	if err != nil {
		return story.Story{}, mapNotFound(err)
	}
	return toStory(row), nil
}

func (a *Adapter) DeleteStory(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteStory(ctx, pgUUID(id))
}

func (a *Adapter) OwnerByStory(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.GetStoryOwner(ctx, pgUUID(id))
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row), nil
}

func (a *Adapter) CountChapters(ctx context.Context, storyID uuid.UUID) (int, error) {
	n, err := a.q.CountStoryChapters(ctx, pgUUID(storyID))
	return int(n), err
}

func (a *Adapter) EmptyChapters(ctx context.Context, storyID uuid.UUID) ([]story.ChapterRef, error) {
	rows, err := a.q.EmptyChapters(ctx, pgUUID(storyID))
	if err != nil {
		return nil, err
	}
	out := make([]story.ChapterRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, story.ChapterRef{ID: uuidFrom(r.ID), Title: r.Title})
	}
	return out, nil
}

func (a *Adapter) NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullStoryCoverByAsset(ctx, pgUUID(assetID))
}

// ── chapters ────────────────────────────────────────────────────────────

func (a *Adapter) CreateChapter(ctx context.Context, storyID uuid.UUID, title, bodyMd string, sortOrder int) (story.Chapter, error) {
	row, err := a.q.CreateStoryChapter(ctx, CreateStoryChapterParams{StoryID: pgUUID(storyID), Title: title, BodyMd: bodyMd, SortOrder: int32(sortOrder)})
	if err != nil {
		return story.Chapter{}, err
	}
	return toChapter(row), nil
}

func (a *Adapter) GetChapter(ctx context.Context, id uuid.UUID) (story.Chapter, error) {
	row, err := a.q.GetStoryChapter(ctx, pgUUID(id))
	if err != nil {
		return story.Chapter{}, mapNotFound(err)
	}
	return toChapter(row), nil
}

func (a *Adapter) ListChapters(ctx context.Context, storyID uuid.UUID) ([]story.Chapter, error) {
	rows, err := a.q.ListStoryChapters(ctx, pgUUID(storyID))
	if err != nil {
		return nil, err
	}
	out := make([]story.Chapter, 0, len(rows))
	for _, r := range rows {
		out = append(out, toChapter(r))
	}
	return out, nil
}

func (a *Adapter) UpdateChapter(ctx context.Context, id uuid.UUID, title, bodyMd *string) (story.Chapter, error) {
	row, err := a.q.UpdateStoryChapter(ctx, UpdateStoryChapterParams{Title: title, BodyMd: bodyMd, ID: pgUUID(id)})
	if err != nil {
		return story.Chapter{}, mapNotFound(err)
	}
	return toChapter(row), nil
}

func (a *Adapter) DeleteChapter(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteStoryChapter(ctx, pgUUID(id))
}

func (a *Adapter) ReorderChapters(ctx context.Context, storyID uuid.UUID, orderedIDs []uuid.UUID) error {
	return a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		for i, id := range orderedIDs {
			if err := q.UpdateStoryChapterOrder(ctx, UpdateStoryChapterOrderParams{
				ID: pgUUID(id), SortOrder: int32((i + 1) * 10), StoryID: pgUUID(storyID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Adapter) OwnerAndStoryByChapter(ctx context.Context, chapterID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := a.q.GetStoryOwnerByChapter(ctx, pgUUID(chapterID))
	if err != nil {
		return uuid.Nil, uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row.OwnerUserID), uuidFrom(row.StoryID), nil
}

// ── mapping helpers ────────────────────────────────────────────────────

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return story.ErrNotFound
	}
	return err
}

func toStory(r Story) story.Story {
	return story.Story{
		ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title, Description: r.Description,
		CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status, CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
}

func toChapter(r StoryChapter) story.Chapter {
	return story.Chapter{
		ID: uuidFrom(r.ID), StoryID: uuidFrom(r.StoryID), Title: r.Title, BodyMd: r.BodyMd,
		SortOrder: int(r.SortOrder), CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
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
