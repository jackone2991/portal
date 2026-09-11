package music

// Bulk track import from a zip (0038).
//
// The API stores the uploaded archive under an import/ prefix and enqueues
// `music:import_zip`; the worker spools it to a temp file, walks the audio
// entries, reads each one's tags with ffprobe, ingests it through mediaapi and
// creates a track. The job row carries a per-file report the client polls.
//
// Deliberately much shorter than the comic zip import it mirrors, for one
// structural reason: audio assets are marked ready inside `/complete` (no
// transcode step — see media.completeAudio), so there is nothing to wait for.
// The comic importer's poll-to-ready machinery has no counterpart here; an
// imported track is playable the moment its row exists.

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	musicapi "github.com/portal/backend/internal/modules/music/api"
)

// Import guardrails. Sized for a personal library drop — a few hundred tracks,
// a couple of gigabytes — rather than for an archive of everything. Both the
// upload and the unpack spool to disk, so memory stays flat regardless.
const (
	importMaxZipBytes = 4 << 30 // 4 GiB
	importMaxEntries  = 2000
	// Per-entry compression-ratio ceiling. Audio is already compressed, so a
	// legitimate mp3/flac entry barely shrinks; anything claiming a 50× ratio is
	// a zip bomb, not a song.
	importMaxRatio = 50
	// A single track that will not fit in memory is not a track. This bounds the
	// per-entry buffer that goes to mediaapi.Ingest.
	importMaxEntryBytes = 512 << 20 // 512 MiB
	importTaskTimeout   = 6 * time.Hour
	probeTimeout        = 20 * time.Second
)

// Extensions the importer accepts. Everything else in the zip — cover art,
// playlists, .DS_Store, nfo files — is skipped silently rather than reported as
// a failure, because "your archive contains a folder icon" is not news.
var importAudioExt = map[string]string{
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".flac": "audio/flac",
	".ogg":  "audio/ogg",
	".oga":  "audio/ogg",
	".opus": "audio/opus",
	".wav":  "audio/wav",
	".wma":  "audio/x-ms-wma",
}

