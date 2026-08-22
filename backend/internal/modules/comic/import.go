package comic

// Zip chapter import (SPEC-02 P1.7). The API stores the uploaded zip under an
// import/ prefix and enqueues comic:import_zip; the worker (RunImport) spools the
// zip to a temp file, unpacks the image entries (guarded), then: (A) creates every
// chapter, (B) ingests EVERY image via mediaapi up front (each a committed tenant
// tx → process_image can see it) so the parallel "image" queue stays saturated,
// while (C) opportunistically bulk-polling assets to ready (one query per round)
// and paging each chapter the moment its images finish — so progress tracks
// transcode live and chapters commit incrementally. The persisted job carries a
// per-file report the client polls.

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/rs/zerolog/log"

	comicapi "github.com/portal/backend/internal/modules/comic/api"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Import guardrails (P1.7). Sized for whole-comic archives (folder-per-chapter):
// a full long-running series is ~350 chapters / ~42k images / ~4.2 GB (measured on
// a real scrape), so the caps are set an order of magnitude above a "big comic"
// rather than at it — a 3 GiB / 20k-entry pair rejected exactly the archives this
// feature exists for. Both spool to a temp file (bounded memory) and the zip is
// deleted from storage once the import finishes. Image variants transcode in
// parallel on the "image" queue (IMAGE_CONCURRENCY), so the poll budget is per
// image amortized across that pool, capped under the task's 12h lease.
const (
	importMaxZipBytes  = 16 << 30 // 16 GiB
	importMaxEntries   = 100000
	importMaxRatio     = 100 // per-entry compression ratio (zip-bomb guard)
	importPollInterval = 2 * time.Second
	importPollFloor    = 2 * time.Minute
	importPollPerImage = 5 * time.Second // amortized budget per image (parallel image pool)
	importPollCeiling  = 11 * time.Hour  // hard cap, safely under the 12h task Timeout
)

var importImageExt = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true, ".avif": true, ".bmp": true}

// CreateImport registers a zip-import job for a chapter. ownerID is the caller
// (already owner-gated by the chapter middleware).
func (s *Service) CreateImport(ctx context.Context, chapterID, ownerID uuid.UUID) (ImportJob, error) {
	comicID, err := s.repo.ChapterComic(ctx, chapterID)
	if err != nil {
		return ImportJob{}, err
	}
	return s.repo.CreateImport(ctx, comicID, chapterID, ownerID)
}

// CreateComicImport registers a comic-level (multi-chapter) import job.
func (s *Service) CreateComicImport(ctx context.Context, comicID, ownerID uuid.UUID) (ImportJob, error) {
	return s.repo.CreateComicImport(ctx, comicID, ownerID)
}

// GetImport returns a job for the owner (404 to others — don't leak existence).
func (s *Service) GetImport(ctx context.Context, importID, ownerID uuid.UUID) (ImportJob, error) {
	job, err := s.repo.GetImport(ctx, importID)
	if err != nil {
		return ImportJob{}, err
	}
	if job.OwnerUserID != ownerID {
		return ImportJob{}, ErrNotFound
	}
	return job, nil
}

