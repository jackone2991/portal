package people

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	peopleapi "github.com/portal/backend/internal/modules/people/api"
	"github.com/portal/backend/internal/platform/server"
)

// Service holds the people business logic. loc is the instance-default timezone
// (D-17 stores a per-user TZ later; v1 uses one default for the scan + endpoint).
type Service struct {
	repo   Repository
	events EventPublisher
	loc    *time.Location
	// runInUserTenant scopes the worker notice INSERT to the person's owner org
	// (ADR-07 1b) so people_birthday_notices.tenant_id's DEFAULT is set. nil → direct.
	runInUserTenant func(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error
}

// runScoped runs fn in the target user's tenant scope (ADR-07 1b) on the worker;
// a nil runInUserTenant (API side / tests) runs fn directly.
func (s *Service) runScoped(ctx context.Context, userID uuid.UUID, fn func(context.Context) error) error {
	if s.runInUserTenant == nil {
		return fn(ctx)
	}
	return s.runInUserTenant(ctx, userID, fn)
}

type ListResult struct {
	Items      []Person
	NextCursor string
}

// ══ CRUD (P0.2) ═════════════════════════════════════════════════════════

func (s *Service) CreatePerson(ctx context.Context, in CreatePersonInput) (Person, error) {
	in.DisplayName = strings.TrimSpace(in.DisplayName)
	if !validName(in.DisplayName) {
		return Person{}, ErrValidation
	}
	if err := validateBirthday(in.Birthday); err != nil {
		return Person{}, err
	}
	return s.repo.CreatePerson(ctx, in)
}

func (s *Service) GetPerson(ctx context.Context, userID, id uuid.UUID) (Person, error) {
	return s.repo.GetPerson(ctx, userID, id)
}

func (s *Service) ListPeople(ctx context.Context, userID uuid.UUID, cursor string, limit int) (ListResult, error) {
	if limit <= 0 || limit > maxLimit {
		limit = defaultLimit
	}
	in := ListInput{UserID: userID, Limit: limit + 1}
	if cursor != "" {
		name, id, err := decodeCursor(cursor)
		if err != nil {
			return ListResult{}, ErrBadCursor
		}
		in.CursorName, in.CursorID = name, id
	}
	rows, err := s.repo.ListPeople(ctx, in)
	if err != nil {
		return ListResult{}, err
	}
	var res ListResult
	if len(rows) > limit {
		res.NextCursor = encodeCursor(rows[limit-1])
		rows = rows[:limit]
	}
	res.Items = rows
	return res, nil
}

func (s *Service) UpdatePerson(ctx context.Context, in UpdatePersonInput) (Person, error) {
	if in.DisplayName != nil {
		n := strings.TrimSpace(*in.DisplayName)
		if !validName(n) {
			return Person{}, ErrValidation
		}
		in.DisplayName = &n
	}
	if in.SetBirthday {
		if err := validateBirthday(in.Birthday); err != nil {
			return Person{}, err
		}
	}
	p, err := s.repo.UpdatePerson(ctx, in)
	if err != nil {
		return Person{}, err
	}
	// A birthday edit/clear resets current + future notices so a corrected date
	// fires fresh this year (P0.2). Best-effort.
	if in.SetBirthday {
		year := time.Now().In(s.loc).Year()
		if derr := s.repo.DeleteFutureNotices(ctx, in.ID, year); derr != nil {
			log.Warn().Err(derr).Str("person", in.ID.String()).Msg("people: reset notices failed")
		}
	}
	return p, nil
}

func (s *Service) DeletePerson(ctx context.Context, userID, id uuid.UUID) error {
	return s.repo.DeletePerson(ctx, userID, id)
}

// ══ Upcoming birthdays (P0.3) ═══════════════════════════════════════════

func (s *Service) UpcomingBirthdays(ctx context.Context, userID uuid.UUID, days int, now time.Time) ([]UpcomingBirthday, error) {
	if days < 1 {
		days = 14
	}
	if days > 366 {
		days = 366
	}
	rows, err := s.repo.ListSolarBirthdays(ctx, userID)
	if err != nil {
		return nil, err
	}
	var out []UpcomingBirthday
	for _, r := range rows {
		occ, daysUntil := nextOccurrence(now, r.Month, r.Day, s.loc)
		if daysUntil > days {
			continue
		}
		ub := UpcomingBirthday{PersonID: r.PersonID, DisplayName: r.DisplayName, NextOccurrence: occ, DaysUntil: daysUntil}
		if r.Year != nil {
			age := occ.Year() - *r.Year
			ub.AgeTurning = &age
		}
		out = append(out, ub)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].DaysUntil < out[j].DaysUntil })
	return out, nil
}

