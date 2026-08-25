package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	"github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/platform/server"
	"github.com/portal/backend/internal/platform/storage"
)

// EventAssetDeleted is emitted once an asset row is gone (see events.md).
const EventAssetDeleted = "media:asset_deleted"

// Ingest guardrails (SPEC-01 P0.1).
const (
	maxUploadBytes   = 50 << 20 // 50 MiB image cap
	sniffBytes       = 512      // header bytes read for magic-byte detection
	defaultListLimit = 50
	maxListLimit     = 100
	purgeStuckAfter  = 5 // consecutive failed purges before an error-level log
)

// Service holds the media business logic. Construct via the module.
type Service struct {
	store     storage.Storage
	repo      Repository
	enqueue   Enqueuer
	events    EventPublisher // optional: media:asset_deleted
	baseURL   string         // public API base, e.g. https://api.portal.localhost
	uploadTTL time.Duration

	mu         sync.Mutex        // guards purgeFails
	purgeFails map[uuid.UUID]int // per-asset consecutive purge-failure counter
}

// UploadSession is what the client needs to PUT the original directly to storage.
type UploadSession struct {
	Asset   Asset
	URL     string
	Method  string
	Headers map[string]string
}

// ListOptions are the library query filters (P0.4).
type ListOptions struct {
	Kind   string // "" / "all" = any
	Status string // "" / "all" = any; "processing" also matches "uploading"
	Cursor string // opaque base64 keyset cursor
	Limit  int
}

// ListResult is a keyset page plus the cursor for the next one ("" = last page).
type ListResult struct {
	Assets     []Asset
	NextCursor string
}

// CreateUploadSession registers an asset (status=uploading) and returns a
// presigned PUT the browser uses to send the original straight to the bucket.
func (s *Service) CreateUploadSession(ctx context.Context, ownerID uuid.UUID, filename, contentType string, size int64) (*UploadSession, error) {
	id := uuid.New()
	ext := pickExt(filename, contentType)
	sourceKey := fmt.Sprintf("uploads/%s/original%s", id, ext)

	kind := "video"
	if strings.HasPrefix(contentType, "image/") {
		kind = "image"
	}

	asset, err := s.repo.CreateAsset(ctx, CreateAssetInput{
		ID:               id,
		OwnerID:          ownerID,
		Kind:             kind,
		SourceKey:        sourceKey,
		MimeType:         contentType,
		SizeBytes:        size,
		Title:            filename,
		OriginalFilename: filename,
		Origin:           "upload",
	})
	if err != nil {
		return nil, err
	}

	pre, err := s.store.PresignPut(ctx, sourceKey, contentType, s.uploadTTL)
	if err != nil {
		return nil, err
	}
	return &UploadSession{Asset: asset, URL: pre.URL, Method: pre.Method, Headers: pre.Headers}, nil
}

// IngestImage ingests raw image bytes as a media asset, running the same pipeline
// as a browser upload (create → store → complete, which enqueues process_image).
// Returns the new asset id. Callers (e.g. the comic zip-import worker) run this
// inside a tenant-scoped ctx so the asset + its variants land in the right tenant.
func (s *Service) IngestImage(ctx context.Context, ownerID uuid.UUID, filename, contentType string, data []byte) (uuid.UUID, error) {
	sess, err := s.CreateUploadSession(ctx, ownerID, filename, contentType, int64(len(data)))
	if err != nil {
		return uuid.Nil, err
	}
	if err := s.UploadSource(ctx, ownerID, sess.Asset.ID, bytes.NewReader(data), contentType); err != nil {
		return uuid.Nil, err
	}
	if err := s.CompleteUpload(ctx, ownerID, sess.Asset.ID); err != nil {
		return uuid.Nil, err
	}
	return sess.Asset.ID, nil
}