// ImportReportEntry is one line of the per-file report the client polls.
type ImportReportEntry struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	TrackID string `json:"track_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Error   string `json:"error,omitempty"`
}

// CreateImport registers an empty job. The zip arrives in a second request, so
// the client has an id to attach the upload to and to poll.
func (s *Service) CreateImport(ctx context.Context, ownerID uuid.UUID) (ImportJob, error) {
	return s.repo.CreateImport(ctx, ownerID)
}

// GetImport returns a job, owner-checked.
func (s *Service) GetImport(ctx context.Context, importID, ownerID uuid.UUID) (ImportJob, error) {
	job, err := s.repo.GetImport(ctx, importID)
	if err != nil {
		return ImportJob{}, err
	}
	if job.OwnerUserID != ownerID {
		return ImportJob{}, ErrNotFound // not "forbidden": do not confirm the id exists
	}
	return job, nil
}

func (s *Service) ListImports(ctx context.Context, ownerID uuid.UUID, limit int) ([]ImportJob, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.repo.ListImports(ctx, ownerID, limit)
}

// SaveImportZip streams the upload to storage and enqueues the worker.
func (s *Service) SaveImportZip(ctx context.Context, importID, ownerID uuid.UUID, body io.Reader) (ImportJob, error) {
	if s.store == nil || s.enqueue == nil {
		return ImportJob{}, errors.New("music: import not configured")
	}
	if _, err := s.GetImport(ctx, importID, ownerID); err != nil {
		return ImportJob{}, err
	}

	// Spool to a temp file: the S3 SDK needs a seekable body with a known length
	// (a raw request stream fails), and it lets the cap be enforced before
	// anything reaches the bucket.
	tmp, err := os.CreateTemp("", "music-zip-*")
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
		return ImportJob{}, fmt.Errorf("%w: tệp zip %s vượt giới hạn %s",
			ErrValidation, humanBytes(n), humanBytes(importMaxZipBytes))
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return ImportJob{}, err
	}

	key := fmt.Sprintf("import/music/%s.zip", importID)
	if err := s.store.Put(ctx, key, tmp, "application/zip"); err != nil {
		log.Error().Err(err).Int64("bytes", n).Str("key", key).Msg("music: import store.Put failed")
		return ImportJob{}, err
	}

	updated, err := s.repo.SetImportUpload(ctx, importID, key)
	if err != nil {
		return ImportJob{}, err
	}

	// MaxRetry(0): a retry would re-create every track already imported, and the
	// report is how a partial failure is meant to be handled. Timeout keeps the
	// asynq lease alive for the whole run — the default ~30 min lease would
	// expire mid-import on a large archive and re-queue it, duplicating tracks.
	payload, _ := json.Marshal(musicapi.ImportZipPayload{
		ImportID: importID.String(),
		OwnerID:  ownerID.String(),
	})
	task := asynq.NewTask(musicapi.TaskImportZip, payload,
		asynq.Queue("default"), asynq.Timeout(importTaskTimeout), asynq.MaxRetry(0))
	if _, err := s.enqueue.Enqueue(task); err != nil {
		return ImportJob{}, err
	}
	return updated, nil
}

// RunImport is the worker body (music:import_zip). Per-entry failures land in
// the report; only a job-level problem fails the whole run.
//
// Every database touch runs inside the OWNER'S tenant scope, opened from the
// task payload. The worker serves no request, so there is no ambient tenant, and
// `music_imports` / `music_tracks` / `assets` are all RLS-fenced — an unscoped
// read there does not come back empty, it errors outright
// ("unrecognized configuration parameter app.current_tenant").
//
// The scopes are deliberately many and short rather than one long one: a
// several-hundred-track import inside a single transaction would hold it open
// for minutes, pin the snapshot, and throw away every track already created if
// the last entry failed. One scope to read the job, one per entry, one to finish.
func (s *Service) RunImport(ctx context.Context, importID, ownerID uuid.UUID) error {
	if s.store == nil || s.media == nil || s.runInTenant == nil {
		log.Error().Str("import", importID.String()).Msg("music: import worker not configured")
		return nil
	}

	var job ImportJob
	read := func() error {
		return s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
			var err error
			job, err = s.repo.GetImport(ctx, importID)
			return err
		})
	}
	if err := read(); err != nil {
		return err
	}
	// The enqueue happens after SetImportUpload commits, but a fast worker can
	// still dequeue before that commit is visible — likelier after a slow
	// multi-gigabyte upload. Wait briefly rather than fail a valid import.
	for i := 0; job.UploadRef == "" && i < 25; i++ {
		time.Sleep(200 * time.Millisecond)
		if err := read(); err != nil {
			return err
		}
	}
	if job.UploadRef == "" {
		return s.failImport(ctx, importID, ownerID, "chưa có tệp zip", "")
	}

	rc, err := s.store.Get(ctx, job.UploadRef)
	if err != nil {
		return s.failImport(ctx, importID, ownerID, "không đọc được tệp đã tải lên", job.UploadRef)
	}
	tmp, err := os.CreateTemp("", "music-import-*")
	if err != nil {
		_ = rc.Close()
		return s.failImport(ctx, importID, ownerID, "không tạo được tệp tạm", job.UploadRef)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	size, err := io.Copy(tmp, rc)
	_ = rc.Close()
	if err != nil {
		return s.failImport(ctx, importID, ownerID, "không tải được tệp zip về", job.UploadRef)
	}
	zr, err := zip.NewReader(tmp, size)
	if err != nil {
		return s.failImport(ctx, importID, ownerID, "tệp không phải zip hợp lệ", job.UploadRef)
	}

	entries := audioEntries(zr)
	if len(entries) > importMaxEntries {
		return s.failImport(ctx, importID, ownerID,
			fmt.Sprintf("quá nhiều bài (%d > %d)", len(entries), importMaxEntries), job.UploadRef)
	}
	if len(entries) == 0 {
		return s.failImport(ctx, importID, ownerID, "zip không có tệp âm thanh nào", job.UploadRef)
	}
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		return s.repo.StartImport(ctx, importID, len(entries))
	}); err != nil {
		return err
	}

	report := make([]ImportReportEntry, 0, len(entries))
	succeeded, failed := 0, 0
	for _, f := range entries {
		// One committed scope per track: a track that imported stays imported
		// even if a later entry blows up, which is the whole point of a report.
		var entry ImportReportEntry
		if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
			entry = s.importOne(ctx, ownerID, f)
			if !entry.OK {
				// Roll this entry's scope back rather than commit a half-made
				// track; the report line survives because it lives out here.
				return errEntryFailed
			}
			return nil
		}); err != nil && !errors.Is(err, errEntryFailed) {
			entry = ImportReportEntry{Name: path.Base(f.Name), Error: err.Error()}
		}
		if entry.OK {
			succeeded++
		} else {
			failed++
		}
		report = append(report, entry)
	}

	// `done` even with failures: the job finished and the report is the record of
	// what happened. `failed` is reserved for "the job could not run at all",
	// which is a different thing for the client to show.
	status := "done"
	blob, _ := json.Marshal(report)
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		return s.repo.FinishImport(ctx, importID, status, succeeded, failed, string(blob), "")
	}); err != nil {
		return err
	}
	_ = s.store.Delete(ctx, job.UploadRef)
	log.Info().Str("import", importID.String()).Int("ok", succeeded).Int("failed", failed).
		Msg("music: zip import finished")
	return nil
}

// importOne unpacks a single entry, reads its tags and creates the track.
func (s *Service) importOne(ctx context.Context, ownerID uuid.UUID, f *zip.File) ImportReportEntry {
	name := path.Base(f.Name)
	entry := ImportReportEntry{Name: name}

	if f.UncompressedSize64 > importMaxEntryBytes {
		entry.Error = "tệp quá lớn (" + humanBytes(int64(f.UncompressedSize64)) + ")"
		return entry
	}
	// Zip-bomb guard: a legitimate audio file is already compressed, so a huge
	// declared-to-stored ratio means the entry is not what it claims to be.
	if f.CompressedSize64 > 0 && f.UncompressedSize64/f.CompressedSize64 > importMaxRatio {
		entry.Error = "tỉ lệ nén bất thường"
		return entry
	}

	rc, err := f.Open()
	if err != nil {
		entry.Error = "không mở được tệp trong zip"
		return entry
	}
	data, err := io.ReadAll(io.LimitReader(rc, importMaxEntryBytes+1))
	_ = rc.Close()
	if err != nil {
		entry.Error = "không đọc được tệp trong zip"
		return entry
	}
	if int64(len(data)) > importMaxEntryBytes {
		entry.Error = "tệp quá lớn"
		return entry
	}

	meta := s.probeTags(ctx, name, data)
	entry.Title = meta.Title

	assetID, err := s.media.Ingest(ctx, ownerID, name, importAudioExt[strings.ToLower(filepath.Ext(name))], data)
	if err != nil {
		entry.Error = "không lưu được tệp âm thanh: " + err.Error()
		return entry
	}

	track, err := s.CreateTrack(ctx, CreateTrackInput{
		OwnerID:      ownerID,
		Title:        meta.Title,
		Artist:       optional(meta.Artist),
		Album:        optional(meta.Album),
		AudioAssetID: &assetID,
	})
	if err != nil {
		entry.Error = "không tạo được bài hát: " + err.Error()
		return entry
	}

	entry.OK = true
	entry.TrackID = track.ID.String()
	return entry
}

// trackMeta is what the importer manages to learn about a file.
type trackMeta struct {
	Title  string
	Artist string
	Album  string
}

// probeTags reads embedded tags with ffprobe, falling back to the filename.
//
// ffprobe rather than a Go tag library: it is already a hard dependency of this
// image (the media pipeline shells out to it for every video), it reads every
// container the importer accepts, and adding a parser per format to get the same
// answer would be work with a worse result.
//
// It needs a file on disk, so the entry is spooled — the bytes are in memory
// anyway, and the alternative (piping to stdin) makes ffprobe unable to seek,
// which is exactly what it must do to reach a trailing ID3v1 block.
func (s *Service) probeTags(ctx context.Context, name string, data []byte) trackMeta {
	meta := titleFromFilename(name)

	tmp, err := os.CreateTemp("", "music-probe-*"+filepath.Ext(name))
	if err != nil {
		return meta
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return meta
	}
	_ = tmp.Close()

	pctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	out, err := exec.CommandContext(pctx, "ffprobe",
		"-v", "error", "-show_format", "-of", "json", tmp.Name()).Output()
	if err != nil {
		// Not an import failure: a file with no readable tags still imports fine
		// under its filename, which is what most people's libraries look like.
		log.Debug().Err(err).Str("file", name).Msg("music: ffprobe tags unavailable")
		return meta
	}

	var probe struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if json.Unmarshal(out, &probe) != nil {
		return meta
	}

	tags := lowerTags(probe.Format.Tags)
	if t := firstTag(tags, "title"); t != "" {
		meta.Title = t
	}
	if a := firstTag(tags, "artist", "album_artist", "performer"); a != "" {
		meta.Artist = a
	}
	if al := firstTag(tags, "album"); al != "" {
		meta.Album = al
	}
	return meta
}

// lowerTags normalises tag keys. They vary by container and by tagger — mp3
// gives TITLE or title, mp4 gives title, Vorbis gives TITLE — so every lookup
// downstream would otherwise have to try several spellings.
func lowerTags(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[strings.ToLower(k)] = strings.TrimSpace(v)
	}
	return out
}

func firstTag(tags map[string]string, keys ...string) string {
	for _, k := range keys {
		if v := tags[k]; v != "" {
			return v
		}
	}
	return ""
}

// titleFromFilename is the fallback when a file carries no tags. It understands
// the one convention that is nearly universal in downloaded music —
// "01 - Artist - Title.mp3" and its shorter forms — and otherwise hands back the
// bare stem rather than guessing.
func titleFromFilename(name string) trackMeta {
	stem := strings.TrimSpace(strings.TrimSuffix(name, filepath.Ext(name)))
	stem = stripTrackNumber(stem)

	parts := strings.Split(stem, " - ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}

	switch len(parts) {
	case 0:
		return trackMeta{Title: stem}
	case 1:
		return trackMeta{Title: parts[0]}
	default:
		// Only the FIRST separator splits artist from title: a dash inside the
		// title is common ("Paranoid - Android"), and splitting on every one of
		// them would truncate those songs.
		return trackMeta{Artist: parts[0], Title: strings.Join(parts[1:], " - ")}
	}
}

// stripTrackNumber removes a leading position marker ("01 - ", "1. ", "003_")
// from a filename stem. It is an ordering hint, not part of anything's name.
//
// The number must be followed by a real separator — '.', '-' or '_' — never by a
// bare space. "01 Creep" and "99 Luftballons" are indistinguishable by shape, and
// mangling a real title ("Luftballons") is a worse failure than leaving a track
// number in one ("01 Creep"), which the embedded tags usually fix anyway.
func stripTrackNumber(stem string) string {
	i := 0
	for i < len(stem) && i < 3 && stem[i] >= '0' && stem[i] <= '9' {
		i++
	}
	if i == 0 {
		return stem
	}
	rest := strings.TrimLeft(stem[i:], " ")
	if rest == "" || (rest[0] != '.' && rest[0] != '-' && rest[0] != '_') {
		return stem
	}
	trimmed := strings.TrimSpace(rest[1:])
	if trimmed == "" {
		return stem // the number WAS the name; keep it rather than return nothing
	}
	return trimmed
}

// audioEntries picks the importable files out of the archive, in a stable order.
//
// Sorted by full path so a zip laid out as album folders imports album by album,
// and so two runs of the same archive produce the same order — a report the user
// can compare against a previous one is worth more than raw zip order.
func audioEntries(zr *zip.Reader) []*zip.File {
	out := make([]*zip.File, 0, len(zr.File))
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		// The zip spec says entry names use a forward slash, but Windows tools
		// (PowerShell's Compress-Archive among them) write backslashes.
		// Normalising first is what makes
		// the path checks below mean the same thing for every archive — without
		// it, a Windows-made zip smuggles its __MACOSX stubs straight past them.
		name := strings.ReplaceAll(f.Name, "\\", "/")
		base := path.Base(name)
		// macOS archives carry a __MACOSX shadow tree of AppleDouble stubs that
		// look like real entries and are not.
		if strings.HasPrefix(base, "._") || strings.HasPrefix(name, "__MACOSX/") ||
			strings.Contains(name, "/__MACOSX/") {
			continue
		}
		if _, ok := importAudioExt[strings.ToLower(path.Ext(base))]; ok {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// errEntryFailed rolls one entry's tenant scope back without being reported as a
// job-level error — that failure is already captured in the report line.
var errEntryFailed = errors.New("music: import entry failed")

func (s *Service) failImport(ctx context.Context, importID, ownerID uuid.UUID, msg, uploadRef string) error {
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		return s.repo.FinishImport(ctx, importID, "failed", 0, 0, "[]", msg)
	}); err != nil {
		log.Error().Err(err).Str("import", importID.String()).Msg("music: could not record import failure")
	}
	if uploadRef != "" {
		_ = s.store.Delete(ctx, uploadRef)
	}
	// Returning nil: the job is recorded as failed and a retry would only fail
	// the same way. Asynq's archive is not where a user-facing error belongs.
	log.Warn().Str("import", importID.String()).Str("reason", msg).Msg("music: zip import failed")
	return nil
}

func optional(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
}
