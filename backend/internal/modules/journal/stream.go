package journal

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	defaultStreamLimit = 30
	maxStreamLimit     = 50
)

// StreamCard is one rendered life-stream card (SPEC-06 P0.2). Journal cards carry
// BodyMd/Mood; system cards carry a synthesized Title/Href from the render mapping.
type StreamCard struct {
	ID           uuid.UUID
	SourceModule string
	EventType    string
	OccurredAt   time.Time
	BodyMd       *string
	Mood         *string
	Title        string
	Href         string
	Payload      json.RawMessage
}

type StreamResult struct {
	Items      []StreamCard
	NextCursor string
}

// Stream returns the caller's merged timeline (P0.2): journal items full, system
// items compact with synthesized title/href. Cursor keyed on (occurred_at, id).
func (s *Service) Stream(ctx context.Context, userID uuid.UUID, cursor string, limit int) (StreamResult, error) {
	if limit <= 0 || limit > maxStreamLimit {
		limit = defaultStreamLimit
	}
	in := StreamListInput{UserID: userID, Limit: limit + 1}
	if cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err != nil {
			return StreamResult{}, ErrBadCursor
		}
		in.CursorAt, in.CursorID = at, id
	}
	rows, err := s.repo.ListStream(ctx, in)
	if err != nil {
		return StreamResult{}, err
	}
	var res StreamResult
	if len(rows) > limit {
		last := rows[limit-1]
		res.NextCursor = base64.RawURLEncoding.EncodeToString([]byte(last.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + last.ID.String()))
		rows = rows[:limit]
	}
	for _, r := range rows {
		res.Items = append(res.Items, toCard(r))
	}
	return res, nil
}

func toCard(r StreamItem) StreamCard {
	c := StreamCard{ID: r.ID, SourceModule: r.SourceModule, EventType: r.EventType, OccurredAt: r.OccurredAt, Payload: r.Payload}
	if r.SourceModule == "journal" {
		c.BodyMd, c.Mood = r.BodyMd, r.Mood
		return c
	}
	c.Title, c.Href = renderSystem(r.EventType, r.RefID, r.Payload)
	return c
}

// renderSystem is the per-event-type render mapping (P0.2). An unmapped type
// falls back to a generic card (never an error).
func renderSystem(eventType string, refID uuid.UUID, payload json.RawMessage) (title, href string) {
	var p map[string]any
	_ = json.Unmarshal(payload, &p)
	str := func(k string) string {
		if v, ok := p[k].(string); ok {
			return v
		}
		return ""
	}
	num := func(k string) int64 {
		if v, ok := p[k].(float64); ok {
			return int64(v)
		}
		return 0
	}
	switch eventType {
	case "media:asset_ready":
		t := str("title")
		if t == "" {
			t = "A file"
		}
		return t + " is ready", "/library/media"
	case "media:playback_completed":
		t := str("title")
		if t == "" {
			t = "a video"
		}
		return "Finished watching " + t, "/library/media"
	case "bank:transaction_created", "bank:transaction_updated":
		amt := num("amount")
		if isTruthy(p["is_transfer"]) {
			return fmt.Sprintf("Moved %s", formatVND(amt)), "/bank/transactions"
		}
		if str("direction") == "credit" {
			return fmt.Sprintf("Income %s", formatVND(amt)), "/bank/transactions"
		}
		return fmt.Sprintf("Spent %s", formatVND(amt)), "/bank/transactions"
	case "comic:chapter_published":
		t := str("title")
		if t == "" {
			t = "A chapter"
		}
		return t + " published", "/library/comic"
	case "people:birthday_upcoming":
		name := str("display_name")
		days := num("days_until")
		person := str("person_id")
		when := "soon"
		if days == 0 {
			when = "today"
		} else if days == 1 {
			when = "tomorrow"
		} else {
			when = fmt.Sprintf("in %d days", days)
		}
		return fmt.Sprintf("%s — birthday %s", name, when), "/people/" + person
	default:
		return strings.TrimPrefix(eventType, "media:"), ""
	}
}

func isTruthy(v any) bool { b, _ := v.(bool); return b }

// formatVND groups integer minor units as VND thousands (mirrors the frontend).
func formatVND(minor int64) string {
	neg := minor < 0
	if neg {
		minor = -minor
	}
	s := fmt.Sprintf("%d", minor)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	if neg {
		return "-" + b.String()
	}
	return b.String()
}

// ══ system-event ingest (P0.1b) — called by the consumer task handlers ══

