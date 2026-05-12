package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
)

// refreshTokenBytes is the raw entropy length. 32 bytes = 256 bits.
const refreshTokenBytes = 32

// RefreshStore is the persistence interface for refresh tokens. The sqlc-
// generated repository implements this; tests use an in-memory fake.
type RefreshStore interface {
	Create(ctx context.Context, in CreateRefreshTokenInput) (RefreshTokenRow, error)
	GetByHash(ctx context.Context, hash []byte) (RefreshTokenRow, error)
	MarkReplaced(ctx context.Context, id uuid.UUID, replacedBy uuid.UUID) error
	Revoke(ctx context.Context, id uuid.UUID, reason string) error
	RevokeChain(ctx context.Context, anyTokenID uuid.UUID, reason string) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error
}

type CreateRefreshTokenInput struct {
	UserID    uuid.UUID
	TokenHash []byte
	ExpiresAt time.Time
	ParentID  *uuid.UUID
	IP        net.IP
	UserAgent string
}

type RefreshTokenRow struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	TokenHash     []byte
	ExpiresAt     time.Time
	CreatedAt     time.Time
	RevokedAt     sql.NullTime
	RevokeReason  sql.NullString
	ReplacedByID  *uuid.UUID
	ParentID      *uuid.UUID
}

// RefreshManager handles issue, rotate, and revoke for refresh tokens.
type RefreshManager struct {
	store RefreshStore
	ttl   time.Duration
}

func NewRefreshManager(store RefreshStore, ttl time.Duration) (*RefreshManager, error) {
	if store == nil {
		return nil, fmt.Errorf("auth: refresh store required")
	}
	if ttl <= 0 || ttl > 90*24*time.Hour {
		return nil, fmt.Errorf("auth: refresh ttl must be (0, 90d]; got %s", ttl)
	}
	return &RefreshManager{store: store, ttl: ttl}, nil
}

// IssueResult is what the manager returns when minting a refresh token.
// Plaintext is shown to the client exactly once; the row is stored hashed.
type IssueResult struct {
	Plaintext string
	Row       RefreshTokenRow
}

// Issue creates a new refresh token rooted (no parent). Used at login.
func (m *RefreshManager) Issue(ctx context.Context, userID uuid.UUID, ip net.IP, userAgent string) (IssueResult, error) {
	return m.issue(ctx, userID, nil, ip, userAgent)
}

// Rotate validates the presented plaintext token, revokes it as 'rotated',
// and issues a fresh token chained off the same parent. Reuse detection:
// if the presented token is already revoked (replaced or otherwise), the
// entire rotation chain is revoked and ErrTokenReused is returned.
func (m *RefreshManager) Rotate(ctx context.Context, plaintext string, ip net.IP, userAgent string) (IssueResult, uuid.UUID, error) {
	hash := hashToken(plaintext)

	row, err := m.store.GetByHash(ctx, hash)
	if err != nil {
		return IssueResult{}, uuid.Nil, ErrTokenInvalid
	}

	// constant-time hash comparison (already-known-equal lookup, but defense in depth)
	if subtle.ConstantTimeCompare(row.TokenHash, hash) != 1 {
		return IssueResult{}, uuid.Nil, ErrTokenInvalid
	}

	now := time.Now().UTC()
	if row.ExpiresAt.Before(now) {
		return IssueResult{}, uuid.Nil, ErrTokenExpired
	}

	// Reuse detection: a token presented after rotation indicates either a
	// replay attack or a stolen token. Burn the entire chain.
	if row.RevokedAt.Valid {
		_ = m.store.RevokeChain(ctx, row.ID, "reuse_detected")
		return IssueResult{}, row.UserID, ErrTokenReused
	}

	// Issue new token chained to the *same* parent as the current one. Then
	// mark the current one as replaced. The parent_id semantics give us a
	// linear chain rooted at the original login.
	parent := row.ID
	res, err := m.issue(ctx, row.UserID, &parent, ip, userAgent)
	if err != nil {
		return IssueResult{}, row.UserID, err
	}
	if err := m.store.MarkReplaced(ctx, row.ID, res.Row.ID); err != nil {
		// We already issued the new token — best-effort revoke the old.
		_ = m.store.Revoke(ctx, row.ID, "rotation_partial")
		return IssueResult{}, row.UserID, fmt.Errorf("auth: mark replaced: %w", err)
	}
	return res, row.UserID, nil
}

// Revoke marks a single refresh token as revoked. Use at logout.
func (m *RefreshManager) Revoke(ctx context.Context, plaintext, reason string) error {
	hash := hashToken(plaintext)
	row, err := m.store.GetByHash(ctx, hash)
	if err != nil {
		return ErrTokenInvalid
	}
	return m.store.Revoke(ctx, row.ID, reason)
}

// RevokeAllForUser is the logout-all action. Pair with BumpUserTokenVersion
// to also invalidate access tokens already in the wild.
func (m *RefreshManager) RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	return m.store.RevokeAllForUser(ctx, userID, reason)
}

func (m *RefreshManager) issue(ctx context.Context, userID uuid.UUID, parent *uuid.UUID, ip net.IP, ua string) (IssueResult, error) {
	plaintext, err := generateRefreshTokenPlaintext()
	if err != nil {
		return IssueResult{}, err
	}
	row, err := m.store.Create(ctx, CreateRefreshTokenInput{
		UserID:    userID,
		TokenHash: hashToken(plaintext),
		ExpiresAt: time.Now().UTC().Add(m.ttl),
		ParentID:  parent,
		IP:        ip,
		UserAgent: ua,
	})
	if err != nil {
		return IssueResult{}, fmt.Errorf("auth: persist refresh token: %w", err)
	}
	return IssueResult{Plaintext: plaintext, Row: row}, nil
}

func generateRefreshTokenPlaintext() (string, error) {
	buf := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("auth: rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func hashToken(plaintext string) []byte {
	h := sha256.Sum256([]byte(plaintext))
	return h[:]
}

// Sanity helper for tests — exposes an error to mark "no row found" so
// in-memory stores can return it consistently.
var ErrNoRow = errors.New("auth: refresh token not found")
