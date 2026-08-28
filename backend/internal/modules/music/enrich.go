package music

// Track enrichment: pull the things that live INSIDE an audio file — embedded
// cover art, and any tags the track is missing — out of it after the fact.
//
// A separate pass from the import on purpose. Cover art is the expensive half:
// each one is an ffmpeg extraction, a second asset ingest, and then a wait for
// the image pipeline to produce variants (music.validateImageAsset refuses a
// cover that is not `ready`). Doing that inline would turn a fast "your 300
// tracks are in" into a slow "your 300 tracks are still importing", for a
// picture nobody is looking at yet.
//
// So the import lands the tracks and the client asks for the rest, per track or
// per import job. Enrichment only ever FILLS GAPS — it never overwrites a title,
// artist, album or cover that already has a value, because by the time it runs
// the user may have typed one.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	mediaapi "github.com/portal/backend/internal/modules/media/api"
	musicapi "github.com/portal/backend/internal/modules/music/api"
)

const (
	// A cover big enough to be worth keeping is a few hundred KB; anything past
	// this is not album art, and the bytes go through memory.
	enrichMaxCoverBytes = 16 << 20 // 16 MiB
	// Audio has to be spooled whole for ffmpeg to seek. Same ceiling as one
	// import entry, for the same reason.
	enrichMaxAudioBytes = 512 << 20
	enrichTaskTimeout   = 10 * time.Minute
	// The ingested cover runs through the image pipeline on the parallel `image`
	// queue. Poll until it is ready, because a cover that is not ready is refused
	// by validateImageAsset — but give up rather than hold the task open.
	enrichCoverPollEvery = 1 * time.Second
	enrichCoverPollFor   = 90 * time.Second
	enrichProbeTimeout   = 20 * time.Second
	enrichExtractTimeout = 60 * time.Second
)

// EnqueueEnrich schedules enrichment for one track. Owner-checked here so the
// worker never has to second-guess the payload.
func (s *Service) EnqueueEnrich(ctx context.Context, trackID, ownerID uuid.UUID) error {
	if s.enqueue == nil {
		return fmt.Errorf("music: enrichment not configured")
	}
	track, err := s.repo.GetTrack(ctx, trackID)
	if err != nil {
		return err
	}
	if track.OwnerID != ownerID {
		return ErrNotFound
	}
	if track.AudioAssetID == nil {
		return fmt.Errorf("%w: bài hát chưa gắn tệp âm thanh", ErrValidation)
	}
	return s.enqueueEnrich(trackID, ownerID)
}

// EnqueueEnrichForImport schedules enrichment for every track an import created.
// Returns how many were queued so the client can show progress against a total.
func (s *Service) EnqueueEnrichForImport(ctx context.Context, importID, ownerID uuid.UUID) (int, error) {
	if s.enqueue == nil {
		return 0, fmt.Errorf("music: enrichment not configured")
	}
	job, err := s.GetImport(ctx, importID, ownerID)
	if err != nil {
		return 0, err
	}

	var report []ImportReportEntry
	if len(job.Report) > 0 {
		_ = json.Unmarshal(job.Report, &report)
	}

	queued := 0
	for _, entry := range report {
		if !entry.OK || entry.TrackID == "" {
			continue
		}
		trackID, err := uuid.Parse(entry.TrackID)
		if err != nil {
			continue
		}
		// A track deleted between the import and this call is not an error worth
		// failing the batch over.
		if err := s.enqueueEnrich(trackID, ownerID); err != nil {
			log.Warn().Err(err).Str("track", entry.TrackID).Msg("music: could not queue enrichment")
			continue
		}
		queued++
	}
	return queued, nil
}

func (s *Service) enqueueEnrich(trackID, ownerID uuid.UUID) error {
	payload, _ := json.Marshal(musicapi.EnrichTrackPayload{
		TrackID: trackID.String(),
		OwnerID: ownerID.String(),
	})
	// The `image` queue does the heavy half; this task mostly waits on it, so it
	// belongs on `default` where it will not starve the transcode pool.
	// MaxRetry(1): a transient storage blip deserves one more go, but a file with
	// no embedded art will never grow one.
	task := asynq.NewTask(musicapi.TaskEnrichTrack, payload,
		asynq.Queue("default"), asynq.Timeout(enrichTaskTimeout), asynq.MaxRetry(1))
	_, err := s.enqueue.Enqueue(task)
	return err
}

