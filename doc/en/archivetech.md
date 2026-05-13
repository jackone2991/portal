# ArchiveTech — System Functional Specification

> Canonical feature spec for the Portal ecosystem + multimedia platform.
> Derived from UI mocks in `template-main/portal/document/anh{1,2,3}.png`
> and from architectural decisions captured in [CLAUDE.md](../../CLAUDE.md).
>
> **Status legend** used throughout this document:
>
> - **[BUILT]**  — code is in `backend/` already
> - **[PARTIAL]** — partially built; schema or interfaces present but not wired
> - **[PLANNED]** — designed but no code yet
>
> When the spec conflicts with current code, the **spec wins** — adjust code,
> not the other way around. Update this doc as decisions evolve.

---

## 1. Vision

A self-hosted media platform with a **fine-grained, hierarchical access-control system** good enough for organizational deployments (clinics, studios, archives) — not just consumer accounts. Three pillars:

1. **Media domains** — Movies, Music, Stories. Upload → transcode → stream pipeline.
2. **Organizational access control** — User Groups, Users, User Roles, Policies, file-gated Permissions.
3. **Operational integrity** — full audit trail, instant revocation, OIDC SSO, no shared secrets in code.

The mocks show ArchiveTech as a **policy-driven, group-scoped** system, not a flat-role one. The data model below reflects that.

---

## 2. Core access-control model

The mocks introduce four entities that interact:

```text
                 ┌────────────┐
                 │ User Group │  hierarchical (parent_id), can be duplicated
                 └─────┬──────┘
              members  │  has policies
                ┌──────┴──────┐
                ▼             ▼
            ┌──────┐     ┌────────┐
            │ User │     │ Policy │  reusable permission bundle
            └──┬───┘     └───┬────┘
   has policies│             │ has permissions
               ▼             ▼
            ┌──────┐     ┌────────────┐
            │Policy│ ◄── │ Permission │  atomic; some are file-gated
            └──────┘     └────────────┘
```

### 2.1 Definitions

| Term | Definition | Cardinality |
|------|-----------|-------------|
| **User Group** | Organizational container. Can be parent of other groups (department → team → squad). Holds users, defines its own policy set. Source-of-truth for "who can act in this scope". | Hierarchical |
| **User** | Authenticated principal. Belongs to ≥1 User Group. May carry **per-user policies** that override or extend group policies. | Many-to-many with groups |
| **User Role** | A label inside a User Group (Manager, Junior, Reviewer). Currently presented as a group-scoped role. **Implementation note**: model as a Policy bundle attached to the user-in-group relation, not as an independent role table — this avoids the "global vs scoped role" ambiguity. | Group-scoped |
| **Policy** | A named, reusable set of Permissions ("Radiologist", "Read-Only Auditor"). Activated/deactivated independently of grants. Can be attached to either a User Group or a User. | 1 policy → many perms |
| **Permission** | The atomic action token: `<resource>:<action>[:<scope>]`. Some permissions are **file-gated**: they require an uploaded file (license, certificate, signed agreement) to become effective. | Smallest unit |

### 2.2 Effective-permission resolution

Compute a user's effective set in this order, then **deduplicate**:

1. Walk the user's User Group ancestry (group → parent → root).
2. For each group on the path, collect every **active** policy attached to it.
3. Add every **active** policy attached directly to the user.
4. For each policy, expand into its permissions, **filtering out file-gated permissions whose required file is missing or expired**.
5. Apply the wildcard / scope rules from [permission.go](../../backend/internal/rbac/permission.go).

Cached per `(user_id, token_version)` in Redis. Bumping `users.token_version` is the canonical invalidation channel.

### 2.3 Conflict & precedence

- **Deny is not currently in scope.** Policies grant only. If two paths would deny+allow the same permission, the allow wins. This keeps reasoning tractable; revisit if/when stricter compliance demands it.
- **Per-user policies are additive**, not overrides. If a user is in a group with `movies:read` and they have a personal policy with `movies:write:own`, they get both.
- **File-gated permissions disappear silently** when their file expires. The audit log records when a permission becomes ineffective.