// SaveImportZip streams the uploaded zip to storage (500 MB cap) and enqueues the
// worker task. Owner-checked.
func (s *Service) SaveImportZip(ctx context.Context, importID, ownerID uuid.UUID, body io.Reader) (ImportJob, error) {
	if s.store == nil || s.enqueue == nil {
		return ImportJob{}, errors.New("comic: import not configured")
	}
	job, err := s.repo.GetImport(ctx, importID)
	if err != nil {
		return ImportJob{}, err
	}
	if job.OwnerUserID != ownerID {
		return ImportJob{}, ErrNotFound
	}

	// Spool to a temp file: the S3 SDK/MinIO needs a seekable body with a known
	// length (a raw request stream fails), and it lets us enforce the cap first.
	tmp, err := os.CreateTemp("", "comic-zip-*")
	if err != nil {
		return ImportJob{}, err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, err := io.Copy(tmp, io.LimitReader(body, importMaxZipBytes+1))
	if err != nil {
		return ImportJob{}, err
	}
	if n > importMaxZipBytes {
		return ImportJob{}, fmt.Errorf("%w: zip %s vượt giới hạn %s", ErrValidation, humanBytes(n), humanBytes(importMaxZipBytes))
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return ImportJob{}, err
	}
	key := fmt.Sprintf("import/%s.zip", importID)
	if err := s.store.Put(ctx, key, tmp, "application/zip"); err != nil {
		log.Error().Err(err).Int64("bytes", n).Str("key", key).Msg("comic: import store.Put failed")
		return ImportJob{}, err
	}

	updated, err := s.repo.SetImportUpload(ctx, importID, key)
	if err != nil {
		return ImportJob{}, err
	}
	if err := s.enqueueImportZip(importID); err != nil {
		return ImportJob{}, err
	}
	return updated, nil
}

// enqueueImportZip dispatches the comic:import_zip worker for a job whose upload is
// ready. A whole-comic import can run for tens of minutes; without a generous
// Timeout asynq's ~30-min task lease expires mid-run, cancels the context, and
// re-queues the task → it re-creates every chapter (duplicates). Timeout keeps the
// lease alive; MaxRetry(0) means a genuine failure aborts instead of re-running.
func (s *Service) enqueueImportZip(importID uuid.UUID) error {
	payload, _ := json.Marshal(comicapi.ImportZipPayload{ImportID: importID.String()})
	task := asynq.NewTask(comicapi.TaskImportZip, payload, asynq.Queue("default"), asynq.Timeout(12*time.Hour), asynq.MaxRetry(0))
	_, err := s.enqueue.Enqueue(task)
	return err
}

// RunImport is the worker body (comic:import_zip). Best-effort: any hard failure
// marks the job failed; per-entry failures are collected in the report.
func (s *Service) RunImport(ctx context.Context, importID uuid.UUID) error {
	job, err := s.repo.GetImport(ctx, importID)
	if err != nil {
		return err
	}
	if s.store == nil || s.media == nil || s.runInTenant == nil {
		return s.failImport(ctx, importID, "import not configured", nil)
	}
	// The task is enqueued from SaveImportZip *outside* the request's pg tx, so the
	// worker can dequeue and read the job before SetImportUpload's commit is visible
	// (more likely after a slow multi-GB upload). Wait briefly for upload_ref rather
	// than failing a perfectly valid import (SPEC-02 P1.7).
	for i := 0; job.UploadRef == nil && i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if job, err = s.repo.GetImport(ctx, importID); err != nil {
			return err
		}
	}
	if job.UploadRef == nil {
		return s.failImport(ctx, importID, "no upload", nil)
	}
	owner := job.OwnerUserID

	// 1. spool the zip to a temp file (bounded memory; archive/zip needs ReaderAt).
	rc, err := s.store.Get(ctx, *job.UploadRef)
	if err != nil {
		return s.failImport(ctx, importID, "cannot read upload", job.UploadRef)
	}
	tmp, err := os.CreateTemp("", "comic-import-*.zip")
	if err != nil {
		rc.Close()
		return s.failImport(ctx, importID, "temp file", job.UploadRef)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	n, copyErr := io.Copy(tmp, io.LimitReader(rc, importMaxZipBytes+1))
	rc.Close()
	if copyErr != nil {
		return s.failImport(ctx, importID, "spool zip", job.UploadRef)
	}

	// 2. open + collect image entries (guarded).
	zr, err := zip.NewReader(tmp, n)
	if err != nil {
		return s.failImport(ctx, importID, "not a valid zip", job.UploadRef)
	}
	type entry struct {
		name string
		f    *zip.File
	}
	var entries []entry
	for _, f := range zr.File {
		name := f.Name
		if f.FileInfo().IsDir() || strings.Contains(name, "..") || strings.HasPrefix(name, "/") || strings.HasPrefix(name, "__MACOSX/") {
			continue
		}
		if !importImageExt[strings.ToLower(path.Ext(name))] {
			continue
		}
		if f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > importMaxRatio {
			continue // zip-bomb guard
		}
		entries = append(entries, entry{name: name, f: f})
	}
	if len(entries) > importMaxEntries {
		return s.failImport(ctx, importID, fmt.Sprintf("quá nhiều ảnh (%d > %d)", len(entries), importMaxEntries), job.UploadRef)
	}
	if len(entries) == 0 {
		return s.failImport(ctx, importID, "zip không có ảnh hợp lệ", job.UploadRef)
	}

	// Group into chapters. A chapter-level job (chapter_id set) → one group into
	// that chapter. A comic-level job groups images by their top-level folder →
	// one chapter per folder, natural-sorted (handles a wrapper folder too).
	type group struct {
		title   string
		chapter *uuid.UUID // existing chapter (chapter-level), else create per folder
		entries []entry
	}
	byBase := func(a, b entry) bool { return natLess(path.Base(a.name), path.Base(b.name)) }
	var groups []group
	if job.ChapterID != nil {
		sort.Slice(entries, func(i, j int) bool { return byBase(entries[i], entries[j]) })
		groups = []group{{chapter: job.ChapterID, entries: entries}}
	} else {
		byDir := map[string][]entry{}
		for _, e := range entries {
			byDir[path.Dir(e.name)] = append(byDir[path.Dir(e.name)], e)
		}
		dirs := make([]string, 0, len(byDir))
		for d := range byDir {
			dirs = append(dirs, d)
		}
		sort.Slice(dirs, func(i, j int) bool { return natLess(dirs[i], dirs[j]) })
		for _, d := range dirs {
			es := byDir[d]
			sort.Slice(es, func(i, j int) bool { return byBase(es[i], es[j]) })
			groups = append(groups, group{title: chapterTitleFromDir(d), entries: es})
		}
	}

	// Idempotent sync (P1.8): reconcile scraped chapters against what the comic
	// already has so re-syncing UPDATES the list in place instead of appending a
	// duplicate copy. Per matched title:
	//   • present WITH pages → skip the group (no re-import, no duplicate)
	//   • present but EMPTY  → refill that same chapter in place (repairs gaps)
	//   • absent             → create it at the slot its own title implies.
	// sort_order comes from the chapter NUMBER in the title, so it does not depend on
	// which batch happens to finish first: a sync may scrape chapters in parallel and
	// import them out of order, and the reader still sees them in order.
	// A title carrying no number (hand-made zips) falls back to appending after
	// MAX(sort_order) — baseSort is MAX, not chapter COUNT, because the unique index
	// is (comic_id, sort_order) and a chapter deleted from the middle leaves
	// count < max, so (count+i)*10 would collide with a live row ("tạo chương lỗi").
	baseSort := 0
	usedSort := map[int]bool{}
	if job.ChapterID == nil {
		existingID := map[string]uuid.UUID{}
		if chs, err := s.repo.ListChapters(ctx, job.ComicID); err == nil {
			for _, c := range chs {
				existingID[c.Title] = c.ID
				usedSort[c.SortOrder] = true
				if c.SortOrder > baseSort {
					baseSort = c.SortOrder
				}
			}
		}
		emptyTitle := map[string]bool{}
		if refs, err := s.repo.ChaptersWithoutPages(ctx, job.ComicID); err == nil {
			for _, r := range refs {
				emptyTitle[r.Title] = true
			}
		}
		kept := groups[:0]
		for _, g := range groups {
			id, exists := existingID[g.title]
			if exists && !emptyTitle[g.title] {
				continue // already present & complete → skip, don't duplicate
			}
			if exists { // present but empty → refill this exact chapter in place
				cid := id
				g.chapter = &cid
			}
			kept = append(kept, g)
		}
		groups = kept
	}

	// total = files that will actually be processed (after skipping present chapters)
	total := 0
	for _, g := range groups {
		total += len(g.entries)
	}
	_ = s.repo.StartImport(ctx, importID, total)

	report := make([]ImportFileResult, 0, len(entries))
	succ, fail := 0, 0

	// Phase A — create every chapter up front (cheap: a few inserts). A group
	// whose chapter fails is reported failed here and skipped in phases B/D.
	type chapterPlan struct {
		id         uuid.UUID
		startCount int
		ok         bool
	}
	plans := make([]chapterPlan, len(groups))
	for gi, g := range groups {
		if g.chapter != nil {
			plans[gi] = chapterPlan{id: *g.chapter, ok: true}
			if pages, err := s.repo.ListPages(ctx, *g.chapter); err == nil {
				plans[gi].startCount = len(pages)
			}
			continue
		}
		want, ok := chapterSortOrder(g.title)
		if !ok {
			want = baseSort + (gi+1)*10 // untitled/numberless → keep appending
		}
		ch, cerr := s.createChapterAt(ctx, owner, job.ComicID, g.title, want, usedSort)
		if cerr != nil {
			for _, e := range g.entries {
				report = append(report, ImportFileResult{Name: e.name, OK: false, Error: "tạo chương lỗi"})
				fail++
			}
			continue
		}
		plans[gi] = chapterPlan{id: ch.ID, ok: true}
	}

	// Phases B–D: (B) ingest EVERY image up front in parallel (feeds the transcode
	// pool fast), then (C) drain-poll to ready, paging each chapter the moment its
	// images finish — so pages commit incrementally (a crash mid-drain keeps chapters
	// already done) and `succeeded` climbs live. results[gi][ord].assetID==Nil ⇒ that
	// slot failed to read/ingest (already reported) and is skipped when paging.
	type made struct {
		name    string
		assetID uuid.UUID
	}
	type slot struct{ gi, ord int }
	results := make([][]made, len(groups))
	for gi := range groups {
		results[gi] = make([]made, len(groups[gi].entries))
		for ord := range results[gi] {
			results[gi][ord].name = groups[gi].entries[ord].name
		}
	}
	owners := make(map[uuid.UUID]slot)
	resolved := make(map[uuid.UUID]bool) // ready=true / failed=false; absent=pending
	var allIDs []uuid.UUID
	ingestedCount := make([]int, len(groups))
	resolvedCount := make([]int, len(groups))
	chapterFull := make([]bool, len(groups)) // every image of the chapter ingested
	chapterPaged := make([]bool, len(groups))

	// pageChapter writes gi's ready assets as pages in entry order (idempotent via
	// chapterPaged). Called once a chapter's assets are all resolved.
	pageChapter := func(gi int) {
		chapterPaged[gi] = true
		var pageInputs []PageInput
		n := 0
		for ord := range results[gi] {
			id := results[gi][ord].assetID
			if id == uuid.Nil || !resolved[id] {
				continue
			}
			pageInputs = append(pageInputs, PageInput{AssetID: id, SortOrder: (plans[gi].startCount + n + 1) * 10})
			n++
		}
		if len(pageInputs) == 0 {
			return
		}
		if perr := s.runInTenant(ctx, owner, func(ctx context.Context) error {
			_, e := s.CreatePages(ctx, plans[gi].id, pageInputs)
			return e
		}); perr != nil {
			log.Error().Err(perr).Str("import", importID.String()).Msg("comic: import create pages")
		}
	}

	// resolveRound bulk-polls the still-pending assets once (one tenant tx + one
	// query), records ready/failed, and pages any chapter that just completed.
	resolveRound := func() {
		pending := pendingIDs(allIDs, resolved)
		if len(pending) == 0 {
			return
		}
		var statuses map[uuid.UUID]mediaapi.AssetStatus
		perr := s.runInTenant(ctx, owner, func(ctx context.Context) error {
			var e error
			statuses, e = s.media.AssetStatuses(ctx, pending)
			return e
		})
		if perr != nil {
			return
		}
		for id, st := range statuses {
			if _, seen := resolved[id]; seen {
				continue
			}
			sl := owners[id]
			switch st {
			case mediaapi.StatusReady:
				resolved[id] = true
				resolvedCount[sl.gi]++
				report = append(report, ImportFileResult{Name: results[sl.gi][sl.ord].name, OK: true})
				succ++
			case mediaapi.StatusFailed:
				resolved[id] = false
				resolvedCount[sl.gi]++
				report = append(report, ImportFileResult{Name: results[sl.gi][sl.ord].name, OK: false, Error: "xử lý lỗi"})
				fail++
			}
		}
		for gi := range groups {
			if plans[gi].ok && chapterFull[gi] && !chapterPaged[gi] && resolvedCount[gi] == ingestedCount[gi] {
				pageChapter(gi)
			}
		}
		_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, trimReport(report))
	}

	// Phase B — parallel ingest. Read zip entries serially (archive/zip over a
	// shared file isn't concurrent-safe), then fan the DB+storage ingest out to a
	// bounded pool: ingest is I/O-bound (a MinIO PUT + a tenant tx per image), so a
	// sequential loop leaves most of the box's cores idle and starves the transcode
	// pool. An unbuffered channel bounds in-memory image buffers to ~ingestN. mu
	// guards the shared bookkeeping; results[gi][ord] is written per-unique-index.
	var mu sync.Mutex
	type ingestTask struct {
		gi, ord int
		name    string
		data    []byte
	}
	tasks := make(chan ingestTask)
	var wg sync.WaitGroup
	ingestN := importIngestConcurrency()
	for w := 0; w < ingestN; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				var assetID uuid.UUID
				ierr := s.runInTenant(ctx, owner, func(ctx context.Context) error {
					id, e2 := s.media.IngestImage(ctx, owner, path.Base(t.name), mimeByExt(t.name), t.data)
					assetID = id
					return e2
				})
				mu.Lock()
				if ierr != nil {
					report = append(report, ImportFileResult{Name: t.name, OK: false, Error: "nạp lỗi"})
					fail++
				} else {
					results[t.gi][t.ord].assetID = assetID
					owners[assetID] = slot{t.gi, t.ord}
					allIDs = append(allIDs, assetID)
					ingestedCount[t.gi]++
				}
				mu.Unlock()
			}
		}()
	}
	for gi, g := range groups {
		if !plans[gi].ok {
			continue
		}
		for ord, e := range g.entries {
			data, rerr := readZipEntry(e.f)
			if rerr != nil {
				mu.Lock()
				report = append(report, ImportFileResult{Name: e.name, OK: false, Error: "đọc lỗi"})
				fail++
				mu.Unlock()
				continue
			}
			tasks <- ingestTask{gi: gi, ord: ord, name: e.name, data: data}
		}
	}
	close(tasks)
	wg.Wait()
	for gi := range groups {
		if plans[gi].ok {
			chapterFull[gi] = true
		}
	}
	_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, trimReport(report))

	// Phase C — drain: keep resolving until every ingested asset is ready/failed
	// (or the budget runs out), paging chapters as they complete.
	if len(allIDs) > 0 {
		deadline := time.Now().Add(pollBudget(len(allIDs)))
		for len(resolved) < len(allIDs) && time.Now().Before(deadline) {
			resolveRound()
			if len(resolved) < len(allIDs) {
				time.Sleep(importPollInterval)
			}
		}
		// anything still pending at the deadline → not ready
		for _, id := range allIDs {
			if _, seen := resolved[id]; !seen {
				resolved[id] = false
				sl := owners[id]
				report = append(report, ImportFileResult{Name: results[sl.gi][sl.ord].name, OK: false, Error: "không sẵn sàng"})
				fail++
			}
		}
	}

	// Phase D — final sweep: page any chapter not yet paged (deadline cut it off,
	// or its poll never completed) for whatever reached ready.
	for gi := range groups {
		if plans[gi].ok && chapterFull[gi] && !chapterPaged[gi] {
			pageChapter(gi)
		}
	}
	_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, trimReport(report))

	if err := s.repo.FinishImport(ctx, importID, ImportDone, succ, fail, trimReport(report), nil); err != nil {
		log.Error().Err(err).Str("import", importID.String()).Msg("comic: finish import")
	}
	_ = s.store.Delete(ctx, *job.UploadRef)
	return nil
}

