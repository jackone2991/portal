package db_test

// RLS isolation test — the exit criterion ADR-07 §"Implementation plan" step 8
// demands: "tenant B cannot read tenant A's rows on the portal_app role."
//
// This is the repo's first test that touches a real database, and it has to be:
// every claim it makes is about PostgreSQL policy evaluation, which no in-memory
// fake can model. It follows the platform/storage/s3_test.go convention and
// SKIPS unless it is pointed at a database, so `go test ./...` stays hermetic.
//
//	RLS_TEST_ADMIN_URL  superuser/owner DSN — creates and removes fixtures
//	RLS_TEST_APP_URL    portal_app DSN — the role the app runs as after cutover
//
// Run it with:
//
//	RLS_TEST_ADMIN_URL='postgres://portal:change-me@127.0.0.1:5432/portal?sslmode=disable' \
//	RLS_TEST_APP_URL='postgres://portal_app:change-me@127.0.0.1:5432/portal?sslmode=disable' \
//	go test ./internal/platform/db -run TestRLS -v
//
// It writes only rows it owns, inside two throwaway personal orgs, and removes
// them in a t.Cleanup that runs as the admin role.

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	platformdb "github.com/portal/backend/internal/platform/db"
)

type fixture struct {
	adminPool *pgxpool.Pool
	appDB     *platformdb.DB
	orgA      uuid.UUID
	orgB      uuid.UUID
	userA     uuid.UUID
	userB     uuid.UUID
}

func setup(t *testing.T) *fixture {
	t.Helper()
	adminURL, appURL := os.Getenv("RLS_TEST_ADMIN_URL"), os.Getenv("RLS_TEST_APP_URL")
	if adminURL == "" || appURL == "" {
		t.Skip("RLS_TEST_ADMIN_URL / RLS_TEST_APP_URL not set — skipping the RLS isolation suite")
	}
	ctx := context.Background()

	adminPool, err := pgxpool.New(ctx, adminURL)
	if err != nil {
		t.Fatalf("admin pool: %v", err)
	}
	t.Cleanup(adminPool.Close)

	appPool, err := platformdb.NewPool(ctx, appURL)
	if err != nil {
		t.Fatalf("app pool: %v", err)
	}
	t.Cleanup(appPool.Close)

	// The app role must be the one the cutover targets: no superuser, no
	// BYPASSRLS. Without this check the whole suite could pass against `portal`
	// and prove nothing at all.
	var isSuper, bypass bool
	var who string
	if err := appPool.QueryRow(ctx,
		`SELECT current_user, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`,
	).Scan(&who, &isSuper, &bypass); err != nil {
		t.Fatalf("probe app role: %v", err)
	}
	if isSuper || bypass {
		t.Fatalf("RLS_TEST_APP_URL connects as %q (superuser=%v bypassrls=%v) — that role bypasses RLS, so this suite would pass vacuously", who, isSuper, bypass)
	}

	f := &fixture{
		adminPool: adminPool,
		appDB:     platformdb.New(appPool),
		userA:     uuid.New(),
		userB:     uuid.New(),
		orgA:      uuid.New(),
		orgB:      uuid.New(),
	}

	mk := func(user, org uuid.UUID, tag string) {
		if _, err := adminPool.Exec(ctx,
			`INSERT INTO users (id, email, display_name) VALUES ($1, $2, $3)`,
			user, fmt.Sprintf("rls-%s-%s@test.invalid", tag, user), "RLS "+tag,
		); err != nil {
			t.Fatalf("seed user %s: %v", tag, err)
		}
		if _, err := adminPool.Exec(ctx,
			`INSERT INTO organizations (id, kind, slug, name, owner_id) VALUES ($1, 'personal', $2, $3, $4)`,
			org, "rls-"+tag+"-"+org.String()[:8], "RLS "+tag, user,
		); err != nil {
			t.Fatalf("seed org %s: %v", tag, err)
		}
	}
	mk(f.userA, f.orgA, "a")
	mk(f.userB, f.orgB, "b")

	t.Cleanup(func() {
		ctx := context.Background()
		for _, org := range []uuid.UUID{f.orgA, f.orgB} {
			_, _ = adminPool.Exec(ctx, `DELETE FROM comics WHERE tenant_id = $1`, org)
			_, _ = adminPool.Exec(ctx, `DELETE FROM bank_categories WHERE tenant_id = $1`, org)
			_, _ = adminPool.Exec(ctx, `DELETE FROM organizations WHERE id = $1`, org)
		}
		for _, u := range []uuid.UUID{f.userA, f.userB} {
			_, _ = adminPool.Exec(ctx, `DELETE FROM users WHERE id = $1`, u)
		}
	})
	return f
}