// UploadSource streams the original through the API to storage. This is the
// dev-friendly path (the browser only talks to the API, not the bucket directly);
// the presigned-PUT session is the direct browser→bucket path for prod/R2.
// The body is spooled to a temp file so the S3 SDK gets a seekable stream.
func (s *Service) UploadSource(ctx context.Context, ownerID, assetID uuid.UUID, body io.Reader, contentType string) error {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return err
	}
	if asset.Status != StatusUploading {
		return fmt.Errorf("%w: asset already submitted", ErrNotReady)
	}

	if contentType == "" {
		contentType = asset.MimeType
	}
	// Already seekable (e.g. the zip-import's bytes.Reader) → the S3 SDK can length
	// + hash it in place; skip the temp-file spool (avoids per-image disk churn).
	if rs, ok := body.(io.ReadSeeker); ok {
		if _, err := rs.Seek(0, io.SeekStart); err != nil {
			return err
		}
		return s.store.Put(ctx, asset.SourceKey, rs, contentType)
	}

	tmp, err := os.CreateTemp("", "upload-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	if _, err := io.Copy(tmp, body); err != nil {
		return fmt.Errorf("buffer upload: %w", err)
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}
	return s.store.Put(ctx, asset.SourceKey, tmp, contentType)
}

// CompleteUpload confirms the original landed and dispatches processing. For
// images it sniffs magic bytes (never the extension/declared type) + re-checks
// size, rejecting HEIC/HEIF and oversized/unknown bodies by deleting the object
// and marking the asset failed (SPEC-01 P0.1). Videos keep the transcode path.
func (s *Service) CompleteUpload(ctx context.Context, ownerID, assetID uuid.UUID) error {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return err
	}

	size, err := s.store.Size(ctx, asset.SourceKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("%w: upload not found", ErrNotReady)
		}
		return err
	}

	if asset.Kind == "image" {
		return s.completeImage(ctx, asset, size)
	}
	return s.completeVideo(ctx, asset)
}

func (s *Service) completeImage(ctx context.Context, asset Asset, size int64) error {
	// Belt-and-braces size cap (HEAD re-check): the presign policy should already
	// bound this, but a bypassed/unsupported policy is caught here.
	if size > maxUploadBytes {
		_ = s.store.Delete(ctx, asset.SourceKey)
		_ = s.repo.MarkFailed(ctx, asset.ID, "file too large: exceeds the 50 MB limit")
		return ErrFileTooLarge
	}

	head, err := s.readHead(ctx, asset.SourceKey, sniffBytes)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return fmt.Errorf("%w: upload not found", ErrNotReady)
		}
		return err
	}
	if _, ok := sniffImageType(head); !ok {
		heic := isHEIC(head)
		detail := "unsupported image format — only JPEG, PNG and WebP are accepted"
		if heic {
			detail = "HEIC/HEIF images are not supported — export or convert the photo to JPEG on your device and upload again"
		}
		_ = s.store.Delete(ctx, asset.SourceKey)
		_ = s.repo.MarkFailed(ctx, asset.ID, detail)
		return &UnsupportedFormatError{Detail: detail, HEIC: heic}
	}

	if err := s.repo.MarkProcessing(ctx, asset.ID); err != nil {
		return err
	}
	task, err := worker.NewProcessImageTask(worker.ProcessImagePayload{
		AssetID:     asset.ID.String(),
		SourceKey:   asset.SourceKey,
		OwnerUserID: asset.OwnerID.String(),
	})
	if err != nil {
		return err
	}
	_, err = s.enqueue.Enqueue(task)
	return err
}

func (s *Service) completeVideo(ctx context.Context, asset Asset) error {
	if err := s.repo.MarkProcessing(ctx, asset.ID); err != nil {
		return err
	}
	outputKey := fmt.Sprintf("hls/%s", asset.ID)
	task, err := worker.NewTranscodeTask(worker.TranscodePayload{
		AssetID:     asset.ID.String(),
		SourceKey:   asset.SourceKey,
		OutputKey:   outputKey,
		OwnerUserID: asset.OwnerID.String(),
	})
	if err != nil {
		return err
	}
	_, err = s.enqueue.Enqueue(task)
	return err
}

