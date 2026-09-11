package accountrepo

// Adapter methods for handler.AdminStore — the admin console's reads and
// writes. Split into its own file because the console is a distinct surface
// from the auth path in adapter.go, not because it is a distinct type: it is
// the same *Adapter, so cmd/api still constructs exactly one.

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/account/handler"
)

var _ handler.AdminStore = (*Adapter)(nil)

// ── users ──────────────────────────────────────────────────────────────────

func (a *Adapter) ListUsers(ctx context.Context, f handler.ListUsersFilter) ([]handler.AdminUser, int64, error) {
	// The query treats an empty `q` as "no search" and a NULL status as "any
	// state", so the zero-value filter is the unfiltered directory.
	status := narg(f.Status)

	rows, err := a.q.ListUsersAdmin(ctx, ListUsersAdminParams{
		Status: status,
		Q:      f.Query,
		Lim:    int32(f.Limit),
		Off:    int32(f.Offset),
	})
	if err != nil {
		return nil, 0, err
	}
	total, err := a.q.CountUsersAdmin(ctx, CountUsersAdminParams{Status: status, Q: f.Query})
	if err != nil {
		return nil, 0, err
	}

	out := make([]handler.AdminUser, 0, len(rows))
	for _, r := range rows {
		out = append(out, handler.AdminUser{
			ID:             uuidFrom(r.ID),
			Email:          r.Email,
			DisplayName:    r.DisplayName,
			AvatarURL:      derefStr(r.AvatarUrl),
			ApprovalStatus: r.ApprovalStatus,
			ApprovalNote:   derefStr(r.ApprovalNote),
			ApprovedAt:     timePtr(r.ApprovedAt),
			ApprovedBy:     uuidPtrFrom(r.ApprovedBy),
			Disabled:       r.DisabledAt.Valid,
			HasPassword:    r.HasPassword,
			CreatedAt:      r.CreatedAt.Time,
			RoleCodes:      r.RoleCodes,
		})
	}
	return out, total, nil
}

func (a *Adapter) CountUsersByStatus(ctx context.Context) (map[string]int64, error) {
	rows, err := a.q.CountUsersByApprovalStatus(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, r := range rows {
		out[r.ApprovalStatus] = r.Total
	}
	return out, nil
}

func (a *Adapter) GetAdminUser(ctx context.Context, id uuid.UUID) (handler.AdminUser, error) {
	r, err := a.q.GetUserAdmin(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return handler.AdminUser{}, handler.ErrUserNotFound
		}
		return handler.AdminUser{}, err
	}
	return handler.AdminUser{
		ID:             uuidFrom(r.ID),
		Email:          r.Email,
		DisplayName:    r.DisplayName,
		AvatarURL:      derefStr(r.AvatarUrl),
		ApprovalStatus: r.ApprovalStatus,
		ApprovalNote:   derefStr(r.ApprovalNote),
		ApprovedAt:     timePtr(r.ApprovedAt),
		ApprovedBy:     uuidPtrFrom(r.ApprovedBy),
		Disabled:       r.DisabledAt.Valid,
		HasPassword:    r.HasPassword,
		CreatedAt:      r.CreatedAt.Time,
		RoleCodes:      r.RoleCodes,
	}, nil
}

func (a *Adapter) SetApproval(ctx context.Context, in handler.SetApprovalInput) (handler.AdminUser, error) {
	if _, err := a.q.SetUserApproval(ctx, SetUserApprovalParams{
		ID:         pgUUID(in.UserID),
		Status:     in.Status,
		Note:       strPtrOrNil(in.Note),
		ApprovedBy: pgUUIDPtr(in.ApprovedBy),
	}); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return handler.AdminUser{}, handler.ErrUserNotFound
		}
		return handler.AdminUser{}, err
	}
	// Re-read rather than mapping the RETURNING row: the caller renders the same
	// shape as the directory, roles included, and SetUserApproval does not join
	// user_roles.
	return a.GetAdminUser(ctx, in.UserID)
}

