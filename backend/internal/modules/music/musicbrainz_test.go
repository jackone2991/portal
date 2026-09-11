package music

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
)

// What matters about this client is what it REFUSES. Filling a library with
// plausible-but-wrong albums is worse than leaving fields empty, and much harder
// to undo — so the score floor, the query escaping and the earliest-release rule
// are the parts worth pinning down.

func testClient(t *testing.T, handler http.HandlerFunc) *MBClient {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	// No Redis: the throttle degrades to a local sleep, which these tests skip
	// past by keeping the interval irrelevant (one request each).
	return newMBClient(LookupConfig{
		Enabled:     true,
		Contact:     "test@example.com",
		BaseURL:     srv.URL,
		CoverArtURL: srv.URL,
	}, nil)
}

func recordingsJSON(t *testing.T, recs ...mbRecording) string {
	t.Helper()
	b, err := json.Marshal(mbRecordingSearch{Recordings: recs})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

/* ── the score floor ──────────────────────────────────────────────── */

// A low-scoring result is not a match. This is the single most important rule in
// the file: below the floor the honest answer is "we don't know".
func TestSearchRecordingRefusesALowScore(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(recordingsJSON(t, mbRecording{
			ID:    "b1a9c0de-0000-4000-8000-000000000001",
			Score: mbMinScore - 1,
			Title: "Something Vaguely Similar",
		})))
	})

	match, err := c.SearchRecording(context.Background(), "Artist", "Title")
	if err != nil {
		t.Fatalf("SearchRecording: %v", err)
	}
	if match != nil {
		t.Errorf("match = %+v, want nil — a %d-point result is a guess", match, mbMinScore-1)
	}
}

func TestSearchRecordingAcceptsAConfidentScore(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(recordingsJSON(t, mbRecording{
			ID:           "b1a9c0de-0000-4000-8000-000000000001",
			Score:        99,
			Title:        "Creep",
			ArtistCredit: []mbArtistCredit{{Name: "Radiohead"}},
			Releases: []mbRelease{
				{ID: "b1a9c0de-0000-4000-8000-0000000000a2", Title: "Pablo Honey", Date: "1993-02-22"},
			},
			Tags: []mbTag{{Name: "rock", Count: 12}, {Name: "britpop", Count: 3}},
		})))
	})

	match, err := c.SearchRecording(context.Background(), "Radiohead", "Creep")
	if err != nil {
		t.Fatalf("SearchRecording: %v", err)
	}
	if match == nil {
		t.Fatal("match = nil, want a confident match")
	}
	if match.Artist != "Radiohead" || match.Album != "Pablo Honey" {
		t.Errorf("match = %+v, want Radiohead / Pablo Honey", match)
	}
	if match.Year == nil || *match.Year != 1993 {
		t.Errorf("year = %v, want 1993", match.Year)
	}
	if match.Genre != "rock" {
		t.Errorf("genre = %q, want the most-voted tag %q", match.Genre, "rock")
	}
}

// The API returns results score-descending, so the first sub-threshold result
// ends the scan — a later high score would mean the ordering broke, not that a
// better match was hiding.
func TestSearchRecordingStopsAtTheFirstWeakResult(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(recordingsJSON(t,
			mbRecording{ID: "b1a9c0de-0000-4000-8000-000000000001", Score: 10, Title: "Weak"},
			mbRecording{ID: "b1a9c0de-0000-4000-8000-000000000002", Score: 99, Title: "Strong"},
		)))
	})

	match, err := c.SearchRecording(context.Background(), "", "x")
	if err != nil {
		t.Fatalf("SearchRecording: %v", err)
	}
	if match != nil {
		t.Errorf("match = %+v, want nil", match)
	}
}

func TestSearchRecordingIsDisabledWithoutContact(t *testing.T) {
	// Enabled but anonymous: MusicBrainz answers 403 to a generic agent, and a
	// shared fake identity is how every deployment gets blocked at once.
	c := newMBClient(LookupConfig{Enabled: true, Contact: "  "}, nil)
	if _, err := c.SearchRecording(context.Background(), "a", "b"); err != ErrLookupDisabled {
		t.Errorf("err = %v, want ErrLookupDisabled", err)
	}
}

/* ── query construction ───────────────────────────────────────────── */

// A title containing a quote must not become query syntax — unescaped, a song
// called `Say "Hello"` turns a search into a parse error.
func TestSearchRecordingEscapesTheQuery(t *testing.T) {
	var gotQuery string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(recordingsJSON(t)))
	})

	if _, err := c.SearchRecording(context.Background(), `AC\DC`, `Say "Hello"`); err != nil {
		t.Fatalf("SearchRecording: %v", err)
	}
	if !strings.Contains(gotQuery, `\"Hello\"`) {
		t.Errorf("query = %q, want the inner quotes escaped", gotQuery)
	}
	if !strings.Contains(gotQuery, `AC\\DC`) {
		t.Errorf("query = %q, want the backslash escaped", gotQuery)
	}
}

