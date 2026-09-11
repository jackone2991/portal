package handler

// The admin console: the user directory, the registration approval queue, and
// the role/permission matrix. Everything here lives behind RequireAuth plus a
// per-route permission (see account/module.go for the route→permission map).
//
// ── Why this file is careful about the ACTOR ────────────────────────────────
//
// A permission matrix that anyone with `rbac:role:write` can edit is not an
// access-control system, it is a self-service escalation form: an admin could
// grant `*` to the `user` role, or hand themselves `users:approve`, and the
// approval gate this module exists to enforce would evaporate. Three rules
// close that, and every mutation below routes through them:
//
//  1. NO ESCALATION — you cannot grant a permission you do not hold yourself,
//     and you cannot assign a role whose effective permissions exceed your own.
//     A superadmin holds `*`, so this constrains nobody who is already
//     unrestricted, and constrains everybody who is not.
//  2. NO SELF-EDIT — you cannot change your own roles, approval or enabled
//     state. Otherwise the last superadmin can demote themselves and leave the
//     install with no one able to administer it.
//  3. NO SILENT DE-ESCALATION — every change that alters somebody's effective
//     permissions bumps their token_version, which is simultaneously the RBAC
//     cache key (rbac:perms:<user>:v<N>). Without it a revoke would sit behind
//     the cache TTL, which is exactly the window an attacker wants.

import (
	"context"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/rbac"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/server"
)

// Approval states. Mirrors the users_approval_status_check constraint in
// migration 0031 — the DB is the authority, this is the Go-side vocabulary.
const (
	ApprovalPending  = "pending"
	ApprovalApproved = "approved"
	ApprovalRejected = "rejected"
)

// SuperadminRole is the one role that only a wildcard holder may hand out, and
// the role the founding account takes on a first-run install.
const SuperadminRole = "superadmin"

// PermApproveUser gates every approval decision. It is granted to no role by
// migration 0031 — `superadmin` reaches it through the `*` wildcard — and it is
// also the permission the registration notification fans out to.
//
// approvalResource is its resource segment, which is all the SQL prefilter in
// ListPermissionHolders needs.
const (
	PermApproveUser  = "users:approve"
	approvalResource = "users"
)

// maxApprovalNote caps the reviewer's reason. It is shown back to the rejected
// user verbatim, so it is a message, not a document.
const maxApprovalNote = 500

// ── store ──────────────────────────────────────────────────────────────────

// AdminUser is the directory projection: enough to render a row and decide
// what to do about it, with no credential material.
type AdminUser struct {
	ID             uuid.UUID
	Email          string
	DisplayName    string
	AvatarURL      string
	ApprovalStatus string
	ApprovalNote   string
	ApprovedAt     *time.Time
	ApprovedBy     *uuid.UUID
	Disabled       bool
	HasPassword    bool
	CreatedAt      time.Time
	RoleCodes      []string
}

// AdminRole carries the parent's CODE rather than only its id, because every
// consumer (matrix, hierarchy tree, escalation check) works in codes.
type AdminRole struct {
	ID          uuid.UUID
	Code        string
	Name        string
	Description string
	ParentCode  string
	IsSystem    bool
	UserCount   int64
}

// AdminPermission is one row of the permission catalog.
type AdminPermission struct {
	Code        string
	Description string
}

type ListUsersFilter struct {
	Status string // "" = every state
	Query  string // matches email or display name, case-insensitive
	Limit  int
	Offset int
}

type SetApprovalInput struct {
	UserID     uuid.UUID
	Status     string
	Note       string
	ApprovedBy *uuid.UUID
}

// CreateUserInput is an admin-provisioned account. Roles are NOT set here — the
// caller assigns them afterwards through SetUserRoles, so the escalation guard
// lives in exactly one place instead of two.
type CreateUserInput struct {
	Email        string
	DisplayName  string
	PasswordHash string
	// Status is decided by the handler from the creator's own authority, not by
	// the request body. See CreateUser.
	Status     string
	ApprovedBy *uuid.UUID
}

type UpdateUserInput struct {
	Email       string
	DisplayName string
}

type SaveRoleInput struct {
	Code        string
	Name        string
	Description string
	ParentCode  string
}