func (a *Adapter) SetUserDisabled(ctx context.Context, id uuid.UUID, disabled bool) error {
	if disabled {
		return a.q.DisableUser(ctx, pgUUID(id))
	}
	return a.q.EnableUser(ctx, pgUUID(id))
}

func (a *Adapter) ReplaceUserRoles(ctx context.Context, userID uuid.UUID, codes []string, grantedBy *uuid.UUID) error {
	return a.q.ReplaceUserRoles(ctx, ReplaceUserRolesParams{
		UserID:    pgUUID(userID),
		Codes:     codes,
		GrantedBy: pgUUIDPtr(grantedBy),
	})
}

// ListPermissionHolders returns (user, effective permission code) pairs for
// every enabled + approved account, narrowed to codes whose resource segment is
// `resource` or `*`. The grammar match happens above this layer — see
// handler.approverIDs.
func (a *Adapter) ListPermissionHolders(ctx context.Context, resource string) ([]handler.PermissionHolder, error) {
	rows, err := a.q.ListPermissionHoldersByResource(ctx, resource)
	if err != nil {
		return nil, err
	}
	out := make([]handler.PermissionHolder, 0, len(rows))
	for _, r := range rows {
		out = append(out, handler.PermissionHolder{UserID: uuidFrom(r.UserID), Code: r.Code})
	}
	return out, nil
}

// ── create / edit / delete ─────────────────────────────────────────────────

func (a *Adapter) CreateUser(ctx context.Context, in handler.CreateUserInput) (uuid.UUID, error) {
	id, err := a.q.CreateAdminUser(ctx, CreateAdminUserParams{
		Email:          in.Email,
		DisplayName:    in.DisplayName,
		PasswordHash:   strPtrOrNil(in.PasswordHash),
		ApprovalStatus: in.Status,
		ApprovedBy:     pgUUIDPtr(in.ApprovedBy),
	})
	if err != nil {
		if isUniqueViolation(err) { // users.email is UNIQUE
			return uuid.Nil, handler.ErrEmailTaken
		}
		return uuid.Nil, err
	}
	return uuidFrom(id), nil
}

func (a *Adapter) UpdateUser(ctx context.Context, id uuid.UUID, in handler.UpdateUserInput) error {
	_, err := a.q.UpdateAdminUser(ctx, UpdateAdminUserParams{
		ID:          pgUUID(id),
		Email:       in.Email,
		DisplayName: in.DisplayName,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return handler.ErrEmailTaken
		}
		if errors.Is(err, pgx.ErrNoRows) {
			return handler.ErrUserNotFound
		}
		return err
	}
	return nil
}

func (a *Adapter) RevokeAllSessions(ctx context.Context, id uuid.UUID, reason string) error {
	return a.q.RevokeAllRefreshTokensForUser(ctx, RevokeAllRefreshTokensForUserParams{
		UserID:       pgUUID(id),
		RevokeReason: strPtrOrNil(reason),
	})
}

// DeleteUser reports ErrUserNotFound when nothing matched, so a double-submit
// reads as 404 rather than a silent success.
func (a *Adapter) DeleteUser(ctx context.Context, id uuid.UUID) error {
	rows, err := a.q.DeleteUser(ctx, pgUUID(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return handler.ErrUserNotFound
	}
	return nil
}

// isUniqueViolation identifies SQLSTATE 23505, which for these two writes only
// ever comes from the email index.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// ── roles + permissions ────────────────────────────────────────────────────

func (a *Adapter) ListAdminRoles(ctx context.Context) ([]handler.AdminRole, error) {
	rows, err := a.q.ListRolesAdmin(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]handler.AdminRole, 0, len(rows))
	for _, r := range rows {
		out = append(out, handler.AdminRole{
			ID:          uuidFrom(r.ID),
			Code:        r.Code,
			Name:        r.Name,
			Description: derefStr(r.Description),
			ParentCode:  derefStr(r.ParentCode),
			IsSystem:    r.IsSystem,
			UserCount:   r.UserCount,
		})
	}
	return out, nil
}

func (a *Adapter) ListPermissions(ctx context.Context) ([]handler.AdminPermission, error) {
	rows, err := a.q.ListPermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]handler.AdminPermission, 0, len(rows))
	for _, p := range rows {
		out = append(out, handler.AdminPermission{Code: p.Code, Description: derefStr(p.Description)})
	}
	return out, nil
}

