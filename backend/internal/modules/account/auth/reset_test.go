package auth

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeResetStore struct {
	rows map[uuid.UUID]*PasswordResetTokenRow
}

func newFakeResetStore() *fakeResetStore {
	return &fakeResetStore{rows: map[uuid.UUID]*PasswordResetTokenRow{}}
}

func (s *fakeResetStore) CreatePasswordResetToken(_ context.Context, in CreatePasswordResetTokenInput) (PasswordResetTokenRow, error) {
	row := PasswordResetTokenRow{
		ID:        uuid.New(),
		UserID:    in.UserID,
		TokenHash: in.TokenHash,
		ExpiresAt: in.ExpiresAt,
		CreatedAt: time.Now().UTC(),
	}
	s.rows[row.ID] = &row
	return row, nil
}

func (s *fakeResetStore) GetPasswordResetTokenByHash(_ context.Context, hash []byte) (PasswordResetTokenRow, error) {
	for _, r := range s.rows {
		if bytes.Equal(r.TokenHash, hash) {
			return *r, nil
		}
	}
	return PasswordResetTokenRow{}, ErrNoRow
}

func (s *fakeResetStore) MarkPasswordResetTokenUsed(_ context.Context, id uuid.UUID) error {
	if r, ok := s.rows[id]; ok && !r.UsedAt.Valid {
		r.UsedAt = sql.NullTime{Time: time.Now().UTC(), Valid: true}
	}
	return nil
}

func TestResetManagerIssueVerifyReuse(t *testing.T) {
	ctx := context.Background()
	store := newFakeResetStore()
	m, err := NewResetManager(store, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	uid := uuid.New()

	pt, err := m.Issue(ctx, uid)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if pt == "" {
		t.Fatal("empty plaintext")
	}
	if len(store.rows) != 1 {
		t.Fatalf("stored %d rows, want 1", len(store.rows))
	}

	// valid token verifies to the right user
	row, err := m.Verify(ctx, pt)
	if err != nil {
		t.Fatalf("verify valid: %v", err)
	}
	if row.UserID != uid {
		t.Fatalf("row user = %v, want %v", row.UserID, uid)
	}

	// unknown token → invalid
	if _, err := m.Verify(ctx, "not-a-real-token"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("unknown token = %v, want ErrTokenInvalid", err)
	}

	// consumed → reuse rejected
	if err := m.MarkUsed(ctx, row.ID); err != nil {
		t.Fatalf("mark used: %v", err)
	}
	if _, err := m.Verify(ctx, pt); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("reused token = %v, want ErrResetTokenUsed", err)
	}
}

func TestResetManagerExpired(t *testing.T) {
	ctx := context.Background()
	store := newFakeResetStore()
	m, err := NewResetManager(store, time.Hour)
	if err != nil {
		t.Fatal(err)
	}

	// Craft an already-expired token whose hash matches a known plaintext.
	pt := "expired-plaintext-token"
	if _, err := store.CreatePasswordResetToken(ctx, CreatePasswordResetTokenInput{
		UserID:    uuid.New(),
		TokenHash: hashToken(pt),
		ExpiresAt: time.Now().UTC().Add(-time.Minute),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Verify(ctx, pt); !errors.Is(err, ErrTokenExpired) {
		t.Fatalf("expired token = %v, want ErrTokenExpired", err)
	}
}

func TestNewResetManagerValidation(t *testing.T) {
	if _, err := NewResetManager(nil, time.Hour); err == nil {
		t.Error("nil store should error")
	}
	if _, err := NewResetManager(newFakeResetStore(), 0); err == nil {
		t.Error("zero ttl should error")
	}
	if _, err := NewResetManager(newFakeResetStore(), 48*time.Hour); err == nil {
		t.Error("ttl > 24h should error")
	}
}
