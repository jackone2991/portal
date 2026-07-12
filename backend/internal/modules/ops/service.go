package ops

import (
	"context"
	"time"
)

// Service holds the ops read-side logic — the freshness sentinel (P0.5). The
// nightly backup task lives on *Module (worker side); it never touches Service.
type Service struct {
	repo Repository
}

// StatusResult is the GET /ops/status projection (P0.5). HoursSinceSuccess and
// LastSuccessAt are nil together when a backup has never succeeded.
type StatusResult struct {
	LastSuccessAt     *time.Time
	HoursSinceSuccess *float64
	State             State
	LastRun           *BackupRun
}

// Status assembles the freshness verdict. It reads three cheap single-row
// queries and folds them through the pure computeState (never-ran → stale/null).
func (s *Service) Status(ctx context.Context) (StatusResult, error) {
	lastRun, err := s.repo.GetLastRun(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	lastSuccess, err := s.repo.GetLastSuccessfulRun(ctx)
	if err != nil {
		return StatusResult{}, err
	}
	lastCompleted, err := s.repo.GetLastCompletedRun(ctx)
	if err != nil {
		return StatusResult{}, err
	}

	var lastSuccessAt *time.Time
	if lastSuccess != nil && lastSuccess.FinishedAt != nil {
		lastSuccessAt = lastSuccess.FinishedAt
	}
	lastCompletedFailed := lastCompleted != nil && lastCompleted.Status == StatusFailed

	now := time.Now().UTC()
	return StatusResult{
		LastSuccessAt:     lastSuccessAt,
		HoursSinceSuccess: hoursSince(lastSuccessAt, now),
		State:             computeState(lastSuccessAt, lastCompletedFailed, now),
		LastRun:           lastRun,
	}, nil
}
