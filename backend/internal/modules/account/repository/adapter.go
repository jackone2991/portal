package accountrepo

// Adapter bridges the sqlc-generated Queries to the domain interfaces the
// account module declares:
//
//   - middleware.AuthSnapshotFetcher   (GetUserAuthSnapshot)
//   - rbac.PermissionFetcher           (GetEffectivePermissions)
//   - auth.RefreshStore                (Create/GetByHash/MarkReplaced/Revoke/…)
//   - handler.UserStore                (GetUserByEmail/CreateLocalUser/AssignRoleByCode/Bump/ListUserRoleCodes)
//   - audit.EventStore                 (WriteAuditEvent)
//   - account.APIUserFetcher           (GetUserSummaryByID)
//
// One value satisfies them all; cmd/api constructs it once with NewAdapter and
// passes it into every account.Deps slot. All type juggling (pgtype ↔ domain)
// lives here so the module code stays sqlc-agnostic.

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"net/netip"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	accountapi "github.com/portal/backend/internal/modules/account/api"
	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/handler"
	"github.com/portal/backend/internal/modules/account/middleware"
	"github.com/portal/backend/internal/platform/audit"
)

// Adapter wraps *Queries. Construct with NewAdapter.
type Adapter struct {
	q *Queries
}

// NewAdapter builds the adapter over any pgx-compatible handle (pool or tx).
func NewAdapter(db DBTX) *Adapter { return &Adapter{q: New(db)} }

// ── middleware.AuthSnapshotFetcher / handler.UserUpserter ───────────

func (a *Adapter) GetUserAuthSnapshot(ctx context.Context, id uuid.UUID) (middleware.UserAuthSnapshot, error) {
	row, err := a.q.GetUserAuthSnapshot(ctx, pgUUID(id))
	if err != nil {
		return middleware.UserAuthSnapshot{}, err
	}
	return middleware.UserAuthSnapshot{
		ID:           uuidFrom(row.ID),
		Email:        row.Email,
		DisplayName:  row.DisplayName,
		TokenVersion: int(row.TokenVersion),
		Disabled:     row.DisabledAt.Valid,
	}, nil
}

// ── rbac.PermissionFetcher ──────────────────────────────────────────

func (a *Adapter) GetEffectivePermissions(ctx context.Context, userID uuid.UUID) ([]string, error) {
	return a.q.GetEffectivePermissions(ctx, pgUUID(userID))
}

// ── handler.UserStore (local password auth, ADR-06) ─────────────────

func (a *Adapter) GetUserByEmail(ctx context.Context, email string) (handler.LocalUser, error) {
	u, err := a.q.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return handler.LocalUser{}, handler.ErrUserNotFound
		}
		return handler.LocalUser{}, err
	}
	return toLocalUser(u), nil
}

func (a *Adapter) CreateLocalUser(ctx context.Context, in handler.CreateLocalUserInput) (handler.LocalUser, error) {
	u, err := a.q.CreateLocalUser(ctx, CreateLocalUserParams{
		Email:        in.Email,
		DisplayName:  in.DisplayName,
		PasswordHash: strPtrOrNil(in.PasswordHash),
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation (email)
			return handler.LocalUser{}, handler.ErrEmailTaken
		}
		return handler.LocalUser{}, err
	}
	return toLocalUser(u), nil
}

// AssignRoleByCode grants a role by its code (e.g. "user") to a user. Used at
// registration to seed the default role. granted_by is NULL (system grant).
func (a *Adapter) AssignRoleByCode(ctx context.Context, userID uuid.UUID, roleCode string) error {
	role, err := a.q.GetRoleByCode(ctx, roleCode)
	if err != nil {
		return err
	}
	return a.q.AssignRoleToUser(ctx, AssignRoleToUserParams{
		UserID: pgUUID(userID),
		RoleID: role.ID,
	})
}

func toLocalUser(u User) handler.LocalUser {
	return handler.LocalUser{
		ID:           uuidFrom(u.ID),
		Email:        u.Email,
		DisplayName:  u.DisplayName,
		PasswordHash: derefStr(u.PasswordHash),
		TokenVersion: int(u.TokenVersion),
		Disabled:     u.DisabledAt.Valid,
	}
}