// Get returns the asset (owner-scoped) plus a playback URL when ready.
func (s *Service) Get(ctx context.Context, ownerID, assetID uuid.UUID) (Asset, string, error) {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return Asset{}, "", err
	}
	return asset, s.hlsURL(asset), nil
}

// List returns a keyset page of the caller's assets (P0.4).
func (s *Service) List(ctx context.Context, ownerID uuid.UUID, opts ListOptions) (ListResult, error) {
	limit := opts.Limit
	if limit <= 0 || limit > maxListLimit {
		limit = defaultListLimit
	}

	kind := opts.Kind
	if kind == "all" {
		kind = ""
	}

	in := ListCursorInput{
		OwnerID:  ownerID,
		Kind:     kind,
		Statuses: expandStatuses(opts.Status),
		Limit:    limit + 1, // fetch one extra to detect a following page
	}
	if opts.Cursor != "" {
		at, cid, err := decodeCursor(opts.Cursor)
		if err != nil {
			return ListResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, cid
	}

	rows, err := s.repo.ListByOwnerCursor(ctx, in)
	if err != nil {
		return ListResult{}, err
	}

	var res ListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Assets = rows
	return res, nil
}

// DeleteAsset soft-deletes then purges an asset (P0.3). Authorization is the
// caller's responsibility (RequireOwnerOrPermission at the route). Idempotent:
// a missing asset returns ErrNotFound (never 500).
func (s *Service) DeleteAsset(ctx context.Context, id uuid.UUID) error {
	asset, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return err // ErrNotFound → idempotent 404
	}
	if asset.Status != StatusDeleting {
		if err := s.repo.SetStatusDeleting(ctx, id); err != nil {
			return err
		}
	}
	return s.purgeAsset(ctx, asset)
}

// DownloadOriginal streams the byte-identical uploaded original for the owner
// (P0.5). Returns the reader, its content type and the download filename.
func (s *Service) DownloadOriginal(ctx context.Context, ownerID, id uuid.UUID) (io.ReadCloser, string, string, error) {
	asset, err := s.owned(ctx, ownerID, id)
	if err != nil {
		return nil, "", "", err
	}
	if asset.Status == StatusDeleting {
		return nil, "", "", ErrNotFound
	}
	if asset.Status == StatusUploading {
		return nil, "", "", ErrNotReady // source object may be partial
	}

	rc, err := s.store.Get(ctx, asset.SourceKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, "", "", ErrNotFound
		}
		return nil, "", "", err
	}

	filename := asset.OriginalFilename
	if filename == "" {
		ext := path.Ext(asset.SourceKey)
		if ext == "" {
			if exts, _ := mime.ExtensionsByType(asset.MimeType); len(exts) > 0 {
				ext = exts[0]
			}
		}
		filename = asset.ID.String() + ext
	}
	ct := asset.MimeType
	if ct == "" {
		ct = "application/octet-stream"
	}
	return rc, ct, filename, nil
}

// ServeVariant streams a derived variant object (WebP). Public/unauthenticated,
// like /hls/* — variants carry no EXIF/GPS (all metadata stripped). Missing or
// deleting assets, and unknown variant names, resolve to ErrNotFound.
func (s *Service) ServeVariant(ctx context.Context, id uuid.UUID, variant string) (io.ReadCloser, string, error) {
	switch variant {
	case "thumb", "medium", "poster":
	default:
		return nil, "", ErrNotFound
	}

	asset, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if asset.Status == StatusDeleting {
		return nil, "", ErrNotFound
	}

	v, err := s.repo.GetVariant(ctx, id, variant)
	if err != nil {
		return nil, "", err // ErrNotFound if absent
	}
	rc, err := s.store.Get(ctx, v.StorageKey)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	return rc, "image/webp", nil
}

