package handler

// Create / edit / delete for the admin console's user directory.
//
// Split from admin.go because these three carry guards the read-only and
// approval routes do not need, and the guards are the substance:
//
//   - CREATE cannot be a way around the approval gate. The new account's
//     approval state comes from the CREATOR's authority, never from the body.
//   - EDIT and DELETE cannot be a takeover route. Email is the login identifier,
//     so being able to change a more privileged account's email — or delete it —
//     is worth as much as being handed its permissions.
//   - DELETE cascades. Every FK to `users` is ON DELETE CASCADE, so it also
//     destroys the account's assets, comics, movies, tracks, stories, ledger,
//     journal entries, people and organizations. Postgres will not object and
//     there is no undo, so the request must name the account it is destroying.

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/rbac"
	"github.com/portal/backend/internal/platform/audit"
	"github.com/portal/backend/internal/platform/server"
)

// defaultRole is what both self-registration and admin provisioning seed, so a
// new account can read the catalogue and nothing more.
const defaultRole = "user"

// ── POST /admin/users ──────────────────────────────────────────────────────

// CreateUser provisions an account directly, without a registration.
//
// Roles are deliberately NOT settable here: the caller assigns them afterwards
// through SetUserRoles, which keeps the escalation guard in exactly one place
// instead of two that have to agree.
func (h *AdminHandler) CreateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	email := normalizeEmail(body.Email)
	if !validEmail(email) {
		problem(w, http.StatusBadRequest, "invalid-email", "Enter a valid email address.")
		return
	}
	// A password is required, not optional. An account with no credential cannot
	// sign in and cannot recover either — the reset link goes to an address
	// nobody has proven they own — so "create without a password" would only
	// produce accounts that look real and are not.
	if len(body.Password) < minPasswordLen {
		problem(w, http.StatusBadRequest, "password-policy", "The password must be at least 8 characters.")
		return
	}
	hash, err := auth.HashPassword(body.Password)
	if err != nil {
		server.Internal(w)
		return
	}

	// The approval state is taken from the creator's authority, never from the
	// request. Otherwise this form is a way around the approval gate for anybody
	// holding `users:write:any`.
	set, err := h.actorPermissions(r, actor)
	if err != nil {
		server.Internal(w)
		return
	}
	status := ApprovalPending
	var approvedBy *uuid.UUID
	if set.AllowsCode(PermApproveUser) {
		status = ApprovalApproved
		approvedBy = &actor.UserID
	}

	id, err := h.Store.CreateUser(r.Context(), CreateUserInput{
		Email:        email,
		DisplayName:  nonEmpty(strings.TrimSpace(body.DisplayName), emailLocalPart(email)),
		PasswordHash: hash,
		Status:       status,
		ApprovedBy:   approvedBy,
	})
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			problem(w, http.StatusConflict, "email-taken", "An account with this email already exists.")
			return
		}
		server.Internal(w)
		return
	}

	// Baseline role, same as self-registration, and best-effort for the same
	// reason: a role-assign hiccup must not leave a half-created account behind.
	roleErr := h.Store.ReplaceUserRoles(r.Context(), id, []string{defaultRole}, &actor.UserID)

	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionUserCreated,
		ActorID:    &actor.UserID,
		TargetKind: "user",
		TargetID:   id.String(),
		Metadata: map[string]any{
			"email":           email,
			"approval_status": status,
			"role_seed_error": errString(roleErr),
		},
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	})

	u, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		server.Internal(w)
		return
	}
	server.JSON(w, http.StatusCreated, adminUserJSON(u))
}

// ── PATCH /admin/users/{id} ────────────────────────────────────────────────

// UpdateUser edits the email, the display name, and optionally sets a new
// password.
//
// Editing yourself is allowed here, unlike roles and approval: renaming yourself
// locks nobody out. Email is editable because it IS the login identifier, so a
// typo made at registration would otherwise be unfixable from the UI.
func (h *AdminHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	var body struct {
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
		Password    string `json:"password"`
	}
	if !server.Decode(w, r, &body) {
		return
	}
	// Validate the password BEFORE writing the profile, so a rejected password
	// cannot leave the email already changed.
	if body.Password != "" && len(body.Password) < minPasswordLen {
		problem(w, http.StatusBadRequest, "password-policy", "The password must be at least 8 characters.")
		return
	}

	before, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}
	if id != actor.UserID {
		denial, err := h.targetAuthorityDenial(r, actor, before)
		if err != nil {
			server.Internal(w)
			return
		}
		if denial != "" {
			problem(w, http.StatusForbidden, "escalation", denial)
			return
		}
	}

	email := normalizeEmail(nonEmpty(body.Email, before.Email))
	if !validEmail(email) {
		problem(w, http.StatusBadRequest, "invalid-email", "Enter a valid email address.")
		return
	}
	name := nonEmpty(strings.TrimSpace(body.DisplayName), before.DisplayName)

	if err := h.Store.UpdateUser(r.Context(), id, UpdateUserInput{Email: email, DisplayName: name}); err != nil {
		if errors.Is(err, ErrEmailTaken) {
			problem(w, http.StatusConflict, "email-taken", "An account with this email already exists.")
			return
		}
		notFoundOrInternal(w, err, "user not found")
		return
	}

	if body.Password != "" {
		hash, err := auth.HashPassword(body.Password)
		if err != nil {
			server.Internal(w)
			return
		}
		if err := h.Store.SetPassword(r.Context(), id, hash); err != nil {
			server.Internal(w)
			return
		}
		// Both revocation channels, same as the self-service reset: the bump kills
		// outstanding access tokens, and burning the refresh chain stops the
		// browser silently minting new ones from a cookie the OLD password issued.
		if _, err := h.Store.BumpUserTokenVersion(r.Context(), id); err != nil {
			server.Internal(w)
			return
		}
		_ = h.Store.RevokeAllSessions(r.Context(), id, "admin_password_change")
	}

	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionUserUpdated,
		ActorID:    &actor.UserID,
		TargetKind: "user",
		TargetID:   id.String(),
		Metadata: map[string]any{
			"email_before":     before.Email,
			"email_after":      email,
			"password_changed": body.Password != "",
		},
		IP:        clientIP(r),
		UserAgent: r.UserAgent(),
	})

	u, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		server.Internal(w)
		return
	}
	server.JSON(w, http.StatusOK, adminUserJSON(u))
}

