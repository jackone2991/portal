package journal

// Migration backfill test (SPEC-12 T1/T3; backlog P1 17a). The migrations
// 0044 and 0045 move the photo and geo links the old composer smuggled into
// journal bodies into columns, assert their own post-condition, and re-encode
// on the way down. CI applies them from zero, which proves the DDL and the
// empty-table pass; this test proves the LOOPS: it builds a throwaway database
// up to 0043 from the migration files, seeds bodies the way the pre-T1
// composer wrote them (`git show b6d8a89~1:frontend/src/lib/attachments.ts`:
// text, then `![photo](asset:<id>)`, then `[name](geo:lat,lon)`, joined by
// blank lines), and runs 0044 → 0045 → down → down → up → up, then every
// input the migrations must refuse.
//
// It needs a superuser: RLS_TEST_ADMIN_URL, the same variable the RLS suite
// uses — CI sets it to the throwaway Postgres; locally point it at the host
// cluster's owner role. Unset it skips locally and fails under CI, exactly as
// platform/db's rls_test.go does, and for the same reason.

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	bfUser  = "11111111-1111-1111-1111-111111111111"
	bfOrg   = "22222222-2222-2222-2222-222222222222"
	bfAsset = "33333333-3333-3333-3333-333333333333" // exists in `assets`
	bfGone  = "44444444-4444-4444-4444-444444444444" // never existed
)

// The seeds. Four are bodies exactly as the composer wrote them; "inline" is a
// hand-typed shape the composer never produced, kept because the decoder
// accepted it and the migration must too. The inline coordinates are written
// the way encodeLocation interpolated a JS Number (no padding), so the down
// path's four-decimal re-rendering is visible rather than hidden.
var bfSeeds = map[string]string{
	"both":      "Cà phê sáng ở đây\n\n![photo](asset:" + bfAsset + ")\n\n[Cộng Cà Phê](geo:21.0285,105.8542)",
	"gone":      "![photo](asset:" + bfGone + ")", // photo-only, and the Asset no longer exists
	"placeOnly": "[Đà Lạt](geo:11.9404,108.4583)",
	"plain":     "no place here",
	"inline":    "before [Hồ Tây](geo:21.05,105.82) after",
}

// Fixed ids so rows can be read back by name.
var bfIDs = map[string]string{
	"both":      "aaaaaaaa-0000-0000-0000-000000000001",
	"gone":      "aaaaaaaa-0000-0000-0000-000000000002",
	"placeOnly": "aaaaaaaa-0000-0000-0000-000000000003",
	"plain":     "aaaaaaaa-0000-0000-0000-000000000004",
	"inline":    "aaaaaaaa-0000-0000-0000-000000000005",
}

type bfLocation struct{ name, lat, lon string }

type bfRow struct {
	body     string
	assetIDs []string
	loc      *bfLocation // nil = the three columns are NULL (or absent)
}

