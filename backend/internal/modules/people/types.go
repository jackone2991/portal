package people

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrNotFound        = errors.New("people: person not found")
	ErrInvalidBirthday = errors.New("people: invalid birthday")
	ErrValidation      = errors.New("people: validation error")
	ErrBadCursor       = errors.New("people: invalid cursor")
)

const (
	maxNameLen   = 120
	defaultLimit = 50
	maxLimit     = 200
)

// Birthday is month/day with an optional year (many people won't share a year).
type Birthday struct {
	Month    int
	Day      int
	Year     *int
	Calendar string // solar | lunar (default solar)
}

// Person is the module's internal record.
type Person struct {
	ID            uuid.UUID
	DisplayName   string
	Relationship  *string
	Birthday      *Birthday
	Contact       json.RawMessage
	NoteMd        *string
	AvatarAssetID *uuid.UUID
	// Circle is the closed set the right-rail sections read; Relationship stays
	// the free text you actually wrote about this person (0035).
	Circle string
	// LinkedUserID is the portal account this entry stands for, when it stands
	// for one. Nil for everyone who has no account here — the common case.
	LinkedUserID *uuid.UUID
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// The circles a person can be filed under (0035).
const (
	CircleCloseFriend = "close_friend"
	CircleFamily      = "family"
	CircleOther       = "other"
)

// ValidCircle reports whether c is one of the three circles.
func ValidCircle(c string) bool {
	return c == CircleCloseFriend || c == CircleFamily || c == CircleOther
}

// UpcomingBirthday is one row of the upcoming-birthdays response (P0.3).
type UpcomingBirthday struct {
	PersonID       uuid.UUID
	DisplayName    string
	NextOccurrence time.Time // date
	DaysUntil      int
	AgeTurning     *int // only when the birth year is known
}

// ── inputs ────────────────────────────────────────────────────────────

type CreatePersonInput struct {
	UserID       uuid.UUID
	DisplayName  string
	Relationship *string
	Birthday     *Birthday
	Contact      json.RawMessage
	NoteMd       *string
	Circle       *string
	LinkedUserID *uuid.UUID
}

// UpdatePersonInput carries presence flags so a PATCH can clear fields. When
// SetBirthday is true, Birthday (possibly nil) replaces the four columns.
type UpdatePersonInput struct {
	UserID          uuid.UUID
	ID              uuid.UUID
	DisplayName     *string
	SetRelationship bool
	Relationship    *string
	SetNote         bool
	NoteMd          *string
	Contact         json.RawMessage
	SetBirthday     bool
	Birthday        *Birthday
	Circle          *string
}

type ListInput struct {
	UserID     uuid.UUID
	CursorName string
	CursorID   uuid.UUID
	Limit      int
	// Circle filters to one section; empty means every circle.
	Circle string
}

// SolarBirthday is a person with a resolvable (solar) birthday — the scan and
// the upcoming endpoint iterate these.
type SolarBirthday struct {
	PersonID    uuid.UUID
	UserID      uuid.UUID
	DisplayName string
	Month       int
	Day         int
	Year        *int
}

// PendingNotice is an unpublished outbox row joined with its person (P0.4).
type PendingNotice struct {
	NoticeID    uuid.UUID
	Threshold   int
	Year        int
	PersonID    uuid.UUID
	UserID      uuid.UUID
	DisplayName string
}

// Repository is the persistence surface.
type Repository interface {
	CreatePerson(ctx context.Context, in CreatePersonInput) (Person, error)
	GetPerson(ctx context.Context, userID, id uuid.UUID) (Person, error)
	ListPeople(ctx context.Context, in ListInput) ([]Person, error)
	// ListLinkedUserIDs returns the portal accounts already in this user's
	// registry — what the suggestion list subtracts.
	ListLinkedUserIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error)
	UpdatePerson(ctx context.Context, in UpdatePersonInput) (Person, error)
	DeletePerson(ctx context.Context, userID, id uuid.UUID) error
	ListSolarBirthdays(ctx context.Context, userID uuid.UUID) ([]SolarBirthday, error)
	AllSolarBirthdays(ctx context.Context) ([]SolarBirthday, error)

	// outbox (P0.4)
	InsertNotice(ctx context.Context, personID uuid.UUID, year, threshold int) error
	PendingNotices(ctx context.Context) ([]PendingNotice, error)
	MarkNoticeEmitted(ctx context.Context, noticeID uuid.UUID) error
	DeleteFutureNotices(ctx context.Context, personID uuid.UUID, minYear int) error
}

// EventPublisher fans a domain event out (platform/events). Optional.
type EventPublisher interface {
	Publish(ctx context.Context, name string, payload any) error
}
