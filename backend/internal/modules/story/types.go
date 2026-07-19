package story

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

var (
	ErrNotFound          = errors.New("story: not found")
	ErrInvalidCoverAsset = errors.New("story: invalid cover asset")
	ErrValidation        = errors.New("story: validation error")
	ErrBadCursor         = errors.New("story: invalid cursor")
)

const (
	StatusDraft     = "draft"
	StatusPublished = "published"
	maxTitleLen     = 200
	maxBodyLen      = 200000
	defaultLimit    = 30
	maxLimit        = 50
)

// ChapterRef names a chapter for the not-publishable error.
type ChapterRef struct {
	ID    uuid.UUID `json:"id"`
	Title string    `json:"title"`
}

// NotPublishableError lists the empty-body chapters blocking publish.
type NotPublishableError struct {
	Chapters []ChapterRef
}

func (e *NotPublishableError) Error() string { return "story: not publishable" }

// Story is the module's internal story record (+ derived chapter count on lists).
type Story struct {
	ID           uuid.UUID
	OwnerID      uuid.UUID
	Title        string
	Description  *string
	CoverAssetID *uuid.UUID
	Status       string
	ChapterCount int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Chapter struct {
	ID        uuid.UUID
	StoryID   uuid.UUID
	Title     string
	BodyMd    string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ── inputs ────────────────────────────────────────────────────────────

type CreateStoryInput struct {
	OwnerID      uuid.UUID
	Title        string
	Description  *string
	CoverAssetID *uuid.UUID
}

type UpdateStoryInput struct {
	ID           uuid.UUID
	Title        *string
	Description  *string
	SetCover     bool
	CoverAssetID *uuid.UUID
}

type ListInput struct {
	OwnerID  uuid.UUID
	CursorAt time.Time
	CursorID uuid.UUID
	Limit    int
}

// Repository is the persistence surface. ReorderChapters runs in a tx.
type Repository interface {
	CreateStory(ctx context.Context, in CreateStoryInput) (Story, error)
	GetStory(ctx context.Context, id uuid.UUID) (Story, error)
	ListPublished(ctx context.Context, in ListInput) ([]Story, error)
	ListOwn(ctx context.Context, in ListInput) ([]Story, error)
	UpdateStory(ctx context.Context, in UpdateStoryInput) (Story, error)
	SetStatus(ctx context.Context, id uuid.UUID, status string) (Story, error)
	DeleteStory(ctx context.Context, id uuid.UUID) error
	OwnerByStory(ctx context.Context, id uuid.UUID) (uuid.UUID, error)
	CountChapters(ctx context.Context, storyID uuid.UUID) (int, error)
	EmptyChapters(ctx context.Context, storyID uuid.UUID) ([]ChapterRef, error)
	NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error

	CreateChapter(ctx context.Context, storyID uuid.UUID, title, bodyMd string, sortOrder int) (Chapter, error)
	GetChapter(ctx context.Context, id uuid.UUID) (Chapter, error)
	ListChapters(ctx context.Context, storyID uuid.UUID) ([]Chapter, error)
	UpdateChapter(ctx context.Context, id uuid.UUID, title, bodyMd *string) (Chapter, error)
	DeleteChapter(ctx context.Context, id uuid.UUID) error
	ReorderChapters(ctx context.Context, storyID uuid.UUID, orderedIDs []uuid.UUID) error
	OwnerAndStoryByChapter(ctx context.Context, chapterID uuid.UUID) (owner, storyID uuid.UUID, err error)
}

// MediaAPI is the slice of media/api story needs (cover validation).
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