// pendingIDs returns the ids not yet resolved in done (ready/failed), shrinking
// each poll round so AssetStatuses only asks about assets still in flight.
func pendingIDs(all []uuid.UUID, done map[uuid.UUID]bool) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(all)-len(done))
	for _, id := range all {
		if _, ok := done[id]; !ok {
			out = append(out, id)
		}
	}
	return out
}

// chapterTitleFromDir derives a chapter title from a zip folder path.
// chapterNumRe pulls the first number out of a chapter title. The scraper names its
// folders with the bare chapter number ("12", "12.5"); hand-made zips tend to use
// "Chương 12" / "Chapter 12" / "Chap 12,5". A hyphen counts as a decimal point too:
// sources write side-chapters as "216-1", and reading only the "216" out of that
// would aim it at the same slot as chapter 216 itself.
var chapterNumRe = regexp.MustCompile(`(\d+(?:[.,-]\d+)?)`)

// chapterDecimalSep normalises the three separators above to a parseable ".".
var chapterDecimalSep = strings.NewReplacer(",", ".", "-", ".")

// maxChapterNumber caps what we accept as a chapter number. sort_order is an int4,
// and the ×10 below has to stay inside it; anything larger is not a chapter number
// but some other digit run in the title, so fall back to appending.
const maxChapterNumber = 10_000_000