// AdminStore is the slice of the repository this console needs. Deliberately
// one interface: these operations are only ever used together, and splitting
// them would only spread the same adapter across more names.
type AdminStore interface {
	ListUsers(ctx context.Context, f ListUsersFilter) ([]AdminUser, int64, error)
	CountUsersByStatus(ctx context.Context) (map[string]int64, error)
	GetAdminUser(ctx context.Context, id uuid.UUID) (AdminUser, error)
	SetApproval(ctx context.Context, in SetApprovalInput) (AdminUser, error)
	SetUserDisabled(ctx context.Context, id uuid.UUID, disabled bool) error
	ReplaceUserRoles(ctx context.Context, userID uuid.UUID, codes []string, grantedBy *uuid.UUID) error

	CreateUser(ctx context.Context, in CreateUserInput) (uuid.UUID, error)
	UpdateUser(ctx context.Context, id uuid.UUID, in UpdateUserInput) error
	SetPassword(ctx context.Context, id uuid.UUID, hash string) error
	// RevokeAllSessions burns the refresh chain. Pairs with a token_version bump:
	// the bump kills access tokens, this stops the browser silently minting new
	// ones off the refresh cookie.
	RevokeAllSessions(ctx context.Context, id uuid.UUID, reason string) error
	// DeleteUser is a HARD delete that cascades across every content table.
	DeleteUser(ctx context.Context, id uuid.UUID) error
	// ListPermissionHolders backs the last-approver guard; same method the
	// registration fan-out uses.
	ListPermissionHolders(ctx context.Context, resource string) ([]PermissionHolder, error)

	ListAdminRoles(ctx context.Context) ([]AdminRole, error)
	ListPermissions(ctx context.Context) ([]AdminPermission, error)
	// ListRoleGrants returns role code → directly granted permission codes.
	// Inherited permissions are NOT folded in; the hierarchy walk happens above
	// this layer so the matrix can show "direct" and "inherited" differently.
	ListRoleGrants(ctx context.Context) (map[string][]string, error)
	ReplaceRolePermissions(ctx context.Context, roleID uuid.UUID, codes []string, grantedBy *uuid.UUID) error

	CreateRole(ctx context.Context, in SaveRoleInput) (AdminRole, error)
	UpdateRole(ctx context.Context, id uuid.UUID, in SaveRoleInput) (AdminRole, error)
	DeleteRole(ctx context.Context, id uuid.UUID) error

	// BumpTokenVersionForRole re-keys the permission cache for everyone holding
	// the role OR any role that inherits from it.
	BumpTokenVersionForRole(ctx context.Context, roleID uuid.UUID) error
	BumpUserTokenVersion(ctx context.Context, id uuid.UUID) (int, error)
}

// ErrRoleNotFound / ErrRoleProtected are the two role-mutation refusals the
// store may report. Anything else is a genuine failure.
var (
	ErrRoleNotFound  = errors.New("account: role not found")
	ErrRoleProtected = errors.New("account: role is a system role")
)

// AdminHandler serves the console. Engine is required — the escalation guard is
// not optional, so a nil engine is a wiring bug, not a degraded mode.
type AdminHandler struct {
	Store  AdminStore
	Engine *rbac.Engine
	Audit  *audit.Logger
}

// ── users ──────────────────────────────────────────────────────────────────

// GET /admin/users?status=&q=&limit=&offset=
func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	if status != "" && !validApprovalStatus(status) {
		badRequest(w, "unknown status filter")
		return
	}
	limit := server.Limit(r, 25, 100)
	offset := server.AtoiSafe(r.URL.Query().Get("offset"))
	if offset < 0 {
		offset = 0
	}

	users, total, err := h.Store.ListUsers(r.Context(), ListUsersFilter{
		Status: status,
		Query:  strings.TrimSpace(r.URL.Query().Get("q")),
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		server.Internal(w)
		return
	}

	counts, err := h.Store.CountUsersByStatus(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}

	out := make([]map[string]any, 0, len(users))
	for _, u := range users {
		out = append(out, adminUserJSON(u))
	}
	server.JSON(w, http.StatusOK, map[string]any{
		"users":  out,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"counts": map[string]any{
			ApprovalPending:  counts[ApprovalPending],
			ApprovalApproved: counts[ApprovalApproved],
			ApprovalRejected: counts[ApprovalRejected],
		},
	})
}

