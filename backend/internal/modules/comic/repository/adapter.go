package comicrepo

// Adapter bridges the sqlc-generated Queries to comic.Repository. Reorder
// operations (DEFERRABLE sort_order uniques) run through an injected RunInTx so
// they execute on the request's tenant-scoped tx when one is open (else a fresh
// pool tx).

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/comic"
)

type Adapter struct {
	q       *Queries
	db      DBTX // raw handle for the jsonb writes (pass json as text, not []byte)
	runInTx func(context.Context, func(pgx.Tx) error) error
}

// NewAdapter builds the adapter over a DBTX (the context-aware platform/db.Conn)
// plus a RunInTx that opens/reuses the request transaction for reorder methods.
func NewAdapter(db DBTX, runInTx func(context.Context, func(pgx.Tx) error) error) *Adapter {
	return &Adapter{q: New(db), db: db, runInTx: runInTx}
}

var _ comic.Repository = (*Adapter)(nil)

// ── comics ────────────────────────────────────────────────────────────

func (a *Adapter) CreateComic(ctx context.Context, in comic.CreateComicInput) (comic.Comic, error) {
	row, err := a.q.CreateComic(ctx, CreateComicParams{
		OwnerUserID:  pgUUID(in.OwnerID),
		Title:        in.Title,
		Description:  in.Description,
		CoverAssetID: optUUID(in.CoverAssetID),
	})
	if err != nil {
		return comic.Comic{}, err
	}
	return toComic(row), nil
}

func (a *Adapter) GetComic(ctx context.Context, id uuid.UUID) (comic.Comic, error) {
	row, err := a.q.GetComic(ctx, pgUUID(id))
	if err != nil {
		return comic.Comic{}, mapNotFound(err)
	}
	return toComic(row), nil
}

func (a *Adapter) ListPublished(ctx context.Context, in comic.ListInput) ([]comic.Comic, error) {
	rows, err := a.q.ListPublishedComics(ctx, ListPublishedComicsParams{
		CursorUpdatedAt: optTS(in.CursorAt),
		CursorID:        pgUUID(in.CursorID),
		Lim:             int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]comic.Comic, 0, len(rows))
	for _, r := range rows {
		out = append(out, comic.Comic{
			ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title,
			Description: r.Description, CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status,
			ReadingDirection: r.ReadingDirection,
			CreatedAt:        r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time, ChapterCount: int(r.ChapterCount),
		})
	}
	return out, nil
}

func (a *Adapter) ListOwn(ctx context.Context, in comic.ListInput) ([]comic.Comic, error) {
	rows, err := a.q.ListOwnComics(ctx, ListOwnComicsParams{
		OwnerUserID:     pgUUID(in.OwnerID),
		CursorUpdatedAt: optTS(in.CursorAt),
		CursorID:        pgUUID(in.CursorID),
		Lim:             int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]comic.Comic, 0, len(rows))
	for _, r := range rows {
		out = append(out, comic.Comic{
			ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title,
			Description: r.Description, CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status,
			ReadingDirection: r.ReadingDirection,
			CreatedAt:        r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time, ChapterCount: int(r.ChapterCount),
		})
	}
	return out, nil
}

func (a *Adapter) UpdateComic(ctx context.Context, in comic.UpdateComicInput) (comic.Comic, error) {
	row, err := a.q.UpdateComic(ctx, UpdateComicParams{
		Title:            in.Title,
		Description:      in.Description,
		ReadingDirection: in.ReadingDirection,
		SetCover:         in.SetCover,
		CoverAssetID:     optUUID(in.CoverAssetID),
		ID:               pgUUID(in.ID),
	})
	if err != nil {
		return comic.Comic{}, mapNotFound(err)
	}
	return toComic(row), nil
}

func (a *Adapter) SetStatus(ctx context.Context, id uuid.UUID, status string) (comic.Comic, error) {
	row, err := a.q.UpdateComicStatus(ctx, UpdateComicStatusParams{ID: pgUUID(id), Status: status})
	if err != nil {
		return comic.Comic{}, mapNotFound(err)
	}
	return toComic(row), nil
}

