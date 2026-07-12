package journal

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	journalapi "github.com/portal/backend/internal/modules/journal/api"
)

type streamKey struct {
	src, evt string
	ref      uuid.UUID
}
type streamRow struct {
	id         uuid.UUID
	user       uuid.UUID
	payload    json.RawMessage
	occurredAt time.Time
}

// fakeRepo is an in-memory Repository for service tests.
type fakeRepo struct {
	rows      map[uuid.UUID]Entry
	stream    map[streamKey]*streamRow
	createErr error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{rows: map[uuid.UUID]Entry{}, stream: map[streamKey]*streamRow{}}
}

func (f *fakeRepo) InsertStreamItem(_ context.Context, user uuid.UUID, src, evt string, ref uuid.UUID, payload json.RawMessage, occ time.Time) error {
	k := streamKey{src, evt, ref}
	if _, ok := f.stream[k]; ok {
		return nil // ON CONFLICT DO NOTHING
	}
	f.stream[k] = &streamRow{id: uuid.New(), user: user, payload: payload, occurredAt: occ}
	return nil
}
func (f *fakeRepo) UpsertStreamItem(_ context.Context, user uuid.UUID, src, evt string, ref uuid.UUID, payload json.RawMessage, occ time.Time) error {
	k := streamKey{src, evt, ref}
	if r, ok := f.stream[k]; ok {
		r.payload, r.occurredAt = payload, occ
		return nil
	}
	f.stream[k] = &streamRow{id: uuid.New(), user: user, payload: payload, occurredAt: occ}
	return nil
}
func (f *fakeRepo) DeleteStreamItem(_ context.Context, src, evt string, ref uuid.UUID) error {
	delete(f.stream, streamKey{src, evt, ref})
	return nil
}
func (f *fakeRepo) DeleteStreamByRef(_ context.Context, src string, ref uuid.UUID) error {
	for k := range f.stream {
		if k.src == src && k.ref == ref {
			delete(f.stream, k)
		}
	}
	return nil
}
func (f *fakeRepo) ListStream(_ context.Context, in StreamListInput) ([]StreamItem, error) {
	var out []StreamItem
	for k, r := range f.stream {
		if r.user != in.UserID {
			continue
		}
		out = append(out, StreamItem{ID: r.id, SourceModule: k.src, EventType: k.evt, RefID: k.ref, Payload: r.payload, OccurredAt: r.occurredAt})
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].OccurredAt.After(out[i].OccurredAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if in.Limit > 0 && len(out) > in.Limit {
		out = out[:in.Limit]
	}
	return out, nil
}
func (f *fakeRepo) streamCount() int { return len(f.stream) }

func (f *fakeRepo) CreateEntry(_ context.Context, in CreateEntryInput) (Entry, error) {
	if f.createErr != nil {
		return Entry{}, f.createErr
	}
	e := Entry{
		ID: uuid.New(), UserID: in.UserID, BodyMd: in.BodyMd, Mood: in.Mood,
		OccurredAt: in.OccurredAt, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.rows[e.ID] = e
	return e, nil
}

func (f *fakeRepo) GetEntry(_ context.Context, userID, id uuid.UUID) (Entry, error) {
	e, ok := f.rows[id]
	if !ok || e.UserID != userID {
		return Entry{}, ErrEntryNotFound
	}
	return e, nil
}

func (f *fakeRepo) ListByUserCursor(_ context.Context, in ListInput) ([]Entry, error) {
	var out []Entry
	for _, e := range f.rows {
		if e.UserID != in.UserID {
			continue
		}
		if !in.CursorAt.IsZero() && !e.OccurredAt.Before(in.CursorAt) {
			continue // simplistic: only strictly-older rows (enough for the count test)
		}
		out = append(out, e)
	}
	// newest first
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].OccurredAt.After(out[i].OccurredAt) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	if in.Limit > 0 && len(out) > in.Limit {
		out = out[:in.Limit]
	}
	return out, nil
}

func (f *fakeRepo) PatchEntry(_ context.Context, in PatchEntryInput) (Entry, error) {
	e, ok := f.rows[in.ID]
	if !ok || e.UserID != in.UserID {
		return Entry{}, ErrEntryNotFound
	}
	if in.BodyMd != nil {
		e.BodyMd = *in.BodyMd
	}
	if in.Mood != nil {
		e.Mood = in.Mood
	}
	if in.OccurredAt != nil {
		e.OccurredAt = *in.OccurredAt
	}
	e.UpdatedAt = time.Now()
	f.rows[in.ID] = e
	return e, nil
}

func (f *fakeRepo) DeleteEntry(_ context.Context, userID, id uuid.UUID) error {
	e, ok := f.rows[id]
	if !ok || e.UserID != userID {
		return ErrEntryNotFound
	}
	delete(f.rows, id)
	return nil
}

// spyPublisher records Publish calls.
type spyPublisher struct {
	calls []string // event names published
}

func (s *spyPublisher) Publish(_ context.Context, name string, _ any) error {
	s.calls = append(s.calls, name)
	return nil
}

func newSvc() (*Service, *fakeRepo, *spyPublisher) {
	repo := newFakeRepo()
	pub := &spyPublisher{}
	return &Service{repo: repo, events: pub}, repo, pub
}

func TestCreateEmitsExactlyOnceAfterCommit(t *testing.T) {
	svc, _, pub := newSvc()
	_, err := svc.Create(context.Background(), CreateParams{UserID: uuid.New(), BodyMd: "hello"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if len(pub.calls) != 1 || pub.calls[0] != journalapi.EventEntryCreated {
		t.Fatalf("want exactly one %q publish, got %v", journalapi.EventEntryCreated, pub.calls)
	}
}

func TestCreateValidationPublishesNothing(t *testing.T) {
	cases := []struct {
		name string
		p    CreateParams
		want error
	}{
		{"asset_ids rejected", CreateParams{UserID: uuid.New(), BodyMd: "x", HasAssetIDs: true}, ErrInvalidAsset},
		{"empty body", CreateParams{UserID: uuid.New(), BodyMd: ""}, ErrInvalidBody},
		{"too-long body", CreateParams{UserID: uuid.New(), BodyMd: strings.Repeat("a", 20001)}, ErrInvalidBody},
		{"blank mood", CreateParams{UserID: uuid.New(), BodyMd: "ok", Mood: ptr("   ")}, ErrInvalidMood},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, pub := newSvc()
			_, err := svc.Create(context.Background(), tc.p)
			if !errors.Is(err, tc.want) {
				t.Fatalf("want %v, got %v", tc.want, err)
			}
			if len(pub.calls) != 0 {
				t.Fatalf("validation failure must publish nothing, got %v", pub.calls)
			}
		})
	}
}

func TestCreateRollbackPublishesNothing(t *testing.T) {
	repo := newFakeRepo()
	repo.createErr = errors.New("db down")
	pub := &spyPublisher{}
	svc := &Service{repo: repo, events: pub}
	if _, err := svc.Create(context.Background(), CreateParams{UserID: uuid.New(), BodyMd: "ok"}); err == nil {
		t.Fatal("want error")
	}
	if len(pub.calls) != 0 {
		t.Fatalf("rolled-back create must publish nothing, got %v", pub.calls)
	}
}

func TestGetOwnerScopedNotFound(t *testing.T) {
	svc, _, _ := newSvc()
	owner := uuid.New()
	e, _ := svc.Create(context.Background(), CreateParams{UserID: owner, BodyMd: "mine"})
	if _, err := svc.Get(context.Background(), uuid.New(), e.ID); !errors.Is(err, ErrEntryNotFound) {
		t.Fatalf("another user must get ErrEntryNotFound, got %v", err)
	}
	if _, err := svc.Get(context.Background(), owner, e.ID); err != nil {
		t.Fatalf("owner fetch: %v", err)
	}
}

func TestListCursorPaginates(t *testing.T) {
	svc, _, _ := newSvc()
	u := uuid.New()
	base := time.Now().UTC()
	for i := 0; i < 5; i++ {
		at := base.Add(-time.Duration(i) * time.Hour)
		if _, err := svc.Create(context.Background(), CreateParams{UserID: u, BodyMd: "e", OccurredAt: &at}); err != nil {
			t.Fatal(err)
		}
	}
	p1, err := svc.List(context.Background(), u, "", 2)
	if err != nil || len(p1.Items) != 2 || p1.NextCursor == "" {
		t.Fatalf("page1: items=%d cursor=%q err=%v", len(p1.Items), p1.NextCursor, err)
	}
	p2, err := svc.List(context.Background(), u, p1.NextCursor, 2)
	if err != nil || len(p2.Items) == 0 {
		t.Fatalf("page2: items=%d err=%v", len(p2.Items), err)
	}
	// no overlap between page 1 and page 2
	seen := map[uuid.UUID]bool{}
	for _, e := range p1.Items {
		seen[e.ID] = true
	}
	for _, e := range p2.Items {
		if seen[e.ID] {
			t.Fatalf("duplicate entry %s across pages", e.ID)
		}
	}
}

func ptr(s string) *string { return &s }

// ── life-stream projection tests (SPEC-06) ────────────────────────────

func newStreamSvc() (*Service, *fakeRepo) {
	f := newFakeRepo()
	return &Service{repo: f}, f
}

func TestStreamIdempotency(t *testing.T) {
	svc, f := newStreamSvc()
	ctx := context.Background()
	payload := []byte(`{"asset_id":"` + uuid.NewString() + `","owner_user_id":"` + uuid.NewString() + `","title":"clip"}`)
	_ = svc.OnAssetReady(ctx, payload)
	_ = svc.OnAssetReady(ctx, payload) // Asynq redelivery
	if f.streamCount() != 1 {
		t.Fatalf("stream items = %d, want 1 (idempotent)", f.streamCount())
	}
}

func TestStreamImportSkipped(t *testing.T) {
	svc, f := newStreamSvc()
	payload := []byte(`{"asset_id":"` + uuid.NewString() + `","owner_user_id":"` + uuid.NewString() + `","origin":"import"}`)
	_ = svc.OnAssetReady(context.Background(), payload)
	if f.streamCount() != 0 {
		t.Fatalf("import asset produced %d items, want 0 (flood guard)", f.streamCount())
	}
}

func TestStreamTransferCollapse(t *testing.T) {
	svc, f := newStreamSvc()
	ctx := context.Background()
	user := uuid.NewString()
	transfer := uuid.NewString()
	leg := func() []byte {
		return []byte(`{"transaction_id":"` + uuid.NewString() + `","user_id":"` + user + `","transfer_id":"` + transfer + `","is_transfer":true,"amount":5000000,"occurred_at":"2026-06-10"}`)
	}
	_ = svc.OnBankCreated(ctx, leg())
	_ = svc.OnBankCreated(ctx, leg()) // the other leg, same transfer_id
	if f.streamCount() != 1 {
		t.Fatalf("two transfer legs → %d items, want 1 (collapsed on transfer_id)", f.streamCount())
	}
}

func TestStreamAssetDeletedRemoves(t *testing.T) {
	svc, f := newStreamSvc()
	ctx := context.Background()
	asset := uuid.NewString()
	owner := uuid.NewString()
	_ = svc.OnAssetReady(ctx, []byte(`{"asset_id":"`+asset+`","owner_user_id":"`+owner+`","title":"clip"}`))
	_ = svc.OnPlaybackCompleted(ctx, []byte(`{"asset_id":"`+asset+`","user_id":"`+owner+`","title":"clip"}`))
	if f.streamCount() != 2 {
		t.Fatalf("setup = %d items, want 2", f.streamCount())
	}
	_ = svc.OnAssetDeleted(ctx, []byte(`{"asset_id":"`+asset+`","owner_user_id":"`+owner+`"}`))
	if f.streamCount() != 0 {
		t.Fatalf("after asset delete = %d items, want 0 (all event_types for the ref)", f.streamCount())
	}
}

func TestStreamReadMapping(t *testing.T) {
	svc, _ := newStreamSvc()
	ctx := context.Background()
	user := uuid.New()
	person := uuid.NewString()
	_ = svc.OnBirthdayUpcoming(ctx, []byte(`{"notice_id":"`+uuid.NewString()+`","person_id":"`+person+`","user_id":"`+user.String()+`","display_name":"Mẹ","days_until":3}`))

	res, err := svc.Stream(ctx, user, "", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != 1 {
		t.Fatalf("stream items = %d, want 1", len(res.Items))
	}
	c := res.Items[0]
	if c.SourceModule != "people" || c.Title == "" || c.Href != "/people/"+person {
		t.Fatalf("mapped card = %+v, want people title + /people/%s href", c, person)
	}
}