// PurgeOrphans is the hourly janitor body (P0.3): re-purge tombstoned assets past
// the 15-min grace window, and mark abandoned (>24h) upload sessions failed.
// Idempotent per asset.
func (s *Service) PurgeOrphans(ctx context.Context) error {
	deleting, err := s.repo.ListForPurge(ctx)
	if err != nil {
		return err
	}
	for _, a := range deleting {
		err := s.purgeAsset(ctx, a)
		s.recordPurge(a.ID, err)
		if err != nil {
			log.Warn().Err(err).Str("asset", a.ID.String()).Msg("purge_orphans: asset purge failed")
		}
	}

	abandoned, err := s.repo.ListAbandonedUploads(ctx)
	if err != nil {
		return err
	}
	for _, a := range abandoned {
		if err := s.repo.MarkFailed(ctx, a.ID, "upload abandoned"); err != nil {
			log.Warn().Err(err).Str("asset", a.ID.String()).Msg("purge_orphans: mark abandoned failed")
		}
	}
	return nil
}

// AssetOwner returns an asset's owner id — the RequireOwnerOrPermission extractor
// (P0.3). ErrNotFound when the asset does not exist.
func (s *Service) AssetOwner(ctx context.Context, id uuid.UUID) (uuid.UUID, error) {
	return s.repo.GetAssetOwner(ctx, id)
}

// HLSObject streams a file (manifest or segment) from an asset's HLS output.
// Public (playback is unauthenticated for v1); path is sanitised against traversal.
func (s *Service) HLSObject(ctx context.Context, assetID uuid.UUID, sub string) (io.ReadCloser, string, error) {
	asset, err := s.repo.GetAsset(ctx, assetID)
	if err != nil {
		return nil, "", err
	}
	if asset.Status != StatusReady || asset.OutputPrefix == "" {
		return nil, "", ErrNotReady
	}
	clean := path.Clean("/" + sub) // collapse .. and leading slashes
	if clean == "/" || strings.Contains(clean, "..") {
		return nil, "", ErrNotFound
	}
	key := asset.OutputPrefix + clean // clean starts with "/"
	rc, err := s.store.Get(ctx, key)
	if err != nil {
		return nil, "", err
	}
	return rc, contentTypeFor(sub), nil
}

// ── purge internals ─────────────────────────────────────────────────

// purgeAsset removes every storage object for the asset, then its variant + asset
// rows, then emits media:asset_deleted (best-effort).
func (s *Service) purgeAsset(ctx context.Context, asset Asset) error {
	if err := s.purgeObjects(ctx, asset); err != nil {
		return err
	}
	if err := s.repo.DeleteVariants(ctx, asset.ID); err != nil {
		return err
	}
	if err := s.repo.DeleteAsset(ctx, asset.ID); err != nil {
		return err
	}
	s.emitDeleted(ctx, asset.ID, asset.OwnerID)
	return nil
}

// purgeObjects deletes the three per-asset storage subtrees: the uploaded
// original, the HLS output, and the derived variants. DeletePrefix on an empty
// subtree is a no-op, so this is idempotent and covers stragglers (partial
// uploads, orphaned poster objects).
func (s *Service) purgeObjects(ctx context.Context, asset Asset) error {
	if err := s.store.DeletePrefix(ctx, fmt.Sprintf("uploads/%s/", asset.ID)); err != nil {
		return err
	}
	if asset.OutputPrefix != "" {
		if err := s.store.DeletePrefix(ctx, strings.TrimRight(asset.OutputPrefix, "/")+"/"); err != nil {
			return err
		}
	}
	if err := s.store.DeletePrefix(ctx, fmt.Sprintf("variants/%s/", asset.ID)); err != nil {
		return err
	}
	return nil
}

func (s *Service) recordPurge(id uuid.UUID, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.purgeFails == nil {
		s.purgeFails = make(map[uuid.UUID]int)
	}
	if err == nil {
		delete(s.purgeFails, id)
		return
	}
	s.purgeFails[id]++
	if s.purgeFails[id] >= purgeStuckAfter {
		log.Error().Err(err).Str("asset", id.String()).Int("attempts", s.purgeFails[id]).
			Msg("purge_orphans: asset purge is stuck")
	}
}