func (s *Service) OnAssetReady(ctx context.Context, payload []byte) error {
	var p struct {
		AssetID     string `json:"asset_id"`
		OwnerUserID string `json:"owner_user_id"`
		Origin      string `json:"origin"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	if p.Origin == "import" {
		return nil // zip-import flood guard
	}
	return s.insertSystem(ctx, payload, p.OwnerUserID, "media", "media:asset_ready", p.AssetID, time.Now())
}

func (s *Service) OnPlaybackCompleted(ctx context.Context, payload []byte) error {
	var p struct {
		AssetID string `json:"asset_id"`
		UserID  string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	return s.insertSystem(ctx, payload, p.UserID, "media", "media:playback_completed", p.AssetID, time.Now())
}

func (s *Service) OnAssetDeleted(ctx context.Context, payload []byte) error {
	var p struct {
		AssetID string `json:"asset_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	assetID, err := uuid.Parse(p.AssetID)
	if err != nil {
		return nil
	}
	return s.repo.DeleteStreamByRef(ctx, "media", assetID)
}

func (s *Service) OnBankCreated(ctx context.Context, payload []byte) error {
	return s.bankUpsert(ctx, payload, false)
}
func (s *Service) OnBankUpdated(ctx context.Context, payload []byte) error {
	return s.bankUpsert(ctx, payload, true)
}

func (s *Service) OnBankDeleted(ctx context.Context, payload []byte) error {
	userID, refID, _, ok := bankRef(payload)
	if !ok {
		return nil
	}
	_ = userID
	return s.repo.DeleteStreamItem(ctx, "bank", "bank:transaction_created", refID)
}

func (s *Service) OnBirthdayUpcoming(ctx context.Context, payload []byte) error {
	var p struct {
		NoticeID string `json:"notice_id"`
		UserID   string `json:"user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	return s.insertSystem(ctx, payload, p.UserID, "people", "people:birthday_upcoming", p.NoticeID, time.Now())
}

func (s *Service) OnComicPublished(ctx context.Context, payload []byte) error {
	var p struct {
		ChapterID   string `json:"chapter_id"`
		OwnerUserID string `json:"owner_user_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	return s.insertSystem(ctx, payload, p.OwnerUserID, "comic", "comic:chapter_published", p.ChapterID, time.Now())
}

// OnComicDeleted removes the published-chapter card when a chapter (or a whole
// comic, one event per chapter) is deleted — SPEC-02 P1.9 / SPEC-06 P0.1. Keyed
// on chapter_id, the same ref the published card used. Idempotent: a delete for
// a chapter that was never published (or already removed) is a no-op.
func (s *Service) OnComicDeleted(ctx context.Context, payload []byte) error {
	var p struct {
		ChapterID string `json:"chapter_id"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return nil
	}
	chID, err := uuid.Parse(p.ChapterID)
	if err != nil {
		return nil
	}
	return s.repo.DeleteStreamItem(ctx, "comic", "comic:chapter_published", chID)
}

// bankUpsert projects a bank:transaction_* create/update. Transfers collapse to
// one item keyed by transfer_id (P0.1). occurred_at from the payload's date.
func (s *Service) bankUpsert(ctx context.Context, payload []byte, update bool) error {
	userID, refID, occurredAt, ok := bankRef(payload)
	if !ok {
		return nil
	}
	if update {
		return s.repo.UpsertStreamItem(ctx, userID, "bank", "bank:transaction_created", refID, payload, occurredAt)
	}
	return s.repo.InsertStreamItem(ctx, userID, "bank", "bank:transaction_created", refID, payload, occurredAt)
}

// bankRef pulls (user, ref_id, occurred_at) from a bank event payload. ref_id is
// transfer_id when is_transfer, else transaction_id (SPEC-03 P0.7 / SPEC-06 P0.1).
func bankRef(payload []byte) (uuid.UUID, uuid.UUID, time.Time, bool) {
	var p struct {
		TransactionID string `json:"transaction_id"`
		UserID        string `json:"user_id"`
		TransferID    string `json:"transfer_id"`
		IsTransfer    bool   `json:"is_transfer"`
		OccurredAt    string `json:"occurred_at"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	user, err := uuid.Parse(p.UserID)
	if err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	refStr := p.TransactionID
	if p.IsTransfer && p.TransferID != "" {
		refStr = p.TransferID
	}
	ref, err := uuid.Parse(refStr)
	if err != nil {
		return uuid.Nil, uuid.Nil, time.Time{}, false
	}
	occurredAt := time.Now()
	if t, err := time.Parse("2006-01-02", p.OccurredAt); err == nil {
		occurredAt = t
	}
	return user, ref, occurredAt, true
}

func (s *Service) insertSystem(ctx context.Context, payload []byte, userStr, module, eventType, refStr string, occurredAt time.Time) error {
	user, err := uuid.Parse(userStr)
	if err != nil {
		return nil
	}
	ref, err := uuid.Parse(refStr)
	if err != nil {
		return nil
	}
	return s.repo.InsertStreamItem(ctx, user, module, eventType, ref, payload, occurredAt)
}
