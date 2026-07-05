package media

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/media/worker"
	"github.com/portal/backend/internal/platform/storage"
)

// Service holds the media business logic. Construct via the module.
type Service struct {
	store     storage.Storage
	repo      Repository
	enqueue   Enqueuer
	baseURL   string // public API base, e.g. https://api.portal.localhost
	uploadTTL time.Duration
}

// UploadSession is what the client needs to PUT the original directly to storage.
type UploadSession struct {
	Asset   Asset
	URL     string
	Method  string
	Headers map[string]string
}

// CreateUploadSession registers an asset (status=uploading) and returns a
// presigned PUT the browser uses to send the original straight to the bucket.
func (s *Service) CreateUploadSession(ctx context.Context, ownerID uuid.UUID, filename, contentType string, size int64) (*UploadSession, error) {
	id := uuid.New()
	ext := pickExt(filename, contentType)
	sourceKey := fmt.Sprintf("uploads/%s/original%s", id, ext)

	asset, err := s.repo.CreateAsset(ctx, CreateAssetInput{
		ID:        id,
		OwnerID:   ownerID,
		Kind:      "video",
		SourceKey: sourceKey,
		MimeType:  contentType,
		SizeBytes: size,
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
	if contentType == "" {
		contentType = asset.MimeType
	}
	return s.store.Put(ctx, asset.SourceKey, tmp, contentType)
}

// CompleteUpload confirms the original landed and enqueues transcoding.
func (s *Service) CompleteUpload(ctx context.Context, ownerID, assetID uuid.UUID) error {
	asset, err := s.owned(ctx, ownerID, assetID)
	if err != nil {
		return err
	}

	if ok, err := s.store.Exists(ctx, asset.SourceKey); err != nil {
		return err
	} else if !ok {
		return fmt.Errorf("%w: upload not found", ErrNotReady)
	}

	if err := s.repo.MarkProcessing(ctx, assetID); err != nil {
		return err
	}

	outputKey := fmt.Sprintf("hls/%s", assetID)
	task, err := worker.NewTranscodeTask(worker.TranscodePayload{
		AssetID:   assetID.String(),
		SourceKey: asset.SourceKey,
		OutputKey: outputKey,
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

func (s *Service) List(ctx context.Context, ownerID uuid.UUID) ([]Asset, error) {
	return s.repo.ListByOwner(ctx, ownerID, 50, 0)
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
	if a.Status != StatusReady {
		return ""
	}
	return fmt.Sprintf("%s/api/v1/assets/%s/hls/index.m3u8", strings.TrimRight(s.baseURL, "/"), a.ID)
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