func (a *Adapter) DeleteComic(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteComic(ctx, pgUUID(id))
}

func (a *Adapter) OwnerByComic(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.GetComicOwner(ctx, pgUUID(id))
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row), nil
}

func (a *Adapter) ChaptersWithoutPages(ctx context.Context, comicID uuid.UUID) ([]comic.ChapterRef, error) {
	rows, err := a.q.ChaptersWithoutPages(ctx, pgUUID(comicID))
	if err != nil {
		return nil, err
	}
	out := make([]comic.ChapterRef, 0, len(rows))
	for _, r := range rows {
		out = append(out, comic.ChapterRef{ID: uuidFrom(r.ID), Title: r.Title})
	}
	return out, nil
}

func (a *Adapter) CountChapters(ctx context.Context, comicID uuid.UUID) (int, error) {
	n, err := a.q.CountChapters(ctx, pgUUID(comicID))
	return int(n), err
}

// ── chapters ──────────────────────────────────────────────────────────

func (a *Adapter) CreateChapter(ctx context.Context, comicID uuid.UUID, title string, sortOrder int) (comic.Chapter, error) {
	row, err := a.q.CreateChapter(ctx, CreateChapterParams{ComicID: pgUUID(comicID), Title: title, SortOrder: int32(sortOrder)})
	if err != nil {
		return comic.Chapter{}, err
	}
	return toChapter(row), nil
}

func (a *Adapter) GetChapter(ctx context.Context, id uuid.UUID) (comic.Chapter, error) {
	row, err := a.q.GetChapter(ctx, pgUUID(id))
	if err != nil {
		return comic.Chapter{}, mapNotFound(err)
	}
	return toChapter(row), nil
}

func (a *Adapter) ListChapters(ctx context.Context, comicID uuid.UUID) ([]comic.Chapter, error) {
	rows, err := a.q.ListChaptersByComic(ctx, pgUUID(comicID))
	if err != nil {
		return nil, err
	}
	out := make([]comic.Chapter, 0, len(rows))
	for _, r := range rows {
		out = append(out, toChapter(r))
	}
	return out, nil
}

func (a *Adapter) UpdateChapter(ctx context.Context, id uuid.UUID, title *string) (comic.Chapter, error) {
	row, err := a.q.UpdateChapter(ctx, UpdateChapterParams{Title: title, ID: pgUUID(id)})
	if err != nil {
		return comic.Chapter{}, mapNotFound(err)
	}
	return toChapter(row), nil
}

func (a *Adapter) DeleteChapter(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteChapter(ctx, pgUUID(id))
}