// EnrichTrack is the worker body (music:enrich_track).
//
// Every step is optional: a track with no art, no tags, or an unreadable file is
// simply left as it is. Returning an error here would re-queue the task to fail
// the same way, so only genuine infrastructure faults propagate.
func (s *Service) EnrichTrack(ctx context.Context, trackID, ownerID uuid.UUID) error {
	if s.media == nil || s.runInTenant == nil {
		log.Error().Msg("music: enrichment worker not configured")
		return nil
	}

	var track Track
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		var err error
		track, err = s.repo.GetTrack(ctx, trackID)
		return err
	}); err != nil {
		return err
	}
	if track.OwnerID != ownerID || track.AudioAssetID == nil {
		return nil
	}

	// Nothing left to learn: the track already has everything this pass can add.
	if track.CoverAssetID != nil && nonBlank(track.Artist) && nonBlank(track.Album) {
		return nil
	}

	path, cleanup, err := s.spoolAudio(ctx, ownerID, *track.AudioAssetID)
	if err != nil {
		log.Warn().Err(err).Str("track", trackID.String()).Msg("music: enrichment could not read the audio")
		return nil
	}
	defer cleanup()

	tags := probeFileTags(ctx, path)

	var coverID *uuid.UUID
	if track.CoverAssetID == nil {
		coverID = s.extractCover(ctx, ownerID, trackID, path)
	}

	patch, changed := enrichPatch(track, tags, coverID)
	if !changed {
		return nil
	}
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		_, err := s.repo.UpdateTrack(ctx, patch)
		return err
	}); err != nil {
		log.Error().Err(err).Str("track", trackID.String()).Msg("music: enrichment could not save")
		return err
	}
	log.Info().Str("track", trackID.String()).Bool("cover", patch.SetCover).
		Msg("music: track enriched")
	return nil
}

// spoolAudio streams the original to a temp file. ffmpeg needs to seek — an
// embedded picture can sit at either end of the container — so a pipe will not do.
//
// Only the LOOKUP runs inside the tenant scope. `assets` is RLS-fenced, so
// resolving the storage key outside one finds nothing ("media: asset not
// found") even though the file is right there; but the reader it hands back is
// an object-store stream, not a database cursor, so copying a 500 MB body has
// no business holding a transaction open.
func (s *Service) spoolAudio(ctx context.Context, ownerID, assetID uuid.UUID) (string, func(), error) {
	var rc io.ReadCloser
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		var err error
		rc, _, err = s.media.OpenOriginal(ctx, ownerID, assetID)
		return err
	}); err != nil {
		return "", func() {}, err
	}
	defer rc.Close()

	tmp, err := os.CreateTemp("", "music-enrich-*")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmp.Name())
	}
	n, err := io.Copy(tmp, io.LimitReader(rc, enrichMaxAudioBytes+1))
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	if n > enrichMaxAudioBytes {
		cleanup()
		return "", func() {}, fmt.Errorf("music: audio too large to enrich (%s)", humanBytes(n))
	}
	return tmp.Name(), cleanup, nil
}

// extractCover pulls the embedded picture out and ingests it as an image asset,
// returning its id once the image pipeline has it ready. nil means "no usable
// cover", which is the common case and never an error.
func (s *Service) extractCover(ctx context.Context, ownerID, trackID uuid.UUID, audioPath string) *uuid.UUID {
	ext := attachedPictureExt(ctx, audioPath)
	if ext == "" {
		return nil // no attached picture in this file
	}

	out := audioPath + "-cover" + ext
	defer os.Remove(out)

	// `-c copy` because the picture is already a complete JPEG/PNG inside the
	// container: re-encoding would cost CPU and quality to produce the same image.
	ectx, cancel := context.WithTimeout(ctx, enrichExtractTimeout)
	defer cancel()
	if err := exec.CommandContext(ectx, "ffmpeg", "-v", "error", "-y",
		"-i", audioPath, "-an", "-c:v", "copy", "-frames:v", "1", out).Run(); err != nil {
		log.Debug().Err(err).Str("track", trackID.String()).Msg("music: cover extraction failed")
		return nil
	}

	data, err := os.ReadFile(out)
	if err != nil || len(data) == 0 || len(data) > enrichMaxCoverBytes {
		return nil
	}

	assetID, err := s.ingestCover(ctx, ownerID, trackID, ext, data)
	if err != nil {
		log.Warn().Err(err).Str("track", trackID.String()).Msg("music: cover ingest failed")
		return nil
	}
	return assetID
}

