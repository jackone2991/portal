package ops

import (
	"sort"
	"testing"
	"time"
)

// mustDate parses a yyyy-mm-dd test date (UTC).
func mustDate(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.Parse(dumpDateLayout, s)
	if err != nil {
		t.Fatalf("bad date %q: %v", s, err)
	}
	return d
}

// dumpsFor builds DumpMeta values from dated keys (order is deliberately mixed
// in the tests to prove keep() is input-order-independent).
func dumpsFor(t *testing.T, dates ...string) []DumpMeta {
	t.Helper()
	out := make([]DumpMeta, 0, len(dates))
	for _, s := range dates {
		out = append(out, DumpMeta{Key: dumpKeyForDate(mustDate(t, s)), Date: mustDate(t, s)})
	}
	return out
}

// keySet returns the sorted keys of a DumpMeta slice for set comparison.
func keySet(ds []DumpMeta) []string {
	out := make([]string, 0, len(ds))
	for _, d := range ds {
		out = append(out, d.Key)
	}
	sort.Strings(out)
	return out
}

func wantKeys(t *testing.T, dates ...string) []string {
	t.Helper()
	out := make([]string, 0, len(dates))
	for _, s := range dates {
		out = append(out, dumpKeyForDate(mustDate(t, s)))
	}
	sort.Strings(out)
	return out
}

func equalKeys(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestKeepRetention(t *testing.T) {
	// Sundays in the fixtures (verified): 2026-07-05, 2026-07-12,
	// 2026-06-28, 2026-06-21, 2026-06-14, 2026-06-07.
	now := mustDate(t, "2026-07-13")

	tests := []struct {
		name      string
		dates     []string
		wantKeep  []string
		wantPrune []string
	}{
		{
			name: "12 consecutive daily dumps: 7 daily + the extra Sunday, prune the oldest 4",
			dates: []string{
				"2026-07-01", "2026-07-02", "2026-07-03", "2026-07-04",
				"2026-07-05", "2026-07-06", "2026-07-07", "2026-07-08",
				"2026-07-09", "2026-07-10", "2026-07-11", "2026-07-12",
			},
			// daily(7): 07-06..07-12 ; weekly Sundays present: 07-12 (in daily), 07-05.
			wantKeep: []string{
				"2026-07-05", "2026-07-06", "2026-07-07", "2026-07-08",
				"2026-07-09", "2026-07-10", "2026-07-11", "2026-07-12",
			},
			wantPrune: []string{"2026-07-01", "2026-07-02", "2026-07-03", "2026-07-04"},
		},
		{
			name: "weekly window extends past daily; caps at 4 Sundays; keeps an old non-Sunday daily",
			dates: []string{
				"2026-07-08", "2026-07-09", "2026-07-10", "2026-07-11", "2026-07-12", // recent daily (07-12 Sun)
				"2026-06-30",                                                         // Tuesday — 7th most recent → kept by daily
				"2026-07-05", "2026-06-28", "2026-06-21", "2026-06-14", "2026-06-07", // Sundays
			},
			// daily(7): 07-12,11,10,09,08,07-05,06-30.
			// weekly(4 newest Sundays): 07-12, 07-05, 06-28, 06-21.
			wantKeep: []string{
				"2026-07-12", "2026-07-11", "2026-07-10", "2026-07-09", "2026-07-08",
				"2026-07-05", "2026-06-30", "2026-06-28", "2026-06-21",
			},
			wantPrune: []string{"2026-06-14", "2026-06-07"},
		},
		{
			name:      "fewer than the daily window: keep all, prune none",
			dates:     []string{"2026-07-10", "2026-07-11", "2026-07-12"},
			wantKeep:  []string{"2026-07-10", "2026-07-11", "2026-07-12"},
			wantPrune: nil,
		},
		{
			name:      "empty input: nothing kept, nothing pruned",
			dates:     nil,
			wantKeep:  nil,
			wantPrune: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			kept, pruned := keep(dumpsFor(t, tc.dates...), now)

			if got, want := keySet(kept), wantKeys(t, tc.wantKeep...); !equalKeys(got, want) {
				t.Errorf("keep set mismatch\n got:  %v\n want: %v", got, want)
			}
			if got, want := keySet(pruned), wantKeys(t, tc.wantPrune...); !equalKeys(got, want) {
				t.Errorf("prune set mismatch\n got:  %v\n want: %v", got, want)
			}
			// kept ∪ pruned must partition the input (no loss, no duplication).
			if len(kept)+len(pruned) != len(tc.dates) {
				t.Errorf("partition size = %d, want %d", len(kept)+len(pruned), len(tc.dates))
			}
		})
	}
}

func TestParseDumpDate(t *testing.T) {
	tests := []struct {
		key     string
		wantOK  bool
		wantDay string
	}{
		{"backups/pg/2026-07-12.dump", true, "2026-07-12"},
		{"backups/pg/2026-01-01.dump", true, "2026-01-01"},
		{LatestManifestKey, false, ""},            // the manifest is not a dump
		{"backups/pg/notadate.dump", false, ""},   // unparseable date
		{"backups/pg/2026-07-12.json", false, ""}, // wrong suffix
		{"other/2026-07-12.dump", false, ""},      // wrong prefix
		{"backups/pg/2026-13-40.dump", false, ""}, // invalid month/day
	}
	for _, tc := range tests {
		got, ok := parseDumpDate(tc.key)
		if ok != tc.wantOK {
			t.Errorf("parseDumpDate(%q) ok = %v, want %v", tc.key, ok, tc.wantOK)
			continue
		}
		if ok && got.Format(dumpDateLayout) != tc.wantDay {
			t.Errorf("parseDumpDate(%q) = %s, want %s", tc.key, got.Format(dumpDateLayout), tc.wantDay)
		}
	}
}

func TestDumpKeyRoundTrip(t *testing.T) {
	d := mustDate(t, "2026-07-12")
	key := dumpKeyForDate(d)
	if want := "backups/pg/2026-07-12.dump"; key != want {
		t.Fatalf("dumpKeyForDate = %q, want %q", key, want)
	}
	got, ok := parseDumpDate(key)
	if !ok || !got.Equal(d) {
		t.Fatalf("round-trip failed: ok=%v got=%s", ok, got)
	}
}
