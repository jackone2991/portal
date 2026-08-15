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
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	comicapi "github.com/portal/backend/internal/modules/comic/api"
	mediaapi "github.com/portal/backend/internal/modules/media/api"
)

// Import guardrails (P1.7). Sized for whole-comic archives (folder-per-chapter):
// a 100-chapter comic is ~10k images / ~2 GB. Image variants transcode in
// parallel on the "image" queue (IMAGE_CONCURRENCY), so the poll budget is per
// image amortized across that pool, capped under the task's 12h lease.
const (
	importMaxZipBytes  = 3 << 30 // 3 GiB
	importMaxEntries   = 20000
	importMaxRatio     = 100 // per-entry compression ratio (zip-bomb guard)
	importPollInterval = 2 * time.Second
	importPollFloor    = 2 * time.Minute
	importPollPerImage = 5 * time.Second  // amortized budget per image (parallel image pool)
	importPollCeiling  = 11 * time.Hour   // hard cap, safely under the 12h task Timeout
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
		return ImportJob{}, fmt.Errorf("%w: zip exceeds 500MB", ErrValidation)
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
	// A whole-comic import can run for tens of minutes; without a generous Timeout
	// asynq's ~30-min task lease expires mid-run, cancels the context, and re-queues
	// the task → it re-creates every chapter (duplicates). Timeout keeps the lease
	// alive; MaxRetry(0) means a genuine failure aborts instead of re-running.
	payload, _ := json.Marshal(comicapi.ImportZipPayload{ImportID: importID.String()})
	task := asynq.NewTask(comicapi.TaskImportZip, payload, asynq.Queue("default"), asynq.Timeout(12*time.Hour), asynq.MaxRetry(0))
	if _, err := s.enqueue.Enqueue(task); err != nil {
		return ImportJob{}, err
	}
	return updated, nil
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

	_ = s.repo.StartImport(ctx, importID, len(entries))
	baseChapters := 0
	if job.ChapterID == nil {
		if n, err := s.repo.CountChapters(ctx, job.ComicID); err == nil {
			baseChapters = n
		}
	}

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
		var ch Chapter
		cerr := s.runInTenant(ctx, owner, func(ctx context.Context) error {
			var e error
			ch, e = s.repo.CreateChapter(ctx, job.ComicID, g.title, (baseChapters+gi+1)*10)
			return e
		})
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
		_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, report)
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
	_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, report)

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
	_ = s.repo.UpdateImportProgress(ctx, importID, succ, fail, report)

	if err := s.repo.FinishImport(ctx, importID, ImportDone, succ, fail, report, nil); err != nil {
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
