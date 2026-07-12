// Package api is the public surface of the people module (SPEC-08).
//
// people owns an owner-scoped address book (contacts + birthdays). Its cross-
// module contract is the emit-only people:birthday_upcoming event (P0.4),
// produced by the daily people:scan_birthdays task. Consumers (stream, notify)
// attach as they land; emission is day-one regardless (ADR-08).
package api

import "github.com/google/uuid"

const (
	// TaskScanBirthdays is the daily periodic scan (P0.4). Registered on the
	// shared scheduler in cmd/worker.
	TaskScanBirthdays = "people:scan_birthdays"

	// EventBirthdayUpcoming is emitted once per (person, threshold, year).
	EventBirthdayUpcoming = "people:birthday_upcoming"
)

// BirthdayUpcomingEvent is the people:birthday_upcoming payload (events.md).
type BirthdayUpcomingEvent struct {
	NoticeID    uuid.UUID `json:"notice_id"`
	PersonID    uuid.UUID `json:"person_id"`
	UserID      uuid.UUID `json:"user_id"`
	DisplayName string    `json:"display_name"`
	DaysUntil   int       `json:"days_until"`
}