---

## 3. Module catalog

Module tags map to the screens in `anh1/2/3.png`.

### 3.1 Module: User Group Management  *(anh1, anh3)*

| Feature | Status | Notes |
|---------|--------|-------|
| List user groups (grid view, search, paginated) | [PLANNED] | Top page in anh1 |
| Create user group (modal, parent group selector) | [PLANNED] | Modal in anh3; auto-set `parent_id` from current view |
| Open group profile (overview + members + policies) | [PLANNED] | Middle page in anh1 |
| Edit group description + metadata | [PLANNED] | |
| **Delete group** — cascades to child groups *(per anh1: "deleting child groups eradicated")* | [PLANNED] | Hard delete; warn and require typing the group code; audit event mandatory |
| **Duplicate group** | [PLANNED] | Deep-copy: new group under same parent + clone policy attachments + (option) clone members |
| Attach / detach Policy to group (search modal with preview) | [PLANNED] | Search-modal flow in anh3 |
| Show inherited policies (read-only badges from ancestor groups) | [PLANNED] | Crucial for understanding effective perms |

Schema delta (planned):

```sql
CREATE TABLE user_groups (
    id          UUID PRIMARY KEY,
    code        TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    parent_id   UUID REFERENCES user_groups(id) ON DELETE CASCADE,
    -- soft fields for the duplicate flow
    cloned_from UUID REFERENCES user_groups(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE user_group_members (
    group_id    UUID REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id)       ON DELETE CASCADE,
    role_label  TEXT,                  -- 'Manager', 'Junior', etc. (nullable)
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
```

### 3.2 Module: User Management  *(anh1, anh3)*

| Feature | Status | Notes |
|---------|--------|-------|
| Per-user profile page (almost a "super-page" of group profile) | [PLANNED] | Bottom page in anh1 |
| Inline policies-table for the user (grant/revoke per-user policies) | [PLANNED] | The exclusive table per anh1 |
| Create user inside a group (modal: name, email, profile type) | [PLANNED] | Create modal in anh3 |
| Move user between groups | [PLANNED] | Should bump `token_version` to force re-resolve |
| Disable / enable user | [BUILT] | `users.disabled_at` + `DisableUser`/`EnableUser` queries |
| Delete user (cascades to policies) | [PLANNED] | |
| List user's effective permissions (debug view) | [PLANNED] | Critical for support; show source policy chain |

Schema delta:

```sql
CREATE TABLE user_policy_attachments (
    user_id    UUID REFERENCES users(id)    ON DELETE CASCADE,
    policy_id  UUID REFERENCES policies(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID REFERENCES users(id)    ON DELETE SET NULL,
    expires_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, policy_id)
);
```

### 3.3 Module: Policy Management  *(anh2)*

The "Policies" tab is a flat catalog of reusable permission bundles.

| Feature | Status | Notes |
|---------|--------|-------|
| Policy list (cards with checkboxes preview) | [PLANNED] | anh2 top |
| Create policy (name + description + functionality blurb) | [PLANNED] | "+ CREATE NEW POLICY" |
| Activate / deactivate policy globally | [PLANNED] | Disabled policies are skipped during effective-perm resolution |
| Policy detail page (functionality + permission list) | [PLANNED] | anh2 middle |
| Add permission to policy | [PLANNED] | "+ ADD NEW PERMISSION" — inline row appears (anh2 bottom) |
| Remove permission from policy | [PLANNED] | |
| Delete policy (with cascade audit) | [PLANNED] | Refuse if attached anywhere; force user to detach first |
| Duplicate policy | [PLANNED] | Same shape as group duplicate |

Schema delta:

```sql
CREATE TABLE policies (
    id           UUID PRIMARY KEY,
    code         TEXT UNIQUE NOT NULL,
    name         TEXT NOT NULL,
    description  TEXT,
    functionality TEXT,                -- the long-form blurb in anh2
    is_active    BOOLEAN NOT NULL DEFAULT true,
    is_system    BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE policy_permissions (
    policy_id     UUID REFERENCES policies(id)    ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    -- file-gated permission: required uploaded file to enable
    requires_file BOOLEAN NOT NULL DEFAULT false,
    file_label    TEXT,                -- 'Radiologist license', 'NDA', etc.
    PRIMARY KEY (policy_id, permission_id)
);
CREATE TABLE group_policy_attachments (
    group_id   UUID REFERENCES user_groups(id) ON DELETE CASCADE,
    policy_id  UUID REFERENCES policies(id)    ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    granted_by UUID REFERENCES users(id)       ON DELETE SET NULL,
    PRIMARY KEY (group_id, policy_id)
);
```

### 3.4 Module: Permission Management *(anh2)*

The atomic units. Mostly seeded; rarely user-edited.

| Feature | Status | Notes |
|---------|--------|-------|
| Permission catalog (read-only for most users) | [BUILT] | Migration 0002 seeded 36 permissions |
| Create custom permission (admin-only) | [BUILT] | `CreatePermission` query exists |
| **File-gated permission infrastructure** | [PLANNED] | New: `user_permission_files` table tracks which user has uploaded which file for which permission |
| Verify uploaded file (manual review queue) | [PLANNED] | Admin must approve uploads before file-gated perm activates |
| File expiration & renewal reminders | [PLANNED] | Cron job; emit audit event when a perm becomes ineffective |

Schema delta for file-gated permissions:

```sql
CREATE TABLE user_permission_files (
    id            UUID PRIMARY KEY,
    user_id       UUID NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    file_key      TEXT NOT NULL,                -- S3 key
    uploaded_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    reviewed_at   TIMESTAMPTZ,
    reviewed_by   UUID REFERENCES users(id) ON DELETE SET NULL,
    status        TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | rejected | expired
    expires_at    TIMESTAMPTZ,
    note          TEXT
);
CREATE UNIQUE INDEX user_permission_files_active_idx
    ON user_permission_files (user_id, permission_id)
    WHERE status = 'approved';
```

### 3.5 Module: Authentication  *(no mock — backend-only)*

| Feature | Status | Notes |
|---------|--------|-------|
| OIDC login (Authentik) with state + nonce | [BUILT] | [oidc.go](../../backend/internal/auth/oidc.go) |
| Access token (HS256, rotating `kid`, 5 min) | [BUILT] | [jwt.go](../../backend/internal/auth/jwt.go) |
| Refresh token rotation + reuse detection | [BUILT] | [refresh.go](../../backend/internal/auth/refresh.go) |
| Logout / logout-all | [BUILT] | [auth.go handler](../../backend/internal/handler/auth.go) |
| Session list per user (devices + revoke) | [PARTIAL] | Query exists (`ListActiveRefreshTokensForUser`); UI [PLANNED] |
| Wire issuer + verifier + handler in `cmd/api/main.go` | [PLANNED] | Blocked on `make sqlc` |
| Repository adapters (UserUpserter, RefreshStore, etc.) | [PLANNED] | Wrap sqlc-generated code |

### 3.6 Module: Search & Discovery *(anh3)*

| Feature | Status | Notes |
|---------|--------|-------|
| Policy search modal with permission-set preview | [PLANNED] | anh3 bottom — search by name; modal previews perms before commit |
| User search (by email, by group) | [PLANNED] | |
| Audit log search (by actor, action, time range) | [PLANNED] | Query exists (`ListAuditEvents`) |
| Global search bar (header in every screenshot) | [PLANNED] | Cross-entity: groups, policies, users — TanStack Query + debounce |

### 3.7 Module: Audit & Compliance

| Feature | Status | Notes |
|---------|--------|-------|
| Append-only audit log table | [BUILT] | Migration 0002 |
| Audit logger best-effort writes | [BUILT] | [audit/logger.go](../../backend/internal/audit/logger.go) |
| Audit viewer UI (table + filters) | [PLANNED] | Restricted to `audit:read` |
| Export audit range (CSV/JSON) | [PLANNED] | Async job — large ranges shouldn't tie up the API |
| Retention policy & archival to cold storage | [PLANNED] | Cloudflare R2 archive bucket |

