package handler

// The admin console's security properties, exercised through the HTTP surface.
//
// The permission middleware decides WHO may reach these routes; these tests
// cover what the handlers do once someone is through the door — the rules that
// stop a legitimate `rbac:role:write` holder from writing themselves a bigger
// role. Those are the ones that turn a matrix editor into a privilege-escalation
// form if they regress, and none of them are visible in the route table.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/portal/backend/internal/modules/account/auth"
	"github.com/portal/backend/internal/modules/account/rbac"
)

/* ── fakes ────────────────────────────────────────────────────────── */

// stubLoader hands the engine a fixed permission set for every principal, which
// is all these tests need: they vary the ACTOR's authority, never the lookup.
type stubLoader struct{ codes []string }

func (l stubLoader) LoadEffective(context.Context, uuid.UUID, int) (rbac.Set, error) {
	return rbac.NewSet(l.codes), nil
}

type fakeAdminStore struct {
	users  map[uuid.UUID]AdminUser
	roles  []AdminRole
	grants map[string][]string
	perms  []AdminPermission

	replacedRoles map[uuid.UUID][]string
	replacedPerms map[uuid.UUID][]string
	bumped        []uuid.UUID

	// CRUD side-effects the tests assert on.
	created         []AdminUser
	deleted         []uuid.UUID
	passwordSet     []uuid.UUID
	sessionsRevoked []uuid.UUID
	// holders is what ListPermissionHolders returns — the last-approver guard
	// reads it, so a store with none behaves like an install with no approvers.
	holders []PermissionHolder
}

func newFakeStore() *fakeAdminStore {
	return &fakeAdminStore{
		users: map[uuid.UUID]AdminUser{},
		roles: []AdminRole{
			{ID: uuid.New(), Code: "guest", Name: "Guest", IsSystem: true},
			{ID: uuid.New(), Code: "user", Name: "User", ParentCode: "guest", IsSystem: true},
			{ID: uuid.New(), Code: "creator", Name: "Creator", ParentCode: "user", IsSystem: true},
			{ID: uuid.New(), Code: "superadmin", Name: "Super", ParentCode: "creator", IsSystem: true},
		},
		grants: map[string][]string{
			"guest":      {"music:read"},
			"user":       {"profile:read"},
			"creator":    {"music:write:own"},
			"superadmin": {"*"},
		},
		perms: []AdminPermission{
			{Code: "*"}, {Code: "music:read"}, {Code: "music:write:own"},
			{Code: "profile:read"}, {Code: "users:approve"},
		},
		replacedRoles: map[uuid.UUID][]string{},
		replacedPerms: map[uuid.UUID][]string{},
	}
}

func (f *fakeAdminStore) roleID(code string) uuid.UUID {
	for _, r := range f.roles {
		if r.Code == code {
			return r.ID
		}
	}
	return uuid.Nil
}

func (f *fakeAdminStore) ListUsers(context.Context, ListUsersFilter) ([]AdminUser, int64, error) {
	out := make([]AdminUser, 0, len(f.users))
	for _, u := range f.users {
		out = append(out, u)
	}
	return out, int64(len(out)), nil
}
func (f *fakeAdminStore) CountUsersByStatus(context.Context) (map[string]int64, error) {
	return map[string]int64{}, nil
}
func (f *fakeAdminStore) GetAdminUser(_ context.Context, id uuid.UUID) (AdminUser, error) {
	u, ok := f.users[id]
	if !ok {
		return AdminUser{}, ErrUserNotFound
	}
	return u, nil
}
func (f *fakeAdminStore) SetApproval(_ context.Context, in SetApprovalInput) (AdminUser, error) {
	u, ok := f.users[in.UserID]
	if !ok {
		return AdminUser{}, ErrUserNotFound
	}
	u.ApprovalStatus = in.Status
	u.ApprovalNote = in.Note
	f.users[in.UserID] = u
	return u, nil
}
func (f *fakeAdminStore) SetUserDisabled(_ context.Context, id uuid.UUID, disabled bool) error {
	u, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	u.Disabled = disabled
	f.users[id] = u
	return nil
}
func (f *fakeAdminStore) ReplaceUserRoles(_ context.Context, userID uuid.UUID, codes []string, _ *uuid.UUID) error {
	f.replacedRoles[userID] = codes
	u := f.users[userID]
	u.RoleCodes = codes
	f.users[userID] = u
	return nil
}
func (f *fakeAdminStore) CreateUser(_ context.Context, in CreateUserInput) (uuid.UUID, error) {
	for _, u := range f.users {
		if u.Email == in.Email {
			return uuid.Nil, ErrEmailTaken
		}
	}
	id := uuid.New()
	f.users[id] = AdminUser{
		ID: id, Email: in.Email, DisplayName: in.DisplayName,
		ApprovalStatus: in.Status, HasPassword: in.PasswordHash != "",
	}
	f.created = append(f.created, f.users[id])
	return id, nil
}

