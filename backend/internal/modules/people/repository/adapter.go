package peoplerepo

// Adapter bridges the sqlc-generated Queries to people.Repository.

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/portal/backend/internal/modules/people"
)

type Adapter struct{ q *Queries }

func NewAdapter(db *pgxpool.Pool) *Adapter { return &Adapter{q: New(db)} }

var _ people.Repository = (*Adapter)(nil)

func (a *Adapter) CreatePerson(ctx context.Context, in people.CreatePersonInput) (people.Person, error) {
	p := CreatePersonParams{
		UserID:       pgUUID(in.UserID),
		DisplayName:  in.DisplayName,
		Relationship: in.Relationship,
		NoteMd:       in.NoteMd,
	}
	if in.Birthday != nil {
		p.BirthMonth = i32p(in.Birthday.Month)
		p.BirthDay = i32p(in.Birthday.Day)
		p.BirthYear = i32pp(in.Birthday.Year)
		p.BirthCalendar = in.Birthday.Calendar
	}
	if len(in.Contact) > 0 {
		p.Contact = []byte(in.Contact)
	}
	row, err := a.q.CreatePerson(ctx, p)
	if err != nil {
		return people.Person{}, err
	}
	return toPerson(row), nil
}

func (a *Adapter) GetPerson(ctx context.Context, userID, id uuid.UUID) (people.Person, error) {
	row, err := a.q.GetPerson(ctx, GetPersonParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		return people.Person{}, mapNotFound(err)
	}
	return toPerson(row), nil
}

func (a *Adapter) ListPeople(ctx context.Context, in people.ListInput) ([]people.Person, error) {
	rows, err := a.q.ListPeople(ctx, ListPeopleParams{
		UserID: pgUUID(in.UserID), CursorName: in.CursorName, CursorID: pgUUID(in.CursorID), Lim: int32(in.Limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]people.Person, 0, len(rows))
	for _, r := range rows {
		out = append(out, toPerson(r))
	}
	return out, nil
}

func (a *Adapter) UpdatePerson(ctx context.Context, in people.UpdatePersonInput) (people.Person, error) {
	p := UpdatePersonParams{
		DisplayName:     in.DisplayName,
		SetRelationship: in.SetRelationship,
		Relationship:    in.Relationship,
		SetNote:         in.SetNote,
		NoteMd:          in.NoteMd,
		SetBirthday:     in.SetBirthday,
		ID:              pgUUID(in.ID),
		UserID:          pgUUID(in.UserID),
	}
	if len(in.Contact) > 0 {
		p.Contact = []byte(in.Contact)
	}
	if in.SetBirthday && in.Birthday != nil {
		p.BirthMonth = i32p(in.Birthday.Month)
		p.BirthDay = i32p(in.Birthday.Day)
		p.BirthYear = i32pp(in.Birthday.Year)
		cal := in.Birthday.Calendar
		p.BirthCalendar = &cal
	}
	row, err := a.q.UpdatePerson(ctx, p)
	if err != nil {
		return people.Person{}, mapNotFound(err)
	}
	return toPerson(row), nil
}

func (a *Adapter) DeletePerson(ctx context.Context, userID, id uuid.UUID) error {
	_, err := a.q.DeletePerson(ctx, DeletePersonParams{ID: pgUUID(id), UserID: pgUUID(userID)})
	if err != nil {
		return mapNotFound(err)
	}
	return nil
}

func (a *Adapter) ListSolarBirthdays(ctx context.Context, userID uuid.UUID) ([]people.SolarBirthday, error) {
	rows, err := a.q.ListSolarBirthdays(ctx, pgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]people.SolarBirthday, 0, len(rows))
	for _, r := range rows {
		out = append(out, people.SolarBirthday{
			PersonID: uuidFrom(r.ID), UserID: userID, DisplayName: r.DisplayName,
			Month: int(*r.BirthMonth), Day: int(*r.BirthDay), Year: i32ToIntP(r.BirthYear),
		})
	}
	return out, nil
}

func (a *Adapter) AllSolarBirthdays(ctx context.Context) ([]people.SolarBirthday, error) {
	rows, err := a.q.AllSolarBirthdays(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]people.SolarBirthday, 0, len(rows))
	for _, r := range rows {
		out = append(out, people.SolarBirthday{
			PersonID: uuidFrom(r.ID), UserID: uuidFrom(r.UserID), DisplayName: r.DisplayName,
			Month: int(*r.BirthMonth), Day: int(*r.BirthDay), Year: i32ToIntP(r.BirthYear),
		})
	}
	return out, nil
}

func (a *Adapter) InsertNotice(ctx context.Context, personID uuid.UUID, year, threshold int) error {
	return a.q.InsertNotice(ctx, InsertNoticeParams{PersonID: pgUUID(personID), Year: int32(year), Threshold: int32(threshold)})
}

func (a *Adapter) PendingNotices(ctx context.Context) ([]people.PendingNotice, error) {
	rows, err := a.q.PendingNotices(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]people.PendingNotice, 0, len(rows))
	for _, r := range rows {
		out = append(out, people.PendingNotice{
			NoticeID: uuidFrom(r.ID), Threshold: int(r.Threshold), Year: int(r.Year),
			PersonID: uuidFrom(r.PersonID), UserID: uuidFrom(r.UserID), DisplayName: r.DisplayName,
		})
	}
	return out, nil
}

func (a *Adapter) MarkNoticeEmitted(ctx context.Context, noticeID uuid.UUID) error {
	return a.q.MarkNoticeEmitted(ctx, pgUUID(noticeID))
}

func (a *Adapter) DeleteFutureNotices(ctx context.Context, personID uuid.UUID, minYear int) error {
	return a.q.DeleteFutureNotices(ctx, DeleteFutureNoticesParams{PersonID: pgUUID(personID), Year: int32(minYear)})
}

// ── mapping ────────────────────────────────────────────────────────────

func mapNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return people.ErrNotFound
	}
	return err
}

func toPerson(r PeoplePerson) people.Person {
	p := people.Person{
		ID: uuidFrom(r.ID), DisplayName: r.DisplayName, Relationship: r.Relationship,
		Contact: json.RawMessage(r.Contact), NoteMd: r.NoteMd, AvatarAssetID: uuidPtr(r.AvatarAssetID),
		CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time,
	}
	if r.BirthMonth != nil && r.BirthDay != nil {
		p.Birthday = &people.Birthday{Month: int(*r.BirthMonth), Day: int(*r.BirthDay), Year: i32ToIntP(r.BirthYear), Calendar: r.BirthCalendar}
	}
	return p
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidPtr(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	id := uuid.UUID(p.Bytes)
	return &id
}

func i32p(v int) *int32 { x := int32(v); return &x }

func i32pp(p *int) *int32 {
	if p == nil {
		return nil
	}
	x := int32(*p)
	return &x
}

func i32ToIntP(p *int32) *int {
	if p == nil {
		return nil
	}
	x := int(*p)
	return &x
}