// ── DELETE /admin/users/{id} ───────────────────────────────────────────────

// DeleteUser removes the account permanently, and its content with it.
//
// `confirm_email` must match the target. That is not UI politeness — it is
// enforced here so a misclick, a stale tab or a replayed request cannot reach a
// cascading DELETE. "Disable" stays the reversible option and is what the UI
// leads with.
func (h *AdminHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireActor(w, r)
	if !ok {
		return
	}
	id, ok := pathUUID(w, r)
	if !ok {
		return
	}
	if id == actor.UserID {
		problem(w, http.StatusForbidden, "self-target", "You cannot delete your own account.")
		return
	}

	var body struct {
		ConfirmEmail string `json:"confirm_email"`
	}
	if !server.Decode(w, r, &body) {
		return
	}

	target, err := h.Store.GetAdminUser(r.Context(), id)
	if err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}
	if !strings.EqualFold(strings.TrimSpace(body.ConfirmEmail), target.Email) {
		problem(w, http.StatusBadRequest, "confirmation-mismatch",
			"Type the account's email address to confirm this deletion.")
		return
	}

	denial, err := h.targetAuthorityDenial(r, actor, target)
	if err != nil {
		server.Internal(w)
		return
	}
	if denial != "" {
		problem(w, http.StatusForbidden, "escalation", denial)
		return
	}

	lastApprover, err := h.wouldStrandApprovals(r.Context(), id)
	if err != nil {
		server.Internal(w)
		return
	}
	if lastApprover {
		problem(w, http.StatusConflict, "last-approver",
			"This is the last account that can approve registrations. Give another account "+
				"that permission first, or nobody will be able to approve a signup.")
		return
	}

	if err := h.Store.DeleteUser(r.Context(), id); err != nil {
		notFoundOrInternal(w, err, "user not found")
		return
	}

	h.Audit.Write(r.Context(), audit.Event{
		Action:     audit.ActionUserDeleted,
		ActorID:    &actor.UserID,
		TargetKind: "user",
		TargetID:   id.String(),
		Metadata:   map[string]any{"email": target.Email, "roles": nonNil(target.RoleCodes)},
		IP:         clientIP(r),
		UserAgent:  r.UserAgent(),
	})
	w.WriteHeader(http.StatusNoContent)
}

// ── shared guards ──────────────────────────────────────────────────────────

func (h *AdminHandler) actorPermissions(r *http.Request, actor *auth.Identity) (rbac.Set, error) {
	return h.Engine.Effective(r.Context(), rbac.Principal{
		UserID: actor.UserID, TokenVersion: actor.TokenVersion,
	})
}

// targetAuthorityDenial returns a non-empty reason when the actor may not act on
// this account because it outranks them, or an error if the check itself failed.
// A failed check is a 500, never a silent pass — this is the guard that stops an
// editor from taking over a superadmin by changing its email.
func (h *AdminHandler) targetAuthorityDenial(r *http.Request, actor *auth.Identity, target AdminUser) (string, error) {
	if len(target.RoleCodes) == 0 {
		return "", nil
	}
	roles, err := h.Store.ListAdminRoles(r.Context())
	if err != nil {
		return "", err
	}
	grants, err := h.Store.ListRoleGrants(r.Context())
	if err != nil {
		return "", err
	}
	set, err := h.actorPermissions(r, actor)
	if err != nil {
		return "", err
	}

	byCode := make(map[string]AdminRole, len(roles))
	for _, role := range roles {
		byCode[role.Code] = role
	}
	for _, code := range target.RoleCodes {
		if code == SuperadminRole && !set.AllowsCode("*") {
			return "Only a superadmin can act on a superadmin account.", nil
		}
		for _, perm := range effectiveRolePermissions(code, byCode, grants) {
			if !set.AllowsCode(perm) {
				return "This account holds \"" + perm + "\", which you do not.", nil
			}
		}
	}
	return "", nil
}

// wouldStrandApprovals reports whether removing `losing` leaves the install with
// nobody able to approve a registration.
//
// Registration is approve-first, so zero approvers means signups are accepted
// that no one can ever let in — a dead end that still looks like a working
// system. ListPermissionHolders already skips disabled and non-approved
// accounts, so the same question covers deleting, disabling and un-approving.
func (h *AdminHandler) wouldStrandApprovals(ctx context.Context, losing uuid.UUID) (bool, error) {
	holders, err := h.Store.ListPermissionHolders(ctx, approvalResource)
	if err != nil {
		return false, err
	}
	for _, id := range approverIDs(holders, PermApproveUser) {
		if id != losing {
			return false, nil
		}
	}
	return true, nil
}

func errString(err error) any {
	if err == nil {
		return nil
	}
	return err.Error()
}