### 3.8 Module: Media domain (movies / music / stories)

Out of scope for the access-control redesign but listed for completeness; functionality from the original [README.md](README.md) (now removed) carries forward.

| Feature | Status | Notes |
|---------|--------|-------|
| Asset upload (S3 multipart with presigned URLs) | [PARTIAL] | OpenAPI defined; handler [PLANNED] |
| Transcode worker (FFmpeg → HLS) | [PARTIAL] | Stub in [transcode.go](../../backend/internal/worker/transcode.go) |
| Thumbnail worker | [PARTIAL] | Stub in [thumbnail.go](../../backend/internal/worker/thumbnail.go) |
| Movies / Music / Stories CRUD | [PLANNED] | Domain packages under `internal/domain/` |
| Vidstack player integration on the frontend | [PLANNED] | |
| Comments, ratings, watchlist | [PLANNED] | Permission-gated via `comments:write` / `comments:delete:*` |
| Search across content (Postgres FTS → Meilisearch) | [PLANNED] | |

---

## 4. UI page inventory

Mapping screenshots to React Server Components / Pages on the frontend.

| Page | Mock | Path (planned) |
|------|------|----------------|
| User Group list | anh1 top | `app/admin/groups/page.tsx` |
| User Group profile | anh1 middle | `app/admin/groups/[id]/page.tsx` |
| User Profile (admin view) | anh1 bottom | `app/admin/users/[id]/page.tsx` |
| Create User Group modal | anh3 | `components/admin/CreateGroupDialog.tsx` |
| Create User Profile modal | anh3 | `components/admin/CreateUserDialog.tsx` |
| Policy list | anh2 top | `app/admin/policies/page.tsx` |
| Policy detail | anh2 middle | `app/admin/policies/[id]/page.tsx` |
| Add permission inline row | anh2 bottom | inline state on policy detail |
| Policy search modal | anh3 bottom | `components/admin/PolicySearchDialog.tsx` |
| Audit log viewer | (no mock) | `app/admin/audit/page.tsx` |
| Session/device manager | (no mock) | `app/account/sessions/page.tsx` |

All admin routes go through `RequirePermission` middleware on the backend; the frontend additionally hides UI affordances for which the user lacks the corresponding permission code. The server is always authoritative.

---

## 5. API surface delta

Beyond the auth endpoints already in [shared/openapi.yaml](../../shared/openapi.yaml):

```text
POST   /admin/groups                    create
GET    /admin/groups                    list
GET    /admin/groups/{id}               profile (members, attached policies, inherited policies)
PATCH  /admin/groups/{id}               edit
DELETE /admin/groups/{id}               delete (cascade)
POST   /admin/groups/{id}/duplicate     deep-copy
POST   /admin/groups/{id}/members       add user
DELETE /admin/groups/{id}/members/{u}   remove user
POST   /admin/groups/{id}/policies      attach policy
DELETE /admin/groups/{id}/policies/{p}  detach policy

POST   /admin/policies                  create
GET    /admin/policies                  list
GET    /admin/policies/{id}             detail
PATCH  /admin/policies/{id}             edit (incl. activate/deactivate)
DELETE /admin/policies/{id}             delete (refuse if attached)
POST   /admin/policies/{id}/duplicate
POST   /admin/policies/{id}/permissions          add permission
DELETE /admin/policies/{id}/permissions/{p}      remove permission

POST   /admin/users/{id}/policies                attach personal policy
DELETE /admin/users/{id}/policies/{p}            detach
GET    /admin/users/{id}/effective-permissions   debug view (shows source chain)

POST   /me/permission-files             upload file for a file-gated perm
GET    /me/permission-files             list own files + status
GET    /admin/permission-files/pending  review queue (admin)
POST   /admin/permission-files/{id}/review   approve/reject

GET    /admin/audit                     paginated, filterable
GET    /admin/audit/export              async export job
```

Permission requirements per endpoint live in the OpenAPI `x-required-permission` extension (to be added) and enforced by `RequirePermission` middleware.

---

## 6. Data model: deltas vs. current schema

The four migrations needed on top of `0002_rbac`:

| Migration | Purpose |
|-----------|---------|
| `0003_user_groups.up.sql` | `user_groups`, `user_group_members`. Bump `token_version` for affected users on group changes. |
| `0004_policies.up.sql` | `policies`, `policy_permissions`, `group_policy_attachments`, `user_policy_attachments`. |
| `0005_file_gated_permissions.up.sql` | `user_permission_files` + review workflow columns. |
| `0006_effective_permissions_view.up.sql` | Materialized view (or function) computing effective perms per user with file-gating applied. Refreshed on grant/revoke or via trigger. |

The existing `roles` table is kept for **system-level coarse roles** (admin, superadmin) — it stays useful for "who can administer this whole system" decisions. The Policy model layers on top for fine-grained, group-scoped grants.

---

## 7. Phasing roadmap

Ordered by least-blocking and most-leverage:

### Phase 0 — Wire what's built  *(no new features)*

- `make sqlc` generates `internal/repository/`.
- Adapters for `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`.
- `cmd/api/main.go` constructs `Issuer`, `Verifier`, `RefreshManager`, `rbac.Engine`, mounts `/auth/*`.
- Dev seed script: 1 superadmin user, 1 default group.
- **Exit criteria**: end-to-end OIDC login works against a local Authentik.

### Phase 1 — Group + Policy data plane

- Migrations 0003 + 0004.
- Update RBAC engine: effective-permission query joins user → groups (recursive) → policies → permissions.
- Cache key still namespaced by `token_version`.
- API endpoints in section 5 (groups + policies + attachments).
- **Exit criteria**: a user in group "Radiologists" inherits the group's "Radiologist" policy and the engine reports the correct effective set.

### Phase 2 — Admin UI

- Frontend pages from section 4.
- Search modal + global search bar.
- Effective-permissions debug view (section 5: `GET /admin/users/{id}/effective-permissions`).
- **Exit criteria**: admin can replicate every screen in `anh1/2/3.png`.

### Phase 3 — File-gated permissions

- Migration 0005.
- Upload endpoint via S3 presigned URL.
- Review queue UI (admin) + email/notify on submission.
- Cron: file expiration → emit audit + invalidate cache.
- **Exit criteria**: a permission requiring an uploaded license becomes effective after admin review and disappears at expiry.

### Phase 4 — Audit & compliance

- Audit viewer + filters.
- Export to R2 archive bucket (async).
- Retention policy.
- **Exit criteria**: full audit trail can be replayed for any user/group action over the last N days.

### Phase 5 — Media domain (parallel-ready)

Separate track that doesn't block 0–4. See `internal/domain/{movie,music,story}/` packages and the upload + transcode pipeline already stubbed. Permission codes already seeded for it.

---

## 8. Non-goals (for now)

State these explicitly so future PRs don't drift:

- **Deny rules.** Not implemented. If a real compliance need appears, model as `policy_permissions.effect` enum rather than retrofitting onto the matcher.
- **Time-bounded grants beyond `expires_at`.** No business-hours / geo / device fences.
- **Federated multi-tenant.** All groups live in one DB. Splitting tenants per DB is a Phase-N exercise.
- **Self-service password reset.** Authentik owns this; Portal never sees passwords.
- **Mobile native app.** PWA via Next.js only.

---

## 9. Open questions

These need product input, not code:

1. **Group-deletion confirmation UX.** Type-the-code like GitHub, or 2-step modal? Mocks don't show.
2. **Role-label vs Policy attached-to-membership.** The mocks call out "User Roles" inside a group (Manager, Junior). Are those purely cosmetic labels or do they grant permissions? Decision proposed: cosmetic label *only*; permissions come from policies attached to the group + user. Confirm.
3. **Per-user policies — additive only, or override?** Spec defaults to additive. Confirm or escalate.
4. **File-gated permission expiry behaviour.** Hard cut-off, or grace period? Default: hard cut-off + audit.
5. **Policy versioning.** When a policy changes mid-flight, do users on an old session see the old set until next refresh, or instant? Spec defaults to "instant via cache invalidation"; confirm.