// ══ Scan + event (P0.4) ═════════════════════════════════════════════════

var birthdayThresholds = []int{3, 0}

// ScanBirthdays reserves notice rows for due (person, threshold, year) slots,
// then publishes every pending (unpublished) notice and marks it emitted —
// at-least-once via the outbox, dedup by the composite PK.
func (s *Service) ScanBirthdays(ctx context.Context, now time.Time) error {
	rows, err := s.repo.AllSolarBirthdays(ctx)
	if err != nil {
		return err
	}
	for _, r := range rows {
		occ, daysUntil := nextOccurrence(now, r.Month, r.Day, s.loc)
		for _, T := range birthdayThresholds {
			if daysUntil >= 0 && daysUntil <= T {
				if err := s.runScoped(ctx, r.UserID, func(ctx context.Context) error {
					return s.repo.InsertNotice(ctx, r.PersonID, occ.Year(), T)
				}); err != nil {
					log.Warn().Err(err).Str("person", r.PersonID.String()).Msg("people: insert notice failed")
				}
			}
		}
	}
	pending, err := s.repo.PendingNotices(ctx)
	if err != nil {
		return err
	}
	for _, n := range pending {
		if s.events != nil {
			// days_until = the threshold that matched (the intended reminder distance).
			if err := s.events.Publish(ctx, peopleapi.EventBirthdayUpcoming, peopleapi.BirthdayUpcomingEvent{
				NoticeID: n.NoticeID, PersonID: n.PersonID, UserID: n.UserID, DisplayName: n.DisplayName, DaysUntil: n.Threshold,
			}); err != nil {
				log.Warn().Err(err).Str("notice", n.NoticeID.String()).Msg("people: birthday event publish failed")
				continue // leave emitted_at NULL → retried next scan
			}
		}
		if err := s.repo.MarkNoticeEmitted(ctx, n.NoticeID); err != nil {
			log.Warn().Err(err).Str("notice", n.NoticeID.String()).Msg("people: mark emitted failed")
		}
	}
	return nil
}

// ── birthday math (P0.3; shared by scan + endpoint) ───────────────────

// nextOccurrence returns the next (month, day) date on or after today in loc,
// with days_until. Feb-29 in a non-leap year celebrates on Feb-28 (P0.3).
func nextOccurrence(now time.Time, month, day int, loc *time.Location) (time.Time, int) {
	nl := now.In(loc)
	today := time.Date(nl.Year(), nl.Month(), nl.Day(), 0, 0, 0, 0, loc)
	occ := occurrenceInYear(today.Year(), month, day, loc)
	if occ.Before(today) {
		occ = occurrenceInYear(today.Year()+1, month, day, loc)
	}
	days := int(math.Round(occ.Sub(today).Hours() / 24))
	return occ, days
}

func occurrenceInYear(year, month, day int, loc *time.Location) time.Time {
	if month == 2 && day == 29 && !isLeapYear(year) {
		day = 28 // celebrate Feb-28 in non-leap years
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, loc)
}

func isLeapYear(y int) bool { return y%4 == 0 && (y%100 != 0 || y%400 == 0) }

// ── validation ────────────────────────────────────────────────────────

func validateBirthday(b *Birthday) error {
	if b == nil {
		return nil
	}
	if b.Month < 1 || b.Month > 12 || b.Day < 1 || b.Day > 31 {
		return ErrInvalidBirthday
	}
	if b.Calendar != "" && b.Calendar != "solar" && b.Calendar != "lunar" {
		return ErrInvalidBirthday
	}
	if b.Year != nil {
		if *b.Year < 1900 || *b.Year > time.Now().Year() {
			return ErrInvalidBirthday
		}
		if !validCalendarDate(*b.Year, b.Month, b.Day) {
			return ErrInvalidBirthday
		}
		return nil
	}
	// No year: validate against a leap year so Feb-29 is allowed.
	if !validCalendarDate(2000, b.Month, b.Day) {
		return ErrInvalidBirthday
	}
	return nil
}

// validCalendarDate rejects impossible dates (Feb-30, Apr-31): time.Date
// normalizes overflow, so a mismatch after construction means the input was bad.
func validCalendarDate(y, m, d int) bool {
	t := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
	return t.Year() == y && int(t.Month()) == m && t.Day() == d
}

func validName(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= maxNameLen
}

// cursor "<display_name>|<id>", base64url.
func encodeCursor(p Person) string {
	return server.EncodeCursor(p.DisplayName, p.ID)
}

func decodeCursor(s string) (string, uuid.UUID, error) {
	return server.DecodeCursor(s)
}