func (f *fakeAdminStore) UpdateUser(_ context.Context, id uuid.UUID, in UpdateUserInput) error {
	u, ok := f.users[id]
	if !ok {
		return ErrUserNotFound
	}
	for other, v := range f.users {
		if other != id && v.Email == in.Email {
			return ErrEmailTaken
		}
	}
	u.Email, u.DisplayName = in.Email, in.DisplayName
	f.users[id] = u
	return nil
}

func (f *fakeAdminStore) SetPassword(_ context.Context, id uuid.UUID, hash string) error {
	f.passwordSet = append(f.passwordSet, id)
	u := f.users[id]
	u.HasPassword = hash != ""
	f.users[id] = u
	return nil
}

func (f *fakeAdminStore) RevokeAllSessions(_ context.Context, id uuid.UUID, _ string) error {
	f.sessionsRevoked = append(f.sessionsRevoked, id)
	return nil
}

func (f *fakeAdminStore) DeleteUser(_ context.Context, id uuid.UUID) error {
	if _, ok := f.users[id]; !ok {
		return ErrUserNotFound
	}
	delete(f.users, id)
	f.deleted = append(f.deleted, id)
	return nil
}

func (f *fakeAdminStore) ListPermissionHolders(context.Context, string) ([]PermissionHolder, error) {
	return f.holders, nil
}

func (f *fakeAdminStore) ListAdminRoles(context.Context) ([]AdminRole, error) { return f.roles, nil }
func (f *fakeAdminStore) ListPermissions(context.Context) ([]AdminPermission, error) {
	return f.perms, nil
}
func (f *fakeAdminStore) ListRoleGrants(context.Context) (map[string][]string, error) {
	return f.grants, nil
}
func (f *fakeAdminStore) ReplaceRolePermissions(_ context.Context, roleID uuid.UUID, codes []string, _ *uuid.UUID) error {
	f.replacedPerms[roleID] = codes
	return nil
}
func (f *fakeAdminStore) CreateRole(_ context.Context, in SaveRoleInput) (AdminRole, error) {
	r := AdminRole{ID: uuid.New(), Code: in.Code, Name: in.Name, ParentCode: in.ParentCode}
	f.roles = append(f.roles, r)
	return r, nil
}
func (f *fakeAdminStore) UpdateRole(_ context.Context, id uuid.UUID, in SaveRoleInput) (AdminRole, error) {
	return AdminRole{ID: id, Code: in.Code, Name: in.Name, ParentCode: in.ParentCode}, nil
}
func (f *fakeAdminStore) DeleteRole(context.Context, uuid.UUID) error { return nil }
func (f *fakeAdminStore) BumpTokenVersionForRole(_ context.Context, id uuid.UUID) error {
	f.bumped = append(f.bumped, id)
	return nil
}
func (f *fakeAdminStore) BumpUserTokenVersion(_ context.Context, id uuid.UUID) (int, error) {
	f.bumped = append(f.bumped, id)
	return 2, nil
}

