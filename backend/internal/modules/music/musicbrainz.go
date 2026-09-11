package music

// The MusicBrainz + Cover Art Archive client.
//
// This is the only outbound third-party call in the codebase, and it is the
// reason the whole lookup feature is opt-in. Three things about it are not
// negotiable and are enforced here rather than left to callers:
//
//  1. ONE REQUEST PER SECOND, globally. MusicBrainz states this as a hard limit
//     and blocks clients that exceed it. The throttle lives in Redis, not in a
//     process-local ticker, because two worker replicas with a ticker each would
//     make two requests a second and get the whole deployment banned.
//  2. A REAL USER-AGENT with contact details. MusicBrainz requires it and returns
//     403 to generic agents. There is no default — an operator supplies contact
//     info or the feature stays off, because a shared fake UA is how everyone
//     gets blocked at once.
//  3. NO GUESSING. A search returns a score; below the threshold the answer is
//     "no confident match", recorded as such. Filling a library with plausible
//     wrong albums is worse than leaving it sparse, and much harder to undo.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog/log"
)

const (
	// MusicBrainz asks for at most one request per second per client. The window
	// is a little over a second so clock skew and round-trip jitter cannot make
	// two calls land inside the same second.
	mbMinInterval = 1100 * time.Millisecond
	// How long to wait for the throttle before giving up on this track. Longer
	// than the interval so a queue of tracks makes progress, short enough that a
	// stuck token does not pin a worker.
	mbThrottleWait = 30 * time.Second
	mbHTTPTimeout  = 15 * time.Second

	// MusicBrainz scores a search 0-100. Below this the match is a guess.
	// 88 is deliberately strict: the input is often a filename, and a wrong
	// album silently attached to a track is worse than an empty field.
	mbMinScore = 88

	// Cover Art Archive sizes. 500px is the sweet spot — large enough for a
	// detail page, small enough that a few hundred of them do not bloat storage.
	caaFrontSize     = "front-500"
	caaMaxCoverBytes = 8 << 20
)

// ErrLookupDisabled is returned when nobody has turned the feature on.
var ErrLookupDisabled = errors.New("music: catalogue lookup is disabled")

// LookupConfig is what an operator must supply to enable outbound lookups.
type LookupConfig struct {
	Enabled bool
	// Contact is the email or URL MusicBrainz can reach the operator at. It goes
	// into the User-Agent, which their policy requires. Empty disables the
	// feature no matter what Enabled says — an anonymous client is one that gets
	// everyone blocked.
	Contact string
	// BaseURL / CoverArtURL are overridable for tests and for anyone running a
	// local mirror (which is the polite way to do this at volume).
	BaseURL     string
	CoverArtURL string
}

func (c LookupConfig) active() bool { return c.Enabled && strings.TrimSpace(c.Contact) != "" }

// MBClient talks to MusicBrainz. Construct via newMBClient.
type MBClient struct {
	cfg   LookupConfig
	http  *http.Client
	redis *redis.Client // global throttle; nil falls back to a local sleep
	agent string
}

func newMBClient(cfg LookupConfig, rdb *redis.Client) *MBClient {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://musicbrainz.org/ws/2"
	}
	if cfg.CoverArtURL == "" {
		cfg.CoverArtURL = "https://coverartarchive.org"
	}
	return &MBClient{
		cfg:   cfg,
		redis: rdb,
		http:  &http.Client{Timeout: mbHTTPTimeout},
		// Format MusicBrainz asks for: Application/Version ( contact ).
		agent: fmt.Sprintf("Portal/1.0 ( %s )", strings.TrimSpace(cfg.Contact)),
	}
}

// MBMatch is what a successful lookup found. Every field is optional — a release
// with no date or no tags is perfectly normal.
type MBMatch struct {
	RecordingID uuid.UUID
	ReleaseID   *uuid.UUID
	Title       string
	Artist      string
	Album       string
	Year        *int
	Genre       string
	Score       int
}

// SearchRecording finds the best match for an artist/title pair.
//
// Returns (nil, nil) for "nothing confident enough", which is an ordinary
// outcome and not an error: most libraries contain something the catalogue has
// never heard of.
func (c *MBClient) SearchRecording(ctx context.Context, artist, title string) (*MBMatch, error) {
	if !c.cfg.active() {
		return nil, ErrLookupDisabled
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, nil
	}

	// Lucene query. Both terms are quoted so punctuation in a title cannot turn
	// into query syntax, and the artist is only added when known — an unqualified
	// title search is what produces confident matches for the wrong song.
	q := `recording:` + luceneQuote(title)
	if a := strings.TrimSpace(artist); a != "" {
		q += ` AND artist:` + luceneQuote(a)
	}

	endpoint := fmt.Sprintf("%s/recording?query=%s&fmt=json&limit=5",
		strings.TrimRight(c.cfg.BaseURL, "/"), url.QueryEscape(q))

	var payload mbRecordingSearch
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}

	for _, rec := range payload.Recordings {
		if rec.Score < mbMinScore {
			break // results are score-ordered, so the rest are worse
		}
		id, err := uuid.Parse(rec.ID)
		if err != nil {
			continue
		}
		m := &MBMatch{RecordingID: id, Title: rec.Title, Score: rec.Score}
		if len(rec.ArtistCredit) > 0 {
			m.Artist = rec.ArtistCredit[0].Name
		}
		// The earliest release is the original one; later ones are reissues and
		// compilations, whose year would be wrong for "when did this come out".
		if rel := earliestRelease(rec.Releases); rel != nil {
			if rid, err := uuid.Parse(rel.ID); err == nil {
				m.ReleaseID = &rid
			}
			m.Album = rel.Title
			m.Year = yearFromDate(rel.Date)
		}
		m.Genre = topTag(rec.Tags)
		return m, nil
	}
	return nil, nil
}