// GET /admin/users/{id}
func (h *AdminHandler) GetUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	u, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}
	server.JSON(w, http.StatusOK, adminUserJSON(u))
}

// POST /admin/users/{id}/approve
func (h *AdminHandler) Approve(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, ApprovalApproved)
}

// POST /admin/users/{id}/reject
func (h *AdminHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, ApprovalRejected)
}

// POST /admin/users/{id}/revoke — back to pending. The account keeps existing
// and keeps its password; it simply cannot sign in until re-approved. Useful
// when an approval was a mistake but a refusal would be too strong.
func (h *AdminHandler) RevokeApproval(w http.ResponseWriter, r *http.Request) {
	h.decide(w, r, ApprovalPending)
}

func (h *AdminHandler) decide(w http.ResponseWriter, r *http.Request, status string) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if id == actor.UserID {
		problem(w, http.StatusForbidden, "self-target", "You cannot change your own approval state.")
		return
	}

	var body struct {
		Note string `json:"note"`
	}
	// An empty body is fine — the note is optional on every decision.
	if r.ContentLength > 0 && !server.Decode(w, r, &body) {
		return
	}
	note := strings.TrimSpace(body.Note)
	if len(note) > maxApprovalNote {
		badRequest(w, "note is too long")
		return
	}

	// Taking approval away from the last person who can approve leaves the
	// install accepting signups nobody can ever let in.
	if status != ApprovalApproved {
		stranded, err := h.wouldStrandApprovals(r.Context(), id)
		if err != nil {
			server.Internal(w)
			return
		}
		if stranded {
			problem(w, http.StatusConflict, "last-approver",
				"This is the last account that can approve registrations. Give another account "+
					"that permission first.")
			return
		}
	}

	approver := actor.UserID
	in := SetApprovalInput{UserID: id, Status: status, Note: note, ApprovedBy: &approver}
	if status == ApprovalPending {
		in.ApprovedBy = nil
	}

	u, err := h.Store.SetApproval(r.Context(), in)
	if err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}

	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ApprovalAction(status),
		ActorID:    &actor.UserID,
		TargetKind: "user",
		TargetID:   id.String(),
		Metadata:   map[string]any{"status": status, "note": note, "email": u.Email},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	server.JSON(w, http.StatusOK, adminUserJSON(u))
}

// POST /admin/users/{id}/disable | /enable
func (h *AdminHandler) SetDisabled(disabled bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actor, ok := requireActor(w, r)
		if !ok {
			return
		}
		id, ok := pathUUID(w, r)
		if !ok {
			return
		}
		if id == actor.UserID {
			problem(w, http.StatusForbidden, "self-target", "You cannot disable your own account.")
			return
		}
		// A disabled account is excluded from the approver set, so switching off
		// the last one strands the queue exactly as deleting it would.
		if disabled {
			stranded, err := h.wouldStrandApprovals(r.Context(), id)
			if err != nil {
				server.Internal(w)
				return
			}
			if stranded {
				problem(w, http.StatusConflict, "last-approver",
					"This is the last account that can approve registrations. Give another account "+
						"that permission first.")
				return
			}
		}
		if err := h.Store.SetUserDisabled(r.Context(), id, disabled); err != nil {
			notFoundOrInternal(w, err, "user not found")
			return
		}
		u, err := h.Store.GetAdminUser(r.Context(), id)
		if err != nil {
			notFoundOrInternal(w, err, "user not found")
			return
		}
		action := audit.ActionUserEnabled
		if disabled {
			action = audit.ActionUserDisabled
		}
		h.Audit.Write(r.Context(), audit.Event{
			Action:     action,
			ActorID:    &actor.UserID,
			TargetKind: "user",
			TargetID:   id.String(),
			Metadata:   map[string]any{"email": u.Email},
			IP:         clientIP(r),
			UserAgent:  r.UserAgent(),
		})
		server.JSON(w, http.StatusOK, adminUserJSON(u))
	}
}