// inTenant runs fn inside a committed tenant scope, mirroring exactly what
// RequireTenant and the worker's runInUserTenant do in production.
func (f *fixture) inTenant(t *testing.T, org uuid.UUID, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := f.appDB.BeginTenantScope(ctx, org)
	if err != nil {
		return fmt.Errorf("begin tenant scope: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// ══ the exit criterion ══════════════════════════════════════════════════════

// Tenant B must not see tenant A's row. This is the whole point of ADR-07.
func TestRLSTenantCannotReadAnotherTenantsRows(t *testing.T) {
	f := setup(t)
	comicID := uuid.New()

	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO comics (id, owner_user_id, title) VALUES ($1, $2, 'Tenant A private comic')`,
			comicID, f.userA)
		return err
	}); err != nil {
		t.Fatalf("tenant A could not write its own row: %v", err)
	}

	// A sees it.
	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM comics WHERE id = $1`, comicID).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("tenant A sees %d of its own rows, want 1", n)
		}
		return nil
	}); err != nil {
		t.Fatalf("owner read: %v", err)
	}

	// B does not.
	if err := f.inTenant(t, f.orgB, func(ctx context.Context, tx pgx.Tx) error {
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM comics WHERE id = $1`, comicID).Scan(&n); err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("TENANT ISOLATION BREACH: tenant B can read %d of tenant A's rows", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// USING filters reads; WITH CHECK is what stops a write landing in someone
// else's tenant. They are separate policy clauses and need separate proof.
func TestRLSTenantCannotWriteIntoAnotherTenant(t *testing.T) {
	f := setup(t)

	err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO comics (id, owner_user_id, title, tenant_id) VALUES ($1, $2, 'smuggled', $3)`,
			uuid.New(), f.userA, f.orgB)
		return err
	})
	if err == nil {
		t.Fatal("TENANT ISOLATION BREACH: a row was written into another tenant — the WITH CHECK clause is not enforcing")
	}
}

// An UPDATE must not be able to move a row across tenants either.
func TestRLSTenantCannotRelocateARow(t *testing.T) {
	f := setup(t)
	comicID := uuid.New()

	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO comics (id, owner_user_id, title) VALUES ($1, $2, 'stays put')`, comicID, f.userA)
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `UPDATE comics SET tenant_id = $1 WHERE id = $2`, f.orgB, comicID)
		return err
	})
	if err == nil {
		t.Fatal("TENANT ISOLATION BREACH: a row was relocated into another tenant by UPDATE")
	}
}

// Tenant B's DELETE must not reach tenant A's row. RLS turns it into a no-op
// rather than an error, so the assertion is on the row still existing.
func TestRLSTenantCannotDeleteAnotherTenantsRow(t *testing.T) {
	f := setup(t)
	comicID := uuid.New()

	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO comics (id, owner_user_id, title) VALUES ($1, $2, 'not yours to delete')`, comicID, f.userA)
		return err
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := f.inTenant(t, f.orgB, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx, `DELETE FROM comics WHERE id = $1`, comicID)
		return err
	}); err != nil {
		t.Fatalf("cross-tenant delete errored unexpectedly: %v", err)
	}

	var n int
	if err := f.adminPool.QueryRow(context.Background(),
		`SELECT count(*) FROM comics WHERE id = $1`, comicID).Scan(&n); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if n != 1 {
		t.Fatal("TENANT ISOLATION BREACH: tenant B deleted tenant A's row")
	}
}

// ══ the failure mode this cutover is most likely to hit ═════════════════════

// A write outside a tenant scope must FAIL, not silently land somewhere. This is
// the guard that turns "someone forgot BeginTenantScope" from a data-corruption
// bug into a loud error — and it is exactly what the media worker's missing
// scope would have tripped (see worker.inTenant).
func TestRLSWriteWithoutATenantScopeFails(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	_, err := f.appDB.Conn().Exec(ctx,
		`INSERT INTO comics (id, owner_user_id, title) VALUES ($1, $2, 'unscoped')`,
		uuid.New(), f.userA)
	if err == nil {
		t.Fatal("an INSERT with no tenant scope succeeded — tenant_id's DEFAULT is not fail-closed")
	}
	t.Logf("unscoped insert correctly refused: %v", err)
}

