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
	mediaapi "github.com/portal/backend/internal/modules/media/api"
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

// fakeRepo is an in-memory Repository for service tests. With requireScope
// set, the worker-side deletes and the Attachment strip count every call that
// arrives without the scopeKey marker — the test's stand-in for "portal_app
// errors on an unscoped RLS table". (The inserts are not guarded: the seed
// helpers call CreateEntry with a bare context.)
type fakeRepo struct {
	rows          map[uuid.UUID]Entry
	stream        map[streamKey]*streamRow
	createErr     error
	requireScope  bool
	unscopedCalls int
	stripCalls    int
}

func (f *fakeRepo) checkScope(ctx context.Context) {
	if f.requireScope && ctx.Value(scopeKey{}) == nil {
		f.unscopedCalls++
	}
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
func (f *fakeRepo) DeleteStreamItem(ctx context.Context, src, evt string, ref uuid.UUID) error {
	f.checkScope(ctx)
	delete(f.stream, streamKey{src, evt, ref})
	return nil
}
func (f *fakeRepo) DeleteStreamByRef(ctx context.Context, src string, ref uuid.UUID) error {
	f.checkScope(ctx)
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
		it := StreamItem{ID: r.id, SourceModule: k.src, EventType: k.evt, RefID: k.ref, Payload: r.payload, OccurredAt: r.occurredAt}
		if k.src == "journal" { // the LEFT JOIN onto journal_entries
			if e, ok := f.rows[k.ref]; ok {
				body := e.BodyMd
				it.BodyMd, it.Mood, it.AssetIDs, it.Location = &body, e.Mood, e.AssetIDs, e.Location
			}
		}
		out = append(out, it)
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
		ID: uuid.New(), UserID: in.UserID, BodyMd: in.BodyMd, Mood: in.Mood, AssetIDs: in.AssetIDs, Location: in.Location,
		OccurredAt: in.OccurredAt, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	f.rows[e.ID] = e
	_ = f.InsertStreamItem(context.Background(), e.UserID, "journal", "journal:entry_created", e.ID, nil, e.OccurredAt)
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
	if in.AssetIDs != nil {
		e.AssetIDs = *in.AssetIDs
	}
	if in.SetLocation {
		e.Location = in.Location // nil clears
	}
	if in.OccurredAt != nil {
		e.OccurredAt = *in.OccurredAt
	}
	e.UpdatedAt = time.Now()
	f.rows[in.ID] = e
	return e, nil
}

// StripAssetFromEntries mirrors the SQL: only the owner's rows, only rows that
// carry the id, order of the rest preserved; the row count is what changed.
func (f *fakeRepo) StripAssetFromEntries(ctx context.Context, userID, assetID uuid.UUID) (int, error) {
	f.checkScope(ctx)
	f.stripCalls++
	n := 0
	for id, e := range f.rows {
		if e.UserID != userID {
			continue
		}
		kept := make([]uuid.UUID, 0, len(e.AssetIDs))
		for _, a := range e.AssetIDs {
			if a != assetID {
				kept = append(kept, a)
			}
		}
		if len(kept) == len(e.AssetIDs) {
			continue
		}
		e.AssetIDs, e.UpdatedAt = kept, time.Now()
		f.rows[id] = e
		n++
	}
	return n, nil
}

func (f *fakeRepo) DeleteEntry(_ context.Context, userID, id uuid.UUID) error {
	e, ok := f.rows[id]
	if !ok || e.UserID != userID {
		return ErrEntryNotFound
	}
	delete(f.rows, id)
	return nil
}

// fakeMedia is the Attachment lookup (SPEC-12). It records the context each
// call arrived under so a test can assert the lookup ran inside the request
// scope the middleware opened, and never on a bare background context.
type fakeMedia struct {
	assets map[uuid.UUID]*mediaapi.Asset
	calls  []context.Context
}

func newFakeMedia() *fakeMedia { return &fakeMedia{assets: map[uuid.UUID]*mediaapi.Asset{}} }

func (m *fakeMedia) GetAsset(ctx context.Context, id uuid.UUID) (*mediaapi.Asset, error) {
	m.calls = append(m.calls, ctx)
	return m.assets[id], nil
}

// asset seeds one Asset and returns its id.
func (m *fakeMedia) asset(owner uuid.UUID, kind mediaapi.AssetKind, status mediaapi.AssetStatus) uuid.UUID {
	id := uuid.New()
	m.assets[id] = &mediaapi.Asset{ID: id, OwnerID: owner, Kind: kind, Status: status}
	return id
}

// readyImage is the one shape an Attachment may take.
func (m *fakeMedia) readyImage(owner uuid.UUID) uuid.UUID {
	return m.asset(owner, mediaapi.KindImage, mediaapi.StatusReady)
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
	return &Service{repo: repo, media: newFakeMedia(), events: pub}, repo, pub
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
		{"unknown asset", CreateParams{UserID: uuid.New(), BodyMd: "x", AssetIDs: []uuid.UUID{uuid.New()}}, ErrInvalidAsset},
		{"empty body", CreateParams{UserID: uuid.New(), BodyMd: ""}, ErrInvalidBody},
		{"blank body, no attachment", CreateParams{UserID: uuid.New(), BodyMd: " \n "}, ErrInvalidBody},
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
	payload := []byte(`{"asset_id":"` + uuid.NewString() + `","user_id":"` + uuid.NewString() + `","title":"clip"}`)
	_ = svc.OnPlaybackCompleted(ctx, payload)
	_ = svc.OnPlaybackCompleted(ctx, payload) // Asynq redelivery
	if f.streamCount() != 1 {
		t.Fatalf("stream items = %d, want 1 (idempotent)", f.streamCount())
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
	_ = svc.OnPlaybackCompleted(ctx, []byte(`{"asset_id":"`+asset+`","user_id":"`+owner+`","title":"clip"}`))
	// A second event type on the SAME ref, seeded directly: the point of this
	// test is that the delete sweeps every event_type for a ref, and only one
	// media event is projected now.
	_ = svc.insertSystem(ctx, []byte(`{}`), owner, "media", "media:archived", asset, time.Now())
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

// ── media:asset_deleted → Attachments (SPEC-12 T4, #13) ─────────────

// scopeKey marks a context the fake runInUserTenant has scoped, so the fake
// repo can tell a scoped call from a bare one — on the worker a bare call
// against an RLS-fenced table does not silently do nothing, it errors.
type scopeKey struct{}

// newScopedStreamSvc wires a fake runInUserTenant that records which user it
// was asked to scope and marks the context it hands down.
func newScopedStreamSvc() (*Service, *fakeRepo, *[]uuid.UUID) {
	f := newFakeRepo()
	f.requireScope = true
	scoped := &[]uuid.UUID{}
	svc := &Service{
		repo: f,
		runInUserTenant: func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error {
			*scoped = append(*scoped, userID)
			return fn(context.WithValue(ctx, scopeKey{}, userID))
		},
	}
	return svc, f, scoped
}

func seedEntry(f *fakeRepo, owner uuid.UUID, body string, ids ...uuid.UUID) uuid.UUID {
	e, _ := f.CreateEntry(context.Background(), CreateEntryInput{
		UserID: owner, BodyMd: body, AssetIDs: append([]uuid.UUID{}, ids...), OccurredAt: time.Now(),
	})
	return e.ID
}

func wantIDs(t *testing.T, f *fakeRepo, id uuid.UUID, want ...uuid.UUID) {
	t.Helper()
	e, ok := f.rows[id]
	if !ok {
		t.Fatalf("entry %s is gone; an Entry survives losing its Attachments", id)
	}
	if len(e.AssetIDs) != len(want) {
		t.Fatalf("entry %s asset_ids = %v, want %v", id, e.AssetIDs, want)
	}
	for i := range want {
		if e.AssetIDs[i] != want[i] {
			t.Fatalf("entry %s asset_ids = %v, want %v (order kept)", id, e.AssetIDs, want)
		}
	}
}

// After the event the id is gone from every Entry of the owner that carried
// it, the other ids survive in order, an Entry left with no Attachments and no
// text still exists, another owner's row is untouched, the stream rows for the
// Asset are gone — all inside the owner's tenant scope — and a second delivery
// changes nothing.
func TestAssetDeletedStripsAttachmentFromEveryEntry(t *testing.T) {
	svc, f, scoped := newScopedStreamSvc()
	ctx := context.Background()
	owner, stranger := uuid.New(), uuid.New()
	gone, a, b := uuid.New(), uuid.New(), uuid.New()

	three := seedEntry(f, owner, "three photos", gone, a, b)
	only := seedEntry(f, owner, "", gone) // photo-only: becomes text-less AND photo-less
	other := seedEntry(f, owner, "keeps its own", b)
	theirs := seedEntry(f, stranger, "not the owner's", gone) // the strip is per owner
	_ = svc.OnPlaybackCompleted(ctx, []byte(`{"asset_id":"`+gone.String()+`","user_id":"`+owner.String()+`","title":"clip"}`))
	streamBefore := f.streamCount()
	*scoped = nil // the setup insert above was scoped too; count only the event

	payload := []byte(`{"asset_id":"` + gone.String() + `","owner_user_id":"` + owner.String() + `"}`)
	if err := svc.OnAssetDeleted(ctx, payload); err != nil {
		t.Fatal(err)
	}

	wantIDs(t, f, three, a, b)
	wantIDs(t, f, only)
	if f.rows[only].BodyMd != "" {
		t.Fatalf("photo-only entry body = %q, want it left empty, not deleted or filled", f.rows[only].BodyMd)
	}
	wantIDs(t, f, other, b)
	wantIDs(t, f, theirs, gone)
	if f.streamCount() != streamBefore-1 {
		t.Fatalf("stream rows = %d, want %d (the Asset's card removed)", f.streamCount(), streamBefore-1)
	}
	if len(*scoped) != 1 || (*scoped)[0] != owner {
		t.Fatalf("scoped as %v, want exactly [%s] (the event's owner)", *scoped, owner)
	}
	if f.unscopedCalls != 0 {
		t.Fatalf("%d repository call(s) arrived outside the tenant scope", f.unscopedCalls)
	}

	// Redelivery (Asynq at-least-once): nothing left to strip, nothing changes.
	if err := svc.OnAssetDeleted(ctx, payload); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, f, three, a, b)
	wantIDs(t, f, only)
	wantIDs(t, f, other, b)
	wantIDs(t, f, theirs, gone)
	if len(f.rows) != 4 || f.streamCount() != streamBefore-1 {
		t.Fatalf("second delivery changed the store: %d rows, %d stream items", len(f.rows), f.streamCount())
	}
	if f.stripCalls != 2 {
		t.Fatalf("strip calls = %d, want 2 (one per delivery, both idempotent)", f.stripCalls)
	}
}

// A payload without the owner cannot be scoped; it is dropped, not retried
// forever, and touches nothing.
func TestAssetDeletedWithoutOwnerIsDropped(t *testing.T) {
	svc, f, scoped := newScopedStreamSvc()
	owner, gone := uuid.New(), uuid.New()
	id := seedEntry(f, owner, "x", gone)
	if err := svc.OnAssetDeleted(context.Background(), []byte(`{"asset_id":"`+gone.String()+`"}`)); err != nil {
		t.Fatal(err)
	}
	wantIDs(t, f, id, gone)
	if len(*scoped) != 0 || f.stripCalls != 0 {
		t.Fatalf("an owner-less event reached the repository (scoped %v, strips %d)", *scoped, f.stripCalls)
	}
}

// The bank delete is scoped like the bank inserts — it used to run bare.
func TestBankDeletedRunsInsideOwnerScope(t *testing.T) {
	svc, f, scoped := newScopedStreamSvc()
	ctx := context.Background()
	user, tx := uuid.New(), uuid.New()
	payload := []byte(`{"transaction_id":"` + tx.String() + `","user_id":"` + user.String() + `","occurred_at":"2026-09-19"}`)
	if err := svc.OnBankCreated(ctx, payload); err != nil {
		t.Fatal(err)
	}
	*scoped = nil
	if err := svc.OnBankDeleted(ctx, payload); err != nil {
		t.Fatal(err)
	}
	if f.streamCount() != 0 {
		t.Fatalf("stream rows = %d, want 0", f.streamCount())
	}
	if len(*scoped) != 1 || (*scoped)[0] != user || f.unscopedCalls != 0 {
		t.Fatalf("delete ran scoped as %v with %d unscoped call(s); want [%s] and 0", *scoped, f.unscopedCalls, user)
	}
}