// PUT /admin/users/{id}/roles  {"roles": ["user","creator"]}
//
// Whole-set replacement rather than add/remove, so the UI's checkbox column
// saves as one atomic decision and two admins editing concurrently cannot
// interleave into a set neither of them chose.
func (h *AdminHandler) SetUserRoles(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if id == actor.UserID {
		problem(w, http.StatusForbidden, "self-target",
			"You cannot change your own roles — ask another administrator.")
		return
	}

	var body struct {
		Roles []string `json:"roles"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	known := map[string]AdminRole{}
	for _, role := range roles {
		known[role.Code] = role
	}

	wanted := dedupe(body.Roles)
	for _, code := range wanted {
		if _, exists := known[code]; !exists {
			problem(w, http.StatusBadRequest, "unknown-role", "No such role: "+code)
			return
		}
	}

	// Escalation guard. Computed against the roles being ADDED as well as those
	// being removed: taking a role away from someone more privileged than you is
	// as much an act of administration as granting one.
	current, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}
	touched := symmetricDifference(current.RoleCodes, wanted)
	if len(touched) > 0 {
		grants, err := h.Store.ListRoleGrants(r.Context())
		if err != nil {
			server.Internal(w)
			return
		}
		actorSet, err := h.Engine.Effective(r.Context(), rbac.Principal{
			UserID: actor.UserID, TokenVersion: actor.TokenVersion,
		})
		if err != nil {
			server.Internal(w)
			return
		}
		for _, code := range touched {
			if code == SuperadminRole && !actorSet.AllowsCode("*") {
				problem(w, http.StatusForbidden, "escalation",
					"Only a superadmin can grant or revoke the superadmin role.")
				return
			}
			for _, perm := range effectiveRolePermissions(code, known, grants) {
				if !actorSet.AllowsCode(perm) {
					problem(w, http.StatusForbidden, "escalation",
						"Role \""+code+"\" carries \""+perm+"\", which you do not hold.")
					return
				}
			}
		}
	}

	if err := h.Store.ReplaceUserRoles(r.Context(), id, wanted, &actor.UserID); err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}
	// Their effective permissions just changed; re-key the cache (and with it,
	// any access token minted under the old set).
	if _, err := h.Store.BumpUserTokenVersion(r.Context(), id); err != nil {
		server.Internal(w)
		return
	}

	u, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		server.Internal(w)
		return
	}
	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionRBACRolesReplaced,
		ActorID:    &actor.UserID,
		TargetKind: "user",
		TargetID:   id.String(),
		Metadata:   map[string]any{"before": current.RoleCodes, "after": u.RoleCodes},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	server.JSON(w, http.StatusOK, adminUserJSON(u))
}

// ── roles + permission matrix ──────────────────────────────────────────────

// GET /admin/permission-matrix
//
// The whole grid in one response: every role (with its parent), every
// permission, and per role both the DIRECT grants and the full effective set.
// The two are separate because they mean different things in the UI — a direct
// grant is a checkbox you can clear, an inherited one is a fact about the
// hierarchy that you change by editing the ancestor.
func (h *AdminHandler) PermissionMatrix(w http.ResponseWriter, r *http.Request) {
	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	perms, err := h.Store.ListPermissions(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	grants, err := h.Store.ListRoleGrants(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}

	byCode := map[string]AdminRole{}
	for _, role := range roles {
		byCode[role.Code] = role
	}

	roleOut := make([]map[string]any, 0, len(roles))
	for _, role := range roles {
		direct := grants[role.Code]
		sort.Strings(direct)
		effective := effectiveRolePermissions(role.Code, byCode, grants)
		roleOut = append(roleOut, map[string]any{
			"id":          role.ID,
			"code":        role.Code,
			"name":        role.Name,
			"description": role.Description,
			"parent_code": nilIfEmpty(role.ParentCode),
			"is_system":   role.IsSystem,
			"user_count":  role.UserCount,
			"direct":      nonNil(direct),
			"effective":   nonNil(effective),
		})
	}

	permOut := make([]map[string]any, 0, len(perms))
	for _, p := range perms {
		permOut = append(permOut, map[string]any{
			"code":        p.Code,
			"description": p.Description,
			"group":       permissionGroup(p.Code),
		})
	}

	server.JSON(w, http.StatusOK, map[string]any{
		"roles":       roleOut,
		"permissions": permOut,
	})
}

// PUT /admin/roles/{id}/permissions  {"permissions": ["music:read", …]}
func (h *AdminHandler) SetRolePermissions(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var body struct {
		Permissions []string `json:"permissions"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	var target *AdminRole
	byCode := map[string]AdminRole{}
	for i := range roles {
		byCode[roles[i].Code] = roles[i]
		if roles[i].ID == id {
			target = &roles[i]
		}
	}
	if target == nil {
		server.NotFound(w, server.ProblemType("account", "role_not_found"), "role not found")
		return
	}

	perms, err := h.Store.ListPermissions(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	knownPerm := map[string]bool{}
	for _, p := range perms {
		knownPerm[p.Code] = true
	}
	wanted := dedupe(body.Permissions)
	for _, code := range wanted {
		if !knownPerm[code] {
			problem(w, http.StatusBadRequest, "unknown-permission", "No such permission: "+code)
			return
		}
	}

	grants, err := h.Store.ListRoleGrants(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	actorSet, err := h.Engine.Effective(r.Context(), rbac.Principal{
		UserID: actor.UserID, TokenVersion: actor.TokenVersion,
	})
	if err != nil {
		server.Internal(w)
		return
	}
	// Escalation guard, on both directions of the change: you may not add a
	// permission you lack, and you may not strip one you lack either — otherwise
	// an editor could quietly disarm a role that outranks them.
	for _, code := range symmetricDifference(grants[target.Code], wanted) {
		if !actorSet.AllowsCode(code) {
			problem(w, http.StatusForbidden, "escalation",
				"You cannot grant or revoke \""+code+"\" — you do not hold it yourself.")
			return
		}
	}

	if err := h.Store.ReplaceRolePermissions(r.Context(), id, wanted, &actor.UserID); err != nil {
		notFoundOrInternal(w, err, "role not found")
		return
	}
	if err := h.Store.BumpTokenVersionForRole(r.Context(), id); err != nil {
		server.Internal(w)
		return
	}

	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionRBACPermsReplaced,
		ActorID:    &actor.UserID,
		TargetKind: "role",
		TargetID:   id.String(),
		Metadata: map[string]any{
			"role":   target.Code,
			"before": nonNil(grants[target.Code]),
			"after":  nonNil(wanted),
		},
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	})
	h.PermissionMatrix(w, r)
}

// POST /admin/roles
func (h *AdminHandler) CreateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	in, ok := decodeRoleBody(w, r)
	if !ok {
		return
	}
	if in.Code == "" {
		problem(w, http.StatusBadRequest, "validation", "A role code is required.")
		return
	}
	if !validRoleCode(in.Code) {
		problem(w, http.StatusBadRequest, "validation",
			"A role code is lowercase letters, digits, '-' and '_'.")
		return
	}

	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	byCode := map[string]AdminRole{}
	for _, role := range roles {
		byCode[role.Code] = role
	}
	if _, taken := byCode[in.Code]; taken {
		problem(w, http.StatusConflict, "role-exists", "A role with that code already exists.")
		return
	}
	if in.ParentCode != "" {
		if _, exists := byCode[in.ParentCode]; !exists {
			problem(w, http.StatusBadRequest, "unknown-role", "No such parent role: "+in.ParentCode)
			return
		}
	}

	role, err := h.Store.CreateRole(r.Context(), in)
	if err != nil {
		server.Internal(w)
		return
	}
	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionRBACRoleCreated,
		ActorID:    &actor.UserID,
		TargetKind: "role",
		TargetID:   role.ID.String(),
		Metadata:   map[string]any{"code": role.Code, "parent": role.ParentCode},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	server.JSON(w, http.StatusCreated, adminRoleJSON(role))
}