func (a *Adapter) ListRoleGrants(ctx context.Context) (map[string][]string, error) {
	rows, err := a.q.ListAllRolePermissions(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string][]string{}
	for _, r := range rows {
		out[r.RoleCode] = append(out[r.RoleCode], r.PermissionCode)
	}
	return out, nil
}

func (a *Adapter) ReplaceRolePermissions(ctx context.Context, roleID uuid.UUID, codes []string, grantedBy *uuid.UUID) error {
	return a.q.ReplaceRolePermissions(ctx, ReplaceRolePermissionsParams{
		RoleID:    pgUUID(roleID),
		Codes:     codes,
		GrantedBy: pgUUIDPtr(grantedBy),
	})
}

func (a *Adapter) CreateRole(ctx context.Context, in handler.SaveRoleInput) (handler.AdminRole, error) {
	parent, err := a.roleIDByCode(ctx, in.ParentCode)
	if err != nil {
		return handler.AdminRole{}, err
	}
	r, err := a.q.CreateRole(ctx, CreateRoleParams{
		Code:        in.Code,
		Name:        in.Name,
		Description: strPtrOrNil(in.Description),
		ParentID:    parent,
	})
	if err != nil {
		return handler.AdminRole{}, err
	}
	return handler.AdminRole{
		ID:          uuidFrom(r.ID),
		Code:        r.Code,
		Name:        r.Name,
		Description: derefStr(r.Description),
		ParentCode:  in.ParentCode,
		IsSystem:    r.IsSystem,
	}, nil
}

func (a *Adapter) UpdateRole(ctx context.Context, id uuid.UUID, in handler.SaveRoleInput) (handler.AdminRole, error) {
	parent, err := a.roleIDByCode(ctx, in.ParentCode)
	if err != nil {
		return handler.AdminRole{}, err
	}
	// UpdateRole's WHERE carries `AND is_system = false`, so a system role
	// produces no rows. The handler already refuses those, which makes this the
	// backstop rather than the check.
	r, err := a.q.UpdateRole(ctx, UpdateRoleParams{
		ID:          pgUUID(id),
		Name:        in.Name,
		Description: strPtrOrNil(in.Description),
		ParentID:    parent,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return handler.AdminRole{}, handler.ErrRoleNotFound
		}
		return handler.AdminRole{}, err
	}
	return handler.AdminRole{
		ID:          uuidFrom(r.ID),
		Code:        r.Code,
		Name:        r.Name,
		Description: derefStr(r.Description),
		ParentCode:  in.ParentCode,
		IsSystem:    r.IsSystem,
	}, nil
}

func (a *Adapter) DeleteRole(ctx context.Context, id uuid.UUID) error {
	return a.q.DeleteRole(ctx, pgUUID(id))
}

func (a *Adapter) BumpTokenVersionForRole(ctx context.Context, roleID uuid.UUID) error {
	ids, err := a.q.ListUserIDsAffectedByRole(ctx, pgUUID(roleID))
	if err != nil {
		return err
	}
	if len(ids) == 0 {
		return nil
	}
	return a.q.BumpTokenVersionForUsers(ctx, ids)
}

// roleIDByCode resolves a parent code to an id. An empty code means "no
// parent" — a root role — and yields a NULL, not an error.
func (a *Adapter) roleIDByCode(ctx context.Context, code string) (pgtype.UUID, error) {
	if code == "" {
		return pgtype.UUID{}, nil
	}
	r, err := a.q.GetRoleByCode(ctx, code)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return pgtype.UUID{}, handler.ErrRoleNotFound
		}
		return pgtype.UUID{}, err
	}
	return r.ID, nil
}

// ── small conversions ──────────────────────────────────────────────────────

// narg maps "" to a NULL filter, which is how the admin queries spell "any".
func narg(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func timePtr(t pgtype.Timestamptz) *time.Time {
	if !t.Valid {
		return nil
	}
	v := t.Time
	return &v
}