func TestBackfillMigrationsMoveLinksOutOfBodies(t *testing.T) {
	adminURL := os.Getenv("RLS_TEST_ADMIN_URL")
	if adminURL == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("RLS_TEST_ADMIN_URL not set but CI is — the backfill test must run in CI, not skip")
		}
		t.Skip("RLS_TEST_ADMIN_URL not set — skipping the migration backfill test")
	}
	ctx := context.Background()
	migrations := filepath.Join("..", "..", "..", "db", "migrations")
	file := func(n string) string { return filepath.Join(migrations, n) }
	up44, down44 := file("0044_journal_attachments_in_columns.up.sql"), file("0044_journal_attachments_in_columns.down.sql")
	up45, down45 := file("0045_journal_location_in_columns.up.sql"), file("0045_journal_location_in_columns.down.sql")

	// A database of our own: the migrations are DDL on a fixed schema, so they
	// cannot be replayed on the one CI already migrated to the tip.
	root, err := pgx.Connect(ctx, adminURL)
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	t.Cleanup(func() { root.Close(ctx) })
	name := fmt.Sprintf("portal_backfill_%d", time.Now().UnixNano())
	if _, err := root.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := root.Exec(ctx, "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Logf("drop database %s: %v", name, err)
		}
	})
	db, err := pgx.Connect(ctx, withDatabase(t, adminURL, name))
	if err != nil {
		t.Fatalf("connect %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close(ctx) }) // cleanups are LIFO: closes before the DROP

	// ── schema at 0043, then the old-style rows ─────────────────────────
	for _, f := range upFiles(t, migrations, 43) {
		mustExec(t, db, f)
	}
	for _, s := range []struct {
		stmt string
		args []any
	}{
		{"INSERT INTO users (id, email, display_name, password_hash, approval_status) VALUES ($1, 'backfill@test.local', 'Backfill', 'x', 'approved')", []any{bfUser}},
		{"INSERT INTO organizations (id, kind, slug, name, owner_id) VALUES ($1, 'personal', 'backfill', 'Backfill', $2)", []any{bfOrg, bfUser}},
		// Session-scoped on purpose: journal_entries.tenant_id's DEFAULT reads it on every later INSERT over this connection.
		{"SELECT set_config('app.current_tenant', $1, false)", []any{bfOrg}},
		{"INSERT INTO assets (id, owner_id, kind, status, source_key, mime_type) VALUES ($1, $2, 'image', 'ready', 'backfill/one.png', 'image/png')", []any{bfAsset, bfUser}},
	} {
		if _, err := db.Exec(ctx, s.stmt, s.args...); err != nil {
			t.Fatalf("seed: %v -- %s", err, s.stmt)
		}
	}
	for k, body := range bfSeeds {
		insertEntry(t, db, bfIDs[k], body)
	}

	// ── 0044 up: the photo link leaves the body ─────────────────────────
	mustExec(t, db, up44)
	r := readRows(t, db, false)
	wantRow(t, r, "both", "Cà phê sáng ở đây\n\n[Cộng Cà Phê](geo:21.0285,105.8542)", nil, bfAsset)
	wantRow(t, r, "gone", "", nil) // the Asset is gone: link stripped, nothing stored
	wantRow(t, r, "plain", "no place here", nil)
	if n := countMatching(t, db, `!\[[^\]]*\]\(asset:`); n != 0 {
		t.Fatalf("%d bodies still carry a photo link after 0044", n)
	}

	// ── 0045 up: the place leaves the body ──────────────────────────────
	mustExec(t, db, up45)
	after := readRows(t, db, true)
	wantRow(t, after, "both", "Cà phê sáng ở đây", &bfLocation{"Cộng Cà Phê", "21.0285", "105.8542"}, bfAsset)
	wantRow(t, after, "placeOnly", "", &bfLocation{"Đà Lạt", "11.9404", "108.4583"}) // kept empty (SPEC-12 Further Notes)
	wantRow(t, after, "plain", "no place here", nil)
	wantRow(t, after, "inline", "before after", &bfLocation{"Hồ Tây", "21.0500", "105.8200"}) // the decoder's " " + collapse; four places
	if n := countMatching(t, db, `\[([^\]]*)\]\(geo:`); n != 0 {
		t.Fatalf("%d bodies still carry a geo link after 0045", n)
	}

	// The 0045 CHECK: all-or-nothing, non-blank name, on the Earth. The IS NOT
	// NULL guards are load-bearing — NULL BETWEEN is NULL and a NULL CHECK
	// passes — so the half-a-point shapes are the ones to try.
	for _, bad := range []string{
		"location_name = 'Hanoi', location_lat = NULL, location_lon = NULL",
		"location_name = 'Hanoi', location_lat = 21, location_lon = NULL",
		"location_name = NULL, location_lat = 21, location_lon = 105",
		"location_name = '  ', location_lat = 21, location_lon = 105",
		"location_name = 'Hanoi', location_lat = 91, location_lon = 105",
		"location_name = 'Hanoi', location_lat = 21, location_lon = -181",
	} {
		_, err := db.Exec(ctx, "UPDATE journal_entries SET "+bad+" WHERE id = $1", bfIDs["plain"])
		if !isCheckViolation(err) {
			t.Fatalf("UPDATE %s: err = %v, want the 0045 CHECK (23514)", bad, err)
		}
	}
	if _, err := db.Exec(ctx, "UPDATE journal_entries SET location_name = 'Hanoi', location_lat = 21.0285, location_lon = 105.8542 WHERE id = $1", bfIDs["plain"]); err != nil {
		t.Fatalf("a whole Location must pass the CHECK: %v", err)
	}
	if _, err := db.Exec(ctx, "UPDATE journal_entries SET location_name = NULL, location_lat = NULL, location_lon = NULL WHERE id = $1", bfIDs["plain"]); err != nil {
		t.Fatalf("clearing the Location must pass the CHECK: %v", err)
	}

	// ── down, down: both links are back, as the old composer wrote them ──
	mustExec(t, db, down45)
	mustExec(t, db, down44)
	back := readRows(t, db, false)
	// "both": 0044 down appends the photo after 0045 down appended the place,
	// so the two links swap order — the old decoder read either order.
	wantRow(t, back, "both", "Cà phê sáng ở đây\n\n[Cộng Cà Phê](geo:21.0285,105.8542)\n\n![photo](asset:"+bfAsset+")", nil, bfAsset)
	wantRow(t, back, "gone", "", nil) // dropped on the way up, so nothing to restore — documented
	wantRow(t, back, "placeOnly", "[Đà Lạt](geo:11.9404,108.4583)", nil)
	wantRow(t, back, "inline", "before after\n\n[Hồ Tây](geo:21.0500,105.8200)", nil) // four places now, at the end
	if n := columnCount(t, db, "location_%"); n != 0 {
		t.Fatalf("location columns after 0045 down = %d, want 0", n)
	}
	// 0044 down restores the old body CHECK NOT VALID — the "gone" row cannot
	// satisfy it — so the rollback completed; the old write rule still binds
	// every new row version.
	if err := insertEntryErr(db, "aaaaaaaa-0000-0000-0000-000000000008", ""); !isCheckViolation(err) {
		t.Fatalf("INSERT of an empty body after 0044 down: err = %v, want the restored CHECK (23514)", err)
	}

	// ── up, up again: the same end state (the loops are idempotent) ─────
	mustExec(t, db, up44)
	mustExec(t, db, up45)
	again := readRows(t, db, true)
	for _, k := range []string{"both", "placeOnly", "plain", "inline"} {
		if !sameRow(again[k], after[k]) {
			t.Fatalf("row %q after re-up = %s, want %s", k, describe(again[k]), describe(after[k]))
		}
	}

	// ── what the migrations refuse — each RAISE rolls its DDL back ──────
	// 0045: a point off the Earth, named.
	mustExec(t, db, down45)
	insertEntry(t, db, "aaaaaaaa-0000-0000-0000-000000000009", "nowhere\n\n[Nowhere](geo:95.0000,200.0000)")
	wantRaise(t, exec(db, up45), "aaaaaaaa-0000-0000-0000-000000000009", "off the Earth")
	if n := columnCount(t, db, "location_%"); n != 0 {
		t.Fatalf("location columns survived the failed 0045 = %d, want 0 (the DDL must roll back with the RAISE)", n)
	}
	deleteEntry(t, db, "aaaaaaaa-0000-0000-0000-000000000009")
	// 0045: a body with TWO geo links — only the first moves, so the closing
	// assertion finds a leftover and refuses to complete half-done.
	insertEntry(t, db, "aaaaaaaa-0000-0000-0000-000000000010", "[A](geo:1,2) and [B](geo:3,4)")
	wantRaise(t, exec(db, up45), "still carry a [..](geo:..) link")
	deleteEntry(t, db, "aaaaaaaa-0000-0000-0000-000000000010")
	// 0044: the same with two photo links.
	mustExec(t, db, down44)
	insertEntry(t, db, "aaaaaaaa-0000-0000-0000-000000000011", "![a](asset:"+bfAsset+") ![b](asset:"+bfAsset+")")
	wantRaise(t, exec(db, up44), "still carry a ![..](asset:..) link")
	if n := columnCount(t, db, "asset_ids"); n != 1 {
		t.Fatalf("asset_ids column count = %d after the failed 0044, want 1 (the column predates 0044 and must survive)", n)
	}
}

// ── helpers ─────────────────────────────────────────────────────────

func withDatabase(t *testing.T, dsn, name string) string {
	t.Helper()
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse RLS_TEST_ADMIN_URL: %v", err)
	}
	u.Path = "/" + name
	return u.String()
}