// media_asset_variants is the table the 0020 migration's ⚠️ gate named and the
// one whose INSERT used to smuggle tenant_id out of a subquery over `assets`.
// Under portal_app that subquery is filtered to nothing, so this asserts the
// column DEFAULT (not a subquery) is what supplies tenant_id now.
func TestRLSVariantInsertResolvesTenantFromTheScope(t *testing.T) {
	f := setup(t)
	assetID := uuid.New()

	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`INSERT INTO assets (id, owner_id, kind, status, source_key, mime_type, size_bytes)
			 VALUES ($1, $2, 'image', 'processing', $3, 'image/jpeg', 1024)`,
			assetID, f.userA, "src/"+assetID.String()); err != nil {
			return fmt.Errorf("seed asset: %w", err)
		}
		// No tenant_id column: it must come from app.current_tenant.
		if _, err := tx.Exec(ctx,
			`INSERT INTO media_asset_variants (asset_id, variant, storage_key, width, height, size_bytes)
			 VALUES ($1, 'thumb', $2, 320, 240, 512)`,
			assetID, "variants/"+assetID.String()+"/thumb.webp"); err != nil {
			return fmt.Errorf("insert variant: %w", err)
		}
		var tenant uuid.UUID
		if err := tx.QueryRow(ctx,
			`SELECT tenant_id FROM media_asset_variants WHERE asset_id = $1`, assetID).Scan(&tenant); err != nil {
			return err
		}
		if tenant != f.orgA {
			return fmt.Errorf("variant landed in tenant %v, want %v", tenant, f.orgA)
		}
		return nil
	}); err != nil {
		t.Fatalf("%v", err)
	}

	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM media_asset_variants WHERE asset_id = $1`, assetID)
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, assetID)
	})
}

// ══ the documented exception ════════════════════════════════════════════════

// bank_categories is the one table whose tenant_id is NULLABLE: its NULL rows
// are the shared global seed taxonomy from 0014, deliberately visible to every
// tenant. 0020's comment calls this out and says "the isolation test
// special-cases this one table" — this is that special case.
func TestRLSSharedSeedCategoriesAreVisibleToEveryTenant(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	var seeded int
	if err := f.adminPool.QueryRow(ctx,
		`SELECT count(*) FROM bank_categories WHERE tenant_id IS NULL`).Scan(&seeded); err != nil {
		t.Fatalf("count seed rows: %v", err)
	}
	if seeded == 0 {
		t.Skip("no NULL-tenant seed categories in this database — nothing to assert")
	}

	for name, org := range map[string]uuid.UUID{"A": f.orgA, "B": f.orgB} {
		if err := f.inTenant(t, org, func(ctx context.Context, tx pgx.Tx) error {
			var n int
			if err := tx.QueryRow(ctx,
				`SELECT count(*) FROM bank_categories WHERE tenant_id IS NULL`).Scan(&n); err != nil {
				return err
			}
			if n != seeded {
				return fmt.Errorf("tenant %s sees %d shared seed categories, want %d", name, n, seeded)
			}
			return nil
		}); err != nil {
			t.Error(err)
		}
	}
}

// ══ coverage: policy presence is a property of the schema, not of one table ══

// Every RLS-enabled table must carry a policy AND have FORCE set. Without FORCE
// the owning role skips the policy, which is precisely how this could look
// enforced while being inert. This assertion self-maintains: a new tenant-scoped
// table that forgets either one fails here rather than at the next breach.
func TestRLSEveryProtectedTableHasAPolicyAndForce(t *testing.T) {
	f := setup(t)
	ctx := context.Background()

	rows, err := f.adminPool.Query(ctx, `
		SELECT c.relname, c.relforcerowsecurity,
		       (SELECT count(*) FROM pg_policy p WHERE p.polrelid = c.oid)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'public' AND c.relkind = 'r' AND c.relrowsecurity
		ORDER BY c.relname`)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		var name string
		var forced bool
		var policies int
		if err := rows.Scan(&name, &forced, &policies); err != nil {
			t.Fatalf("scan: %v", err)
		}
		total++
		if policies == 0 {
			t.Errorf("%s has RLS enabled but no policy — it is open to every tenant", name)
		}
		if !forced {
			t.Errorf("%s has RLS but not FORCE — the table owner bypasses the policy", name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("rows: %v", err)
	}
	if total == 0 {
		t.Fatal("no RLS-enabled tables found — migration 0020 has not been applied to this database")
	}
	t.Logf("%d tenant-scoped tables carry a policy with FORCE", total)
}
