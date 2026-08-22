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
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// syncStaleAfter lets a re-trigger through if a prior sync has been stuck in
// "syncing" this long — a lost scraper callback shouldn't strand it forever.
const syncStaleAfter = 15 * time.Minute

// errNoSyncOwner rejects an internal callback that arrived without the owner the
// scraper is required to echo back. Fail closed: without an owner there is no
// tenant to scope to, and the pre-0031 behaviour was to write unscoped.
var errNoSyncOwner = fmt.Errorf("%w: thiếu owner_id", ErrValidation)

// syncOwnerScope runs fn inside ownerID's tenant.
//
// The /internal/comic/* endpoints are shared-secret gated but carry no user
// session, so nothing has put a tenant on the context — yet comic_sync_sources and
// comic_imports are both FORCE ROW LEVEL SECURITY with a tenant_id policy. Writing
// unscoped only appears to work because the app role is currently superuser; the
// moment that changes, every callback fails. The owner echoed back by the scraper
// supplies the missing tenant.
//
// Echoed identity is not trusted on its own: each caller re-reads the row inside
// the scope and compares the real owner, so a forged owner_id resolves to
// ErrNotFound today, and is additionally invisible to RLS once superuser is gone.
func (s *Service) syncOwnerScope(ctx context.Context, ownerID uuid.UUID, fn func(context.Context) error) error {
	if s.runInTenant == nil { // worker side / tests: no tenant plumbing wired
		return fn(ctx)
	}
	return s.runInTenant(ctx, ownerID, fn)
}

// ownedSyncSource loads a source and confirms it really belongs to ownerID.
func (s *Service) ownedSyncSource(ctx context.Context, sourceID, ownerID uuid.UUID) (SyncSource, error) {
	src, err := s.repo.GetSyncSource(ctx, sourceID)
	if err != nil {
		return SyncSource{}, err
	}
	if src.OwnerUserID != ownerID {
		return SyncSource{}, ErrNotFound
	}
	return src, nil
}

// CreateSyncSource registers an external source for a comic (owner-gated upstream
// by the comic middleware). The URL must be absolute http(s).
func (s *Service) CreateSyncSource(ctx context.Context, comicID, ownerID uuid.UUID, sourceURL, chaptersHint string) (SyncSource, error) {
	sourceURL = strings.TrimSpace(sourceURL)
	// SSRF guard — see sourceguard.go. Rechecked before every scrape too.
	u, err := validateSourceURL(ctx, sourceURL, s.sourceAllowlist)
	if err != nil {
		return SyncSource{}, err
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
	// Re-run the SSRF guard right before handing the URL to the scraper: DNS may
	// have changed since the source was created (rebinding), and rows created
	// before this guard landed were never checked at all.
	if _, verr := validateSourceURL(ctx, src.SourceURL, s.sourceAllowlist); verr != nil {
		msg := verr.Error()
		_ = s.repo.UpdateSyncStatus(ctx, src.ID, "failed", nil, &msg, false)
		return SyncSource{}, verr
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
	if serr := s.scraper.StartScrape(ctx, src.ID, src.OwnerUserID, src.SourceURL, src.ChaptersHint, existing); serr != nil {
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
func (s *Service) RequestSyncBatch(ctx context.Context, sourceID, ownerID uuid.UUID) (uuid.UUID, string, error) {
	if ownerID == uuid.Nil {
		return uuid.Nil, "", errNoSyncOwner
	}
	// Read, verify and write all inside the owner's tenant — the read is what makes
	// a forged owner_id useless, and keeping the insert in the same tx means a
	// source never ends up pointing at an import that was rolled back.
	var job ImportJob
	err := s.syncOwnerScope(ctx, ownerID, func(ctx context.Context) error {
		src, e := s.ownedSyncSource(ctx, sourceID, ownerID)
		if e != nil {
			return e
		}
		if job, e = s.repo.CreateComicImport(ctx, src.ComicID, src.OwnerUserID); e != nil {
			return e
		}
		return s.repo.SetSyncLastImport(ctx, sourceID, job.ID) // UI follows the current batch
	})
	if err != nil {
		return uuid.Nil, "", err
	}
	return job.ID, importZipKey(job.ID), nil
}

// SyncBatchUploaded is invoked per batch: ok=true means its zip is uploaded → wire
// the import to it + enqueue the worker; ok=false fails that batch's import job. It
// does NOT change the source (FinalizeSync does, once all batches are done).
func (s *Service) SyncBatchUploaded(ctx context.Context, importID, ownerID uuid.UUID, ok bool, errMsg string) error {
	if ownerID == uuid.Nil {
		return errNoSyncOwner
	}
	err := s.syncOwnerScope(ctx, ownerID, func(ctx context.Context) error {
		// Keyed by import rather than source, so verify ownership on the job itself.
		job, e := s.repo.GetImport(ctx, importID)
		if e != nil {
			return e
		}
		if job.OwnerUserID != ownerID {
			return ErrNotFound
		}
		if !ok {
			m := errMsg
			if m == "" {
				m = "cào lô lỗi"
			}
			return s.repo.FinishImport(ctx, importID, ImportFailed, 0, 0, nil, &m)
		}
		_, e = s.repo.SetImportUpload(ctx, importID, importZipKey(importID))
		return e
	})
	if err != nil || !ok {
		return err
	}
	// Enqueue only after the tx committed — the worker reads the row back.
	if s.enqueue != nil {
		return s.enqueueImportZip(importID)
	}
	return nil
}

// SyncProgress records overall chapter progress on the source so the UI can show
// "cào X/Y chương" across all batches.
func (s *Service) SyncProgress(ctx context.Context, sourceID, ownerID uuid.UUID, scraped, total int) error {
	if ownerID == uuid.Nil {
		return errNoSyncOwner
	}
	return s.syncOwnerScope(ctx, ownerID, func(ctx context.Context) error {
		if _, e := s.ownedSyncSource(ctx, sourceID, ownerID); e != nil {
			return e
		}
		return s.repo.UpdateSyncProgress(ctx, sourceID, scraped, total)
	})
}

// FinalizeSync marks the source done once the scraper has processed every batch.
// failedSummary (blank if none) is stored in last_error and shown on the UI.
func (s *Service) FinalizeSync(ctx context.Context, sourceID, ownerID uuid.UUID, ok bool, failedSummary string) error {
	if ownerID == uuid.Nil {
		return errNoSyncOwner
	}
	status := "done"
	if !ok {
		status = "failed"
	}
	var errp *string
	if failedSummary != "" {
		errp = &failedSummary
	}
	return s.syncOwnerScope(ctx, ownerID, func(ctx context.Context) error {
		if _, e := s.ownedSyncSource(ctx, sourceID, ownerID); e != nil {
			return e
		}
		return s.repo.UpdateSyncStatus(ctx, sourceID, status, nil, errp, ok)
	})
}

func importZipKey(importID uuid.UUID) string { return fmt.Sprintf("import/%s.zip", importID) }
