package worker

import (
	"context"
	"encoding/json"
	"errors"
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

const TaskTypeProcessImage = "media:process_image"

// TaskTypePurgeOrphans is the hourly janitor sweep (SPEC-01 P0.3). Its handler
// body lives on the media Module (Module.PurgeOrphans); the name is defined here
// so cmd/worker can register it on the shared scheduler + light mux.
const TaskTypePurgeOrphans = "media:purge_orphans"

// Resource + variant guardrails (SPEC-01 P0.1).
const (
	// imageMaxDimension caps either dimension. Decoded RGBA at 8,000² ≈ 256 MB
	// peak vs ≈ 576 MB at 12,000² — the difference between "tight" and
	// "OOM-killer bait" on a small VPS (rev 2).
	imageMaxDimension = 8000
	thumbMaxWidth     = 320
	mediumMaxWidth    = 1280
	webpQuality       = "80"
)

// ProcessImagePayload is enqueued when an accepted image upload completes. The
// worker pulls the original from storage, generates web-optimized WebP variants
// (never touching the original), records the variant rows, and marks the asset
// ready with the source dimensions.
type ProcessImagePayload struct {
	AssetID   string `json:"asset_id"`
	SourceKey string `json:"source_key"`
}

// NewProcessImageTask enqueues onto the "heavy" queue (low-concurrency server)
// so large image decodes are serialized (SPEC-01 P0.1 OOM guard).
func NewProcessImageTask(p ProcessImagePayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeProcessImage, body, asynq.Queue("heavy")), nil
}

// ImageProcessor runs the FFmpeg WebP variant pipeline. Construct with
// NewImageProcessor.
type ImageProcessor struct {
	store storage.Storage
	repo  Repo
	pub   Publisher // optional: emits media:asset_ready
}

func NewImageProcessor(store storage.Storage, repo Repo, pub Publisher) *ImageProcessor {
	return &ImageProcessor{store: store, repo: repo, pub: pub}
}

// Handle is the Asynq handler. On failure it marks the asset failed and returns
// nil (a bad source file should not retry forever), mirroring the transcoder.
func (ip *ImageProcessor) Handle(ctx context.Context, task *asynq.Task) error {
	var p ProcessImagePayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		return err
	}
	id, err := uuid.Parse(p.AssetID)
	if err != nil {
		return err
	}

	log.Info().Str("asset", p.AssetID).Str("src", p.SourceKey).Msg("process_image: start")
	if err := ip.run(ctx, id, p); err != nil {
		log.Error().Err(err).Str("asset", p.AssetID).Msg("process_image: failed")
		_ = ip.repo.MarkFailed(ctx, id, truncate(err.Error(), 500))
		return nil
	}
	log.Info().Str("asset", p.AssetID).Msg("process_image: ready")
	return nil
}

func (ip *ImageProcessor) run(ctx context.Context, id uuid.UUID, p ProcessImagePayload) error {
	dir, err := os.MkdirTemp("", "image-*")
	if err != nil {
		return fmt.Errorf("tempdir: %w", err)
	}
	defer os.RemoveAll(dir)

	// 1. download the original (never modified — archival guarantee)
	src := filepath.Join(dir, "source")
	if err := ip.download(ctx, p.SourceKey, src); err != nil {
		return err
	}

	// 2. probe header (before any full decode): dimensions + frame count
	w, h, frames, err := probeImage(ctx, src)
	if err != nil {
		return fmt.Errorf("probe: %w", err)
	}
	if frames > 1 {
		return fmt.Errorf("animated images are not supported (%d frames)", frames)
	}
	if w <= 0 || h <= 0 {
		return errors.New("could not determine image dimensions")
	}
	if w > imageMaxDimension || h > imageMaxDimension {
		return fmt.Errorf("image dimensions %dx%d exceed the %dpx limit", w, h, imageMaxDimension)
	}

	// 3. generate served variants (WebP, auto-oriented, metadata stripped)
	variants := []struct {
		name string
		maxW int
	}{
		{"thumb", thumbMaxWidth},
		{"medium", mediumMaxWidth},
	}
	for _, v := range variants {
		outPath := filepath.Join(dir, v.name+".webp")
		if err := encodeWebP(ctx, src, outPath, v.maxW); err != nil {
			return fmt.Errorf("encode %s: %w", v.name, err)
		}
		key := fmt.Sprintf("variants/%s/%s.webp", id, v.name)
		size, err := ip.upload(ctx, outPath, key)
		if err != nil {
			return fmt.Errorf("upload %s: %w", v.name, err)
		}
		vw, vh := scaledDims(w, h, v.maxW)
		if err := ip.repo.InsertVariant(ctx, id, v.name, key, vw, vh, size); err != nil {
			return fmt.Errorf("insert variant %s: %w", v.name, err)
		}
	}

	// 4. mark ready with the SOURCE dimensions (not a variant's)
	if err := ip.repo.MarkImageReady(ctx, id, w, h); err != nil {
		return err
	}
	emitAssetReady(ctx, ip.pub, ip.repo, id)
	return nil
}

