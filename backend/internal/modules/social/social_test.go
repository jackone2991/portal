package social

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The isolation guarantees for connections live in the database and are proven
// by the RLS suite (internal/platform/db/rls_social_test.go), which is where
// they belong — no in-memory fake can model a policy. What is tested here is
// the decision-making a policy cannot express: what "send a request" should do
// when a row already exists.

type fakeRepo struct {
	rows map[uuid.UUID]Connection
}

func newFakeRepo() *fakeRepo { return &fakeRepo{rows: map[uuid.UUID]Connection{}} }

func (f *fakeRepo) CreateRequest(_ context.Context, requester, addressee uuid.UUID) (Connection, error) {
	for _, c := range f.rows {
		if (c.RequesterID == requester && c.AddresseeID == addressee) ||
			(c.RequesterID == addressee && c.AddresseeID == requester) {
			return Connection{}, ErrExists // the pair unique index, in memory
		}
	}
	c := Connection{ID: uuid.New(), RequesterID: requester, AddresseeID: addressee, Status: StatusPending}
	f.rows[c.ID] = c
	return c, nil
}

func (f *fakeRepo) Get(_ context.Context, id uuid.UUID) (Connection, error) {
	c, ok := f.rows[id]
	if !ok {
		return Connection{}, ErrNotFound
	}
	return c, nil
}

func (f *fakeRepo) FindBetween(_ context.Context, a, b uuid.UUID) (Connection, error) {
	for _, c := range f.rows {
		if (c.RequesterID == a && c.AddresseeID == b) || (c.RequesterID == b && c.AddresseeID == a) {
			return c, nil
		}
	}
	return Connection{}, ErrNotFound
}

func (f *fakeRepo) Accept(_ context.Context, id, addressee uuid.UUID) (Connection, error) {
	c, ok := f.rows[id]
	if !ok || c.AddresseeID != addressee || c.Status != StatusPending {
		return Connection{}, ErrNotFound
	}
	c.Status = StatusAccepted
	f.rows[id] = c
	return c, nil
}

func (f *fakeRepo) Delete(_ context.Context, id, actor uuid.UUID) (Connection, error) {
	c, ok := f.rows[id]
	if !ok || (c.RequesterID != actor && c.AddresseeID != actor) {
		return Connection{}, ErrNotFound
	}
	delete(f.rows, id)
	return c, nil
}

func (f *fakeRepo) List(_ context.Context, me uuid.UUID, kind Kind) ([]Connection, error) {
	var out []Connection
	for _, c := range f.rows {
		switch kind {
		case KindIncoming:
			if c.Status == StatusPending && c.AddresseeID == me {
				out = append(out, c)
			}
		case KindOutgoing:
			if c.Status == StatusPending && c.RequesterID == me {
				out = append(out, c)
			}
		default:
			if c.Status == StatusAccepted && (c.RequesterID == me || c.AddresseeID == me) {
				out = append(out, c)
			}
		}
	}
	return out, nil
}

func (f *fakeRepo) CounterpartIDs(_ context.Context, me uuid.UUID) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, c := range f.rows {
		if c.RequesterID == me || c.AddresseeID == me {
			out = append(out, c.Other(me))
		}
	}
	return out, nil
}

func (f *fakeRepo) CountIncoming(_ context.Context, me uuid.UUID) (int, error) {
	n := 0
	for _, c := range f.rows {
		if c.Status == StatusPending && c.AddresseeID == me {
			n++
		}
	}
	return n, nil
}

func newSvc() (*Service, *fakeRepo) {
	repo := newFakeRepo()
	return &Service{
		repo: repo,
		names: func(_ context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
			m := map[uuid.UUID]string{}
			for _, id := range ids {
				m[id] = "User " + id.String()[:4]
			}
			return m, nil
		},
	}, repo
}

// ══ tests ═══════════════════════════════════════════════════════════════════