// upFiles lists the *.up.sql files whose numeric prefix is ≤ max, in order.
func upFiles(t *testing.T, dir string, max int) []string {
	t.Helper()
	all, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil || len(all) == 0 {
		t.Fatalf("no migrations under %s (%v)", dir, err)
	}
	sort.Strings(all)
	var out []string
	for _, f := range all {
		var n int
		if _, err := fmt.Sscanf(filepath.Base(f), "%04d_", &n); err != nil {
			t.Fatalf("migration name %q has no numeric prefix", f)
		}
		if n <= max {
			out = append(out, f)
		}
	}
	if len(out) != max {
		t.Fatalf("found %d migrations ≤ %d, want %d (the sequence has a gap or the test's cut-off is stale)", len(out), max, max)
	}
	return out
}

// exec runs one migration file as a single multi-statement Exec — one
// implicit transaction, like golang-migrate's per-file transaction — and
// returns what the database said.
func exec(db *pgx.Conn, file string) error {
	sql, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	_, err = db.Exec(context.Background(), string(sql))
	return err
}

func mustExec(t *testing.T, db *pgx.Conn, file string) {
	t.Helper()
	if err := exec(db, file); err != nil {
		t.Fatalf("%s: %v", filepath.Base(file), err)
	}
}

// wantRaise asserts a migration failed and that its message carries every
// fragment — the row id, the reason — so an operator knows what to fix.
func wantRaise(t *testing.T, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatal("migration completed, want a RAISE")
	}
	for _, f := range fragments {
		if !strings.Contains(err.Error(), f) {
			t.Fatalf("RAISE = %v, want it to mention %q", err, f)
		}
	}
}