func (a *Adapter) BumpUserTokenVersion(ctx context.Context, id uuid.UUID) (int, error) {
	tv, err := a.q.BumpUserTokenVersion(ctx, pgUUID(id))
	return int(tv), err
}

func (a *Adapter) ListUserRoleCodes(ctx context.Context, id uuid.UUID) ([]string, error) {
	roles, err := a.q.ListUserRoles(ctx, pgUUID(id))
	if err != nil {
		return nil, err
	}
	codes := make([]string, 0, len(roles))
	for _, r := range roles {
		codes = append(codes, r.Code)
	}
	return codes, nil
}

// ── auth.RefreshStore ───────────────────────────────────────────────

func (a *Adapter) Create(ctx context.Context, in auth.CreateRefreshTokenInput) (auth.RefreshTokenRow, error) {
	row, err := a.q.CreateRefreshToken(ctx, CreateRefreshTokenParams{
		UserID:          pgUUID(in.UserID),
		TokenHash:       in.TokenHash,
		ExpiresAt:       pgTime(in.ExpiresAt),
		ParentID:        pgUUIDPtr(in.ParentID),
		IssuedIp:        ipToAddr(in.IP),
		IssuedUserAgent: strPtrOrNil(in.UserAgent),
	})
	if err != nil {
		return auth.RefreshTokenRow{}, err
	}
	return toRefreshRow(row), nil
}

func (a *Adapter) GetByHash(ctx context.Context, hash []byte) (auth.RefreshTokenRow, error) {
	row, err := a.q.GetRefreshTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.RefreshTokenRow{}, auth.ErrNoRow
		}
		return auth.RefreshTokenRow{}, err
	}
	return toRefreshRow(row), nil
}

func (a *Adapter) MarkReplaced(ctx context.Context, id, replacedBy uuid.UUID) error {
	return a.q.MarkRefreshTokenReplaced(ctx, MarkRefreshTokenReplacedParams{
		ID:           pgUUID(id),
		ReplacedByID: pgUUID(replacedBy),
	})
}

func (a *Adapter) Revoke(ctx context.Context, id uuid.UUID, reason string) error {
	return a.q.RevokeRefreshToken(ctx, RevokeRefreshTokenParams{ID: pgUUID(id), RevokeReason: &reason})
}

func (a *Adapter) RevokeChain(ctx context.Context, anyTokenID uuid.UUID, reason string) error {
	return a.q.RevokeRefreshTokenChain(ctx, RevokeRefreshTokenChainParams{ID: pgUUID(anyTokenID), RevokeReason: &reason})
}

func (a *Adapter) RevokeAllForUser(ctx context.Context, userID uuid.UUID, reason string) error {
	return a.q.RevokeAllRefreshTokensForUser(ctx, RevokeAllRefreshTokensForUserParams{UserID: pgUUID(userID), RevokeReason: &reason})
}

// PurgeExpiredRefreshTokens deletes rows whose expiry has passed. Called by the
// shared periodic scheduler (cmd/worker), not on the request path.
func (a *Adapter) PurgeExpiredRefreshTokens(ctx context.Context) error {
	return a.q.PurgeExpiredRefreshTokens(ctx)
}

// ── auth.ResetStore (password-reset tokens, SPEC-04 P0.3) ───────────

func (a *Adapter) CreatePasswordResetToken(ctx context.Context, in auth.CreatePasswordResetTokenInput) (auth.PasswordResetTokenRow, error) {
	row, err := a.q.CreatePasswordResetToken(ctx, CreatePasswordResetTokenParams{
		UserID:    pgUUID(in.UserID),
		TokenHash: in.TokenHash,
		ExpiresAt: pgTime(in.ExpiresAt),
	})
	if err != nil {
		return auth.PasswordResetTokenRow{}, err
	}
	return toResetRow(row), nil
}

func (a *Adapter) GetPasswordResetTokenByHash(ctx context.Context, hash []byte) (auth.PasswordResetTokenRow, error) {
	row, err := a.q.GetPasswordResetTokenByHash(ctx, hash)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.PasswordResetTokenRow{}, auth.ErrNoRow
		}
		return auth.PasswordResetTokenRow{}, err
	}
	return toResetRow(row), nil
}

