package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	return body
}

// ══ the error contract ══════════════════════════════════════════════════════

// Every error response must carry all four RFC 7807 members. This is the shape
// shared/openapi.yaml declares and the frontend's problemDisplayMessage reads.
func TestProblemEmitsAllFourStandardMembers(t *testing.T) {
	rec := httptest.NewRecorder()
	Problem(rec, http.StatusNotFound, "comic/not-found", "Not Found", "no such comic")

	if ct := rec.Header().Get("Content-Type"); ct != "application/problem+json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store — an error must not be cached", cc)
	}
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	body := decodeBody(t, rec)
	for k, want := range map[string]any{
		"type":   "comic/not-found",
		"title":  "Not Found",
		"detail": "no such comic",
		"status": float64(404),
	} {
		if body[k] != want {
			t.Errorf("problem.%s = %v, want %v", k, body[k], want)
		}
	}
}

// status is derived from the argument, never from the caller's map — otherwise
// an extension member could contradict the status line.
func TestProblemWithCannotOverrideStandardMembers(t *testing.T) {
	rec := httptest.NewRecorder()
	ProblemWith(rec, http.StatusUnprocessableEntity, "comic/not-publishable", "Not publishable",
		"every chapter needs a page",
		map[string]any{
			"chapters": []string{"Chương 3"},
			"status":   999,      // must be ignored
			"detail":   "hijack", // must be ignored
		})

	body := decodeBody(t, rec)
	if body["status"] != float64(422) {
		t.Errorf("problem.status = %v, want 422 — an extension member overrode it", body["status"])
	}
	if body["detail"] != "every chapter needs a page" {
		t.Errorf("problem.detail = %v — an extension member overrode it", body["detail"])
	}
	chapters, ok := body["chapters"].([]any)
	if !ok || len(chapters) != 1 || chapters[0] != "Chương 3" {
		t.Errorf("extension member `chapters` = %v, want it preserved", body["chapters"])
	}
}

func TestGenericProblemHelpers(t *testing.T) {
	for _, tc := range []struct {
		name   string
		write  func(*httptest.ResponseRecorder)
		status int
		detail string
	}{
		{"BadRequest", func(r *httptest.ResponseRecorder) { BadRequest(r, "invalid JSON body") }, 400, "invalid JSON body"},
		{"Unauthorized", func(r *httptest.ResponseRecorder) { Unauthorized(r) }, 401, "authentication required"},
		{"Forbidden", func(r *httptest.ResponseRecorder) { Forbidden(r, "nope") }, 403, "nope"},
		{"Internal", func(r *httptest.ResponseRecorder) { Internal(r) }, 500, "unexpected error"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			tc.write(rec)
			if rec.Code != tc.status {
				t.Errorf("status = %d, want %d", rec.Code, tc.status)
			}
			body := decodeBody(t, rec)
			if body["detail"] != tc.detail {
				t.Errorf("detail = %v, want %q", body["detail"], tc.detail)
			}
			if body["type"] != "about:blank" {
				t.Errorf("type = %v, want about:blank", body["type"])
			}
		})
	}
}

// NotFound takes a module-scoped type: the client must be able to tell "no such
// comic" from "no such asset" when both arrive as 404.
func TestNotFoundCarriesModuleScopedType(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFound(rec, "bank/not-found", "resource not found")
	if body := decodeBody(t, rec); body["type"] != "bank/not-found" {
		t.Errorf("type = %v", body["type"])
	}
}

func TestJSONSetsNoStore(t *testing.T) {
	rec := httptest.NewRecorder()
	JSON(rec, http.StatusCreated, map[string]any{"id": "abc"})

	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want 201", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", cc)
	}
	if body := decodeBody(t, rec); body["id"] != "abc" {
		t.Errorf("body = %v", body)
	}
}

// ══ request decoding ════════════════════════════════════════════════════════

func TestDecodeRejectsMalformedJSONWithProblem(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"nope`))

	var body struct{ Nope string }
	if Decode(rec, req, &body) {
		t.Fatal("Decode accepted malformed JSON")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/problem+json") {
		t.Errorf("Content-Type = %q, want a Problem", ct)
	}
}

