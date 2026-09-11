package music

// Catalogue lookup: the release year, the genre and a proper album cover, from
// MusicBrainz and the Cover Art Archive (0039).
//
// The third and last metadata pass, and the only one that leaves the machine:
//
//	import   → what the FILENAME says          (fast, no I/O beyond the zip)
//	enrich   → what is INSIDE the file          (ffprobe + the embedded picture)
//	lookup   → what only a CATALOGUE knows      (this file — network, throttled)
//
// It is off unless an operator turns it on, because it sends the library's
// artist/title pairs to a third party and that is not a default anyone should
// inherit silently. When off, the endpoints answer 503 with a plain reason
// rather than pretending to work.
//
// Like enrichment, it only ever FILLS GAPS — enforced in SQL this time
// (SetTrackLookupResult COALESCEs every field), so even a caller that passes a
// value for a populated field cannot overwrite it.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/rs/zerolog/log"

	musicapi "github.com/portal/backend/internal/modules/music/api"
)

const (
	// One lookup can make three throttled calls (search, cover, retry headroom)
	// at ~1.1s each, plus the image pipeline wait for the cover it ingests.
	lookupTaskTimeout = 5 * time.Minute
	// A whole-library sweep is bounded so one click cannot queue ten thousand
	// tasks that will take three hours at one request per second. Truncation is
	// reported, never silent.
	lookupMaxBatch = 500
)

// LookupEnabled reports whether outbound catalogue lookups are configured.
// The handler uses it to answer 503 with a reason instead of failing obscurely.
func (s *Service) LookupEnabled() bool { return s.mb != nil && s.mb.cfg.active() }

// EnqueueLookup schedules a catalogue lookup for one track.
func (s *Service) EnqueueLookup(ctx context.Context, trackID, ownerID uuid.UUID) error {
	if !s.LookupEnabled() {
		return ErrLookupDisabled
	}
	if s.enqueue == nil {
		return errors.New("music: lookup not configured")
	}
	track, err := s.repo.GetTrack(ctx, trackID)
	if err != nil {
		return err
	}
	if track.OwnerID != ownerID {
		return ErrNotFound
	}
	if err := s.repo.MarkLookupPending(ctx, trackID); err != nil {
		return err
	}
	return s.enqueueLookup(trackID, ownerID)
}

// EnqueueLookupForImport schedules a lookup for every track an import created.
// Returns how many were queued and how many were dropped by the batch cap.
func (s *Service) EnqueueLookupForImport(ctx context.Context, importID, ownerID uuid.UUID) (queued, skipped int, err error) {
	if !s.LookupEnabled() {
		return 0, 0, ErrLookupDisabled
	}
	job, err := s.GetImport(ctx, importID, ownerID)
	if err != nil {
		return 0, 0, err
	}

	var report []ImportReportEntry
	if len(job.Report) > 0 {
		_ = json.Unmarshal(job.Report, &report)
	}

	for _, entry := range report {
		if !entry.OK || entry.TrackID == "" {
			continue
		}
		id, perr := uuid.Parse(entry.TrackID)
		if perr != nil {
			continue
		}
		if queued >= lookupMaxBatch {
			skipped++
			continue
		}
		if err := s.repo.MarkLookupPending(ctx, id); err != nil {
			continue
		}
		if err := s.enqueueLookup(id, ownerID); err != nil {
			log.Warn().Err(err).Str("track", entry.TrackID).Msg("music: could not queue lookup")
			continue
		}
		queued++
	}
	if skipped > 0 {
		log.Warn().Int("queued", queued).Int("skipped", skipped).
			Msg("music: lookup batch truncated at the cap")
	}
	return queued, skipped, nil
}

func (s *Service) enqueueLookup(trackID, ownerID uuid.UUID) error {
	payload, _ := json.Marshal(musicapi.LookupTrackPayload{
		TrackID: trackID.String(),
		OwnerID: ownerID.String(),
	})
	// MaxRetry(2) with backoff: unlike the local passes, the failure mode here is
	// somebody else's server having a bad minute, and that IS worth retrying.
	// The queue is `default`, so a slow sweep never blocks transcoding.
	task := asynq.NewTask(musicapi.TaskLookupTrack, payload,
		asynq.Queue("default"), asynq.Timeout(lookupTaskTimeout), asynq.MaxRetry(2))
	_, err := s.enqueue.Enqueue(task)
	return err
}