// With no artist the query must not carry an empty artist term, which would
// match nothing rather than searching on the title alone.
func TestSearchRecordingOmitsAnUnknownArtist(t *testing.T) {
	var gotQuery string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(recordingsJSON(t)))
	})

	if _, err := c.SearchRecording(context.Background(), "   ", "Creep"); err != nil {
		t.Fatalf("SearchRecording: %v", err)
	}
	if strings.Contains(gotQuery, "artist:") {
		t.Errorf("query = %q, want no artist term", gotQuery)
	}
}

/* ── release selection ────────────────────────────────────────────── */

// Search results lead with reissues and compilations often enough that taking
// index 0 dates half a library to whenever it was last repackaged.
func TestEarliestReleaseWinsOverTheFirstListed(t *testing.T) {
	got := earliestRelease([]mbRelease{
		{ID: "r1", Title: "Greatest Hits", Date: "2011-05-02"},
		{ID: "r2", Title: "Pablo Honey", Date: "1993-02-22"},
		{ID: "r3", Title: "Remastered", Date: "2009"},
	})
	if got == nil || got.Title != "Pablo Honey" {
		t.Errorf("earliestRelease = %+v, want Pablo Honey (1993)", got)
	}
}

// An album with no date should still be usable — a title with no year beats no
// album at all — but it must never beat one that has a date.
func TestEarliestReleaseHandlesMissingDates(t *testing.T) {
	if got := earliestRelease([]mbRelease{{ID: "r1", Title: "Undated"}}); got == nil || got.Title != "Undated" {
		t.Errorf("earliestRelease = %+v, want the undated release as a fallback", got)
	}
	got := earliestRelease([]mbRelease{
		{ID: "r1", Title: "Undated"},
		{ID: "r2", Title: "Dated", Date: "1999"},
	})
	if got == nil || got.Title != "Dated" {
		t.Errorf("earliestRelease = %+v, want the dated release to win", got)
	}
	if earliestRelease(nil) != nil {
		t.Error("earliestRelease(nil) should be nil")
	}
}

func TestYearFromDateAcceptsPartialDates(t *testing.T) {
	for date, want := range map[string]int{
		"1993":       1993,
		"1993-02":    1993,
		"1993-02-22": 1993,
	} {
		got := yearFromDate(date)
		if got == nil || *got != want {
			t.Errorf("yearFromDate(%q) = %v, want %d", date, got, want)
		}
	}
	// Junk and out-of-range values yield nil rather than a half-parsed number
	// that would then fail the CHECK constraint on insert.
	for _, bad := range []string{"", "19", "abcd", "0100", "9999"} {
		if got := yearFromDate(bad); got != nil {
			t.Errorf("yearFromDate(%q) = %v, want nil", bad, *got)
		}
	}
}

func TestTopTagPicksTheMostVoted(t *testing.T) {
	got := topTag([]mbTag{{Name: "pop", Count: 2}, {Name: "rock", Count: 9}, {Name: "  ", Count: 99}})
	if got != "rock" {
		t.Errorf("topTag = %q, want %q (blank names ignored)", got, "rock")
	}
	if topTag(nil) != "" {
		t.Error("topTag(nil) should be empty")
	}
}

/* ── cover art ────────────────────────────────────────────────────── */

// A release with no artwork is the common case, not a failure.
func TestFetchCoverArtTreats404AsNoArtwork(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	data, _, err := c.FetchCoverArt(context.Background(), mustUUID(t, "b1a9c0de-0000-4000-8000-0000000000a2"))
	if err != nil || data != nil {
		t.Errorf("FetchCoverArt = (%d bytes, %v), want (nil, nil)", len(data), err)
	}
}

// The archive redirects to the Internet Archive; a non-image body means the
// redirect landed somewhere unhelpful, and storing an HTML page as album art is
// worse than storing nothing.
func TestFetchCoverArtRejectsANonImageBody(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html>not an image</html>"))
	})
	data, _, err := c.FetchCoverArt(context.Background(), mustUUID(t, "b1a9c0de-0000-4000-8000-0000000000a2"))
	if err != nil || data != nil {
		t.Errorf("FetchCoverArt = (%d bytes, %v), want (nil, nil)", len(data), err)
	}
}

// Their explicit back-off signals must surface as errors so the task retries,
// rather than being recorded as "no match" — which would be a permanent wrong
// answer to a temporary problem.
func TestSearchRecordingSurfacesRateLimiting(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	if _, err := c.SearchRecording(context.Background(), "a", "b"); err == nil ||
		!strings.Contains(err.Error(), "rate limited") {
		t.Errorf("err = %v, want a rate-limit error", err)
	}
}

func mustUUID(t *testing.T, s string) uuid.UUID {
	t.Helper()
	id, err := uuid.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return id
}