func (ip *ImageProcessor) download(ctx context.Context, key, dst string) error {
	rc, err := ip.store.Get(ctx, key)
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

// upload puts the file at key and returns its byte size (for the variant row).
func (ip *ImageProcessor) upload(ctx context.Context, path, key string) (int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return 0, err
	}
	if err := ip.store.Put(ctx, key, f, "image/webp"); err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// encodeWebP runs FFmpeg to produce one WebP variant: scaled to at most maxW
// wide (never upscaled — scale='min(maxW,iw)'), EXIF orientation baked in
// (default autorotate), all metadata stripped (-map_metadata -1). Alpha
// (PNG transparency) is preserved by libwebp.
func encodeWebP(ctx context.Context, src, dst string, maxW int) error {
	scale := fmt.Sprintf("scale='min(%d,iw)':-2:flags=lanczos", maxW)
	//nolint:gosec // fixed args, paths are server-controlled temp files
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-y", "-i", src,
		"-vf", scale,
		"-map_metadata", "-1",
		"-frames:v", "1",
		"-c:v", "libwebp", "-q:v", webpQuality, "-compression_level", "6",
		dst,
	)
	if combined, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, tail(combined, 400))
	}
	return nil
}

// probeImage reads the header (no full decode) for width, height and frame
// count. nb_frames > 1 flags an animated input (GIF/APNG/animated WebP), which
// v1 rejects. A missing nb_frames is treated as a single still frame.
func probeImage(ctx context.Context, path string) (width, height, frames int, err error) {
	//nolint:gosec // path is a server-controlled temp file
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error", "-select_streams", "v:0",
		"-show_entries", "stream=width,height,nb_frames",
		"-print_format", "json", path,
	).Output()
	if err != nil {
		return 0, 0, 0, fmt.Errorf("ffprobe: %w", err)
	}
	var probed struct {
		Streams []struct {
			Width    int    `json:"width"`
			Height   int    `json:"height"`
			NbFrames string `json:"nb_frames"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(out, &probed); err != nil {
		return 0, 0, 0, err
	}
	if len(probed.Streams) == 0 {
		return 0, 0, 0, errors.New("no image stream found")
	}
	s := probed.Streams[0]
	frames = 1
	if n, e := strconv.Atoi(strings.TrimSpace(s.NbFrames)); e == nil && n > 0 {
		frames = n
	}
	return s.Width, s.Height, frames, nil
}

// scaledDims computes the stored variant dimensions: aspect-preserving, never
// upscaled. FFmpeg's -2 rounds to an even height, so this is a close estimate
// (the exact pixels live in the stored object; the row is metadata).
func scaledDims(w, h, maxW int) (int, int) {
	if w <= maxW {
		return w, h
	}
	nh := int(float64(h) * float64(maxW) / float64(w))
	if nh < 1 {
		nh = 1
	}
	return maxW, nh
}
