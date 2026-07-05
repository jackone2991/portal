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
}

func NewTranscodeTask(p TranscodePayload) (*asynq.Task, error) {
	body, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TaskTypeTranscode, body, asynq.Queue("transcode")), nil
}

// Repo is the minimal persistence surface the transcoder needs. The media
// repository adapter satisfies it (primitive signatures keep this package free
// of a media-package import → no cycle).
type Repo interface {
	MarkReady(ctx context.Context, id uuid.UUID, outputPrefix string, durationMs, width, height *int) error
	MarkFailed(ctx context.Context, id uuid.UUID, msg string) error
}

// Transcoder runs the FFmpeg HLS pipeline. Construct with NewTranscoder.
type Transcoder struct {
	store storage.Storage
	repo  Repo
}

func NewTranscoder(store storage.Storage, repo Repo) *Transcoder {
	return &Transcoder{store: store, repo: repo}
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

	// 5. mark ready
	return t.repo.MarkReady(ctx, id, p.OutputKey, durMs, width, height)
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