/* ── harness ──────────────────────────────────────────────────────── */

// call routes one request through chi so {id} resolves, with `actor` on the
// context exactly as RequireAuth would leave it.
func call(t *testing.T, h *AdminHandler, method, pattern, path string, actor uuid.UUID, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := chi.NewRouter()
	r.Method(method, pattern, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch method + " " + pattern {
		case "PUT /users/{id}/roles":
			h.SetUserRoles(w, req)
		case "PUT /roles/{id}/permissions":
			h.SetRolePermissions(w, req)
		case "POST /users/{id}/approve":
			h.Approve(w, req)
		case "POST /users":
			h.CreateUser(w, req)
		case "PATCH /users/{id}":
			h.UpdateUser(w, req)
		case "DELETE /users/{id}":
			h.DeleteUser(w, req)
		default:
			t.Fatalf("no handler bound for %s %s", method, pattern)
		}
	}))

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(auth.WithIdentity(req.Context(), &auth.Identity{UserID: actor, TokenVersion: 1}))

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func newHandler(store *fakeAdminStore, actorPerms []string) *AdminHandler {
	return &AdminHandler{
		Store:  store,
		Engine: rbac.NewEngine(stubLoader{codes: actorPerms}),
		Audit:  nil, // the logger tolerates a nil receiver; audit is best-effort
	}
}

func problemType(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %s", rec.Body.String())
	}
	s, _ := body["type"].(string)
	return s
}

/* ── escalation guard ─────────────────────────────────────────────── */