// chapterSlotAttempts bounds the walk for a free sort_order when the derived slot is
// taken. Clashes are rare (same number twice) and resolved in a step or two.
const chapterSlotAttempts = 64

// chapterSortOrder derives a stable position from the chapter's title, scaled by 10
// so a ".5" side-story lands between its neighbours (12 → 120, 12.5 → 125, 13 → 130).
// Deriving it from the title instead of arrival order is what lets chapters be
// imported concurrently. ok=false ⇒ no usable number; the caller appends instead.
func chapterSortOrder(title string) (int, bool) {
	m := chapterNumRe.FindStringSubmatch(title)
	if m == nil {
		return 0, false
	}
	n, err := strconv.ParseFloat(chapterDecimalSep.Replace(m[1]), 64)
	if err != nil || n < 0 || n > maxChapterNumber {
		return 0, false
	}
	return int(math.Round(n * 10)), true
}

// createChapterAt inserts the chapter at sortOrder, stepping forward when that slot
// is taken. (comic_id, sort_order) is unique, and two concurrent sync batches can
// derive the same slot for different titles, so a clash is an expected outcome to
// route around rather than an error to report. `used` carries the slots this import
// already knows about; the 23505 retry covers the ones another import took first.
func (s *Service) createChapterAt(ctx context.Context, owner, comicID uuid.UUID, title string, sortOrder int, used map[int]bool) (Chapter, error) {
	var lastErr error
	for range chapterSlotAttempts {
		for used[sortOrder] {
			sortOrder++
		}
		var ch Chapter
		err := s.runInTenant(ctx, owner, func(ctx context.Context) error {
			var e error
			ch, e = s.repo.CreateChapter(ctx, comicID, title, sortOrder)
			return e
		})
		if err == nil {
			used[sortOrder] = true
			return ch, nil
		}
		if !isUniqueViolation(err) {
			return Chapter{}, err
		}
		used[sortOrder] = true // someone else holds it — remember and move on
		sortOrder++
		lastErr = err
	}
	return Chapter{}, fmt.Errorf("no free sort_order near %d: %w", sortOrder, lastErr)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func chapterTitleFromDir(dir string) string {
	if dir == "." || dir == "" {
		return "Chương 1"
	}
	if name := path.Base(dir); name != "" && name != "." {
		return name
	}
	return "Chương 1"
}

func (s *Service) failImport(ctx context.Context, importID uuid.UUID, msg string, uploadRef *string) error {
	m := msg
	_ = s.repo.FinishImport(ctx, importID, ImportFailed, 0, 0, nil, &m)
	if uploadRef != nil && s.store != nil {
		_ = s.store.Delete(ctx, *uploadRef)
	}
	log.Warn().Str("import", importID.String()).Str("reason", msg).Msg("comic: import failed")
	return nil // handled — don't retry the task
}

// importMaxReportEntries caps what the per-file report persists. The report is
// rewritten (as jsonb) every poll round and re-sent on every client poll, so a
// 42k-image import would push megabytes per round for detail nothing reads:
// progress comes from succeeded/failed, and only the failures are diagnostic.
// Below the cap the report is kept whole; above it, failures win the slots.
const importMaxReportEntries = 500

// trimReport bounds what UpdateImportProgress/FinishImport persist (see above).
func trimReport(report []ImportFileResult) []ImportFileResult {
	if len(report) <= importMaxReportEntries {
		return report
	}
	out := make([]ImportFileResult, 0, importMaxReportEntries)
	for _, r := range report { // failures first — they are the ones worth reading
		if !r.OK {
			out = append(out, r)
			if len(out) == importMaxReportEntries {
				return out
			}
		}
	}
	for _, r := range report { // fill any remaining slots with a sample of the successes
		if r.OK {
			out = append(out, r)
			if len(out) == importMaxReportEntries {
				break
			}
		}
	}
	return out
}

// humanBytes renders a byte count for a user-facing message ("4.2 GB").
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit && exp < 3; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}

