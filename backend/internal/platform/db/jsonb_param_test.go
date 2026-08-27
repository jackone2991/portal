package db

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// The pool sets pgx.QueryExecModeExec (see NewPool). In that mode pgx does not
// describe parameters, so it derives each param's wire OID from the Go type — a
// []byte goes out as bytea, and assigning bytea to a jsonb column fails with
// SQLSTATE 22P02 ("invalid input syntax for type json"). A ::jsonb cast in the
// SQL does NOT rescue it; only the Go type matters.
//
// That is easy to reintroduce because sqlc maps a jsonb param to []byte by
// default. The fix is to cast the param `::text::jsonb` in query/*.sql, which
// makes sqlc emit a string (or *string) instead.
//
// This bug shipped silently five times — audit_log, stream_items,
// notifications, people_persons and comic_imports all stopped recording for
// weeks, because the only symptom is a swallowed best-effort log line. This
// test is the guard: it scans the generated repositories for a []byte parameter
// named after a jsonb column.
//
// Read columns are exempt: scanning jsonb *out* into []byte is correct, so only
// `*Params` structs (query inputs) are checked.
func TestNoByteSliceParamsForJSONBColumns(t *testing.T) {
	root := filepath.Join("..", "..", "..")

	jsonbCols := jsonbColumnsFromMigrations(t, filepath.Join(root, "db", "migrations"))
	if len(jsonbCols) == 0 {
		t.Fatal("no jsonb columns found in migrations — the scan is broken, not the code")
	}

	generated, err := filepath.Glob(filepath.Join(root, "internal", "modules", "*", "repository", "*.sql.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(generated) == 0 {
		t.Fatal("no generated repositories found — run `make sqlc`")
	}

	structStart := regexp.MustCompile(`^type (\w+Params) struct`)
	field := regexp.MustCompile(`^\s*(\w+)\s+\[\]byte\b`)

	for _, path := range generated {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var current string
		for _, line := range strings.Split(string(src), "\n") {
			if m := structStart.FindStringSubmatch(line); m != nil {
				current = m[1]
				continue
			}
			if current == "" {
				continue
			}
			if strings.HasPrefix(line, "}") {
				current = ""
				continue
			}
			m := field.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			if col, ok := jsonbCols[strings.ToLower(m[1])]; ok {
				t.Errorf("%s: %s.%s is []byte but %s is a jsonb column.\n"+
					"Under QueryExecModeExec pgx sends []byte as bytea and the write fails with "+
					"SQLSTATE 22P02.\nCast the param `::text::jsonb` in the module's query/*.sql so "+
					"sqlc emits a string, then pass a string from the adapter.",
					filepath.Base(path), current, m[1], col)
			}
		}
	}
}

// jsonbColumnsFromMigrations returns lowercased column names declared jsonb,
// so the guard tracks the schema instead of a hand-maintained list.
func jsonbColumnsFromMigrations(t *testing.T, dir string) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		t.Fatal(err)
	}
	decl := regexp.MustCompile(`(?i)^\s*([a-z_]+)\s+jsonb\b`)
	cols := map[string]string{}
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range strings.Split(string(src), "\n") {
			if m := decl.FindStringSubmatch(line); m != nil {
				cols[strings.ToLower(m[1])] = m[1]
			}
		}
	}
	return cols
}