// PATCH /admin/roles/{id}
func (h *AdminHandler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	in, ok := decodeRoleBody(w, r)
	if !ok {
		return
	}

	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	byCode := map[string]AdminRole{}
	var target *AdminRole
	for i := range roles {
		byCode[roles[i].Code] = roles[i]
		if roles[i].ID == id {
			target = &roles[i]
		}
	}
	if target == nil {
		server.NotFound(w, server.ProblemType("account", "role_not_found"), "role not found")
		return
	}
	if target.IsSystem {
		problem(w, http.StatusForbidden, "role-protected",
			"System roles cannot be edited — create a new role instead.")
		return
	}
	if in.ParentCode != "" {
		if _, exists := byCode[in.ParentCode]; !exists {
			problem(w, http.StatusBadRequest, "unknown-role", "No such parent role: "+in.ParentCode)
			return
		}
		// Cycle guard. The DB CHECK only catches self-parenting; a longer loop
		// would make GetEffectivePermissions recurse forever, so it is refused
		// here — the one place that knows the whole graph.
		if wouldCycle(target.Code, in.ParentCode, byCode) {
			problem(w, http.StatusBadRequest, "role-cycle",
				"That parent would create a cycle in the role hierarchy.")
			return
		}
	}

	// The code is immutable: permission grants, seed data and the escalation
	// guard all key on it, and a rename would silently re-point every one.
	in.Code = target.Code
	role, err := h.Store.UpdateRole(r.Context(), id, in)
	if err != nil {
		notFoundOrInternal(w, err, "role not found")
		return
	}
	if err := h.Store.BumpTokenVersionForRole(r.Context(), id); err != nil {
		server.Internal(w)
		return
	}
	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionRBACRoleUpdated,
		ActorID:    &actor.UserID,
		TargetKind: "role",
		TargetID:   id.String(),
		Metadata:   map[string]any{"code": role.Code, "parent": role.ParentCode},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	server.JSON(w, http.StatusOK, adminRoleJSON(role))
}

