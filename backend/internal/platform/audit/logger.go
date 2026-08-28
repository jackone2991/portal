// Package audit centralises writing security-sensitive events to the
// append-only `audit_log` table. It lives under platform/ because audit is
// cross-cutting: account, tenant, media, bank, … all record events here.
// The account module is a consumer like any other (per [D-25]).
//
// Audit events are best-effort *for the request*: a write failure must not
// block the user-facing operation, but should be logged loudly so an alert
// can fire. (If audit reliability becomes load-bearing, route through Asynq
// with a dedicated queue.)
package audit

import (
	"context"
	"encoding/json"
	"net"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Action codes follow the <module>.<resource>.<action> taxonomy registered in
// backend/MODULES.md §5.3 (period-separated; distinct from Asynq's notify:*
// prefix). These are the account module's events; other modules own their own
// namespaces (bank.*, tenant.*, …). Keep this list canonical — searches and
// dashboards depend on it. Renamed from the legacy auth.*/rbac.*/user.* codes
// to fit the taxonomy per [D-25].
const (
	// session + refresh-token lifecycle
	ActionAuthRegister        = "account.user.registered"
	ActionAuthLogin           = "account.session.login"
	ActionAuthLogout          = "account.session.logout"
	ActionAuthRefresh         = "account.refresh.rotated"
	ActionAuthRefreshReuse    = "account.refresh.reuse_detected" // SECURITY ALERT
	ActionAuthDisabledAttempt = "account.session.disabled_attempt"

	// password reset (SPEC-04 P0.3)
	ActionPasswordResetRequested = "account.password.reset_requested"
	ActionPasswordResetCompleted = "account.password.reset_completed"

	// rbac
	ActionRBACRoleCreated = "account.role.created"
	ActionRBACRoleUpdated = "account.role.updated"
	ActionRBACRoleDeleted = "account.role.deleted"
	ActionRBACRoleGranted = "account.role.granted" // user assignment
	ActionRBACRoleRevoked = "account.role.revoked"
	ActionRBACPermGranted = "account.permission.granted" // role grant
	ActionRBACPermRevoked = "account.permission.revoked"

	// rbac — whole-set edits from the admin console. The granted/revoked pairs
	// above describe a single grant; these describe a checkbox row saved at once,
	// and carry before/after in metadata so the diff is recoverable.
	ActionRBACRolesReplaced = "account.user.roles_changed"
	ActionRBACPermsReplaced = "account.role.permissions_changed"

	// users
	ActionUserCreated  = "account.user.created" // admin-provisioned, not self-registered
	ActionUserUpdated  = "account.user.updated" // email / display name / admin password set
	ActionUserDisabled = "account.user.disabled"
	ActionUserEnabled  = "account.user.enabled"
	// ActionUserDeleted records a HARD delete. Every FK to users cascades, so the
	// row is the only trace left of the account's content — keep the email and
	// roles in the metadata, because nothing else survives to be looked up.
	ActionUserDeleted = "account.user.deleted"

	// layout — the shell's navigation and dashboard composition (migration 0035).
	// Whole-set saves, so the metadata records the size rather than a diff; the
	// previous arrangement is not reconstructible from here, only the fact and
	// the author of the change.
	ActionLayoutMenuSaved    = "layout.menu.saved"
	ActionLayoutWidgetsSaved = "layout.widgets.saved"

	// registration approval (migration 0031). A registration is inert until one
	// of these lands, so they are the record of who let an account in.
	ActionUserApproved        = "account.user.approved"
	ActionUserRejected        = "account.user.rejected"
	ActionUserApprovalRevoked = "account.user.approval_revoked"
	ActionUserPendingAttempt  = "account.session.pending_attempt"
)

// ApprovalAction maps an approval state to its audit action.
func ApprovalAction(status string) string {
	switch status {
	case "approved":
		return ActionUserApproved
	case "rejected":
		return ActionUserRejected
	default:
		return ActionUserApprovalRevoked
	}
}

// EventStore is implemented by the sqlc-generated repo.
type EventStore interface {
	WriteAuditEvent(ctx context.Context, in WriteEventInput) error
}

type WriteEventInput struct {
	ActorID    *uuid.UUID
	ActorKind  string // "user" | "system" | "service"
	Action     string
	TargetKind *string
	TargetID   *string
	Metadata   []byte // JSON
	IP         net.IP
	UserAgent  string
}

// Logger is the recorder. Construct one at startup; reuse everywhere.
type Logger struct {
	store EventStore
}

func New(store EventStore) *Logger {
	return &Logger{store: store}
}

// Event is the ergonomic input form.
type Event struct {
	Action     string
	ActorID    *uuid.UUID
	ActorKind  string // defaults to "user"
	TargetKind string
	TargetID   string
	IP         net.IP
	UserAgent  string
	Metadata   map[string]any
}

// Write persists the event. Errors are logged but never returned: callers
// should not abort a user request on audit failure. See package doc.
func (l *Logger) Write(ctx context.Context, e Event) {
	if l == nil || l.store == nil {
		return
	}

	kind := e.ActorKind
	if kind == "" {
		kind = "user"
	}

	var metaJSON []byte
	if len(e.Metadata) > 0 {
		buf, err := json.Marshal(e.Metadata)
		if err != nil {
			log.Warn().Err(err).Str("action", e.Action).Msg("audit: metadata marshal failed")
		} else {
			metaJSON = buf
		}
	}
	if metaJSON == nil {
		metaJSON = []byte(`{}`)
	}

	in := WriteEventInput{
		ActorID:   e.ActorID,
		ActorKind: kind,
		Action:    e.Action,
		Metadata:  metaJSON,
		IP:        e.IP,
		UserAgent: e.UserAgent,
	}
	if e.TargetKind != "" {
		k := e.TargetKind
		in.TargetKind = &k
	}
	if e.TargetID != "" {
		t := e.TargetID
		in.TargetID = &t
	}
	if err := l.store.WriteAuditEvent(ctx, in); err != nil {
		log.Error().Err(err).Str("action", e.Action).Msg("audit: write failed")
	}
}
