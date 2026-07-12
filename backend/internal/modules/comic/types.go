package comic

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Errors surfaced to the handler. Owner/visibility resolves a hidden row to
// "not found" (draft invisibility → 404, never 403 — don't leak existence).
var (
	ErrNotFound              = errors.New("comic: not found")
	ErrInvalidPageAsset      = errors.New("comic: invalid page asset")
	ErrInvalidCoverAsset     = errors.New("comic: invalid cover asset")
	ErrInvalidProgressTarget = errors.New("comic: invalid progress target")
	ErrValidation            = errors.New("comic: validation error")
	ErrBadCursor             = errors.New("comic: invalid cursor")
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	maxTitleLen     = 200
	defaultLimit    = 30
	maxLimit        = 50
)

// ChapterRef names a chapter for the not-publishable error (P0.2).
type ChapterRef struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

// NotPublishableError lists the chapters blocking publish (empty or none).
type NotPublishableError struct {
	Chapters []ChapterRef
}

func (e *NotPublishableError) Error() string { return "comic: not publishable" }

// Comic is the module's internal comic record (+ derived chapter count on lists).
type Comic struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Title        string
	Description  *string
	CoverAssetID *uuid.UUID
	Status       string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ChapterCount int
}

type Chapter struct {
	ID        uuid.UUID
	ComicID   uuid.UUID
	Title     string
	SortOrder int
	CreatedAt time.Time
}

type Page struct {
	ID        uuid.UUID
	ChapterID uuid.UUID
	AssetID   uuid.UUID
	SortOrder int
}

type Progress struct {
	ChapterID uuid.UUID
	PageID    *uuid.UUID
	UpdatedAt time.Time
}

// ── inputs ────────────────────────────────────────────────────────────

type CreateComicInput struct {
	OwnerID      uuid.UUID
	Title        string
	Description  *string
	CoverAssetID *uuid.UUID
}

type UpdateComicInput struct {
	ID           uuid.UUID
	Title        *string
	Description  *string
	SetCover     bool
	CoverAssetID *uuid.UUID
}

type ListInput struct {
	OwnerID  uuid.UUID // for the "mine" listing
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. Reorder/publish helpers run in a tx.
type Repository interface {
	// comics
	CreateComic(ctx context.Context, in CreateComicInput) (Comic, error)
	GetComic(ctx context.Context, id uuid.UUID) (Comic, error)
	ListPublished(ctx context.Context, in ListInput) ([]Comic, error)
	ListOwn(ctx context.Context, in ListInput) ([]Comic, error)
	UpdateComic(ctx context.Context, in UpdateComicInput) (Comic, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) (Comic, error)
	DeleteComic(ctx context.Context, id uuid.UUID) error
	OwnerByComic(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	ChaptersWithoutPages(ctx context.Context, comicID uuid.UUID) ([]ChapterRef, error)
	CountChapters(ctx context.Context, comicID uuid.UUID) (int, error)

	// chapters
	CreateChapter(ctx context.Context, comicID uuid.UUID, title string, sortOrder int) (Chapter, error)
	GetChapter(ctx context.Context, id uuid.UUID) (Chapter, error)
	ListChapters(ctx context.Context, comicID uuid.UUID) ([]Chapter, error)
	UpdateChapter(ctx context.Context, id uuid.UUID, title *string) (Chapter, error)
	DeleteChapter(ctx context.Context, id uuid.UUID) error
	ReorderChapters(ctx context.Context, comicID uuid.UUID, orderedIDs []uuid.UUID) error
	OwnerAndComicByChapter(ctx context.Context, chapterID uuid.UUID) (owner, comicID uuid.UUID, err error)

	// pages
	CreatePage(ctx context.Context, chapterID, assetID uuid.UUID, sortOrder int) (Page, error)
	GetPage(ctx context.Context, id uuid.UUID) (Page, error)
	ListPages(ctx context.Context, chapterID uuid.UUID) ([]Page, error)
	DeletePage(ctx context.Context, id uuid.UUID) error
	ReorderPages(ctx context.Context, chapterID uuid.UUID, orderedIDs []uuid.UUID) error
	OwnerByPage(ctx context.Context, pageID uuid.UUID) (owner, comicID uuid.UUID, err error)

	// progress
	UpsertProgress(ctx context.Context, userID, comicID, chapterID uuid.UUID, pageID *uuid.UUID) error
	GetProgress(ctx context.Context, userID, comicID uuid.UUID) (Progress, error)
	// PageMembership resolves a page's (chapter, comic) for progress validation.
	PageMembership(ctx context.Context, pageID uuid.UUID) (chapterID, comicID uuid.UUID, err error)
	ChapterComic(ctx context.Context, chapterID uuid.UUID) (comicID uuid.UUID, err error)

	// media:asset_deleted consumer (P0.6)
	DeletePagesByAsset(ctx context.Context, assetID uuid.UUID) error
	NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error
}

// MediaAPI is the slice of media/api comic needs to validate asset references.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