func (a *Adapter) MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error {
	return a.q.MarkPasswordResetTokenUsed(ctx, pgUUID(id))
}

// PurgeExpiredPasswordResetTokens hard-deletes tokens well past expiry (periodic).
func (a *Adapter) PurgeExpiredPasswordResetTokens(ctx context.Context) error {
	return a.q.PurgeExpiredPasswordResetTokens(ctx)
}

// SetPassword replaces a user's Argon2id hash (password reset, P0.3). Pair with
// BumpUserTokenVersion to revoke every existing session.
func (a *Adapter) SetPassword(ctx context.Context, userID uuid.UUID, hash string) error {
	return a.q.SetUserPassword(ctx, SetUserPasswordParams{ID: pgUUID(userID), PasswordHash: strPtrOrNil(hash)})
}

func toResetRow(r PasswordResetToken) auth.PasswordResetTokenRow {
	return auth.PasswordResetTokenRow{
		ID:        uuidFrom(r.ID),
		UserID:    uuidFrom(r.UserID),
		TokenHash: r.TokenHash,
		ExpiresAt: r.ExpiresAt.Time,
		UsedAt:    sql.NullTime{Time: r.UsedAt.Time, Valid: r.UsedAt.Valid},
		CreatedAt: r.CreatedAt.Time,
	}
}

// ── audit.EventStore ────────────────────────────────────────────────

func (a *Adapter) WriteAuditEvent(ctx context.Context, in audit.WriteEventInput) error {
	return a.q.WriteAuditEvent(ctx, WriteAuditEventParams{
		ActorID:    pgUUIDPtr(in.ActorID),
		ActorKind:  in.ActorKind,
		Action:     in.Action,
		TargetKind: in.TargetKind,
		TargetID:   in.TargetID,
		Metadata:   in.Metadata,
		Ip:         ipToAddr(in.IP),
		UserAgent:  strPtrOrNil(in.UserAgent),
	})
}

// ── account.APIUserFetcher (cross-module projection) ────────────────

func (a *Adapter) GetUserSummaryByID(ctx context.Context, id uuid.UUID) (*accountapi.UserSummary, error) {
	u, err := a.q.GetUserByID(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // absent → (nil, nil) per API contract
		}
		return nil, err
	}
	if u.DisabledAt.Valid {
		return nil, nil // disabled users are invisible to other modules
	}
	return &accountapi.UserSummary{
		ID:          uuidFrom(u.ID),
		Email:       u.Email,
		DisplayName: u.DisplayName,
	}, nil
}

// ── helpers ─────────────────────────────────────────────────────────

func toRefreshRow(r RefreshToken) auth.RefreshTokenRow {
	return auth.RefreshTokenRow{
		ID:           uuidFrom(r.ID),
		UserID:       uuidFrom(r.UserID),
		TokenHash:    r.TokenHash,
		ExpiresAt:    r.ExpiresAt.Time,
		CreatedAt:    r.CreatedAt.Time,
		RevokedAt:    sql.NullTime{Time: r.RevokedAt.Time, Valid: r.RevokedAt.Valid},
		RevokeReason: nullString(r.RevokeReason),
		ReplacedByID: uuidPtrFrom(r.ReplacedByID),
		ParentID:     uuidPtrFrom(r.ParentID),
	}
}

func pgUUID(id uuid.UUID) pgtype.UUID { return pgtype.UUID{Bytes: id, Valid: true} }

func pgUUIDPtr(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func uuidFrom(p pgtype.UUID) uuid.UUID {
	if !p.Valid {
		return uuid.Nil
	}
	return uuid.UUID(p.Bytes)
}

func uuidPtrFrom(p pgtype.UUID) *uuid.UUID {
	if !p.Valid {
		return nil
	}
	u := uuid.UUID(p.Bytes)
	return &u
}

func pgTime(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }

func nullString(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func ipToAddr(ip net.IP) *netip.Addr {
	if len(ip) == 0 {
		return nil
	}
	a, ok := netip.AddrFromSlice(ip)
	if !ok {
		return nil
	}
	a = a.Unmap()
	return &a
}
