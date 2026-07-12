package people

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

var vn = time.FixedZone("VN", 7*3600) // UTC+7 (no DST)

func iptr(n int) *int { return &n }

// ── nextOccurrence (P0.3, shared by scan + endpoint) ──────────────────

func TestNextOccurrence(t *testing.T) {
	cases := []struct {
		name       string
		now        time.Time
		month, day int
		wantMonth  time.Month
		wantDay    int
		wantDays   int
	}{
		{"tomorrow", time.Date(2026, 6, 14, 3, 0, 0, 0, vn), 6, 15, time.June, 15, 1},
		{"today", time.Date(2026, 6, 15, 10, 0, 0, 0, vn), 6, 15, time.June, 15, 0},
		{"wraps to next year", time.Date(2026, 12, 31, 0, 0, 0, 0, vn), 1, 1, time.January, 1, 1},
		{"feb29 in non-leap → feb28", time.Date(2027, 1, 1, 0, 0, 0, 0, vn), 2, 29, time.February, 28, 58},
		{"feb29 in leap stays feb29", time.Date(2028, 1, 1, 0, 0, 0, 0, vn), 2, 29, time.February, 29, 59},
	}
	for _, c := range cases {
		occ, days := nextOccurrence(c.now, c.month, c.day, vn)
		if occ.Month() != c.wantMonth || occ.Day() != c.wantDay || days != c.wantDays {
			t.Errorf("%s: got %s (days %d), want %s-%d (days %d)", c.name, occ.Format("Jan-2"), days, c.wantMonth, c.wantDay, c.wantDays)
		}
	}
}

func TestNextOccurrenceTimezone(t *testing.T) {
	// 2026-06-14 23:00 UTC == 2026-06-15 06:00 in UTC+7. A birthday on the 16th is
	// 1 day out in the owner's TZ (2 in UTC) — the regression the spec calls out.
	now := time.Date(2026, 6, 14, 23, 0, 0, 0, time.UTC)
	_, days := nextOccurrence(now, 6, 16, vn)
	if days != 1 {
		t.Fatalf("TZ days_until = %d, want 1 (computed in the owner's TZ)", days)
	}
}

// ── birthday validation (P0.2) ────────────────────────────────────────

func TestBirthdayValidation(t *testing.T) {
	ok := []*Birthday{
		{Month: 3, Day: 15},
		{Month: 2, Day: 29},                   // no year → leap allowed
		{Month: 2, Day: 29, Year: iptr(2000)}, // leap year
		{Month: 12, Day: 31, Year: iptr(1990)},
		nil, // no birthday is valid
	}
	for i, b := range ok {
		if err := validateBirthday(b); err != nil {
			t.Errorf("ok[%d] = %v, want nil", i, err)
		}
	}
	bad := []*Birthday{
		{Month: 2, Day: 30},                   // Feb-30 impossible
		{Month: 4, Day: 31},                   // Apr-31 impossible
		{Month: 13, Day: 1},                   // month out of range
		{Month: 2, Day: 29, Year: iptr(2027)}, // Feb-29 in a non-leap year
		{Month: 1, Day: 1, Year: iptr(1899)},  // year too old
	}
	for i, b := range bad {
		if err := validateBirthday(b); !errors.Is(err, ErrInvalidBirthday) {
			t.Errorf("bad[%d] = %v, want ErrInvalidBirthday", i, err)
		}
	}
}

// ── scan dedup + outbox (P0.4) ────────────────────────────────────────

func newSvc() (*Service, *fakePeople, *fakeEvents) {
	repo := newFakePeople()
	ev := &fakeEvents{}
	return &Service{repo: repo, events: ev, loc: vn}, repo, ev
}

func TestScanDedupAndOutbox(t *testing.T) {
	svc, repo, ev := newSvc()
	ctx := context.Background()
	user := uuid.New()
	// birthday 3 days out
	pid := repo.addPerson(user, "Mẹ", 6, 18)
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, vn)

	svc.ScanBirthdays(ctx, now)
	if ev.count != 1 {
		t.Fatalf("first scan emits = %d, want 1 (the 3-day threshold)", ev.count)
	}
	// re-scan the same day → dedup, no new emit
	svc.ScanBirthdays(ctx, now)
	if ev.count != 1 {
		t.Fatalf("re-scan emits = %d, want still 1 (dedup)", ev.count)
	}
	// day-of → one more (threshold 0)
	svc.ScanBirthdays(ctx, time.Date(2026, 6, 18, 8, 0, 0, 0, vn))
	if ev.count != 2 {
		t.Fatalf("day-of scan emits = %d, want 2", ev.count)
	}
	_ = pid
}

func TestOutboxRetry(t *testing.T) {
	svc, repo, ev := newSvc()
	ctx := context.Background()
	user := uuid.New()
	repo.addPerson(user, "Bố", 6, 18)
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, vn)

	// publish fails → notice stays pending (emitted_at NULL)
	ev.fail = true
	svc.ScanBirthdays(ctx, now)
	if ev.count != 0 || repo.pendingCount() != 1 {
		t.Fatalf("failed publish: count=%d pending=%d, want 0/1", ev.count, repo.pendingCount())
	}
	// next scan re-publishes the pending notice
	ev.fail = false
	svc.ScanBirthdays(ctx, now)
	if ev.count != 1 || repo.pendingCount() != 0 {
		t.Fatalf("retry: count=%d pending=%d, want 1/0", ev.count, repo.pendingCount())
	}
}

