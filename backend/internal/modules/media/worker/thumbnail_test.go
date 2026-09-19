package worker

import (
	"math"
	"testing"
)

func ptr(n int) *int { return &n }

// The poster's plan from the probe (SPEC-01 P0.2): an audio-only container is
// skipped, not failed; the seek is 10 % of the duration capped at 10 s, and
// the first frame when the duration is unknown or zero.
func TestPosterPlan(t *testing.T) {
	if _, err := posterPlan(ptr(60_000), nil); err == nil {
		t.Fatal("no video stream: want the skip error, got none")
	}
	cases := []struct {
		name  string
		durMs *int
		want  float64
	}{
		{"unknown duration → first frame", nil, 0},
		{"zero duration → first frame", ptr(0), 0},
		{"one minute → 6 s", ptr(60_000), 6},
		{"two hours → capped at 10 s", ptr(7_200_000), 10},
		{"exactly 100 s → 10 s", ptr(100_000), 10},
		{"three seconds → 0.3 s", ptr(3_000), 0.3},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := posterPlan(c.durMs, ptr(640))
			if err != nil || math.Abs(got-c.want) > 1e-9 {
				t.Fatalf("posterPlan = %v, %v; want %v", got, err, c.want)
			}
		})
	}
}
