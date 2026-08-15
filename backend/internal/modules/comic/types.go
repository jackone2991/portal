package comic

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"

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
	ID               uuid.UUID
	OwnerID          uuid.UUID
	Title            string
	Description      *string
	CoverAssetID     *uuid.UUID
	Status           string
	ReadingDirection string // 'ltr' | 'rtl' | 'vertical' (SPEC-02 R3)
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ChapterCount     int
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

// Import statuses (P1.7).
const (
	ImportPending    = "pending"
	ImportUploaded   = "uploaded"
	ImportProcessing = "processing"
	ImportDone       = "done"
	ImportFailed     = "failed"
)

// ImportFileResult is one entry's outcome in the persisted per-file report.
type ImportFileResult struct {
	Name  string `json:"name"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// ImportJob is a zip-import job (P1.7), persisted so the client can poll status.
// ChapterID is nil for a comic-level (multi-chapter) import — the worker groups the
// zip's images by top-level folder and creates a chapter per folder.
type ImportJob struct {
	ID          uuid.UUID
	ComicID     uuid.UUID
	ChapterID   *uuid.UUID
	OwnerUserID uuid.UUID
	Status      string
	UploadRef   *string
	Total       int
	Succeeded   int
	Failed      int
	Report      []ImportFileResult
	Error       *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ── inputs ────────────────────────────────────────────────────────────

type CreateComicInput struct {
	OwnerID      uuid.UUID
	Title        string
	Description  *string
	CoverAssetID *uuid.UUID
}

type UpdateComicInput struct {
	ID               uuid.UUID
	Title            *string
	Description      *string
	ReadingDirection *string // nil = unchanged; validated to ltr|rtl|vertical
	SetCover         bool
	CoverAssetID     *uuid.UUID
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

	// import jobs (P1.7)
	CreateImport(ctx context.Context, comicID, chapterID, ownerID uuid.UUID) (ImportJob, error)
	CreateComicImport(ctx context.Context, comicID, ownerID uuid.UUID) (ImportJob, error) // multi-chapter
	GetImport(ctx context.Context, id uuid.UUID) (ImportJob, error)
	SetImportUpload(ctx context.Context, id uuid.UUID, uploadRef string) (ImportJob, error)
	StartImport(ctx context.Context, id uuid.UUID, total int) error
	UpdateImportProgress(ctx context.Context, id uuid.UUID, succeeded, failed int, report []ImportFileResult) error
	FinishImport(ctx context.Context, id uuid.UUID, status string, succeeded, failed int, report []ImportFileResult, errMsg *string) error
}

// MediaAPI is the slice of media/api comic needs to validate asset references and
// (P1.7) ingest imported page images.
type MediaAPI interface {
	GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error)
	AssetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]mediaapi.AssetStatus, error)
	IngestImage(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error)
}

// ObjectStore is the slice of platform/storage the import flow needs (the zip is
// stored under an import/ prefix, streamed by the worker, then deleted).
type ObjectStore interface {
	Put(ctx context.Context, key string, body io.Reader, contentType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Size(ctx context.Context, key string) (int64, error)
	Delete(ctx context.Context, key string) error
}

// Enqueuer dispatches an async task (asynq client). Import runs on the default queue.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// RunInTenant scopes a worker DB write to a user's personal org so tenant_id's
// DEFAULT current_setting is populated (ADR-07 1b). Built in cmd/worker.
type RunInTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
