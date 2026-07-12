package handler

import "testing"

// TestResetSendAllowed pins the pure per-email throttle decision: deny within the
// ≥60s cooldown, deny at/over the hourly cap, allow otherwise.
func TestResetSendAllowed(t *testing.T) {
	cases := []struct {
		name         string
		recentlySent bool
		sentThisHour int
		want         bool
	}{
		{"first send", false, 0, true},
		{"second within hour, gap ok", false, 1, true},
		{"third within hour, gap ok", false, 2, true},
		{"within cooldown", true, 0, false},
		{"within cooldown even at zero count", true, 2, false},
		{"at hourly cap", false, resetEmailMaxPerHour, false},
		{"over hourly cap", false, resetEmailMaxPerHour + 1, false},
	}
	for _, c := range cases {
		if got := resetSendAllowed(c.recentlySent, c.sentThisHour, resetEmailMaxPerHour); got != c.want {
			t.Errorf("%s: resetSendAllowed(%v, %d, %d) = %v, want %v",
				c.name, c.recentlySent, c.sentThisHour, resetEmailMaxPerHour, got, c.want)
		}
	}
}
