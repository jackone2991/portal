package social

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	socialapi "github.com/portal/backend/internal/modules/social/api"
)

// Service is the social domain logic.
type Service struct {
	repo   Repository
	names  NamesFunc
	events EventPublisher
}

// Request asks `target` to connect.
//
// Two shortcuts matter here. If the pair already has a row, the answer is
// ErrExists whichever way it points — a duplicate request is not a new fact. But
// if THEY already asked YOU, sending a request is unambiguously agreement, so it
// accepts theirs instead of failing: the alternative is telling someone "already
// exists" while the request they wanted to accept sits unanswered.
func (s *Service) Request(ctx context.Context, me, target uuid.UUID) (Connection, error) {
	if me == target || target == uuid.Nil {
		return Connection{}, ErrSelf
	}

	if existing, err := s.repo.FindBetween(ctx, me, target); err == nil {
		if existing.Status == StatusPending && existing.AddresseeID == me {
			return s.Accept(ctx, me, existing.ID)
		}
		return Connection{}, ErrExists
	} else if !errors.Is(err, ErrNotFound) {
		return Connection{}, err
	}

	c, err := s.repo.CreateRequest(ctx, me, target)
	if err != nil {
		return Connection{}, err
	}
	s.emit(ctx, socialapi.EventConnectionRequested, c)
	return c, nil
}

// Accept agrees to a pending request. Only the addressee can, which the 0037
// UPDATE policy enforces as well as the statement's predicate.
func (s *Service) Accept(ctx context.Context, me, id uuid.UUID) (Connection, error) {
	c, err := s.repo.Accept(ctx, id, me)
	if err != nil {
		return Connection{}, err
	}
	s.emit(ctx, socialapi.EventConnectionAccepted, c)
	return c, nil
}

// Remove withdraws a request, declines one, or disconnects — one operation,
// because the row means the same thing in all three cases.
func (s *Service) Remove(ctx context.Context, me, id uuid.UUID) error {
	_, err := s.repo.Delete(ctx, id, me)
	return err
}

// List renders one of the three lists from the caller's point of view, with the
// other party's name resolved in a single batch.
func (s *Service) List(ctx context.Context, me uuid.UUID, kind Kind) ([]Party, error) {
	rows, err := s.repo.List(ctx, me, kind)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	ids := make([]uuid.UUID, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.Other(me))
	}
	names := map[uuid.UUID]string{}
	if s.names != nil {
		names, err = s.names(ctx, ids)
		if err != nil {
			return nil, err
		}
	}

	out := make([]Party, 0, len(rows))
	for _, c := range rows {
		other := c.Other(me)
		name := names[other]
		if name == "" {
			// A name that will not resolve is still a real connection; showing it
			// as "Unknown" beats dropping the row and leaving an unanswerable
			// request invisible.
			name = "Unknown"
		}
		out = append(out, Party{
			ID:          c.ID,
			UserID:      other,
			DisplayName: name,
			Status:      c.Status,
			Outgoing:    c.RequesterID == me,
			CreatedAt:   c.CreatedAt,
		})
	}
	return out, nil
}

// CounterpartIDs implements socialapi.API.
func (s *Service) CounterpartIDs(ctx context.Context, userID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.CounterpartIDs(ctx, userID)
}

// CountIncoming backs the header badge.
func (s *Service) CountIncoming(ctx context.Context, me uuid.UUID) (int, error) {
	return s.repo.CountIncoming(ctx, me)
}

// emit publishes a connection event. Best-effort: the connection is already
// committed, and a failed notification must not undo it.
func (s *Service) emit(ctx context.Context, event string, c Connection) {
	if s.events == nil {
		return
	}
	names := map[uuid.UUID]string{}
	if s.names != nil {
		if n, err := s.names(ctx, []uuid.UUID{c.RequesterID, c.AddresseeID}); err == nil {
			names = n
		}
	}
	ev := socialapi.ConnectionEvent{
		ConnectionID:  c.ID.String(),
		RequesterID:   c.RequesterID.String(),
		AddresseeID:   c.AddresseeID.String(),
		RequesterName: names[c.RequesterID],
		AddresseeName: names[c.AddresseeID],
	}
	if err := s.events.Publish(ctx, event, ev); err != nil {
		log.Warn().Err(err).Str("event", event).Str("connection", ev.ConnectionID).Msg("social: publish failed")
	}
}