// importIngestConcurrency is the number of parallel image-ingest workers
// (IMPORT_INGEST_CONCURRENCY, default 4). Ingest is I/O-bound (a MinIO PUT + a
// tenant tx per image); a few in flight keep the transcode pool fed without
// starving. Each worker holds one image buffer, so peak memory ≈ ingestN × image.
func importIngestConcurrency() int {
	if v := os.Getenv("IMPORT_INGEST_CONCURRENCY"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 4
}

func pollBudget(n int) time.Duration {
	b := time.Duration(n) * importPollPerImage
	if b < importPollFloor {
		return importPollFloor
	}
	if b > importPollCeiling {
		return importPollCeiling
	}
	return b
}

func readZipEntry(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 60<<20)) // ≥ media's 50MB image cap
}

func mimeByExt(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	case ".avif":
		return "image/avif"
	case ".bmp":
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}

// natLess is a natural-order string comparison ("2.jpg" < "10.jpg"), case-insensitive.
func natLess(a, b string) bool {
	ai, bi := 0, 0
	for ai < len(a) && bi < len(b) {
		if isDigit(a[ai]) && isDigit(b[bi]) {
			as := ai
			for ai < len(a) && isDigit(a[ai]) {
				ai++
			}
			bs := bi
			for bi < len(b) && isDigit(b[bi]) {
				bi++
			}
			an := strings.TrimLeft(a[as:ai], "0")
			bn := strings.TrimLeft(b[bs:bi], "0")
			if len(an) != len(bn) {
				return len(an) < len(bn)
			}
			if an != bn {
				return an < bn
			}
			continue
		}
		la, lb := lower(a[ai]), lower(b[bi])
		if la != lb {
			return la < lb
		}
		ai++
		bi++
	}
	return len(a)-ai < len(b)-bi
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }
func lower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}