func (s *Service) emitDeleted(ctx context.Context, assetID, ownerID uuid.UUID) {
	if s.events == nil {
		return
	}
	if err := s.events.Publish(ctx, EventAssetDeleted, map[string]any{
		"asset_id":      assetID.String(),
		"owner_user_id": ownerID.String(),
	}); err != nil {
		log.Warn().Err(err).Str("asset", assetID.String()).Msg("asset_deleted: publish failed")
	}
}

// ContinueItems fetches active media progress items for the caller (P0.3).
func (s *Service) ContinueItems(ctx context.Context, userID uuid.UUID, limit int) ([]mediaapi.ContinueItem, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	return s.repo.GetContinueItems(ctx, userID, limit)
}

// LookupAsset returns the cross-module projection of an asset by id (NOT
// owner-scoped — the caller enforces ownership). (nil, nil) when the asset does
// not exist, so a domain vertical validating a page/cover reference (SPEC-02
// P0.1) gets a clean not-found rather than an error.
func (s *Service) LookupAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error) {
	a, err := s.repo.GetAsset(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &mediaapi.Asset{
		ID:         a.ID,
		OwnerID:    a.OwnerID,
		Kind:       mediaapi.AssetKind(a.Kind),
		Status:     mediaapi.AssetStatus(a.Status),
		MimeType:   a.MimeType,
		SizeBytes:  a.SizeBytes,
		DurationMs: a.DurationMs,
		Width:      a.Width,
		Height:     a.Height,
		CreatedAt:  a.CreatedAt,
	}, nil
}

// AssetStatuses returns id→status for the given assets in one query. The comic
// zip-import worker polls thousands of assets to ready; this collapses a
// per-asset round-trip into a single query per poll round. Run inside a
// tenant-scoped ctx (same as LookupAsset).
func (s *Service) AssetStatuses(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]mediaapi.AssetStatus, error) {
	raw, err := s.repo.GetAssetStatuses(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]mediaapi.AssetStatus, len(raw))
	for id, st := range raw {
		out[id] = mediaapi.AssetStatus(st)
	}
	return out, nil
}

// PutProgress saves the playback progress for a video asset (P0.2).
// It clamps position_ms to duration_ms if duration exists.
func (s *Service) PutProgress(ctx context.Context, ownerID, assetID uuid.UUID, positionMs int64) error {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return err
	}
	if asset.Kind != "video" {
		return ErrNotPlayable
	}
	if asset.Status == StatusDeleting {
		return ErrNotFound
	}
	if asset.DurationMs != nil {
		dur := int64(*asset.DurationMs)
		if positionMs > dur {
			positionMs = dur
		}
	} else if positionMs < 0 {
		positionMs = 0
	}

	var completedAt *time.Time
	if asset.DurationMs != nil && *asset.DurationMs > 0 {
		pct := (float64(positionMs) / float64(*asset.DurationMs)) * 100.0
		if pct >= 95.0 {
			_, prevCompletedAt, _, err := s.repo.GetPlaybackProgress(ctx, ownerID, assetID)
			if err != nil && !errors.Is(err, ErrNotFound) {
				return err
			}
			if prevCompletedAt == nil {
				now := time.Now()
				completedAt = &now
				title := asset.Title
				if title == "" {
					title = asset.OriginalFilename
				}
				if s.events != nil {
					_ = s.events.Publish(ctx, "media:playback_completed", map[string]any{
						"asset_id": assetID.String(),
						"user_id":  ownerID.String(),
						"title":    title,
					})
				}
			}
		}
	}

	return s.repo.UpsertPlaybackProgress(ctx, ownerID, assetID, positionMs, completedAt)
}

