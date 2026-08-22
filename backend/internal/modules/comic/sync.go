package comic

// External-source sync (SPEC-02 P1.8). A sync source binds a comic to an external
// URL. Triggering a sync creates a comic-level ImportJob (reusing the P1.7 pipeline)
// and asks the Python scraper service to scrape the source and upload a
// folder-per-chapter zip to the job's import/ key; the scraper then calls the
// sync-callback, which enqueues the existing comic:import_zip worker.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// syncStaleAfter lets a re-trigger through if a prior sync has been stuck in
// "syncing" this long — a lost scraper callback shouldn't strand it forever.
const syncStaleAfter = 15 * time.Minute

// CreateSyncSource registers an external source for a comic (owner-gated upstream
// by the comic middleware). The URL must be absolute http(s).
func (s *Service) CreateSyncSource(ctx context.Context, comicID, ownerID uuid.UUID, sourceURL, chaptersHint string) (SyncSource, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	u, err := url.Parse(sourceURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return SyncSource{}, fmt.Errorf("%w: source_url phải là http(s) hợp lệ", ErrValidation)
	}
	return s.repo.CreateSyncSource(ctx, comicID, ownerID, sourceURL, u.Host, strings.TrimSpace(chaptersHint))
}

// ListSyncSources lists a comic's sources (owner-gated upstream by the comic mw).
func (s *Service) ListSyncSources(ctx context.Context, comicID uuid.UUID) ([]SyncSource, error) {
	return s.repo.ListSyncSources(ctx, comicID)
}

// ownedSource resolves a source for the owner (404 to others — don't leak).
func (s *Service) ownedSource(ctx context.Context, sourceID, ownerID uuid.UUID) (SyncSource, error) {
	src, err := s.repo.GetSyncSource(ctx, sourceID)
	if err != nil {
		return SyncSource{}, err
	}
	if src.OwnerUserID != ownerID {
		return SyncSource{}, ErrNotFound
	}
	return src, nil
}

// DeleteSyncSource removes a source (owner-checked).
func (s *Service) DeleteSyncSource(ctx context.Context, sourceID, ownerID uuid.UUID) error {
	if _, err := s.ownedSource(ctx, sourceID, ownerID); err != nil {
		return err
	}
	return s.repo.DeleteSyncSource(ctx, sourceID)
}

// TriggerSync starts a sync: creates a comic-level import job and hands the scrape
// off to the scraper service. Owner-checked. The scraper uploads the zip to the
// job's import/ key and calls SyncCallback, which enqueues the import worker.
func (s *Service) TriggerSync(ctx context.Context, sourceID, ownerID uuid.UUID) (SyncSource, error) {
	if s.scraper == nil {
		return SyncSource{}, errors.New("comic: sync not configured")
	}
	src, err := s.ownedSource(ctx, sourceID, ownerID)
	if err != nil {
		return SyncSource{}, err
	}
	if src.LastStatus == "syncing" && time.Since(src.UpdatedAt) < syncStaleAfter {
		return src, fmt.Errorf("%w: đang đồng bộ", ErrValidation)
	}
	// Reset progress; the scraper drives the whole batched sync from here.
	_ = s.repo.UpdateSyncProgress(ctx, src.ID, 0, 0)
	if err := s.repo.UpdateSyncStatus(ctx, src.ID, "syncing", nil, nil, false); err != nil {
		log.Error().Err(err).Str("source", src.ID.String()).Msg("comic: update sync status")
	}
	// Existing chapter titles so the scraper can skip re-downloading them (incremental
	// sync). It only skips when there's no explicit ChaptersHint — a hint means
	// "force these", so a targeted re-scrape still runs even for existing chapters.
	var existing []string
	if chs, err := s.repo.ListChapters(ctx, src.ComicID); err == nil {
		existing = make([]string, 0, len(chs))
		for _, c := range chs {
			existing = append(existing, c.Title)
		}
	}
	if serr := s.scraper.StartScrape(ctx, src.ID, src.SourceURL, src.ChaptersHint, existing); serr != nil {
		msg := serr.Error()
		_ = s.repo.UpdateSyncStatus(ctx, src.ID, "failed", nil, &msg, false)
		return SyncSource{}, fmt.Errorf("comic: không gọi được scraper: %w", serr)
	}
	src.LastStatus = "syncing"
	src.TotalChapters, src.ScrapedChapters = 0, 0
	return src, nil
}

