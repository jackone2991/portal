package ops

import "time"

// staleThreshold is the freshness budget: a backup older than this (or none at
// all) reads as stale (P0.5). 26 h gives a nightly job a two-hour grace window.
const staleThreshold = 26 * time.Hour

// computeState is the PURE freshness-sentinel decision (P0.5). Precedence,
// first match wins:
//
//  1. failed — the most recent COMPLETED run failed (something is actively
//     broken; wins even if an older success is still < 26 h old).
//  2. stale  — no success within staleThreshold, INCLUDING never-ran
//     (lastSuccessAt == nil) — a brand-new or silently-dead scheduler must read
//     as a failure state, not ok.
//  3. ok     — otherwise.
//
// lastCompletedFailed is derived from the most recent COMPLETED (ok|failed) run,
// so a still-'running' row never reaches this function and never flips state.
func computeState(lastSuccessAt *time.Time, lastCompletedFailed bool, now time.Time) State {
	if lastCompletedFailed {
		return StateFailed
	}
	if lastSuccessAt == nil || now.Sub(*lastSuccessAt) > staleThreshold {
		return StateStale
	}
	return StateOK
}

// hoursSince returns the fractional hours since lastSuccessAt, or nil when there
// has never been a success (mirrors last_success_at: null in the response).
func hoursSince(lastSuccessAt *time.Time, now time.Time) *float64 {
	if lastSuccessAt == nil {
		return nil
	}
	h := now.Sub(*lastSuccessAt).Hours()
	return &h
}