// GetProgress fetches the playback progress for a video asset (P0.2).
func (s *Service) GetProgress(ctx context.Context, ownerID, assetID uuid.UUID) (int64, *int, *time.Time, time.Time, error) {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return 0, nil, nil, time.Time{}, err
	}
	if asset.Kind != "video" || asset.Status == StatusDeleting {
		return 0, nil, nil, time.Time{}, ErrNotPlayable
	}
	pos, completedAt, updatedAt, err := s.repo.GetPlaybackProgress(ctx, ownerID, assetID)
	if err != nil {
		return 0, nil, nil, time.Time{}, err
	}
	var pct *int
	if asset.DurationMs != nil && *asset.DurationMs > 0 {
		p := int((float64(pos) / float64(*asset.DurationMs)) * 100.0)
		pct = &p
	}
	return pos, pct, completedAt, updatedAt, nil
}

// ── helpers ─────────────────────────────────────────────────────────

func (s *Service) owned(ctx context.Context, ownerID, assetID uuid.UUID) (Asset, error) {
	asset, err := s.repo.GetAsset(ctx, assetID)
	if err != nil {
		return Asset{}, err
	}
	if asset.OwnerID != ownerID {
		return Asset{}, ErrForbidden
	}
	return asset, nil
}

func (s *Service) hlsURL(a Asset) string {
	if a.Kind != "video" || a.Status != StatusReady || a.OutputPrefix == "" {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/assets/%s/hls/index.m3u8", strings.TrimRight(s.baseURL, "/"), a.ID)
}

func (s *Service) readHead(ctx context.Context, key string, n int64) ([]byte, error) {
	rc, err := s.store.GetRange(ctx, key, n)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, n))
}

// expandStatuses maps a library status filter to a DB status set. "processing"
// also matches still-'uploading' sessions so a freshly created asset is never
// invisible under every filter (rev 3). "" / "all" → nil (no filter).
func expandStatuses(status string) []string {
	switch status {
	case "", "all":
		return nil
	case "processing":
		return []string{"processing", "uploading"}
	default:
		return []string{status}
	}
}

// encodeCursor / decodeCursor form the opaque keyset cursor "<created_at>|<id>".
func encodeCursor(a Asset) string {
	return server.EncodeCursor(a.CreatedAt.UTC().Format(time.RFC3339Nano), a.ID)
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	key, id, err := server.DecodeCursor(s)
	if err != nil {
		return time.Time{}, uuid.Nil, err
	}
	at, err := time.Parse(time.RFC3339Nano, key)
	if err != nil {
		return time.Time{}, uuid.Nil, server.ErrBadCursor
	}
	return at, id, nil
}

// sniffImageType decides the content type from magic bytes alone. ok=false for
// any unrecognised body (including HEIC/HEIF — use isHEIC for the helpful hint).
func sniffImageType(b []byte) (string, bool) {
	switch {
	case len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF:
		return "image/jpeg", true
	case len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}):
		return "image/png", true
	case len(b) >= 12 && bytes.Equal(b[:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "image/webp", true
	}
	return "", false
}

// isHEIC recognises the ISO-BMFF ftyp box of HEIC/HEIF containers, checking both
// the major brand and the compatible-brands list (major "mif1" + compatible
// "heic" is common). Lets CompleteUpload return the convert-to-JPEG hint.
func isHEIC(b []byte) bool {
	if len(b) < 12 || !bytes.Equal(b[4:8], []byte("ftyp")) {
		return false
	}
	heicBrands := map[string]bool{
		"heic": true, "heix": true, "heif": true, "heim": true, "heis": true,
		"hevc": true, "hevx": true, "hevm": true, "hevs": true,
		"mif1": true, "msf1": true,
	}
	if heicBrands[string(b[8:12])] {
		return true
	}
	// compatible brands follow the major brand + minor version, from offset 16
	for i := 16; i+4 <= len(b); i += 4 {
		if heicBrands[string(b[i:i+4])] {
			return true
		}
	}
	return false
}

func pickExt(filename, contentType string) string {
	if e := path.Ext(filename); e != "" && len(e) <= 5 {
		return strings.ToLower(e)
	}
	if exts, _ := mime.ExtensionsByType(contentType); len(exts) > 0 {
		return exts[0]
	}
	return ".bin"
}

func contentTypeFor(name string) string {
	switch strings.ToLower(path.Ext(name)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}
