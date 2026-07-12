package ops

import (
	"sort"
	"strings"
	"time"
)

// Storage-key layout for dumps: backups/pg/<yyyy-mm-dd>.dump, plus the LATEST.json
// manifest pointer the restore drill reads (P0.2 / P0.4).
const (
	backupPrefix   = "backups/pg/"
	dumpKeySuffix  = ".dump"
	dumpDateLayout = "2006-01-02"

	// LatestManifestKey is the small pointer object overwritten after every
	// successful run — the restore drill selects from it, never by key-listing.
	LatestManifestKey = backupPrefix + "LATEST.json"
)

// Retention window (P0.2 step 5): the N most recent daily dumps plus the M most
// recent weekly anchors (Sunday dumps).
const (
	dailyKeep  = 7
	weeklyKeep = 4
)

// DumpMeta identifies one stored dump: its storage key and the calendar date
// parsed from the key. Retention operates purely on these.
type DumpMeta struct {
	Key  string
	Date time.Time
}

// keep is the PURE retention selector (P0.2 step 5). It keeps the 7 most recent
// daily dumps plus the 4 most recent weekly anchors (dumps whose date is a
// Sunday); everything else is pruned. Deterministic and input-order-independent.
// The caller is responsible for never pruning the live LATEST target — but since
// the newest dump is always within the daily window, it is inherently retained.
func keep(dumps []DumpMeta, _ time.Time) (kept, pruned []DumpMeta) {
	sorted := append([]DumpMeta(nil), dumps...)
	sort.Slice(sorted, func(i, j int) bool {
		if !sorted[i].Date.Equal(sorted[j].Date) {
			return sorted[i].Date.After(sorted[j].Date) // newest first
		}
		return sorted[i].Key > sorted[j].Key // stable tie-break
	})

	keepKeys := make(map[string]bool, len(sorted))

	// Daily: the most recent `dailyKeep` dumps regardless of weekday.
	for i := 0; i < len(sorted) && i < dailyKeep; i++ {
		keepKeys[sorted[i].Key] = true
	}

	// Weekly: the most recent `weeklyKeep` Sunday dumps (one Sunday per calendar
	// week), extending retention past the daily window.
	weeks := 0
	for _, d := range sorted {
		if d.Date.Weekday() != time.Sunday {
			continue
		}
		keepKeys[d.Key] = true
		if weeks++; weeks >= weeklyKeep {
			break
		}
	}

	for _, d := range sorted {
		if keepKeys[d.Key] {
			kept = append(kept, d)
		} else {
			pruned = append(pruned, d)
		}
	}
	return kept, pruned
}

// parseDumpDate extracts the calendar date embedded in a dump storage key
// ("backups/pg/2026-07-12.dump" → 2026-07-12). It returns false for the manifest
// or any key that isn't a dated dump, so retention never touches non-dumps.
func parseDumpDate(key string) (time.Time, bool) {
	if !strings.HasPrefix(key, backupPrefix) || !strings.HasSuffix(key, dumpKeySuffix) {
		return time.Time{}, false
	}
	datePart := strings.TrimSuffix(strings.TrimPrefix(key, backupPrefix), dumpKeySuffix)
	t, err := time.Parse(dumpDateLayout, datePart)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

// dumpKeyForDate is the inverse of parseDumpDate: the storage key a dump taken on
// the given day is written to (same-day reruns overwrite one object).
func dumpKeyForDate(t time.Time) string {
	return backupPrefix + t.UTC().Format(dumpDateLayout) + dumpKeySuffix
}