// ── upcoming (P0.3) ───────────────────────────────────────────────────

func TestUpcomingBirthdays(t *testing.T) {
	svc, repo, _ := newSvc()
	ctx := context.Background()
	user := uuid.New()
	repo.addPersonY(user, "Soon", 6, 18, iptr(1990)) // 3 days
	repo.addPerson(user, "Later", 8, 1)              // ~47 days
	now := time.Date(2026, 6, 15, 8, 0, 0, 0, vn)

	items, _ := svc.UpcomingBirthdays(ctx, user, 14, now)
	if len(items) != 1 || items[0].DisplayName != "Soon" || items[0].DaysUntil != 3 {
		t.Fatalf("14-day window = %+v, want only Soon at 3 days", items)
	}
	if items[0].AgeTurning == nil || *items[0].AgeTurning != 36 {
		t.Fatalf("age_turning = %v, want 36", items[0].AgeTurning)
	}
}

// ── fakes ─────────────────────────────────────────────────────────────

type fakePerson struct {
	id, user   uuid.UUID
	name       string
	month, day int
	year       *int
}
type notice struct {
	person    uuid.UUID
	year, thr int
	emitted   bool
}
type fakePeople struct {
	persons map[uuid.UUID]*fakePerson
	notices map[string]*notice
}

func newFakePeople() *fakePeople {
	return &fakePeople{persons: map[uuid.UUID]*fakePerson{}, notices: map[string]*notice{}}
}
func nkey(p uuid.UUID, y, t int) string { return p.String() + "|" + itoa(y) + "|" + itoa(t) }
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func (r *fakePeople) addPerson(user uuid.UUID, name string, m, d int) uuid.UUID {
	return r.addPersonY(user, name, m, d, nil)
}
func (r *fakePeople) addPersonY(user uuid.UUID, name string, m, d int, y *int) uuid.UUID {
	id := uuid.New()
	r.persons[id] = &fakePerson{id: id, user: user, name: name, month: m, day: d, year: y}
	return id
}
func (r *fakePeople) pendingCount() int {
	n := 0
	for _, x := range r.notices {
		if !x.emitted {
			n++
		}
	}
	return n
}

func (r *fakePeople) CreatePerson(_ context.Context, _ CreatePersonInput) (Person, error) {
	return Person{}, nil
}
func (r *fakePeople) GetPerson(_ context.Context, _, _ uuid.UUID) (Person, error) {
	return Person{}, ErrNotFound
}
func (r *fakePeople) ListPeople(_ context.Context, _ ListInput) ([]Person, error) { return nil, nil }
func (r *fakePeople) UpdatePerson(_ context.Context, _ UpdatePersonInput) (Person, error) {
	return Person{}, nil
}
func (r *fakePeople) DeletePerson(_ context.Context, _, _ uuid.UUID) error { return nil }
func (r *fakePeople) ListSolarBirthdays(_ context.Context, user uuid.UUID) ([]SolarBirthday, error) {
	var out []SolarBirthday
	for _, p := range r.persons {
		if p.user == user {
			out = append(out, SolarBirthday{PersonID: p.id, UserID: p.user, DisplayName: p.name, Month: p.month, Day: p.day, Year: p.year})
		}
	}
	return out, nil
}
func (r *fakePeople) AllSolarBirthdays(_ context.Context) ([]SolarBirthday, error) {
	var out []SolarBirthday
	for _, p := range r.persons {
		out = append(out, SolarBirthday{PersonID: p.id, UserID: p.user, DisplayName: p.name, Month: p.month, Day: p.day, Year: p.year})
	}
	return out, nil
}
func (r *fakePeople) InsertNotice(_ context.Context, person uuid.UUID, year, threshold int) error {
	k := nkey(person, year, threshold)
	if _, ok := r.notices[k]; !ok {
		r.notices[k] = &notice{person: person, year: year, thr: threshold}
	}
	return nil
}
func (r *fakePeople) PendingNotices(_ context.Context) ([]PendingNotice, error) {
	var out []PendingNotice
	for _, n := range r.notices {
		if n.emitted {
			continue
		}
		p := r.persons[n.person]
		out = append(out, PendingNotice{NoticeID: uuid.New(), Threshold: n.thr, Year: n.year, PersonID: n.person, UserID: p.user, DisplayName: p.name})
	}
	return out, nil
}
func (r *fakePeople) MarkNoticeEmitted(_ context.Context, _ uuid.UUID) error {
	// The fake keys notices by (person,year,threshold), not the surrogate id, so
	// mark all still-pending as emitted (the scan marks each pending it published).
	for _, n := range r.notices {
		if !n.emitted {
			n.emitted = true
		}
	}
	return nil
}
func (r *fakePeople) DeleteFutureNotices(_ context.Context, person uuid.UUID, minYear int) error {
	for k, n := range r.notices {
		if n.person == person && n.year >= minYear {
			delete(r.notices, k)
		}
	}
	return nil
}

type fakeEvents struct {
	count int
	fail  bool
}

func (e *fakeEvents) Publish(_ context.Context, _ string, _ any) error {
	if e.fail {
		return errors.New("publish failed")
	}
	e.count++
	return nil
}