// FetchCoverArt downloads the front cover for a release. Returns (nil, nil) when
// the archive has no image for it, which is common and not a failure.
func (c *MBClient) FetchCoverArt(ctx context.Context, releaseID uuid.UUID) ([]byte, string, error) {
	if !c.cfg.active() {
		return nil, "", ErrLookupDisabled
	}
	// The Cover Art Archive is a separate service from the MusicBrainz web
	// service and is not covered by its rate limit, but it redirects to the
	// Internet Archive and deserves the same courtesy.
	if err := c.throttle(ctx); err != nil {
		return nil, "", err
	}

	endpoint := fmt.Sprintf("%s/release/%s/%s",
		strings.TrimRight(c.cfg.CoverArtURL, "/"), releaseID, caaFrontSize)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", c.agent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, "", nil // no artwork for this release
	}
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("cover art archive: %s", resp.Status)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, caaMaxCoverBytes+1))
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 || len(data) > caaMaxCoverBytes {
		return nil, "", nil
	}
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "image/") {
		return nil, "", nil // a redirect page, not an image
	}
	return data, contentType, nil
}

// getJSON performs one throttled, identified GET.
func (c *MBClient) getJSON(ctx context.Context, endpoint string, out any) error {
	if err := c.throttle(ctx); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", c.agent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusServiceUnavailable,
		resp.StatusCode == http.StatusTooManyRequests:
		// Their explicit "you are going too fast". Surfacing it distinctly means
		// the caller can back off instead of recording a bogus "no match".
		return fmt.Errorf("musicbrainz: rate limited (%s)", resp.Status)
	case resp.StatusCode != http.StatusOK:
		return fmt.Errorf("musicbrainz: %s", resp.Status)
	}
	return json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(out)
}

// throttle blocks until this deployment is allowed another request.
//
// Redis SET NX PX is the whole mechanism: whoever sets the key owns the next
// slot, everyone else waits for it to expire. It is deliberately simple and
// slightly conservative — being a bit slower than allowed costs nothing, being
// faster gets the deployment blocked.
//
// With no Redis it degrades to a fixed sleep, which is correct for a single
// worker and honest about not being correct for several.
func (c *MBClient) throttle(ctx context.Context) error {
	if c.redis == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(mbMinInterval):
			return nil
		}
	}

	deadline := time.Now().Add(mbThrottleWait)
	for {
		ok, err := c.redis.SetNX(ctx, "music:mb:slot", "1", mbMinInterval).Result()
		if err != nil {
			// Redis unavailable: fall back to the local sleep rather than either
			// stalling the queue or hammering MusicBrainz.
			log.Warn().Err(err).Msg("music: lookup throttle unavailable, using local pacing")
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(mbMinInterval):
				return nil
			}
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("musicbrainz: throttle wait exceeded")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(150 * time.Millisecond):
		}
	}
}

/* ── pure helpers ─────────────────────────────────────────────────── */

// The shape of a MusicBrainz recording search. Named rather than inline so the
// helpers below can take the concrete types.
type (
	mbRecordingSearch struct {
		Recordings []mbRecording `json:"recordings"`
	}
	mbRecording struct {
		ID           string           `json:"id"`
		Score        int              `json:"score"`
		Title        string           `json:"title"`
		ArtistCredit []mbArtistCredit `json:"artist-credit"`
		Releases     []mbRelease      `json:"releases"`
		Tags         []mbTag          `json:"tags"`
	}
	mbArtistCredit struct {
		Name string `json:"name"`
	}
	mbRelease struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Date  string `json:"date"`
	}
	mbTag struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
)

// earliestRelease picks the ORIGINAL release rather than the first one returned.
// Search results lead with reissues and compilations often enough that taking
// index 0 dates half a library to whenever it was last repackaged.
//
// A release with no date cannot win, but it can still be the fallback when
// nothing has one — an album title with no year beats no album at all.
func earliestRelease(releases []mbRelease) *mbRelease {
	if len(releases) == 0 {
		return nil
	}
	best := releases[0]
	bestYear := yearFromDate(best.Date)
	for _, r := range releases[1:] {
		y := yearFromDate(r.Date)
		if y == nil {
			continue
		}
		if bestYear == nil || *y < *bestYear {
			best, bestYear = r, y
		}
	}
	return &best
}

// yearFromDate reads the leading year out of a MusicBrainz date, which may be
// "1997", "1997-06" or "1997-06-16". Anything else yields nil rather than a
// half-parsed number.
func yearFromDate(date string) *int {
	if len(date) < 4 {
		return nil
	}
	y, err := strconv.Atoi(date[:4])
	if err != nil || y < 1860 || y > 2200 {
		return nil
	}
	return &y
}

// topTag returns the community tag with the most votes — MusicBrainz has no
// single "genre" field, and its tags are a folksonomy, so the most-voted one is
// the closest thing to a consensus. Ties keep the first, which is arbitrary but
// stable.
func topTag(tags []mbTag) string {
	best, bestCount := "", 0
	for _, t := range tags {
		if t.Count > bestCount && strings.TrimSpace(t.Name) != "" {
			best, bestCount = strings.TrimSpace(t.Name), t.Count
		}
	}
	return best
}

// luceneQuote wraps a term in quotes and escapes what would otherwise be query
// syntax. Song titles contain quotes, colons and backslashes; unescaped, a
// title like `Say "Hello"` becomes a malformed query rather than a search.
func luceneQuote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('"')
	return b.String()
}
