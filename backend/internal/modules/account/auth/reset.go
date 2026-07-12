package auth

// Password-reset tokens (SPEC-04 P0.3). Mirrors the refresh-token construction
// (ADR-06): ≥256-bit CSPRNG raw token shown once, SHA-256 hash stored at rest,
// looked up by hash in constant time, single-use, short TTL. Unlike refresh
// tokens there is no rotation chain — a reset token is minted, verified once,
// and consumed.

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ResetStore is the persistence interface for password-reset tokens. The
// sqlc-generated repository adapter implements it; tests use an in-memory fake.
type ResetStore interface {
	CreatePasswordResetToken(ctx context.Context, in CreatePasswordResetTokenInput) (PasswordResetTokenRow, error)
	GetPasswordResetTokenByHash(ctx context.Context, hash []byte) (PasswordResetTokenRow, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error
}

type CreatePasswordResetTokenInput struct {
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
}

type PasswordResetTokenRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	CreatedAt time.Time
}

// ResetManager mints and verifies password-reset tokens.
type ResetManager struct {
	store ResetStore
	ttl   time.Duration
}

func NewResetManager(store ResetStore, ttl time.Duration) (*ResetManager, error) {
	if store == nil {
		return nil, fmt.Errorf("auth: reset store required")
	}
	if ttl <= 0 || ttl > 24*time.Hour {
		return nil, fmt.Errorf("auth: reset ttl must be (0, 24h]; got %s", ttl)
	}
	return &ResetManager{store: store, ttl: ttl}, nil
}

// Issue mints a single-use reset token and returns the plaintext (shown to the
// user exactly once, via the email link). The row is stored hashed.
func (m *ResetManager) Issue(ctx context.Context, userID uuid.UUID) (string, error) {
	plaintext, err := generateResetTokenPlaintext()
	if err != nil {
		return "", err
	}
	if _, err := m.store.CreatePasswordResetToken(ctx, CreatePasswordResetTokenInput{
		UserID:    userID,
		TokenHash: hashToken(plaintext),
		ExpiresAt: time.Now().UTC().Add(m.ttl),
	}); err != nil {
		return "", fmt.Errorf("auth: persist reset token: %w", err)
	}
	return plaintext, nil
}

// Verify looks up the token by SHA-256 hash (constant-time compare) and rejects
// expired or already-used tokens. Callers must still check the user is not
// disabled and then MarkUsed on success.
func (m *ResetManager) Verify(ctx context.Context, plaintext string) (PasswordResetTokenRow, error) {
	hash := hashToken(plaintext)
	row, err := m.store.GetPasswordResetTokenByHash(ctx, hash)
	if err != nil {
		return PasswordResetTokenRow{}, ErrTokenInvalid
	}
	if subtle.ConstantTimeCompare(row.TokenHash, hash) != 1 {
		return PasswordResetTokenRow{}, ErrTokenInvalid
	}
	if row.UsedAt.Valid {
		return row, ErrResetTokenUsed
	}
	if row.ExpiresAt.Before(time.Now().UTC()) {
		return row, ErrTokenExpired
	}
	return row, nil
}

// MarkUsed consumes the token (idempotent).
func (m *ResetManager) MarkUsed(ctx context.Context, id uuid.UUID) error {
	return m.store.MarkPasswordResetTokenUsed(ctx, id)
}

func generateResetTokenPlaintext() (string, error) {
	buf := make([]byte, refreshTokenBytes) // 32 bytes = 256 bits
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
