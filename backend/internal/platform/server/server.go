// Package server holds the HTTP wire layer every module's handlers share:
// response encoding, the RFC 7807 error contract, request decoding, keyset
// cursors, and query-param parsing.
//
// MODULES.md:48 reserved this package in the canonical tree ("HTTP utilities:
// write JSON, error helpers") and it was never created, so eleven handlers each
// grew a private copy — writeJSON was byte-identical in 13 files, writeProblem
// in 11, encodeCursor in 9. Those copies are what this package replaces.
//
// It contains no business logic and imports no module, per the platform rule.
//
// # The error contract
//
// Problem is the ONLY error writer. Every error response is
// application/problem+json with {type, title, status, detail} (RFC 7807), which
// is what shared/openapi.yaml has always declared and what the frontend's
// problemDisplayMessage reads. The legacy {code, message} shape that
// writeErr/writeError emitted in five places is retired — see ADR-10.
package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// DefaultBodyLimit caps a decoded JSON request body. Handlers that legitimately
// take more (story chapter bodies) use DecodeLimit.
const DefaultBodyLimit int64 = 1 << 20 // 1 MiB

// ══ Responses ═══════════════════════════════════════════════════════════════

// JSON writes v as an application/json response. Encoding errors are swallowed:
// the status line is already on the wire by then, so there is nothing useful
// left to say to the client.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Problem writes an RFC 7807 error. typ is a problem-type URI — either
// "about:blank" for a bare status, or a module-scoped slug such as
// "comic/not-found" that the frontend catalog can key on.
func Problem(w http.ResponseWriter, status int, typ, title, detail string) {
	ProblemWith(w, status, typ, title, detail, nil)
}

// ProblemWith is Problem plus extension members (RFC 7807 §3.2) — e.g. comic's
// not-publishable carries the offending chapter list. Extension keys that
// collide with the four standard members are ignored rather than overwriting
// them, so a caller cannot accidentally rewrite `status`.
func ProblemWith(w http.ResponseWriter, status int, typ, title, detail string, ext map[string]any) {
	body := map[string]any{"type": typ, "title": title, "status": status, "detail": detail}
	for k, v := range ext {
		switch k {
		case "type", "title", "status", "detail":
			continue
		}
		body[k] = v
	}
	w.Header().Set("Content-Type", "application/problem+json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// BadRequest is a 400 with a bare problem type — for malformed input that has
// no domain meaning (unparseable JSON, a non-numeric limit).
func BadRequest(w http.ResponseWriter, detail string) {
	Problem(w, http.StatusBadRequest, "about:blank", "Bad Request", detail)
}

// Unauthorized is the response for a missing or unusable credential.
func Unauthorized(w http.ResponseWriter) {
	Problem(w, http.StatusUnauthorized, "about:blank", "Unauthorized", "authentication required")
}

// Forbidden is the response for a valid credential lacking the permission.
func Forbidden(w http.ResponseWriter, detail string) {
	Problem(w, http.StatusForbidden, "about:blank", "Forbidden", detail)
}

// NotFound takes a module-scoped type so the client can tell "no such comic"
// from "no such asset". Note the repo-wide convention: a resource the caller
// may not see is reported as 404, never 403, so 404 does not imply absence.
func NotFound(w http.ResponseWriter, typ, detail string) {
	Problem(w, http.StatusNotFound, typ, "Not Found", detail)
}

// Internal is the catch-all 500. The detail is deliberately opaque — internal
// error text belongs in logs, not in a response body.
func Internal(w http.ResponseWriter) {
	Problem(w, http.StatusInternalServerError, "about:blank", "Internal Server Error", "unexpected error")
}

// genericCodes are the transport-level codes that carry no domain meaning: a
// client learns nothing from "media/unauthorized" that the 401 did not already
// say, so these collapse to about:blank.
var genericCodes = map[string]bool{
	"unauthorized": true,
	"forbidden":    true,
	"bad_request":  true,
	"internal":     true,
	"not_found":    true,
}

// ProblemType converts a legacy short error code into a problem-type URI.
//
// The modules used to answer with a {code, message} body; that shape is retired
// in favour of RFC 7807, but the codes themselves were a real vocabulary worth
// keeping. A domain code becomes "<module>/<code>" with underscores normalised
// to the hyphens the rest of the problem types use ("email_taken" →
// "account/email-taken"), which is what the frontend catalog keys on.
func ProblemType(module, code string) string {
	if code == "" || genericCodes[code] {
		return "about:blank"
	}
	return module + "/" + strings.ReplaceAll(code, "_", "-")
}

// ══ Requests ════════════════════════════════════════════════════════════════

// Decode reads a JSON body of at most DefaultBodyLimit into v. On failure it has
// already written a 400 — the handler must return.
func Decode(w http.ResponseWriter, r *http.Request, v any) bool {
	return DecodeLimit(w, r, v, DefaultBodyLimit)
}

// DecodeLimit is Decode with an explicit cap, for the handlers that take more
// than a megabyte of JSON.
func DecodeLimit(w http.ResponseWriter, r *http.Request, v any, limit int64) bool {
	if err := json.NewDecoder(io.LimitReader(r.Body, limit)).Decode(v); err != nil {
		BadRequest(w, "invalid JSON body")
		return false
	}
	return true
}

// ══ Query params ════════════════════════════════════════════════════════════

// MaxAtoi caps AtoiSafe's result. A pagination limit is the only thing this
// parses, so a bound well above any real page size is the right shape: it makes
// "?limit=99999999999999999999" a clamp rather than an overflow.
const MaxAtoi = 1_000_000

// AtoiSafe parses a non-negative decimal, returning 0 for anything that is not
// one and clamping at MaxAtoi. It never errors — a bad limit falls back to the
// caller's default rather than failing the request.
func AtoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
		if n > MaxAtoi {
			return MaxAtoi
		}
	}
	return n
}

// Limit reads ?limit=, falling back to def and clamping to max.
func Limit(r *http.Request, def, max int) int {
	n := AtoiSafe(r.URL.Query().Get("limit"))
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// ══ Keyset cursors ══════════════════════════════════════════════════════════

// ErrBadCursor is returned by DecodeCursor for anything that is not one this
// package produced. Modules map it to their own 400 problem type.
var ErrBadCursor = errors.New("server: malformed cursor")

// EncodeCursor packs a keyset position — the sort key plus the tiebreaker id —
// into an opaque base64 token.
//
// The sort key is whatever the list is ordered by: an RFC3339Nano timestamp for
// updated_at lists, a display name for alphabetical ones.
func EncodeCursor(sortKey string, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(sortKey + "|" + id.String()))
}

// DecodeCursor unpacks EncodeCursor. It splits on the LAST separator, not the
// first: a display-name sort key may itself contain "|", while a UUID never
// can, so the last one is always the real boundary.
func DecodeCursor(s string) (sortKey string, id uuid.UUID, err error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return "", uuid.Nil, ErrBadCursor
	}
	raw := string(b)
	i := strings.LastIndex(raw, "|")
	if i < 0 {
		return "", uuid.Nil, ErrBadCursor
	}
	id, err = uuid.Parse(raw[i+1:])
	if err != nil {
		return "", uuid.Nil, ErrBadCursor
	}
	return raw[:i], id, nil
}