// LookupTrack is the worker body (music:lookup_track).
//
// Outcomes are recorded rather than thrown away, because "we asked and the
// catalogue does not have this" is information the user needs — otherwise the
// button looks broken for every track that legitimately has no match.
func (s *Service) LookupTrack(ctx context.Context, trackID, ownerID uuid.UUID) error {
	if !s.LookupEnabled() || s.runInTenant == nil {
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
	if track.OwnerID != ownerID {
		return nil
	}

	artist := ""
	if track.Artist != nil {
		artist = *track.Artist
	}

	match, err := s.mb.SearchRecording(ctx, artist, track.Title)
	if err != nil {
		// A network fault is transient — record it and let asynq retry. Returning
		// the error is what triggers that.
		s.recordLookup(ctx, trackID, ownerID, "failed", truncateNote(err.Error()), nil)
		return err
	}
	if match == nil {
		s.recordLookup(ctx, trackID, ownerID, "no_match",
			fmt.Sprintf("không tìm thấy kết quả đủ tin cậy cho %q", displayQuery(artist, track.Title)), nil)
		return nil
	}

	// Cover art is a second call, so only make it when it can be used.
	var coverID *uuid.UUID
	if track.CoverAssetID == nil && match.ReleaseID != nil {
		coverID = s.fetchAndIngestCover(ctx, ownerID, trackID, *match.ReleaseID)
	}

	note := fmt.Sprintf("MusicBrainz score %d", match.Score)
	s.recordLookup(ctx, trackID, ownerID, "matched", note, &lookupResult{
		match: match, coverID: coverID,
	})
	return nil
}

type lookupResult struct {
	match   *MBMatch
	coverID *uuid.UUID
}

// recordLookup writes the outcome. Every value field goes through the query's
// COALESCE, so this can never clobber something the user set.
func (s *Service) recordLookup(ctx context.Context, trackID, ownerID uuid.UUID, status, note string, res *lookupResult) {
	in := SetLookupInput{ID: trackID, Status: status, Note: note}
	if res != nil && res.match != nil {
		m := res.match
		in.MBRecordingID = &m.RecordingID
		in.MBReleaseID = m.ReleaseID
		in.Year = m.Year
		in.CoverAssetID = res.coverID
		in.Genre = optional(m.Genre)
		in.Artist = optional(m.Artist)
		in.Album = optional(m.Album)
	}
	if err := s.runInTenant(ctx, ownerID, func(ctx context.Context) error {
		return s.repo.SetLookupResult(ctx, in)
	}); err != nil {
		log.Error().Err(err).Str("track", trackID.String()).Msg("music: could not record lookup result")
	}
}

// fetchAndIngestCover pulls the front cover and waits for the image pipeline,
// reusing the same wait the embedded-art path needs. nil means "no artwork", a
// perfectly ordinary answer.
func (s *Service) fetchAndIngestCover(ctx context.Context, ownerID, trackID, releaseID uuid.UUID) *uuid.UUID {
	data, contentType, err := s.mb.FetchCoverArt(ctx, releaseID)
	if err != nil {
		log.Warn().Err(err).Str("track", trackID.String()).Msg("music: cover art fetch failed")
		return nil
	}
	if len(data) == 0 {
		return nil
	}

	ext := ".jpg"
	if strings.Contains(contentType, "png") {
		ext = ".png"
	}
	id, err := s.ingestCover(ctx, ownerID, trackID, ext, data)
	if err != nil {
		log.Warn().Err(err).Str("track", trackID.String()).Msg("music: cover art ingest failed")
		return nil
	}
	return id
}

// displayQuery is what the user sees in a "no match" note — the same string the
// search actually used, so the reason is checkable rather than mysterious.
func displayQuery(artist, title string) string {
	if strings.TrimSpace(artist) == "" {
		return title
	}
	return artist + " – " + title
}

// truncateNote keeps a failure reason short enough to render in a table cell.
func truncateNote(s string) string {
	const max = 200
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
