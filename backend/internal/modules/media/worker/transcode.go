package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/platform/storage"
)

const TaskTypeTranscode = "media:transcode"

// TranscodePayload is enqueued after a media upload completes. The worker pulls
// the original from storage, runs FFmpeg to produce a VOD HLS rendition, uploads
// the manifest + segments under OutputKey, and marks the asset ready.
type TranscodePayload struct {
	AssetID   string   `json:"asset_id"`
	SourceKey string   `json:"source_key"` // storage key of the uploaded original
	OutputKey string   `json:"output_key"` // storage prefix for the HLS output
	Variants  []string `json:"variants,omitempty"`
	// OwnerUserID lets the worker open the asset owner's tenant scope before it
	// writes (ADR-07) — see worker.inTenant.
	OwnerUserID string `json:"owner_user_id"`
}

// NewTranscodeTask enqueues onto the "heavy" queue so a second, low-concurrency
// asynq.Server serializes the expensive decode alongside media:process_image
// (SPEC-01 P0.1 OOM guard — queue weights alone cannot cap parallelism).
func NewTranscodeTask(p TranscodePayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeTranscode, body, asynq.Queue("heavy")), nil
}

// AssetMeta is the minimal projection a worker needs to build a media:asset_ready
// event payload. Primitive fields keep this package free of a media import.
type AssetMeta struct {
	OwnerID uuid.UUID
	Kind    string
	Title   string
	Origin  string
}

// Repo is the minimal persistence surface the media workers need. The media
// repository adapter satisfies it (primitive signatures keep this package free
// of a media-package import → no cycle).
type Repo interface {
	MarkReady(ctx context.Context, id uuid.UUID, outputPrefix string, durationMs, width, height *int) error
	MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
	// MarkImageReady sets status=ready + dimensions with no HLS output_prefix.
	MarkImageReady(ctx context.Context, id uuid.UUID, width, height int) error
	// InsertVariant upserts a derived variant row (thumb/medium/poster).
	InsertVariant(ctx context.Context, assetID uuid.UUID, variant, storageKey string, width, height int, sizeBytes int64) error
	// LoadAssetMeta fetches the fields the media:asset_ready payload carries.
	LoadAssetMeta(ctx context.Context, id uuid.UUID) (AssetMeta, error)
}

// Enqueuer schedules a follow-up task (e.g. poster generation). *asynq.Client
// satisfies it. Kept minimal so the worker package never imports the media one.
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Publisher fans a domain event out to its subscribers (platform/events).
type Publisher interface {
	Publish(ctx context.Context, name string, payload any) error
}

// EventAssetReady is emitted whenever any asset reaches ready (SPEC-01 P1.2).
const EventAssetReady = "media:asset_ready"

// emitAssetReady publishes media:asset_ready best-effort (nil publisher / a
// publish error is logged, never fatal — there is no consumer required for v1).
func emitAssetReady(ctx context.Context, pub Publisher, repo Repo, id uuid.UUID) {
	if pub == nil {
		return
	}
	meta, err := repo.LoadAssetMeta(ctx, id)
	if err != nil {
		log.Warn().Err(err).Str("asset", id.String()).Msg("asset_ready: load meta failed")
		return
	}
	if err := pub.Publish(ctx, EventAssetReady, map[string]any{
		"asset_id":      id.String(),
		"kind":          meta.Kind,
		"owner_user_id": meta.OwnerID.String(),
		"title":         meta.Title, // consumers fall back to original_filename via mediaapi
		"origin":        meta.Origin,
	}); err != nil {
		log.Warn().Err(err).Str("asset", id.String()).Msg("asset_ready: publish failed")
	}
}

// Transcoder runs the FFmpeg HLS pipeline. Construct with NewTranscoder.
type Transcoder struct {
	store   storage.Storage
	repo    Repo
	enqueue Enqueuer    // optional: enqueues the poster task after ready
	pub     Publisher   // optional: emits media:asset_ready
	run_    RunInTenant // optional: nil on the api side, supplied by cmd/worker
}

func NewTranscoder(store storage.Storage, repo Repo, enqueue Enqueuer, pub Publisher, run RunInTenant) *Transcoder {
	return &Transcoder{store: store, repo: repo, enqueue: enqueue, pub: pub, run_: run}
}

