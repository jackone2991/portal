package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	"github.com/portal/backend/internal/platform/storage"
)

const TaskTypeThumbnail = "media:thumbnail"

const posterMaxWidth = 640

// ThumbnailPayload is enqueued after a video transcode completes (SPEC-01 P0.2).
// The worker seeks a representative frame of the ORIGINAL and stores it as the
// asset's `poster` variant. SourceKey is the uploaded original's storage key.
type ThumbnailPayload struct {
	AssetID   string `json:"asset_id"`
	SourceKey string `json:"source_key"`
}

func NewThumbnailTask(p ThumbnailPayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeThumbnail, body, asynq.Queue("thumbnail")), nil
}

// Thumbnailer generates the video poster variant. Construct with NewThumbnailer.
type Thumbnailer struct {
	store storage.Storage
	repo  Repo
}

func NewThumbnailer(store storage.Storage, repo Repo) *Thumbnailer {
	return &Thumbnailer{store: store, repo: repo}
}

// Handle generates a `poster` variant. Poster failure NEVER fails the (already
// ready) video — the asset stays playable, we just log a warning and return nil
// so the job is not retried forever (SPEC-01 P0.2: playback > cosmetics).
func (th *Thumbnailer) Handle(ctx context.Context, task *asynq.Task) error {
	var p ThumbnailPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	id, err := uuid.Parse(p.AssetID)
	if err != nil {
		return err
	}
	if err := th.run(ctx, id, p); err != nil {
		log.Warn().Err(err).Str("asset", p.AssetID).Msg("thumbnail: poster generation skipped")
	}
	return nil
}

func (th *Thumbnailer) run(ctx context.Context, id uuid.UUID, p ThumbnailPayload) error {
	dir, err := os.MkdirTemp("", "poster-*")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "source")
	if err := th.download(ctx, p.SourceKey, src); err != nil {
		return err
	}

	// Probe duration + video-stream count. An audio-only container (0 video
	// streams — e.g. an .mp3 renamed .mp4) gets skipped with a warning, never a
	// failure (SPEC-01 P0.2, rev 2 review pt 5).
	durMs, width, _ := probe(ctx, src) // (durMs, width*, height*) — width nil ⇒ no video
	if width == nil {
		return fmt.Errorf("no video stream (audio-only container?) — poster skipped")
	}

	// Seek to min(10% of duration, 10s), clamped inside short clips.
	seek := 0.0
	if durMs != nil && *durMs > 0 {
		secs := float64(*durMs) / 1000.0
		seek = secs * 0.10
		if seek > 10 {
			seek = 10
		}
		if seek > secs {
			seek = 0
		}
	}

	out := filepath.Join(dir, "poster.webp")
	if err := extractPoster(ctx, src, out, seek); err != nil {
		return err
	}

	key := fmt.Sprintf("variants/%s/poster.webp", id)
	size, err := th.upload(ctx, out, key)
	if err != nil {
		return err
	}

	// Re-probe the generated poster for its true dimensions (metadata row).
	pw, ph := posterMaxWidth, 0
	if _, w, h := probe(ctx, out); w != nil && h != nil {
		pw, ph = *w, *h
	}
	return th.repo.InsertVariant(ctx, id, "poster", key, pw, ph, size)
}

func (th *Thumbnailer) download(ctx context.Context, key, dst string) error {
	rc, err := th.store.Get(ctx, key)
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

func (th *Thumbnailer) upload(ctx context.Context, path, key string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if err := th.store.Put(ctx, key, f, "image/webp"); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// extractPoster seeks to `seek` seconds and grabs one frame scaled to at most
// posterMaxWidth wide, stored as WebP with metadata stripped.
func extractPoster(ctx context.Context, src, dst string, seek float64) error {
	scale := fmt.Sprintf("scale='min(%d,iw)':-2:flags=lanczos", posterMaxWidth)
	//nolint:gosec // fixed args, paths are server-controlled temp files
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y",
		"-ss", fmt.Sprintf("%.3f", seek),
		"-i", src,
		"-frames:v", "1",
		"-vf", scale,
		"-map_metadata", "-1",
		"-c:v", "libwebp", "-q:v", webpQuality,
		dst,
	)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg poster: %w: %s", err, tail(combined, 400))
	}
	return nil
}