// ingestCover stores the picture and waits for the image pipeline to finish with
// it. The wait is the point: validateImageAsset refuses a cover that is not
// `ready`, so setting the reference early would just be rejected.
func (s *Service) ingestCover(ctx context.Context, ownerID, trackID uuid.UUID, ext string, data []byte) (*uuid.UUID, error) {
	var assetID uuid.UUID
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		var err error
		assetID, err = s.media.Ingest(ctx, ownerID,
			fmt.Sprintf("cover-%s%s", trackID, ext), coverMime(ext), data)
		return err
	}); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(enrichCoverPollFor)
	for time.Now().Before(deadline) {
		var status mediaapi.AssetStatus
		if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
			a, err := s.media.GetAsset(ctx, assetID)
			if err != nil {
				return err
			}
			if a != nil {
				status = a.Status
			}
			return nil
		}); err != nil {
			return nil, err
		}
		switch status {
		case mediaapi.StatusReady:
			return &assetID, nil
		case mediaapi.StatusFailed:
			return nil, fmt.Errorf("music: cover asset failed processing")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(enrichCoverPollEvery):
		}
	}
	// Not a hard failure: the asset exists and may well become ready a moment
	// later, but this track keeps no cover rather than a reference the API would
	// refuse. Re-running enrichment picks it up.
	return nil, fmt.Errorf("music: cover did not become ready in time")
}

// attachedPictureExt reports the file extension to extract the embedded picture
// as, or "" when the file has none.
//
// It reads the stream disposition rather than trusting that a video stream in an
// audio file must be art: `attached_pic` is exactly the flag that distinguishes
// cover art from, say, a music video muxed into an m4a.
func attachedPictureExt(ctx context.Context, path string) string {
	pctx, cancel := context.WithTimeout(ctx, enrichProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(pctx, "ffprobe", "-v", "error",
		"-select_streams", "v", "-show_streams", "-of", "json", path).Output()
	if err != nil {
		return ""
	}

	var probe struct {
		Streams []struct {
			CodecName   string         `json:"codec_name"`
			Disposition map[string]int `json:"disposition"`
		} `json:"streams"`
	}
	if json.Unmarshal(out, &probe) != nil {
		return ""
	}
	for _, st := range probe.Streams {
		if st.Disposition["attached_pic"] != 1 {
			continue
		}
		switch st.CodecName {
		case "mjpeg":
			return ".jpg"
		case "png":
			return ".png"
		case "webp":
			return ".webp"
		}
	}
	return ""
}

func coverMime(ext string) string {
	switch ext {
	case ".png":
		return "image/png"
	case ".webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}

// probeFileTags is probeTags' half that reads an already-spooled file: the
// enrichment pass has the path, not the bytes.
func probeFileTags(ctx context.Context, path string) map[string]string {
	pctx, cancel := context.WithTimeout(ctx, enrichProbeTimeout)
	defer cancel()
	out, err := exec.CommandContext(pctx, "ffprobe",
		"-v", "error", "-show_format", "-of", "json", path).Output()
	if err != nil {
		return nil
	}
	var probe struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if json.Unmarshal(out, &probe) != nil {
		return nil
	}
	return lowerTags(probe.Format.Tags)
}

// enrichPatch decides what this pass may write.
//
// The rule it encodes is the whole contract of enrichment: **fill gaps, never
// overwrite**. It runs long after the import, so any value already on the track
// may be something the user typed, and silently replacing that with whatever the
// file's tags happen to say would be worse than adding nothing.
//
// Pure, so the rule is testable without ffprobe or a worker.
func enrichPatch(track Track, tags map[string]string, coverID *uuid.UUID) (UpdateTrackInput, bool) {
	patch := UpdateTrackInput{ID: track.ID}
	changed := false

	if !nonBlank(track.Artist) {
		if v := firstTag(tags, "artist", "album_artist", "performer"); v != "" {
			patch.Artist = &v
			patch.SetArtist = true
			changed = true
		}
	}
	if !nonBlank(track.Album) {
		if v := firstTag(tags, "album"); v != "" {
			patch.Album = &v
			patch.SetAlbum = true
			changed = true
		}
	}
	// The caller only extracts when the track has no cover, but check again here
	// so the rule holds wherever this is called from.
	if track.CoverAssetID == nil && coverID != nil {
		patch.CoverAssetID = coverID
		patch.SetCover = true
		changed = true
	}
	return patch, changed
}

func nonBlank(s *string) bool {
	return s != nil && *s != ""
}