func TestSetUserRolesRefusesGrantingPermissionsTheActorLacks(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "t@x", RoleCodes: []string{"user"}}
	actor := uuid.New()

	// The actor may assign roles, but holds nothing `creator` carries.
	h := newHandler(store, []string{"rbac:role:assign", "users:read:any"})
	rec := call(t, h, http.MethodPut, "/users/{id}/roles", "/users/"+target.String()+"/roles",
		actor, `{"roles":["user","creator"]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if got := problemType(t, rec); got != "account/escalation" {
		t.Errorf("problem type = %q, want account/escalation", got)
	}
	if _, wrote := store.replacedRoles[target]; wrote {
		t.Error("roles were written despite the refusal")
	}
}

func TestSetUserRolesAllowsAWildcardActor(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "t@x", RoleCodes: []string{"user"}}

	h := newHandler(store, []string{"*"})
	rec := call(t, h, http.MethodPut, "/users/{id}/roles", "/users/"+target.String()+"/roles",
		uuid.New(), `{"roles":["user","creator"]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	got := store.replacedRoles[target]
	if len(got) != 2 || got[0] != "creator" || got[1] != "user" {
		t.Errorf("replaced roles = %v, want [creator user]", got)
	}
}

// Only a wildcard holder may hand out superadmin — even an actor who happens to
// satisfy every permission superadmin currently carries. The role is the thing
// being guarded, not today's snapshot of its grants.
func TestSetUserRolesRefusesSuperadminFromNonWildcardActor(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "t@x", RoleCodes: []string{"user"}}

	h := newHandler(store, []string{"rbac:role:assign", "music:read", "profile:read", "music:write:own"})
	rec := call(t, h, http.MethodPut, "/users/{id}/roles", "/users/"+target.String()+"/roles",
		uuid.New(), `{"roles":["superadmin"]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

// Removing a role is as much an act of administration as granting one: an actor
// who cannot grant `creator` must not be able to strip it either.
func TestSetUserRolesRefusesRevokingAPrivilegedRole(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "t@x", RoleCodes: []string{"user", "creator"}}

	h := newHandler(store, []string{"rbac:role:assign", "profile:read"})
	rec := call(t, h, http.MethodPut, "/users/{id}/roles", "/users/"+target.String()+"/roles",
		uuid.New(), `{"roles":["user"]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

func TestSetUserRolesRefusesEditingYourOwnRoles(t *testing.T) {
	store := newFakeStore()
	actor := uuid.New()
	store.users[actor] = AdminUser{ID: actor, Email: "me@x", RoleCodes: []string{"user"}}

	h := newHandler(store, []string{"*"})
	rec := call(t, h, http.MethodPut, "/users/{id}/roles", "/users/"+actor.String()+"/roles",
		actor, `{"roles":["superadmin"]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if got := problemType(t, rec); got != "account/self-target" {
		t.Errorf("problem type = %q, want account/self-target", got)
	}
}

func TestApproveRefusesApprovingYourself(t *testing.T) {
	store := newFakeStore()
	actor := uuid.New()
	store.users[actor] = AdminUser{ID: actor, Email: "me@x", ApprovalStatus: ApprovalPending}

	h := newHandler(store, []string{"*"})
	rec := call(t, h, http.MethodPost, "/users/{id}/approve", "/users/"+actor.String()+"/approve",
		actor, "")

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if store.users[actor].ApprovalStatus != ApprovalPending {
		t.Error("approval state changed despite the refusal")
	}
}

func TestSetRolePermissionsRefusesGrantingWhatTheActorLacks(t *testing.T) {
	store := newFakeStore()
	roleID := store.roleID("user")

	// A legitimate matrix editor who does NOT hold users:approve. Letting this
	// through is exactly how the approval gate would be bypassed.
	h := newHandler(store, []string{"rbac:role:write", "profile:read"})
	rec := call(t, h, http.MethodPut, "/roles/{id}/permissions", "/roles/"+roleID.String()+"/permissions",
		uuid.New(), `{"permissions":["profile:read","users:approve"]}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if _, wrote := store.replacedPerms[roleID]; wrote {
		t.Error("permissions were written despite the refusal")
	}
}

func TestSetRolePermissionsBumpsTokenVersionForTheRole(t *testing.T) {
	store := newFakeStore()
	roleID := store.roleID("user")

	h := newHandler(store, []string{"*"})
	rec := call(t, h, http.MethodPut, "/roles/{id}/permissions", "/roles/"+roleID.String()+"/permissions",
		uuid.New(), `{"permissions":["profile:read","music:read"]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.bumped) != 1 || store.bumped[0] != roleID {
		t.Errorf("bumped = %v, want exactly [%s] — without it a revoke sits behind the RBAC cache TTL",
			store.bumped, roleID)
	}
}

func TestSetRolePermissionsRejectsUnknownCode(t *testing.T) {
	store := newFakeStore()
	roleID := store.roleID("user")

	h := newHandler(store, []string{"*"})
	rec := call(t, h, http.MethodPut, "/roles/{id}/permissions", "/roles/"+roleID.String()+"/permissions",
		uuid.New(), `{"permissions":["not:a:permission"]}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
}

/* ── hierarchy helpers ────────────────────────────────────────────── */

func TestEffectiveRolePermissionsUnionsAncestors(t *testing.T) {
	byCode := map[string]AdminRole{
		"guest":   {Code: "guest"},
		"user":    {Code: "user", ParentCode: "guest"},
		"creator": {Code: "creator", ParentCode: "user"},
	}
	grants := map[string][]string{
		"guest":   {"music:read"},
		"user":    {"profile:read"},
		"creator": {"music:write:own"},
	}

	got := effectiveRolePermissions("creator", byCode, grants)
	want := []string{"music:read", "music:write:own", "profile:read"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("effective = %v, want %v", got, want)
	}
}

// A cycle in the stored hierarchy must not hang the request. UpdateRole refuses
// to create one, but data written before this code existed could still contain
// one, and the matrix endpoint walks every role on every call.
func TestEffectiveRolePermissionsTerminatesOnACycle(t *testing.T) {
	byCode := map[string]AdminRole{
		"a": {Code: "a", ParentCode: "b"},
		"b": {Code: "b", ParentCode: "a"},
	}
	grants := map[string][]string{"a": {"x:read"}, "b": {"y:read"}}

	got := effectiveRolePermissions("a", byCode, grants)
	if len(got) != 2 {
		t.Errorf("effective = %v, want both grants exactly once", got)
	}
}

func TestWouldCycleDetectsAnIndirectLoop(t *testing.T) {
	byCode := map[string]AdminRole{
		"guest":   {Code: "guest"},
		"user":    {Code: "user", ParentCode: "guest"},
		"creator": {Code: "creator", ParentCode: "user"},
	}
	// Re-parenting guest under creator closes guest → user → creator → guest.
	if !wouldCycle("guest", "creator", byCode) {
		t.Error("wouldCycle = false, want true for guest under its own descendant")
	}
	if wouldCycle("creator", "guest", byCode) {
		t.Error("wouldCycle = true for a legitimate re-parent")
	}
}

func TestPermissionGroupSplitsOnTheResourceSegment(t *testing.T) {
	cases := map[string]string{
		"music:write:own": "music",
		"users:read:any":  "users",
		"*":               "*",
	}
	for code, want := range cases {
		if got := permissionGroup(code); got != want {
			t.Errorf("permissionGroup(%q) = %q, want %q", code, got, want)
		}
	}
}

func TestValidRoleCodeRejectsSeparatorsThatBreakTheGrammar(t *testing.T) {
	for _, bad := range []string{"", "Admin", "with space", "colon:code", strings.Repeat("a", 41)} {
		if validRoleCode(bad) {
			t.Errorf("validRoleCode(%q) = true, want false", bad)
		}
	}
	for _, ok := range []string{"admin", "content-editor", "team_lead", "l1"} {
		if !validRoleCode(ok) {
			t.Errorf("validRoleCode(%q) = false, want true", ok)
		}
	}
}

/* ── approval fan-out recipients ──────────────────────────────────── */

// approverIDs is what decides WHO learns that somebody is waiting. Getting it
// wrong is silent: registrations pile up and nobody is told.

func TestApproverIDsFindsAWildcardHolder(t *testing.T) {
	super := uuid.New()
	// A superadmin's whole effective set is the literal "*". A membership test
	// for "users:approve" would find nobody — which is exactly the person the
	// notification exists for.
	got := approverIDs([]PermissionHolder{{UserID: super, Code: "*"}}, PermApproveUser)

	if len(got) != 1 || got[0] != super {
		t.Fatalf("approverIDs = %v, want [%s] — the wildcard holder must match", got, super)
	}
}

func TestApproverIDsSkipsAUserWithoutThePermission(t *testing.T) {
	admin := uuid.New()
	// An `admin` holds every other users:* code but NOT users:approve — the whole
	// point of migration 0031 granting it to no role.
	holders := []PermissionHolder{
		{UserID: admin, Code: "users:read:any"},
		{UserID: admin, Code: "users:write:any"},
		{UserID: admin, Code: "users:delete:any"},
	}
	if got := approverIDs(holders, PermApproveUser); len(got) != 0 {
		t.Errorf("approverIDs = %v, want empty — admin must not be notified", got)
	}
}

func TestApproverIDsGroupsCodesPerUser(t *testing.T) {
	a, b := uuid.New(), uuid.New()
	holders := []PermissionHolder{
		{UserID: a, Code: "users:read:any"},
		{UserID: b, Code: "users:read:any"},
		{UserID: a, Code: "users:approve"}, // a's second row is what qualifies them
	}
	got := approverIDs(holders, PermApproveUser)
	if len(got) != 1 || got[0] != a {
		t.Fatalf("approverIDs = %v, want [%s]", got, a)
	}
}

func TestApproverIDsHonoursResourceAndActionWildcards(t *testing.T) {
	cases := map[string]bool{
		"users:*":           true,
		"*:approve":         true,
		"users:approve":     true,
		"users:approve:any": true,
		"users:approve:own": false, // :own never satisfies a bare requirement
		"users:read":        false,
		"roles:approve":     false,
	}
	for code, want := range cases {
		id := uuid.New()
		got := len(approverIDs([]PermissionHolder{{UserID: id, Code: code}}, PermApproveUser)) == 1
		if got != want {
			t.Errorf("grant %q matches users:approve = %v, want %v", code, got, want)
		}
	}
}

func TestApproverIDsIsStableInInputOrder(t *testing.T) {
	// Order decides who survives the fan-out cap, so it must not be map-random.
	first, second := uuid.New(), uuid.New()
	holders := []PermissionHolder{{UserID: first, Code: "*"}, {UserID: second, Code: "*"}}
	for i := 0; i < 20; i++ {
		got := approverIDs(holders, PermApproveUser)
		if len(got) != 2 || got[0] != first || got[1] != second {
			t.Fatalf("iteration %d: approverIDs = %v, want [%s %s]", i, got, first, second)
		}
	}
}

/* ── create / edit / delete ───────────────────────────────────────── */

// wildcardHolder makes the fake report one approver, so the last-approver guard
// is satisfied and the test under it is about something else.
func withApprover(f *fakeAdminStore) *fakeAdminStore {
	f.holders = append(f.holders, PermissionHolder{UserID: uuid.New(), Code: "*"})
	return f
}

// Creating an account must not be a way around the approval gate: a creator who
// cannot approve produces a PENDING row, exactly like a self-registration.
func TestCreateUserWithoutApprovePermissionLandsPending(t *testing.T) {
	store := newFakeStore()
	h := newHandler(store, []string{"users:write:any"})

	rec := call(t, h, http.MethodPost, "/users", "/users", uuid.New(),
		`{"email":"new@x.test","display_name":"New","password":"correct horse battery"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.created) != 1 {
		t.Fatalf("created %d accounts, want 1", len(store.created))
	}
	if got := store.created[0].ApprovalStatus; got != ApprovalPending {
		t.Errorf("approval_status = %q, want %q — creating must not bypass the gate", got, ApprovalPending)
	}
}

func TestCreateUserByAnApproverIsUsableImmediately(t *testing.T) {
	store := newFakeStore()
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodPost, "/users", "/users", uuid.New(),
		`{"email":"new@x.test","display_name":"New","password":"correct horse battery"}`)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rec.Code, rec.Body.String())
	}
	if got := store.created[0].ApprovalStatus; got != ApprovalApproved {
		t.Errorf("approval_status = %q, want %q", got, ApprovalApproved)
	}
}

func TestCreateUserRejectsAWeakPassword(t *testing.T) {
	store := newFakeStore()
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodPost, "/users", "/users", uuid.New(),
		`{"email":"new@x.test","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.created) != 0 {
		t.Error("an account was created despite the refusal")
	}
}

func TestCreateUserRejectsADuplicateEmail(t *testing.T) {
	store := newFakeStore()
	existing := uuid.New()
	store.users[existing] = AdminUser{ID: existing, Email: "taken@x.test"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodPost, "/users", "/users", uuid.New(),
		`{"email":"taken@x.test","password":"correct horse battery"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
}

// The confirmation is enforced server-side precisely so a misclick or a replayed
// request cannot reach a cascading DELETE.
func TestDeleteUserRequiresTheEmailConfirmation(t *testing.T) {
	store := withApprover(newFakeStore())
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "victim@x.test"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodDelete, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"confirm_email":"wrong@x.test"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.deleted) != 0 {
		t.Error("the account was deleted without a matching confirmation")
	}
}

func TestDeleteUserSucceedsWithTheRightConfirmation(t *testing.T) {
	store := withApprover(newFakeStore())
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "victim@x.test"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodDelete, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"confirm_email":"VICTIM@x.test"}`) // case-insensitive, like the login lookup

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.deleted) != 1 || store.deleted[0] != target {
		t.Errorf("deleted = %v, want [%s]", store.deleted, target)
	}
}

func TestDeleteUserRefusesYourself(t *testing.T) {
	store := withApprover(newFakeStore())
	actor := uuid.New()
	store.users[actor] = AdminUser{ID: actor, Email: "me@x.test"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodDelete, "/users/{id}", "/users/"+actor.String(), actor,
		`{"confirm_email":"me@x.test"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
}

// An install with zero approvers accepts signups nobody can ever let in. The
// fake reports the target as the ONLY holder, so removing them strands the queue.
func TestDeleteUserRefusesTheLastApprover(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "only@x.test"}
	store.holders = []PermissionHolder{{UserID: target, Code: "*"}}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodDelete, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"confirm_email":"only@x.test"}`)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", rec.Code, rec.Body.String())
	}
	if got := problemType(t, rec); got != "account/last-approver" {
		t.Errorf("problem type = %q, want account/last-approver", got)
	}
	if len(store.deleted) != 0 {
		t.Error("the last approver was deleted")
	}
}

// Deleting an account that outranks you is a takeover route, not administration.
func TestDeleteUserRefusesAMorePrivilegedTarget(t *testing.T) {
	store := withApprover(newFakeStore())
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "boss@x.test", RoleCodes: []string{"superadmin"}}
	h := newHandler(store, []string{"users:delete:any", "profile:read"})

	rec := call(t, h, http.MethodDelete, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"confirm_email":"boss@x.test"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.deleted) != 0 {
		t.Error("a superadmin was deleted by a non-wildcard actor")
	}
}

func TestUpdateUserRefusesAMorePrivilegedTarget(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "boss@x.test", RoleCodes: []string{"superadmin"}}
	h := newHandler(store, []string{"users:write:any"})

	rec := call(t, h, http.MethodPatch, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"email":"attacker@x.test"}`)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403; body=%s", rec.Code, rec.Body.String())
	}
	if store.users[target].Email != "boss@x.test" {
		t.Error("the email was changed — that is a takeover, not an edit")
	}
}

// A password set by an admin must end every session the OLD password issued —
// the token_version bump alone leaves the refresh cookie able to mint new ones.
func TestUpdateUserPasswordBumpsAndRevokes(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "u@x.test", DisplayName: "U"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodPatch, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"password":"correct horse battery"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if len(store.passwordSet) != 1 {
		t.Errorf("passwordSet = %v, want one call", store.passwordSet)
	}
	if len(store.bumped) != 1 || store.bumped[0] != target {
		t.Errorf("bumped = %v, want [%s]", store.bumped, target)
	}
	if len(store.sessionsRevoked) != 1 || store.sessionsRevoked[0] != target {
		t.Errorf("sessionsRevoked = %v, want [%s] — the refresh chain must burn too",
			store.sessionsRevoked, target)
	}
}

// A rejected password must not leave the email already changed.
func TestUpdateUserRejectsAWeakPasswordBeforeWritingTheProfile(t *testing.T) {
	store := newFakeStore()
	target := uuid.New()
	store.users[target] = AdminUser{ID: target, Email: "before@x.test", DisplayName: "U"}
	h := newHandler(store, []string{"*"})

	rec := call(t, h, http.MethodPatch, "/users/{id}", "/users/"+target.String(), uuid.New(),
		`{"email":"after@x.test","password":"short"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rec.Code, rec.Body.String())
	}
	if store.users[target].Email != "before@x.test" {
		t.Errorf("email = %q, want it untouched — a partial write is worse than a refusal",
			store.users[target].Email)
	}
}

// Renaming yourself locks nobody out, so unlike roles and approval it is allowed.
func TestUpdateUserAllowsEditingYourself(t *testing.T) {
	store := newFakeStore()
	actor := uuid.New()
	store.users[actor] = AdminUser{ID: actor, Email: "me@x.test", DisplayName: "Old"}
	h := newHandler(store, []string{"users:write:any"})

	rec := call(t, h, http.MethodPatch, "/users/{id}", "/users/"+actor.String(), actor,
		`{"display_name":"New Name"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}
	if store.users[actor].DisplayName != "New Name" {
		t.Errorf("display_name = %q, want %q", store.users[actor].DisplayName, "New Name")
	}
}