// CancelSync stops a running sync: it signals the scraper to stop (best-effort) and
// marks the source 'cancelled'. Chapters already imported are kept. Owner-checked.
func (s *Service) CancelSync(ctx context.Context, sourceID, ownerID uuid.UUID) (SyncSource, error) {
	src, err := s.ownedSource(ctx, sourceID, ownerID)
	if err != nil {
		return SyncSource{}, err
	}
	if s.scraper != nil {
		_ = s.scraper.CancelScrape(ctx, sourceID)
	}
	if err := s.repo.UpdateSyncStatus(ctx, sourceID, "cancelled", nil, nil, false); err != nil {
		return SyncSource{}, err
	}
	src.LastStatus = "cancelled"
	return src, nil
}

// RequestSyncBatch is called by the scraper (shared-secret guarded) to get a fresh
// import job for one batch of chapters. Returns the import id + the storage key the
// scraper must upload that batch's zip to.
func (s *Service) RequestSyncBatch(ctx context.Context, sourceID uuid.UUID) (uuid.UUID, string, error) {
	src, err := s.repo.GetSyncSource(ctx, sourceID)
	if err != nil {
		return uuid.Nil, "", err
	}
	// The import job insert needs a tenant scope (comic_imports.tenant_id default);
	// this endpoint runs outside authTenant, so scope it to the source owner here.
	var job ImportJob
	create := func(ctx context.Context) error {
		var e error
		job, e = s.repo.CreateComicImport(ctx, src.ComicID, src.OwnerUserID)
		return e
	}
	if s.runInTenant != nil {
		err = s.runInTenant(ctx, src.OwnerUserID, create)
	} else {
		err = create(ctx)
	}
	if err != nil {
		return uuid.Nil, "", err
	}
	_ = s.repo.SetSyncLastImport(ctx, sourceID, job.ID) // UI can follow the current batch
	return job.ID, importZipKey(job.ID), nil
}

// SyncBatchUploaded is invoked per batch: ok=true means its zip is uploaded → wire
// the import to it + enqueue the worker; ok=false fails that batch's import job. It
// does NOT change the source (FinalizeSync does, once all batches are done).
func (s *Service) SyncBatchUploaded(ctx context.Context, importID uuid.UUID, ok bool, errMsg string) error {
	if !ok {
		m := errMsg
		if m == "" {
			m = "cào lô lỗi"
		}
		return s.repo.FinishImport(ctx, importID, ImportFailed, 0, 0, nil, &m)
	}
	if _, err := s.repo.SetImportUpload(ctx, importID, importZipKey(importID)); err != nil {
		return err
	}
	if s.enqueue != nil {
		return s.enqueueImportZip(importID)
	}
	return nil
}

// SyncProgress records overall chapter progress on the source so the UI can show
// "cào X/Y chương" across all batches.
func (s *Service) SyncProgress(ctx context.Context, sourceID uuid.UUID, scraped, total int) error {
	return s.repo.UpdateSyncProgress(ctx, sourceID, scraped, total)
}

// FinalizeSync marks the source done once the scraper has processed every batch.
// failedSummary (blank if none) is stored in last_error and shown on the UI.
func (s *Service) FinalizeSync(ctx context.Context, sourceID uuid.UUID, ok bool, failedSummary string) error {
	status := "done"
	if !ok {
		status = "failed"
	}
	var errp *string
	if failedSummary != "" {
		errp = &failedSummary
	}
	return s.repo.UpdateSyncStatus(ctx, sourceID, status, nil, errp, ok)
}

func importZipKey(importID uuid.UUID) string { return fmt.Sprintf("import/%s.zip", importID) }