func (a *Adapter) ReorderChapters(ctx context.Context, comicID uuid.UUID, orderedIDs []uuid.UUID) error {
	return a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		for i, id := range orderedIDs {
			if err := q.UpdateChapterOrder(ctx, UpdateChapterOrderParams{
				ID: pgUUID(id), SortOrder: int32((i + 1) * 10), ComicID: pgUUID(comicID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Adapter) OwnerAndComicByChapter(ctx context.Context, chapterID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := a.q.GetComicOwnerByChapter(ctx, pgUUID(chapterID))
	if err != nil {
		return uuid.Nil, uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row.OwnerUserID), uuidFrom(row.ComicID), nil
}

// ── pages ─────────────────────────────────────────────────────────────

func (a *Adapter) CreatePage(ctx context.Context, chapterID, assetID uuid.UUID, sortOrder int) (comic.Page, error) {
	row, err := a.q.CreatePage(ctx, CreatePageParams{ChapterID: pgUUID(chapterID), AssetID: pgUUID(assetID), SortOrder: int32(sortOrder)})
	if err != nil {
		return comic.Page{}, err
	}
	return toPage(row), nil
}

func (a *Adapter) GetPage(ctx context.Context, id uuid.UUID) (comic.Page, error) {
	row, err := a.q.GetPage(ctx, pgUUID(id))
	if err != nil {
		return comic.Page{}, mapNotFound(err)
	}
	return toPage(row), nil
}

func (a *Adapter) ListPages(ctx context.Context, chapterID uuid.UUID) ([]comic.Page, error) {
	rows, err := a.q.ListPagesByChapter(ctx, pgUUID(chapterID))
	if err != nil {
		return nil, err
	}
	out := make([]comic.Page, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPage(r))
	}
	return out, nil
}

func (a *Adapter) DeletePage(ctx context.Context, id uuid.UUID) error {
	return a.q.DeletePage(ctx, pgUUID(id))
}

func (a *Adapter) ReorderPages(ctx context.Context, chapterID uuid.UUID, orderedIDs []uuid.UUID) error {
	return a.runInTx(ctx, func(tx pgx.Tx) error {
		q := New(tx)
		for i, id := range orderedIDs {
			if err := q.UpdatePageOrder(ctx, UpdatePageOrderParams{
				ID: pgUUID(id), SortOrder: int32((i + 1) * 10), ChapterID: pgUUID(chapterID),
			}); err != nil {
				return err
			}
		}
		return nil
	})
}

func (a *Adapter) OwnerByPage(ctx context.Context, pageID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := a.q.GetComicOwnerByPage(ctx, pgUUID(pageID))
	if err != nil {
		return uuid.Nil, uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row.OwnerUserID), uuidFrom(row.ComicID), nil
}

// ── progress ──────────────────────────────────────────────────────────

func (a *Adapter) UpsertProgress(ctx context.Context, userID, comicID, chapterID uuid.UUID, pageID *uuid.UUID) error {
	return a.q.UpsertProgress(ctx, UpsertProgressParams{
		UserID: pgUUID(userID), ComicID: pgUUID(comicID), ChapterID: pgUUID(chapterID), PageID: optUUID(pageID),
	})
}

func (a *Adapter) GetProgress(ctx context.Context, userID, comicID uuid.UUID) (comic.Progress, error) {
	row, err := a.q.GetProgress(ctx, GetProgressParams{UserID: pgUUID(userID), ComicID: pgUUID(comicID)})
	if err != nil {
		return comic.Progress{}, mapNotFound(err)
	}
	return comic.Progress{ChapterID: uuidFrom(row.ChapterID), PageID: uuidPtr(row.PageID), UpdatedAt: row.UpdatedAt.Time}, nil
}

func (a *Adapter) PageMembership(ctx context.Context, pageID uuid.UUID) (uuid.UUID, uuid.UUID, error) {
	row, err := a.q.PageChapterAndComic(ctx, pgUUID(pageID))
	if err != nil {
		return uuid.Nil, uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row.ChapterID), uuidFrom(row.ComicID), nil
}

func (a *Adapter) ChapterComic(ctx context.Context, chapterID uuid.UUID) (uuid.UUID, error) {
	row, err := a.q.ChapterComic(ctx, pgUUID(chapterID))
	if err != nil {
		return uuid.Nil, mapNotFound(err)
	}
	return uuidFrom(row), nil
}

// ── media:asset_deleted consumer ──────────────────────────────────────

func (a *Adapter) DeletePagesByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.DeletePagesByAsset(ctx, pgUUID(assetID))
}

func (a *Adapter) NullCoverByAsset(ctx context.Context, assetID uuid.UUID) error {
	return a.q.NullCoverByAsset(ctx, pgUUID(assetID))
}

// ── import jobs (P1.7) ────────────────────────────────────────────────

func (a *Adapter) CreateImport(ctx context.Context, comicID, chapterID, ownerID uuid.UUID) (comic.ImportJob, error) {
	row, err := a.q.CreateImport(ctx, CreateImportParams{ComicID: pgUUID(comicID), ChapterID: pgUUID(chapterID), OwnerUserID: pgUUID(ownerID)})
	if err != nil {
		return comic.ImportJob{}, err
	}
	return toImport(row), nil
}

func (a *Adapter) CreateComicImport(ctx context.Context, comicID, ownerID uuid.UUID) (comic.ImportJob, error) {
	row, err := a.q.CreateComicImport(ctx, CreateComicImportParams{ComicID: pgUUID(comicID), OwnerUserID: pgUUID(ownerID)})
	if err != nil {
		return comic.ImportJob{}, err
	}
	return toImport(row), nil
}

func (a *Adapter) GetImport(ctx context.Context, id uuid.UUID) (comic.ImportJob, error) {
	row, err := a.q.GetImport(ctx, pgUUID(id))
	if err != nil {
		return comic.ImportJob{}, mapNotFound(err)
	}
	return toImport(row), nil
}

func (a *Adapter) SetImportUpload(ctx context.Context, id uuid.UUID, uploadRef string) (comic.ImportJob, error) {
	row, err := a.q.SetImportUpload(ctx, SetImportUploadParams{ID: pgUUID(id), UploadRef: &uploadRef})
	if err != nil {
		return comic.ImportJob{}, mapNotFound(err)
	}
	return toImport(row), nil
}

func (a *Adapter) StartImport(ctx context.Context, id uuid.UUID, total int) error {
	return a.q.StartImport(ctx, StartImportParams{ID: pgUUID(id), Total: int32(total)})
}

// UpdateImportProgress / FinishImport pass the report json as TEXT (not []byte):
// under QueryExecModeExec pgx encodes []byte as bytea, which a jsonb column rejects.
func (a *Adapter) UpdateImportProgress(ctx context.Context, id uuid.UUID, succeeded, failed int, report []comic.ImportFileResult) error {
	b, _ := json.Marshal(report)
	_, err := a.db.Exec(ctx, `UPDATE comic_imports SET succeeded=$2, failed=$3, report=$4::jsonb, updated_at=now() WHERE id=$1`,
		pgUUID(id), int32(succeeded), int32(failed), string(b))
	return err
}

func (a *Adapter) FinishImport(ctx context.Context, id uuid.UUID, status string, succeeded, failed int, report []comic.ImportFileResult, errMsg *string) error {
	b, _ := json.Marshal(report)
	_, err := a.db.Exec(ctx, `UPDATE comic_imports SET status=$2, succeeded=$3, failed=$4, report=$5::jsonb, error=$6, updated_at=now() WHERE id=$1`,
		pgUUID(id), status, int32(succeeded), int32(failed), string(b), errMsg)
	return err
}

func toImport(r ComicImport) comic.ImportJob {
	var report []comic.ImportFileResult
	if len(r.Report) > 0 {
		_ = json.Unmarshal(r.Report, &report)
	}
	return comic.ImportJob{
		ID: uuidFrom(r.ID), ComicID: uuidFrom(r.ComicID), ChapterID: uuidPtr(r.ChapterID), OwnerUserID: uuidFrom(r.OwnerUserID),
		Status: r.Status, UploadRef: r.UploadRef, Total: int(r.Total), Succeeded: int(r.Succeeded), Failed: int(r.Failed),
		Report: report, Error: r.Error, CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
}

// ── mapping helpers ───────────────────────────────────────────────────

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return comic.ErrNotFound
	}
	return err
}

func toComic(r Comic) comic.Comic {
	return comic.Comic{
		ID: uuidFrom(r.ID), OwnerID: uuidFrom(r.OwnerUserID), Title: r.Title,
		Description: r.Description, CoverAssetID: uuidPtr(r.CoverAssetID), Status: r.Status,
		ReadingDirection: r.ReadingDirection,
		CreatedAt:        r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
}

func toChapter(r ComicChapter) comic.Chapter {
	return comic.Chapter{ID: uuidFrom(r.ID), ComicID: uuidFrom(r.ComicID), Title: r.Title, SortOrder: int(r.SortOrder), CreatedAt: r.CreatedAt.Time}
}

func toPage(r ComicPage) comic.Page {
	return comic.Page{ID: uuidFrom(r.ID), ChapterID: uuidFrom(r.ChapterID), AssetID: uuidFrom(r.AssetID), SortOrder: int(r.SortOrder)}
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
