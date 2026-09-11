package db_test

// Media ACL suite — the per-user layer added by migration 0032 on top of the
// tenant fence from 0020. Same harness and the same skip rules as rls_test.go
// (RLS_TEST_ADMIN_URL / RLS_TEST_APP_URL); see that file's header to run it.
//
// The claim under test is the one the product makes: two people in the SAME
// tenant cannot see each other's media. Nothing in Go enforces that — the
// handlers deliberately carry no ownership check — so these tests are the only
// place it is proven.

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	platformdb "github.com/portal/backend/internal/platform/db"
)

// inScope runs fn as a specific actor inside org — the exact thing
// RequireTenant pins onto a request (tenant + user + admin flag).
func (f *fixture) inScope(t *testing.T, org, user uuid.UUID, admin bool, fn func(ctx context.Context, tx pgx.Tx) error) error {
	t.Helper()
	ctx := context.Background()
	tx, err := f.appDB.BeginScope(ctx, platformdb.Scope{OrgID: org, UserID: user, Admin: admin})
	if err != nil {
		return fmt.Errorf("begin scope: %w", err)
	}
	if err := fn(ctx, tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// seedAsset inserts one asset owned by `owner` inside `org`, as the admin role
// (bypassing RLS) so the fixture never depends on the policy it is testing.
func (f *fixture) seedAsset(t *testing.T, org, owner uuid.UUID, visibility string) uuid.UUID {
	t.Helper()
	ctx := context.Background()
	id := uuid.New()
	if _, err := f.adminPool.Exec(ctx,
		`INSERT INTO assets (id, owner_id, kind, source_key, mime_type, tenant_id, visibility)
		 VALUES ($1, $2, 'image', $3, 'image/webp', $4, $5)`,
		id, owner, "src/"+id.String(), org, visibility,
	); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	t.Cleanup(func() {
		ctx := context.Background()
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM media_asset_variants WHERE asset_id = $1`, id)
		_, _ = f.adminPool.Exec(ctx, `DELETE FROM assets WHERE id = $1`, id)
	})
	return id
}

func (f *fixture) seedVariant(t *testing.T, org, assetID uuid.UUID) {
	t.Helper()
	if _, err := f.adminPool.Exec(context.Background(),
		`INSERT INTO media_asset_variants (asset_id, variant, storage_key, width, height, size_bytes, tenant_id)
		 VALUES ($1, 'thumb', $2, 320, 200, 1024, $3)`,
		assetID, "variants/"+assetID.String()+"/thumb.webp", org,
	); err != nil {
		t.Fatalf("seed variant: %v", err)
	}
}

// countAssets / countVariants report what a given actor can see.
func countAssets(ctx context.Context, tx pgx.Tx, id uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM assets WHERE id = $1`, id).Scan(&n)
	return n, err
}

func countVariants(ctx context.Context, tx pgx.Tx, assetID uuid.UUID) (int, error) {
	var n int
	err := tx.QueryRow(ctx, `SELECT count(*) FROM media_asset_variants WHERE asset_id = $1`, assetID).Scan(&n)
	return n, err
}

// ══ the claim ═══════════════════════════════════════════════════════════════

// Two members of one tenant must not see each other's media. Before 0032 the
// tenant fence alone let either read the other's rows by id.
func TestRLSMediaMemberCannotReadAnotherMembersAsset(t *testing.T) {
	f := setup(t)
	assetID := f.seedAsset(t, f.orgA, f.userA, "private")
	f.seedVariant(t, f.orgA, assetID)

	// The owner sees it.
	if err := f.inScope(t, f.orgA, f.userA, false, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countAssets(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("owner sees %d of their own assets, want 1", n)
		}
		return nil
	}); err != nil {
		t.Fatalf("owner read: %v", err)
	}

	// A second member of the SAME tenant does not.
	if err := f.inScope(t, f.orgA, f.userB, false, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countAssets(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("MEDIA ACL BREACH: another member of the tenant can read %d of the owner's assets", n)
		}
		v, err := countVariants(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if v != 0 {
			return fmt.Errorf("MEDIA ACL BREACH: another member can read %d variants of the owner's asset — the rendition leaks what the asset does not", v)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The tenant's admin is the documented exception: they administer the org, so
// they can read its media.
func TestRLSMediaTenantAdminCanReadMembersAsset(t *testing.T) {
	f := setup(t)
	assetID := f.seedAsset(t, f.orgA, f.userA, "private")
	f.seedVariant(t, f.orgA, assetID)

	if err := f.inScope(t, f.orgA, f.userB, true, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countAssets(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if n != 1 {
			return fmt.Errorf("tenant admin sees %d assets, want 1", n)
		}
		v, err := countVariants(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if v != 1 {
			return fmt.Errorf("tenant admin sees %d variants, want 1", v)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// An admin of ANOTHER tenant is not an exception: the tenant fence still holds.
func TestRLSMediaAdminOfAnotherTenantSeesNothing(t *testing.T) {
	f := setup(t)
	assetID := f.seedAsset(t, f.orgA, f.userA, "private")

	if err := f.inScope(t, f.orgB, f.userB, true, func(ctx context.Context, tx pgx.Tx) error {
		n, err := countAssets(ctx, tx, assetID)
		if err != nil {
			return err
		}
		if n != 0 {
			return fmt.Errorf("TENANT ISOLATION BREACH: tenant B's admin can read %d of tenant A's assets", n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// ══ public ══════════════════════════════════════════════════════════════════

// A public asset is readable with NO scope at all — that is what makes the
// variant/HLS routes work for a signed-out visitor. The private one beside it
// must stay invisible on the very same connection.
func TestRLSMediaPublicAssetIsReadableUnscoped(t *testing.T) {
	f := setup(t)
	pub := f.seedAsset(t, f.orgA, f.userA, "public")
	priv := f.seedAsset(t, f.orgA, f.userA, "private")
	f.seedVariant(t, f.orgA, pub)
	f.seedVariant(t, f.orgA, priv)

	ctx := context.Background()
	conn := f.appDB.Conn() // no transaction, no GUCs — an anonymous request

	var n int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM assets WHERE id = $1`, pub).Scan(&n); err != nil {
		t.Fatalf("unscoped read of a public asset failed: %v — anonymous media delivery is broken", err)
	}
	if n != 1 {
		t.Fatalf("anonymous caller sees %d public assets, want 1", n)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM media_asset_variants WHERE asset_id = $1`, pub).Scan(&n); err != nil {
		t.Fatalf("unscoped read of a public variant failed: %v", err)
	}
	if n != 1 {
		t.Fatalf("anonymous caller sees %d public variants, want 1", n)
	}

	if err := conn.QueryRow(ctx, `SELECT count(*) FROM assets WHERE id = $1`, priv).Scan(&n); err != nil {
		t.Fatalf("unscoped read of a private asset errored (it must return zero rows, not raise): %v", err)
	}
	if n != 0 {
		t.Fatalf("MEDIA ACL BREACH: an anonymous caller can read %d private assets", n)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM media_asset_variants WHERE asset_id = $1`, priv).Scan(&n); err != nil {
		t.Fatalf("unscoped read of a private variant errored: %v", err)
	}
	if n != 0 {
		t.Fatalf("MEDIA ACL BREACH: an anonymous caller can read %d private variants", n)
	}
}

// Reading a public asset must not imply writing it: "anyone can see this" is
// not "anyone can change this".
func TestRLSMediaPublicAssetIsStillOwnerWritable(t *testing.T) {
	f := setup(t)
	pub := f.seedAsset(t, f.orgA, f.userA, "public")

	// Another member cannot flip it back, nor retitle it.
	if err := f.inScope(t, f.orgA, f.userB, false, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE assets SET visibility = 'private' WHERE id = $1`, pub)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 0 {
			return fmt.Errorf("MEDIA ACL BREACH: a non-owner changed %d rows of someone else's asset", tag.RowsAffected())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	// The owner can.
	if err := f.inScope(t, f.orgA, f.userA, false, func(ctx context.Context, tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE assets SET visibility = 'private' WHERE id = $1`, pub)
		if err != nil {
			return err
		}
		if tag.RowsAffected() != 1 {
			return fmt.Errorf("owner updated %d rows, want 1", tag.RowsAffected())
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// The worker has no user identity — it processes every member's media — so it
// runs as a trusted backend scope. If that stopped working, image and transcode
// pipelines would silently produce nothing.
func TestRLSMediaBackendScopeCanWriteVariants(t *testing.T) {
	f := setup(t)
	assetID := f.seedAsset(t, f.orgA, f.userA, "private")

	if err := f.inTenant(t, f.orgA, func(ctx context.Context, tx pgx.Tx) error {
		_, err := tx.Exec(ctx,
			`INSERT INTO media_asset_variants (asset_id, variant, storage_key, width, height, size_bytes)
			 VALUES ($1, 'medium', $2, 1280, 800, 4096)`,
			assetID, "variants/"+assetID.String()+"/medium.webp")
		return err
	}); err != nil {
		t.Fatalf("the worker's backend scope cannot write a variant — the media pipeline is broken: %v", err)
	}
}
