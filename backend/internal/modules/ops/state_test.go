package ops

import (
	"testing"
	"time"
)

func TestComputeState(t *testing.T) {
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	ago := func(d time.Duration) *time.Time { t := now.Add(-d); return &t }

	tests := []struct {
		name                string
		lastSuccessAt       *time.Time
		lastCompletedFailed bool
		want                State
	}{
		{
			name:          "fresh success 1h ago → ok",
			lastSuccessAt: ago(1 * time.Hour),
			want:          StateOK,
		},
		{
			name:          "success exactly at the 26h budget → ok (within window)",
			lastSuccessAt: ago(26 * time.Hour),
			want:          StateOK,
		},
		{
			name:          "success 27h ago, nothing completed since → stale",
			lastSuccessAt: ago(27 * time.Hour),
			want:          StateStale,
		},
		{
			name:                "success 2h ago FOLLOWED BY a failed run → failed (precedence)",
			lastSuccessAt:       ago(2 * time.Hour),
			lastCompletedFailed: true,
			want:                StateFailed,
		},
		{
			name:          "zero runs ever → stale (never-ran)",
			lastSuccessAt: nil,
			want:          StateStale,
		},
		{
			name:          "recent success then a running (not completed) run → still ok",
			lastSuccessAt: ago(1 * time.Hour),
			// a 'running' row is not a completed run, so lastCompletedFailed stays false
			lastCompletedFailed: false,
			want:                StateOK,
		},
		{
			name:                "failed wins even over a still-fresh older success",
			lastSuccessAt:       ago(30 * time.Minute),
			lastCompletedFailed: true,
			want:                StateFailed,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := computeState(tc.lastSuccessAt, tc.lastCompletedFailed, now); got != tc.want {
				t.Errorf("computeState = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHoursSince(t *testing.T) {
	now := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)

	if got := hoursSince(nil, now); got != nil {
		t.Errorf("hoursSince(nil) = %v, want nil", *got)
	}

	past := now.Add(-3 * time.Hour)
	got := hoursSince(&past, now)
	if got == nil {
		t.Fatalf("hoursSince(non-nil) = nil, want ~3")
	}
	if *got < 2.99 || *got > 3.01 {
		t.Errorf("hoursSince = %v, want ~3", *got)
	}
}