func isCheckViolation(err error) bool {
	var pg *pgconn.PgError
	return errors.As(err, &pg) && pg.Code == "23514"
}

func insertEntryErr(db *pgx.Conn, id, body string) error {
	_, err := db.Exec(context.Background(), "INSERT INTO journal_entries (id, user_id, body_md, occurred_at) VALUES ($1, $2, $3, now())", id, bfUser, body)
	return err
}

func insertEntry(t *testing.T, db *pgx.Conn, id, body string) {
	t.Helper()
	if err := insertEntryErr(db, id, body); err != nil {
		t.Fatalf("insert %s: %v", id, err)
	}
}

func deleteEntry(t *testing.T, db *pgx.Conn, id string) {
	t.Helper()
	if _, err := db.Exec(context.Background(), "DELETE FROM journal_entries WHERE id = $1", id); err != nil {
		t.Fatalf("delete %s: %v", id, err)
	}
}

// readRows reads the seeded rows back by name. withLocation reads the three
// 0045 columns (absent before 0045 / after its down).
func readRows(t *testing.T, db *pgx.Conn, withLocation bool) map[string]bfRow {
	t.Helper()
	cols := "id::text, body_md, asset_ids::text[]"
	if withLocation {
		cols += ", location_name, location_lat::text, location_lon::text"
	}
	rows, err := db.Query(context.Background(), "SELECT "+cols+" FROM journal_entries")
	if err != nil {
		t.Fatalf("read rows: %v", err)
	}
	defer rows.Close()
	byID := map[string]string{}
	for k, id := range bfIDs {
		byID[id] = k
	}
	out := map[string]bfRow{}
	for rows.Next() {
		var id string
		var r bfRow
		var name, lat, lon *string
		var err error
		if withLocation {
			err = rows.Scan(&id, &r.body, &r.assetIDs, &name, &lat, &lon)
		} else {
			err = rows.Scan(&id, &r.body, &r.assetIDs)
		}
		if err != nil {
			t.Fatalf("scan: %v", err)
		}
		if name != nil {
			r.loc = &bfLocation{*name, deref(lat), deref(lon)}
		}
		if k, ok := byID[id]; ok {
			out[k] = r
		}
	}
	return out
}

func wantRow(t *testing.T, rows map[string]bfRow, k, body string, loc *bfLocation, assetIDs ...string) {
	t.Helper()
	r, ok := rows[k]
	if !ok {
		t.Fatalf("row %q is missing", k)
	}
	want := bfRow{body: body, assetIDs: assetIDs, loc: loc}
	if !sameRow(r, want) {
		t.Fatalf("row %q = %s, want %s", k, describe(r), describe(want))
	}
}

func sameRow(a, b bfRow) bool {
	if a.body != b.body || strings.Join(a.assetIDs, ",") != strings.Join(b.assetIDs, ",") {
		return false
	}
	if a.loc == nil || b.loc == nil {
		return a.loc == b.loc
	}
	return *a.loc == *b.loc
}

func describe(r bfRow) string {
	loc := "no location"
	if r.loc != nil {
		loc = fmt.Sprintf("%s @ %s,%s", r.loc.name, r.loc.lat, r.loc.lon)
	}
	return fmt.Sprintf("{body %q, asset_ids %v, %s}", r.body, r.assetIDs, loc)
}

func deref(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func countMatching(t *testing.T, db *pgx.Conn, re string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), "SELECT count(*) FROM journal_entries WHERE body_md ~ $1", re).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func columnCount(t *testing.T, db *pgx.Conn, like string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(context.Background(), "SELECT count(*) FROM information_schema.columns WHERE table_name = 'journal_entries' AND column_name LIKE $1", like).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}
