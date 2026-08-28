package db_test

// Social connection isolation (migration 0037). Same harness and skip rules as
// rls_test.go.
//
// Connections are the one domain table with no tenant_id — a link between two
// accounts cannot belong to one tenant — so the ONLY thing standing between one
// person's friend list and another's is the per-user policy. That makes these
// tests the whole security argument for the table, not a nice-to-have.

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// asUser runs fn scoped to one user. Org is incidental here: the connection
// policies read app.current_user only.
func (f *fixture) asUser(t *testing.T, org, user uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	return f.inScope(t, org, user, false, fn)
}

// seedThirdParty adds a user with their own personal org, for the outsider in
// the tests below.
func (f *fixture) seedThirdParty(t *testing.T) (user, org uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	user, org = uuid.New(), uuid.New()
	if _, err := f.adminPool.Exec(ctx,
		`INSERT INTO users (id, email, display_name) VALUES ($1, $2, 'RLS c')`,
		user, fmt.Sprintf("rls-c-%s@test.invalid", user),
	); err != nil {
		t.Fatalf("seed third user: %v", err)
	}
	if _, err := f.adminPool.Exec(ctx,
		`INSERT INTO organizations (id, kind, slug, name, owner_id) VALUES ($1, 'personal', $2, 'RLS c', $3)`,
		org, "rls-c-"+org.String()[:8], user,
	); err != nil {
		t.Fatalf("seed third org: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org)
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM users WHERE id = $1`, user)
	})
	return user, org
}

func (f *fixture) seedConnection(t *testing.T, requester, addressee uuid.UUID, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	if _, err := f.adminPool.Exec(context.Background(),
		`INSERT INTO social_connections (id, requester_id, addressee_id, status) VALUES ($1, $2, $3, $4)`,
		id, requester, addressee, status,
	); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	t.Cleanup(func() {
		_, _ = f.adminPool.Exec(context.Background(), `DELETE FROM social_connections WHERE id = $1`, id)
	})
	return id
}

func countConnections(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM social_connections WHERE id = $1`, id).Scan(&n)
	return n, err
}

// ══ the claim ═══════════════════════════════════════════════════════════════

// Both parties see their connection; nobody else does.
func TestRLSConnectionVisibleOnlyToItsTwoParties(t *testing.T) {
	f := setup(t)
	outsider, outsiderOrg := f.seedThirdParty(t)
	id := f.seedConnection(t, f.userA, f.userB, "accepted")

	for _, party := range []struct {
		name string
		org  uuid.UUID
		user uuid.UUID
	}{
		{"requester", f.orgA, f.userA},
		{"addressee", f.orgB, f.userB},
	} {
		if err := f.asUser(t, party.org, party.user, func(ctx context.Context, tx pgx.Tx) error {
			n, err := countConnections(ctx, tx, id)
			if err != nil {
				return err
			}
			if n != 1 {
				return fmt.Errorf("%s sees %d of their own connections, want 1", party.name, n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}

	if err := f.asUser(t, outsiderOrg, outsider, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countConnections(ctx, tx, id)
		if err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("SOCIAL ISOLATION BREACH: an unrelated user can read %d connections between two other people", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A pending request must reach the person being asked — if the addressee could
// not see it, they could never answer.
func TestRLSPendingRequestIsVisibleToTheAddressee(t *testing.T) {
	f := setup(t)
	id := f.seedConnection(t, f.userA, f.userB, "pending")

	if err := f.asUser(t, f.orgB, f.userB, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countConnections(ctx, tx, id)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("the addressee sees %d pending requests, want 1 — they could never accept it", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// You cannot send a request as someone else.
func TestRLSCannotForgeARequestFromAnotherUser(t *testing.T) {
	f := setup(t)
	outsider, outsiderOrg := f.seedThirdParty(t)

	err := f.asUser(t, outsiderOrg, outsider, func(ctx context.Context, tx pgx.Tx) error {
		_, e := tx.Exec(ctx,
			`INSERT INTO social_connections (requester_id, addressee_id) VALUES ($1, $2)`,
			f.userA, f.userB)
		return e
	})
	if err == nil {
		t.Fatal("SOCIAL ISOLATION BREACH: a third party inserted a connection request between two other users")
	}
}

// Only the addressee may accept. The requester accepting their own request
// would make the mutual part of a mutual link meaningless.
func TestRLSRequesterCannotAcceptTheirOwnRequest(t *testing.T) {
	f := setup(t)
	id := f.seedConnection(t, f.userA, f.userB, "pending")

	if err := f.asUser(t, f.orgA, f.userA, func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE social_connections SET status = 'accepted' WHERE id = $1`, id)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 0 {
			return fmt.Errorf("SOCIAL ISOLATION BREACH: the requester accepted their own request (%d rows)", tag.RowsAffected())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// ...and the addressee can.
	if err := f.asUser(t, f.orgB, f.userB, func(ctx context.Context, tx pgx.Tx) error {
		tag, e := tx.Exec(ctx, `UPDATE social_connections SET status = 'accepted' WHERE id = $1`, id)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("the addressee accepted %d rows, want 1", tag.RowsAffected())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// With no user scope at all — an anonymous request, or a backend job — the table
// is empty rather than raising, so nothing leaks through an unscoped path.
func TestRLSConnectionsInvisibleWithoutAUserScope(t *testing.T) {
	f := setup(t)
	id := f.seedConnection(t, f.userA, f.userB, "accepted")

	var n int
	if err := f.appDB.Conn().QueryRow(context.Background(),
		`SELECT count(*) FROM social_connections WHERE id = $1`, id).Scan(&n); err != nil {
		t.Fatalf("unscoped read errored instead of returning zero rows: %v", err)
	}
	if n != 0 {
		t.Fatalf("SOCIAL ISOLATION BREACH: an unscoped connection can read %d rows", n)
	}
}