// Asking someone who already asked YOU is agreement, not a duplicate. Returning
// "already exists" here would leave their request sitting unanswered while both
// people believe they have acted.
func TestRequestAcceptsAReverseRequest(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	first, err := svc.Request(ctx, a, b)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}

	got, err := svc.Request(ctx, b, a) // b asks back
	if err != nil {
		t.Fatalf("reverse request: %v", err)
	}
	if got.Status != StatusAccepted {
		t.Fatalf("reverse request left status %q, want accepted", got.Status)
	}
	if got.ID != first.ID {
		t.Fatal("reverse request created a second row instead of accepting the first")
	}
	if len(repo.rows) != 1 {
		t.Fatalf("%d connection rows, want 1", len(repo.rows))
	}
}

// Asking the same person twice is a duplicate, in either direction.
func TestRequestTwiceIsAConflict(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	a, b := uuid.New(), uuid.New()

	if _, err := svc.Request(ctx, a, b); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := svc.Request(ctx, a, b); !errors.Is(err, ErrExists) {
		t.Fatalf("second request = %v, want ErrExists", err)
	}
}

func TestRequestSelfRejected(t *testing.T) {
	svc, _ := newSvc()
	me := uuid.New()
	if _, err := svc.Request(context.Background(), me, me); !errors.Is(err, ErrSelf) {
		t.Fatalf("self request = %v, want ErrSelf", err)
	}
	if _, err := svc.Request(context.Background(), me, uuid.Nil); !errors.Is(err, ErrSelf) {
		t.Fatalf("nil target = %v, want ErrSelf", err)
	}
}

// Every list is rendered from the caller's side: the other person, and which of
// you asked. Getting `Outgoing` backwards would put a Cancel button in front of
// someone who should see Accept.
func TestListIsRenderedFromTheCallersSide(t *testing.T) {
	svc, _ := newSvc()
	ctx := context.Background()
	asker, asked := uuid.New(), uuid.New()
	if _, err := svc.Request(ctx, asker, asked); err != nil {
		t.Fatal(err)
	}

	out, err := svc.List(ctx, asked, KindIncoming)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("addressee sees %d incoming, want 1", len(out))
	}
	if out[0].UserID != asker {
		t.Fatal("incoming row names the wrong person")
	}
	if out[0].Outgoing {
		t.Fatal("a request the caller received is marked outgoing")
	}

	mine, err := svc.List(ctx, asker, KindOutgoing)
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || !mine[0].Outgoing || mine[0].UserID != asked {
		t.Fatalf("outgoing list = %+v", mine)
	}
}

// A name that will not resolve is still a real request. Dropping the row would
// hide something the user has to answer.
func TestListKeepsRowsWithUnresolvableNames(t *testing.T) {
	repo := newFakeRepo()
	svc := &Service{
		repo:  repo,
		names: func(context.Context, []uuid.UUID) (map[uuid.UUID]string, error) { return nil, nil },
	}
	ctx := context.Background()
	asker, asked := uuid.New(), uuid.New()
	if _, err := svc.Request(ctx, asker, asked); err != nil {
		t.Fatal(err)
	}

	out, err := svc.List(ctx, asked, KindIncoming)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].DisplayName != "Unknown" {
		t.Fatalf("list = %+v, want one row labelled Unknown", out)
	}
}

// Withdraw / decline / disconnect are one operation, and only the two parties
// may perform it.
func TestRemoveOnlyByAParty(t *testing.T) {
	svc, repo := newSvc()
	ctx := context.Background()
	a, b, outsider := uuid.New(), uuid.New(), uuid.New()
	c, err := svc.Request(ctx, a, b)
	if err != nil {
		t.Fatal(err)
	}

	if err := svc.Remove(ctx, outsider, c.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("outsider remove = %v, want ErrNotFound", err)
	}
	if len(repo.rows) != 1 {
		t.Fatal("an outsider deleted someone else's connection")
	}
	if err := svc.Remove(ctx, b, c.ID); err != nil {
		t.Fatalf("addressee remove: %v", err)
	}
	if len(repo.rows) != 0 {
		t.Fatal("the connection survived its own removal")
	}
}