// Handle is the Asynq handler. On failure it marks the asset failed and returns
// nil (a bad source file should not retry forever); genuine infra blips are the
// acceptable cost of that simplicity for v1.
func (t *Transcoder) Handle(ctx context.Context, task *asynq.Task) error {
	var p TranscodePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	id, err := uuid.Parse(p.AssetID)
	if err != nil {
		return err
	}

	log.Info().Str("asset", p.AssetID).Str("src", p.SourceKey).Msg("transcode: start")
	if err := t.run(ctx, id, p); err != nil {
		log.Error().Err(err).Str("asset", p.AssetID).Msg("transcode: failed")
		_ = t.repo.MarkFailed(ctx, id, truncate(err.Error(), 500))
		return nil
	}
	log.Info().Str("asset", p.AssetID).Msg("transcode: ready")
	return nil
}

func (t *Transcoder) run(ctx context.Context, id uuid.UUID, p TranscodePayload) error {
	dir, err := os.MkdirTemp("", "transcode-*")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(dir)

	// 1. download the original
	src := filepath.Join(dir, "source")
	if err := t.download(ctx, p.SourceKey, src); err != nil {
		return err
	}

	// 2. probe dimensions + duration
	durMs, width, height := probe(ctx, src)

	// 3. FFmpeg → single-rendition VOD HLS (browser-friendly h264/aac)
	out := filepath.Join(dir, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	//nolint:gosec // fixed args, paths are server-controlled temp files
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", "-i", src,
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "23", "-pix_fmt", "yuv420p",
		"-c:a", "aac", "-b:a", "128k", "-ac", "2",
		"-hls_time", "6", "-hls_playlist_type", "vod",
		"-hls_segment_filename", filepath.Join(out, "seg_%03d.ts"),
		filepath.Join(out, "index.m3u8"),
	)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, tail(combined, 400))
	}

	// 4. upload every output file under OutputKey/
	entries, err := os.ReadDir(out)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if err := t.upload(ctx, filepath.Join(out, e.Name()), p.OutputKey+"/"+e.Name()); err != nil {
			return fmt.Errorf("upload %s: %w", e.Name(), err)
		}
	}

	// 5. mark ready — inside the owner's tenant scope, like every worker write
	// to a tenant-scoped table (ADR-07 increment 1b).
	if err := inTenant(ctx, t.run_, TaskTypeTranscode, p.OwnerUserID, func(ctx context.Context) error {
		return t.repo.MarkReady(ctx, id, p.OutputKey, durMs, width, height)
	}); err != nil {
		return err
	}

	// 6. poster: a light follow-up task (SPEC-01 P0.2) so poster generation runs
	// on the weighted server, never hogging the heavy queue. Best-effort — a
	// missing poster never fails the (already-ready) video.
	if t.enqueue != nil {
		if task, err := NewThumbnailTask(ThumbnailPayload{AssetID: id.String(), SourceKey: p.SourceKey, OwnerUserID: p.OwnerUserID}); err != nil {
			log.Warn().Err(err).Str("asset", p.AssetID).Msg("transcode: build poster task failed")
		} else if _, err := t.enqueue.Enqueue(task); err != nil {
			log.Warn().Err(err).Str("asset", p.AssetID).Msg("transcode: enqueue poster failed")
		}
	}

	// 7. announce readiness (SPEC-01 P1.2). LoadAssetMeta is itself a read of a
	// tenant-scoped table, so it needs the scope too.
	_ = inTenant(ctx, t.run_, TaskTypeTranscode, p.OwnerUserID, func(ctx context.Context) error {
		emitAssetReady(ctx, t.pub, t.repo, id)
		return nil
	})
	return nil
}

func (t *Transcoder) download(ctx context.Context, key, dst string) error {
	rc, err := t.store.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	defer rc.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, rc); err != nil {
		return fmt.Errorf("write source: %w", err)
	}
	return nil
}

func (t *Transcoder) upload(ctx context.Context, path, key string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return t.store.Put(ctx, key, f, contentTypeFor(path))
}

// probe returns (durationMs, width, height) via ffprobe; any field FFprobe can't
// determine comes back nil so the DB column stays NULL.
func probe(ctx context.Context, path string) (durMs, width, height *int) {
	//nolint:gosec // path is a server-controlled temp file
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path,
	).Output()
	if err != nil {
		return nil, nil, nil
	}
	var probed struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probed); err != nil {
		return nil, nil, nil
	}
	for _, s := range probed.Streams {
		if s.CodecType == "video" && s.Width > 0 {
			w, h := s.Width, s.Height
			width, height = &w, &h
			break
		}
	}
	if secs, err := strconv.ParseFloat(strings.TrimSpace(probed.Format.Duration), 64); err == nil && secs > 0 {
		ms := int(secs * 1000)
		durMs = &ms
	}
	return durMs, width, height
}

func contentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m3u8":
		return "application/vnd.apple.mpegurl"
	case ".ts":
		return "video/mp2t"
	default:
		return "application/octet-stream"
	}
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(bytes.TrimSpace(b))
}