func TestDecodeAcceptsValidBody(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"title":"Chương 1"}`))

	var body struct {
		Title string `json:"title"`
	}
	if !Decode(rec, req, &body) {
		t.Fatalf("Decode rejected a valid body: %q", rec.Body.String())
	}
	if body.Title != "Chương 1" {
		t.Errorf("title = %q", body.Title)
	}
}

// The limit truncates the reader, which makes the JSON unparseable — so an
// oversized body is a 400, not a silently accepted prefix.
func TestDecodeLimitRejectsOversizedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	oversized := `{"pad":"` + strings.Repeat("x", 4096) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(oversized))

	var body map[string]string
	if DecodeLimit(rec, req, &body, 128) {
		t.Fatal("DecodeLimit accepted a body past its cap")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// ══ query params ════════════════════════════════════════════════════════════

func TestAtoiSafe(t *testing.T) {
	for in, want := range map[string]int{
		"":                     0,
		"0":                    0,
		"25":                   25,
		"-5":                   0, // the '-' is not a digit → 0, not a negative page size
		"12abc":                0,
		" 12":                  0,
		"99999999999999999999": MaxAtoi, // clamps instead of overflowing int
		"1000001":              MaxAtoi,
	} {
		if got := AtoiSafe(in); got != want {
			t.Errorf("AtoiSafe(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestLimitDefaultsAndClamps(t *testing.T) {
	for _, tc := range []struct {
		query string
		want  int
	}{
		{"", 20},          // absent → default
		{"?limit=0", 20},  // zero → default
		{"?limit=x", 20},  // unparseable → default
		{"?limit=5", 5},   // in range
		{"?limit=50", 50}, // at max
		{"?limit=999", 50},
	} {
		req := httptest.NewRequest(http.MethodGet, "/list"+tc.query, nil)
		if got := Limit(req, 20, 50); got != tc.want {
			t.Errorf("Limit(%q) = %d, want %d", tc.query, got, tc.want)
		}
	}
}

// ══ keyset cursors ══════════════════════════════════════════════════════════

func TestCursorRoundTripsTimestampKey(t *testing.T) {
	id := uuid.MustParse("33333333-3333-4333-8333-333333333333")
	key := time.Date(2026, 8, 25, 10, 30, 0, 123456789, time.UTC).Format(time.RFC3339Nano)

	gotKey, gotID, err := DecodeCursor(EncodeCursor(key, id))
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if gotKey != key || gotID != id {
		t.Errorf("round trip = (%q, %v), want (%q, %v)", gotKey, gotID, key, id)
	}
}

// A display-name sort key may contain the separator. Splitting on the LAST one
// is what makes that safe — a UUID never contains "|".
func TestCursorRoundTripsSortKeyContainingSeparator(t *testing.T) {
	id := uuid.MustParse("44444444-4444-4444-8444-444444444444")
	key := "Nguyễn | Thu Hà"

	gotKey, gotID, err := DecodeCursor(EncodeCursor(key, id))
	if err != nil {
		t.Fatalf("DecodeCursor: %v", err)
	}
	if gotKey != key {
		t.Errorf("sort key = %q, want %q — the cursor split on the wrong separator", gotKey, key)
	}
	if gotID != id {
		t.Errorf("id = %v, want %v", gotID, id)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	for _, in := range []string{
		"not-base64!!",
		"",                                    // empty
		"bm8tc2VwYXJhdG9y",                    // valid base64, no "|"
		"a2V5fG5vdC1hLXV1aWQ",                 // "key|not-a-uuid"
		"MjAyNi0wOC0yNXwxMTExMTExMS0xMTExLTQ", // truncated
	} {
		if _, _, err := DecodeCursor(in); err == nil {
			t.Errorf("DecodeCursor(%q) accepted garbage", in)
		}
	}
}

// The opaque token must not leak the sort key in plain sight — callers treat it
// as opaque, and a readable cursor invites clients to construct their own.
func TestCursorIsURLSafe(t *testing.T) {
	id := uuid.New()
	got := EncodeCursor("2026-08-25T10:30:00Z", id)
	if strings.ContainsAny(got, "+/=") {
		t.Errorf("cursor %q contains characters that need URL escaping", got)
	}
}