// DELETE /admin/roles/{id}
func (h *AdminHandler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		server.Internal(w)
		return
	}
	var target *AdminRole
	for i := range roles {
		if roles[i].ID == id {
			target = &roles[i]
		}
	}
	if target == nil {
		server.NotFound(w, server.ProblemType("account", "role_not_found"), "role not found")
		return
	}
	if target.IsSystem {
		problem(w, http.StatusForbidden, "role-protected", "System roles cannot be deleted.")
		return
	}
	if target.UserCount > 0 {
		problem(w, http.StatusConflict, "role-in-use",
			"This role is still assigned to users. Move them off it first.")
		return
	}
	for _, role := range roles {
		if role.ParentCode == target.Code {
			problem(w, http.StatusConflict, "role-in-use",
				"Role \""+role.Code+"\" inherits from this one. Re-parent it first.")
			return
		}
	}

	if err := h.Store.DeleteRole(r.Context(), id); err != nil {
		notFoundOrInternal(w, err, "role not found")
		return
	}
	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionRBACRoleDeleted,
		ActorID:    &actor.UserID,
		TargetKind: "role",
		TargetID:   id.String(),
		Metadata:   map[string]any{"code": target.Code},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── helpers ────────────────────────────────────────────────────────────────

// effectiveRolePermissions folds a role's own grants together with every
// ancestor's, which is the same union GetEffectivePermissions computes in SQL —
// reproduced here because the matrix needs it for all roles at once, and one
// recursive query per role would be a round trip per grid row.
//
// The walk is bounded by a visited set: a cycle would otherwise hang the
// request, and while UpdateRole refuses to create one, data predating this code
// (or written by hand) could still contain one.
func effectiveRolePermissions(code string, byCode map[string]AdminRole, grants map[string][]string) []string {
	seen := map[string]bool{}
	out := map[string]bool{}
	for cur := code; cur != "" && !seen[cur]; {
		seen[cur] = true
		for _, p := range grants[cur] {
			out[p] = true
		}
		role, ok := byCode[cur]
		if !ok {
			break
		}
		cur = role.ParentCode
	}
	codes := make([]string, 0, len(out))
	for c := range out {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return codes
}

// wouldCycle reports whether making `parent` the parent of `role` closes a loop —
// true when `role` is already an ancestor of `parent`.
func wouldCycle(role, parent string, byCode map[string]AdminRole) bool {
	seen := map[string]bool{}
	for cur := parent; cur != "" && !seen[cur]; {
		if cur == role {
			return true
		}
		seen[cur] = true
		r, ok := byCode[cur]
		if !ok {
			return false
		}
		cur = r.ParentCode
	}
	return false
}

// permissionGroup is the leading resource segment ("music:write:own" → "music"),
// which is how the matrix groups its columns.
func permissionGroup(code string) string {
	if code == "*" {
		return "*"
	}
	if i := strings.IndexByte(code, ':'); i > 0 {
		return code[:i]
	}
	return code
}

func symmetricDifference(a, b []string) []string {
	inA, inB := map[string]bool{}, map[string]bool{}
	for _, s := range a {
		inA[s] = true
	}
	for _, s := range b {
		inB[s] = true
	}
	out := []string{}
	for s := range inA {
		if !inB[s] {
			out = append(out, s)
		}
	}
	for s := range inB {
		if !inA[s] {
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func validApprovalStatus(s string) bool {
	return s == ApprovalPending || s == ApprovalApproved || s == ApprovalRejected
}

// validRoleCode mirrors the permissions_code_format spirit: lowercase, no
// separators that would confuse the permission grammar.
func validRoleCode(s string) bool {
	if len(s) == 0 || len(s) > 40 {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func decodeRoleBody(w http.ResponseWriter, r *http.Request) (SaveRoleInput, bool) {
	var body struct {
		Code        string `json:"code"`
		Name        string `json:"name"`
		Description string `json:"description"`
		ParentCode  string `json:"parent_code"`
	}
	if !server.Decode(w, r, &body) {
		return SaveRoleInput{}, false
	}
	in := SaveRoleInput{
		Code:        strings.ToLower(strings.TrimSpace(body.Code)),
		Name:        strings.TrimSpace(body.Name),
		Description: strings.TrimSpace(body.Description),
		ParentCode:  strings.ToLower(strings.TrimSpace(body.ParentCode)),
	}
	if in.Name == "" {
		in.Name = in.Code
	}
	return in, true
}

func requireActor(w http.ResponseWriter, r *http.Request) (*auth.Identity, bool) {
	id, ok := auth.FromContext(r.Context())
	if !ok || id.IsAnonymous() {
		server.Unauthorized(w)
		return nil, false
	}
	return id, true
}

func pathUUID(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		server.NotFound(w, server.ProblemType("account", "not_found"), "not found")
		return uuid.Nil, false
	}
	return id, true
}

func problem(w http.ResponseWriter, status int, code, detail string) {
	server.Problem(w, status, server.ProblemType("account", code), http.StatusText(status), detail)
}

func badRequest(w http.ResponseWriter, detail string) {
	problem(w, http.StatusBadRequest, "validation", detail)
}

func notFoundOrInternal(w http.ResponseWriter, err error, detail string) {
	if errors.Is(err, ErrUserNotFound) || errors.Is(err, ErrRoleNotFound) {
		server.NotFound(w, server.ProblemType("account", "not_found"), detail)
		return
	}
	server.Internal(w)
}

func adminUserJSON(u AdminUser) map[string]any {
	return map[string]any{
		"id":              u.ID,
		"email":           u.Email,
		"display_name":    u.DisplayName,
		"avatar_url":      nilIfEmpty(u.AvatarURL),
		"approval_status": u.ApprovalStatus,
		"approval_note":   nilIfEmpty(u.ApprovalNote),
		"approved_at":     u.ApprovedAt,
		"approved_by":     u.ApprovedBy,
		"disabled":        u.Disabled,
		"has_password":    u.HasPassword,
		"created_at":      u.CreatedAt,
		"roles":           nonNil(u.RoleCodes),
	}
}

func adminRoleJSON(r AdminRole) map[string]any {
	return map[string]any{
		"id":          r.ID,
		"code":        r.Code,
		"name":        r.Name,
		"description": r.Description,
		"parent_code": nilIfEmpty(r.ParentCode),
		"is_system":   r.IsSystem,
		"user_count":  r.UserCount,
	}
}

// nonNil keeps a nil slice from serialising as JSON `null` where the client
// expects a list it can map over.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}
