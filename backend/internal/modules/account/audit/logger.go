// Package audit centralises writing security-sensitive events to the
// append-only `audit_log` table.
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

// Action codes are dotted strings: <domain>.<entity>.<verb>.
// Keep this list canonical — searches and dashboards depend on it.
const (
	// authentication
	ActionAuthLogin            = "auth.login"
	ActionAuthLogout           = "auth.logout"
	ActionAuthRefresh          = "auth.refresh"
	ActionAuthRefreshReuse     = "auth.refresh.reuse_detected" // SECURITY ALERT
	ActionAuthDisabledAttempt  = "auth.disabled_user_attempt"

	// rbac
	ActionRBACRoleCreated      = "rbac.role.created"
	ActionRBACRoleUpdated      = "rbac.role.updated"
	ActionRBACRoleDeleted      = "rbac.role.deleted"
	ActionRBACRoleGranted      = "rbac.role.granted"  // user assignment
	ActionRBACRoleRevoked      = "rbac.role.revoked"
	ActionRBACPermGranted      = "rbac.perm.granted"  // role grant
	ActionRBACPermRevoked      = "rbac.perm.revoked"

	// users
	ActionUserDisabled         = "user.disabled"
	ActionUserEnabled          = "user.enabled"
	ActionUserDeleted          = "user.deleted"
)

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
