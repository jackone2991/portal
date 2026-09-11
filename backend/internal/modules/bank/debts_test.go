package bank

import "testing"

// The interest maths is the one part of debts that is pure, and the one part
// that fails silently: a wrong rate does not error, it just quietly charges the
// wrong amount every time it is accrued.

func TestAccrueInterest(t *testing.T) {
	const tenMillion = 10_000_000 // minor units

	cases := []struct {
		name       string
		out        int64
		bps        int32
		method     string
		days       int
		want       int64
		wantApprox bool
	}{
		// 12%/yr simple on 10,000,000 for a full year is exactly 1,200,000.
		{name: "simple one year", out: tenMillion, bps: 1200, method: InterestSimple, days: 365, want: 1_200_000},
		// Half a year is half of it — simple interest is linear in time.
		{name: "simple half year", out: tenMillion, bps: 1200, method: InterestSimple, days: 182, want: 598_356},

		// Daily compounding at 12% over a year gives slightly MORE than simple:
		// (1 + 0.12/365)^365 − 1 ≈ 0.12747.
		{name: "compound one year beats simple", out: tenMillion, bps: 1200, method: InterestCompound, days: 365, want: 1_274_746, wantApprox: true},

		// Everything that should produce nothing.
		{name: "no method", out: tenMillion, bps: 1200, method: InterestNone, days: 365, want: 0},
		{name: "zero rate", out: tenMillion, bps: 0, method: InterestSimple, days: 365, want: 0},
		{name: "zero days", out: tenMillion, bps: 1200, method: InterestSimple, days: 0, want: 0},
		{name: "nothing owed", out: 0, bps: 1200, method: InterestSimple, days: 365, want: 0},
		// A settled debt that overpaid must not generate negative interest.
		{name: "negative outstanding", out: -500, bps: 1200, method: InterestSimple, days: 365, want: 0},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := accrueInterest(c.out, c.bps, c.method, c.days)
			if c.wantApprox {
				// Allow a minor-unit of slack for the pow(): the assertion that
				// matters is that it exceeds the simple case, not the last digit.
				if diff := got - c.want; diff > 2 || diff < -2 {
					t.Fatalf("got %d, want ≈%d", got, c.want)
				}
				simple := accrueInterest(c.out, c.bps, InterestSimple, c.days)
				if got <= simple {
					t.Fatalf("compound %d must exceed simple %d over a year", got, simple)
				}
				return
			}
			if got != c.want {
				t.Fatalf("got %d, want %d", got, c.want)
			}
		})
	}
}

// Borrowing drives the liability account negative; lending drives it positive.
// Both must read back as a positive "still owed" number, or the UI shows a debt
// of minus ten million and every comparison against it is inverted.
func TestOutstandingFrom(t *testing.T) {
	cases := []struct {
		direction string
		balance   int64
		want      int64
	}{
		{DirBorrowed, -10_000_000, 10_000_000}, // owe 10m
		{DirBorrowed, -4_000_000, 4_000_000},   // paid 6m of it back
		{DirBorrowed, 0, 0},                    // settled
		{DirLent, 10_000_000, 10_000_000},      // they owe 10m
		{DirLent, 0, 0},                        // collected
	}
	for _, c := range cases {
		if got := outstandingFrom(c.direction, c.balance); got != c.want {
			t.Fatalf("%s balance %d: got %d, want %d", c.direction, c.balance, got, c.want)
		}
	}
}
