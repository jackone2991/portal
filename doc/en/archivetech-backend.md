# ArchiveTech — Backend Architecture for Multi-Tenancy

> Engineering patterns for running Portal's Go backend in a multi-tenant
> setting. The **security model** lives in [authoration.md](authoration.md);
> this doc is about the **code architecture** that implements it safely
> across HTTP requests, background jobs, storage, cache, and observability.
>
> Read after [authoration.md](authoration.md) §3 (the tenant layer). This
> doc assumes you've internalized: PostgreSQL RLS as the enforcement floor,
> `app.current_tenant` as the per-request DB session variable, organizations
> as the data-isolation boundary.

---

## 1. The single rule

> **Every piece of code that touches tenant data must derive the tenant
> from `context.Context`. Never from a function parameter, never from a
> struct field, never from a global.**

Why one rule, that strict: the moment a tenant ID can come from two places, code drifts and a bug eventually picks the wrong one. Single source = no ambiguity, easy to audit. Stack diagnostics like "show me everything that ran for tenant X yesterday" reduce to one tracing field.

Corollary: a function signature like `ListMovies(ctx, orgID)` is **a smell**. The right shape is `ListMovies(ctx)`, with `orgID` resolved internally via `tenant.OrgFromContext(ctx)`.

---

## 2. Layered tenant flow

```text
   ┌──────────────────────────────────────────────────┐
   │ HTTP request                                     │
   │   • RequireAuth → Identity (user, token_version) │
   │   • RequireTenant → tenant.Context (org_id)      │
   │     ↳ opens DB tx, SET LOCAL app.current_tenant  │
   │     ↳ binds tenant.Context to ctx                │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Handler  ← receives ctx with auth + tenant       │
   │   service.Movies.Create(ctx, input)              │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Service  ← business logic, no DB calls           │
   │   repo := r.WithTx(ctx)   ← tx already open      │
   │   repo.Movies.Insert(ctx, ...)                   │
   │   r.Audit.Write(ctx, ...)                        │
   │   r.Storage.Put(ctx, key, ...)                   │
   │   r.Jobs.Enqueue(ctx, asynq.NewTranscodeTask())  │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Repo (sqlc) ← runs against the request's tx;     │
   │   RLS filters every row by app.current_tenant    │
   └──────────────────────────────────────────────────┘

   Worker process — Asynq consumer — symmetric:
   ┌──────────────────────────────────────────────────┐
   │ Task delivered with org_id in payload            │
   │   • TenantTaskMiddleware → opens tx, SET LOCAL   │
   │     app.current_tenant, binds tenant.Context     │
   │   • Task handler runs, identical service calls   │
   └──────────────────────────────────────────────────┘
```

Symmetry between API and worker is deliberate: the same service code runs in both, with the same context shape. The middleware is the only thing that differs.

---

## 3. Tenant context package

```go
// internal/tenant/context.go
package tenant

import (
    "context"
    "fmt"

    "github.com/google/uuid"
)

type Context struct {
    OrgID  uuid.UUID
    OrgCode string  // human-readable; useful in logs
    // Add tier, feature flags, etc. here as the model grows.
    // Keep this struct read-only.
}

type ctxKey struct{}

func With(ctx context.Context, t Context) context.Context {
    return context.WithValue(ctx, ctxKey{}, t)
}

func From(ctx context.Context) (Context, bool) {
    t, ok := ctx.Value(ctxKey{}).(Context)
    return t, ok
}

// Must is for code paths after RequireTenant has run. Panics on miss —
// surfaces a routing bug fast.
func Must(ctx context.Context) Context {
    t, ok := From(ctx)
    if !ok {
        panic("tenant: context missing — wire RequireTenant middleware")
    }
    return t
}

// OrgID is the common-case shortcut.
func OrgID(ctx context.Context) uuid.UUID { return Must(ctx).OrgID }

// MustEqual asserts that two pieces of code agree on the active tenant.
// Use defensively when an input carries an org_id that should match the
// session — e.g., a URL parameter or a job payload. Panic on mismatch is
// safer than silent acceptance.
func MustEqual(ctx context.Context, candidate uuid.UUID) {
    if t := Must(ctx); t.OrgID != candidate {
        panic(fmt.Sprintf("tenant mismatch: ctx=%s arg=%s", t.OrgID, candidate))
    }
}
```

Two helpers (`MustEqual`, `Must`) both panic on misuse. That's intentional — these are programming errors, not runtime errors. A panic at request boundary surfaces in logs and in CI sooner than a silent wrong-tenant write.

---

## 4. Database connection strategy

### 4.1 PgBouncer + RLS interplay

Portal pools through PgBouncer in **transaction pooling mode**. RLS uses a Postgres session variable, which PgBouncer would mishandle if set with plain `SET ...` — but `SET LOCAL` and `set_config(..., true)` are transaction-scoped, so they release with the connection.

The hard rule: **every tenant-scoped request runs inside a single Postgres transaction**, even read-only ones. The tenant guard is set at transaction start.

```go
// internal/repository/db.go
type DB struct {
    pool *pgxpool.Pool
}

// BeginTenantScope starts a tx and pins app.current_tenant for its lifetime.
// Returns a cleanup func that commits-or-rolls-back depending on the error
// stored in *errPtr (Go's classic deferred-tx pattern).
func (d *DB) BeginTenantScope(ctx context.Context, orgID uuid.UUID) (
    pgx.Tx, func(*error), error,
) {
    tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.ReadCommitted})
    if err != nil {
        return nil, nil, err
    }
    if _, err := tx.Exec(ctx,
        "SELECT set_config('app.current_tenant', $1, true)", orgID.String(),
    ); err != nil {
        _ = tx.Rollback(ctx)
        return nil, nil, err
    }
    cleanup := func(errPtr *error) {
        if errPtr != nil && *errPtr != nil {
            _ = tx.Rollback(ctx)
            return
        }
        if cerr := tx.Commit(ctx); cerr != nil && errPtr != nil && *errPtr == nil {
            *errPtr = cerr
        }
    }
    return tx, cleanup, nil
}
```

Middleware uses this; service code uses the resulting `pgx.Tx` indirectly via the `Querier` interface that sqlc generates.

### 4.2 The Querier indirection

sqlc emits per-table `*Queries` types and a `Querier` interface. Portal's repo layer adds one helper:

```go
// internal/repository/repo.go
type Repo struct {
    db *DB
    // Per-domain queriers. They all accept a DBTX (pool, tx, or conn).
    // We always pass the request-scoped tx.
    Movies   *movies.Queries
    Music    *music.Queries
    Stories  *stories.Queries
    Assets   *assets.Queries
    RBAC     *rbac.Queries
    Auth     *auth.Queries
}

// WithTx returns a Repo bound to the request's transaction. Service code
// gets one of these and never sees the *pgxpool.Pool directly.
func (r *Repo) WithTx(tx pgx.Tx) *Repo {
    return &Repo{
        db:      r.db,
        Movies:  movies.New(tx),
        Music:   music.New(tx),
        Stories: stories.New(tx),
        Assets:  assets.New(tx),
        RBAC:    rbac.New(tx),
        Auth:    auth.New(tx),
    }
}
```

The handler does **not** call `WithTx` directly — the tenant middleware put the tx in context. The service pulls it out:

```go
// internal/middleware/tenant.go
type txKey struct{}

func WithTx(ctx context.Context, tx pgx.Tx) context.Context {
    return context.WithValue(ctx, txKey{}, tx)
}
func TxFrom(ctx context.Context) (pgx.Tx, bool) {
    tx, ok := ctx.Value(txKey{}).(pgx.Tx)
    return tx, ok
}

// internal/service/movies/service.go
func (s *Service) Create(ctx context.Context, in CreateInput) (*Movie, error) {
    tx, ok := middleware.TxFrom(ctx)
    if !ok {
        return nil, errors.New("movies: missing tx — wire RequireTenant")
    }
    repo := s.repo.WithTx(tx)
    return repo.Movies.Insert(ctx, ...)
}
```

This shape keeps the service's contract clean while ensuring every DB call lands on the request-scoped transaction.

### 4.3 The system role for cross-tenant work

A **separate** `pgxpool.Pool` connects as `portal_system` (with `BYPASSRLS`). It is wired only into `cmd/sysjobs/` — the API and worker binaries do not import it. This is enforced by package layout:

```
backend/
  cmd/api/main.go      ← imports internal/repository (request-scoped pool)
  cmd/worker/main.go   ← imports internal/repository
  cmd/sysjobs/main.go  ← imports internal/repository AND internal/sysrepository
                          (the BYPASSRLS pool — the only place it's referenced)
```

Add a `go vet`-style check (or `golangci-lint forbidigo`) that forbids importing `internal/sysrepository` from any package outside `cmd/sysjobs/`.

---

## 5. Service layer conventions

### 5.1 Service struct shape

Each domain has one `Service` struct. Dependencies (repo, audit, storage, jobs) are injected once at construction. Methods take `(ctx, input)` and never receive raw IDs that override context.

```go
// internal/service/groups/service.go
type Service struct {
    repo    *repository.Repo
    audit   *audit.Logger
    jobs    *jobs.Client
    cache   *rbac.CachedLoader
}
```

Methods follow a consistent pattern:

1. Pull tenant + identity from ctx.
2. Authorize (the engine call). 
3. Business logic.
4. Audit.
5. Cache invalidation if applicable.

Anti-pattern: a service method that accepts an `orgID` parameter "for cross-tenant operations". If you genuinely need cross-tenant, that lives in `cmd/sysjobs/`, never in the API service layer.

### 5.2 Tenant-aware errors

Errors returned to the handler must not leak data from another tenant. A "not found" for one tenant must look identical to "exists but in another tenant" — both 404.

```go
// internal/service/movies/errors.go
var (
    ErrNotFound      = errors.New("movies: not found")              // 404
    ErrAlreadyExists = errors.New("movies: already exists")         // 409
    ErrFileTooLarge  = errors.New("movies: file exceeds quota")     // 413
)
```

The repository ensures lookup by ID is RLS-scoped; the service translates pgx's `ErrNoRows` to `ErrNotFound`. There is no "cross-tenant exists" path.

---

## 6. Background jobs (Asynq)

### 6.1 Tenant in every payload

Every task payload carries the organization ID. No exceptions.

```go
// internal/worker/transcode.go (updated for multi-tenant)
type TranscodePayload struct {
    OrganizationID uuid.UUID `json:"organization_id"`  // REQUIRED
    AssetID        uuid.UUID `json:"asset_id"`
    SourceKey      string    `json:"source_key"`
    OutputKey      string    `json:"output_key"`
    Variants       []string  `json:"variants,omitempty"`
}

func NewTranscodeTask(ctx context.Context, p TranscodePayload) (*asynq.Task, error) {
    if p.OrganizationID == uuid.Nil {
        p.OrganizationID = tenant.OrgID(ctx)  // safe default — same tenant
    }
    body, err := json.Marshal(p)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TaskTypeTranscode, body, asynq.Queue("transcode")), nil
}
```

`NewTranscodeTask(ctx, p)` reads the active tenant from ctx if not explicitly set. `Enqueue` callers cannot accidentally drop the tenant — the constructor refuses (`uuid.Nil` post-fill is a programmer bug, panic acceptable).

### 6.2 Worker-side middleware

```go
// internal/jobs/tenant.go
func TenantMiddleware(db *repository.DB, mems MembershipFetcher) asynq.MiddlewareFunc {
    return func(next asynq.Handler) asynq.Handler {
        return asynq.HandlerFunc(func(ctx context.Context, t *asynq.Task) error {
            var hdr struct{ OrganizationID uuid.UUID `json:"organization_id"` }
            if err := json.Unmarshal(t.Payload(), &hdr); err != nil {
                return fmt.Errorf("job: malformed payload: %w", err)
            }
            if hdr.OrganizationID == uuid.Nil {
                return fmt.Errorf("job: missing organization_id")
            }
            tx, cleanup, err := db.BeginTenantScope(ctx, hdr.OrganizationID)
            if err != nil {
                return err
            }
            var rerr error
            defer cleanup(&rerr)

            ctx = middleware.WithTx(ctx, tx)
            ctx = tenant.With(ctx, tenant.Context{OrgID: hdr.OrganizationID})
            rerr = next.ProcessTask(ctx, t)
            return rerr
        })
    }
}
```

Now the same `service.Movies.Update(ctx, ...)` works identically when called from an HTTP handler or from a job — both have a tenant-scoped tx in context.

### 6.3 Per-queue or per-tenant scheduling?

Default: **per-queue priority**, not per-tenant. The transcode queue carries jobs from all tenants weighted equally, with priority by job kind (transcode 5, thumbnail 3, default 1).

For enterprise-tier tenants who need SLA isolation (no noisy-neighbor): provision a dedicated worker pool that consumes only from `transcode-tenant-<orgID>`. The `Enqueue` helper picks the right queue based on org tier:

```go
queue := "transcode"
if tier == TierEnterprise {
    queue = fmt.Sprintf("transcode-tenant-%s", orgID)
}
asynq.Queue(queue)
```

Don't do this from day one. Add the queue routing only when a paying customer triggers the requirement.

---

## 7. Object storage (S3 / MinIO / R2)

### 7.1 Key partitioning

Every key starts with `org/<org_id>/`. There are no exceptions, including system uploads (which use `org/system/...`).

```go
// internal/storage/keys.go
func AssetSourceKey(orgID, assetID uuid.UUID, ext string) string {
    return fmt.Sprintf("org/%s/assets/source/%s.%s", orgID, assetID, ext)
}
func AssetHLSPrefix(orgID, assetID uuid.UUID) string {
    return fmt.Sprintf("org/%s/assets/hls/%s/", orgID, assetID)
}
```

Benefits:

- Bucket browsing is naturally segmented (helpful in support).
- Backup boundaries align with prefixes; restoring one tenant is `aws s3 sync s3://portal/org/<id>/ ...`.
- Per-tenant lifecycle rules become trivial (e.g., "delete tenant X's expired uploads").
- Future migration to bucket-per-tenant is a rename, not a re-architect.

### 7.2 Storage client wrapper

The S3 client wrapper takes ctx and prefixes automatically:

```go
// internal/storage/storage.go
func (s *Storage) Put(ctx context.Context, key string, body io.Reader) error {
    full := s.scopeKey(ctx, key)
    _, err := s.s3.PutObject(ctx, &s3.PutObjectInput{
        Bucket: &s.bucket,
        Key:    &full,
        Body:   body,
    })
    return err
}

func (s *Storage) scopeKey(ctx context.Context, key string) string {
    org := tenant.OrgID(ctx)
    if strings.HasPrefix(key, "org/") {
        // Caller already namespaced; verify it matches the active tenant.
        prefix := fmt.Sprintf("org/%s/", org)
        if !strings.HasPrefix(key, prefix) {
            panic(fmt.Sprintf("storage: key %q crosses tenant %s", key, org))
        }
        return key
    }
    return fmt.Sprintf("org/%s/%s", org, key)
}
```

The panic on mismatch is the same defensive posture as `tenant.MustEqual` — trip the trap loudly on misuse.

### 7.3 Presigned URLs

Presigned URLs sign exactly one key, scoped to one tenant. Generate them through the same wrapper so the prefix invariant holds.

For uploads from the browser via S3 multipart, the API generates per-part presigned URLs **only** for the active tenant's prefix. A user can never receive a presigned URL for another tenant's key, even by guessing.

### 7.4 Bucket strategy

| Tier | Strategy |
|------|----------|
| Standard tenants | Single bucket `portal-media`, prefixed `org/<id>/` |
| Enterprise tenants | Dedicated bucket `portal-media-<id>` with separate IAM credentials (deferred — enable when first enterprise contract closes) |

Audit log archive is its own bucket (`portal-audit-archive`) with **immutable bucket policy** (object-lock retention). Compromised app credentials cannot delete logs.

---

## 8. Caching (Redis / DragonflyDB)

### 8.1 Key naming convention

```text
<resource>:<org_id>:<rest>
```

Always, no exceptions. Examples:

- `rbac:perms:<org_id>:<user_id>:v<token_version>`
- `policy:detail:<org_id>:<policy_id>`
- `movie:list:<org_id>:page<n>`
- `session:<user_id>` ← global (user identity, not tenant)

The `<org_id>` segment makes per-tenant invalidation a `SCAN MATCH <resource>:<org_id>:*` away. Without it, "evict everything for tenant X" is impossible without a full flush.

### 8.2 Cache-aside helpers

```go
// internal/cache/keys.go
func TenantKey(ctx context.Context, parts ...string) string {
    org := tenant.OrgID(ctx)
    return fmt.Sprintf("%s:%s:%s", parts[0], org, strings.Join(parts[1:], ":"))
}
```

Builders that don't accept `ctx` are not allowed. Linter rule (golangci-lint `forbidigo`):

```yaml
forbidigo:
  forbid:
    - p: 'redis\.NewClient\(\)\.Set\('
      msg: "use cache.SetTenant; never call redis client directly with raw keys"
```

### 8.3 Cross-tenant caches

The only legitimate global cache is identity (refresh tokens by hash, OIDC provider metadata, user profile by id). These keys explicitly omit `<org_id>` and undergo a code-review checklist: "Is this truly tenant-independent?"

---

## 9. Search

### 9.1 Default: Postgres FTS

Tenant-scoped `tsvector` columns + RLS = isolation for free. No extra configuration.

```sql
-- in 0004_user_groups.up.sql or wherever movies live
ALTER TABLE movies
    ADD COLUMN search_tsv tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(unaccent(title), '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(unaccent(description), '')), 'B')
    ) STORED;

CREATE INDEX movies_search_tsv_idx ON movies USING GIN (search_tsv);
```

### 9.2 Migration to Meilisearch

When/if FTS isn't enough:

- One Meilisearch index **per tenant** — `movies-<org_id>`. Cleanest isolation; Meilisearch supports it natively.
- Reindex on policy change is per-tenant work; doesn't touch other tenants.
- Search keys (Meilisearch API keys) are per-tenant, generated at tenant onboarding, stored encrypted.

Don't do this on day one. Postgres FTS is fine until search relevance materially trails competitors.

---

## 10. Per-tenant configuration

Some tenants need different limits, feature flags, integrations.

```sql
CREATE TABLE organization_settings (
    organization_id     UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    max_storage_bytes   BIGINT,                       -- NULL = unlimited
    max_users           INTEGER,                      -- NULL = unlimited
    max_uploads_per_min INTEGER NOT NULL DEFAULT 30,
    feature_flags       JSONB NOT NULL DEFAULT '{}'::jsonb,
    custom_branding     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Loaded once per request inside `RequireTenant` middleware (cached for ~1 min). Feature flags read via:

```go
if tenant.HasFeature(ctx, "live_streaming") {
    ...
}
```

Rate limits enforced in middleware on top of per-IP limits:

```go
// internal/middleware/tenant_ratelimit.go
// Token bucket per (org_id, action). Limit comes from organization_settings.
```

---

## 11. Tenant lifecycle

### 11.1 Onboarding

`POST /admin/organizations` (system-superadmin only) runs in a single transaction:

1. Insert into `organizations`.
2. Create default User Group (`<org_code>-default`) under the org.
3. Attach the seed `Owner` policy to the group (admin perms scoped to the org).
4. Create `organization_memberships` row for the inviting user as owner.
5. Audit `tenant.organization.created`.

### 11.2 Suspension

Toggle `organizations.is_active = false`. The `RequireTenant` middleware refuses requests for inactive orgs (returns 503 `tenant_suspended`). Existing sessions are NOT auto-revoked — when the user retries with their token, the middleware kicks them out. This is gentler than mass logout.

### 11.3 Hard deletion

Two-phase:

1. **Soft delete** (`deleted_at = now()`). All API access blocked. Data remains for 30 days. Restorable with a single `UPDATE`.
2. **Hard delete** (Asynq cron job after 30 days):
   - Delete S3 objects under `org/<id>/`.
   - DELETE from every tenant-scoped table (RLS-bypass — `cmd/sysjobs/`).
   - Verify zero rows remain referencing the org_id.
   - Insert a final `tenant.organization.purged` event into the audit archive bucket (the audit table itself is gone).
   - Email confirmation to org's last known billing contact.

### 11.4 Export

`POST /admin/organizations/{id}/export` (org admin OR system-superadmin) enqueues a job that:

- Streams every tenant-scoped table to a single ND-JSON file per table.
- Copies S3 objects under `org/<id>/` to a one-time-use export bucket.
- Generates a signed download URL (valid 7 days).
- Notifies the requesting admin when ready.

This is what GDPR / customer churn / due diligence asks for. Build it in Phase 1; cheaper than retrofitting.

---

## 12. Observability

### 12.1 Logging

Every log line has `org_id` and `user_id` fields. The logger middleware reads ctx and decorates:

```go
// internal/middleware/logger.go
zerolog.Ctx(ctx).With().
    Str("org_id", t.OrgID.String()).
    Str("user_id", id.UserID.String()).
    Str("request_id", requestID).
    Logger()
```

Background jobs do the same in `TenantMiddleware`.

### 12.2 Metrics

Prometheus labels with `org_id` are tempting but **explode cardinality**. Two-tier strategy:

- Top-K metrics: bucket large tenants individually, all others into `org_id="other"`. Refresh K daily.
- Per-tenant detail: when needed, query the audit log + DB counters, not Prometheus.

```go
// metrics/tenant.go — bucket cardinality
func TenantLabel(orgID uuid.UUID) string {
    if topK.Contains(orgID) {
        return orgID.String()
    }
    return "other"
}
```

### 12.3 Tracing

OpenTelemetry spans always set `tenant.id` and `tenant.code` as attributes. Cross-service traces (API → worker via Asynq) propagate the trace context in the task payload alongside `organization_id`.

```go
type TaskHeader struct {
    OrganizationID uuid.UUID `json:"organization_id"`
    TraceID        string    `json:"trace_id,omitempty"`
    SpanID         string    `json:"span_id,omitempty"`
}
```

---

## 13. Testing

### 13.1 RLS test discipline

Every PR touching a tenant-scoped table includes a parametric test:

```go
// internal/repository/rls_test.go
func TestRLS_Movies_Isolation(t *testing.T) {
    db := testdb.Fresh(t)
    orgA := testdb.NewOrg(t, db, "alpha")
    orgB := testdb.NewOrg(t, db, "beta")

    asA := testdb.AsTenant(orgA)(db)
    asB := testdb.AsTenant(orgB)(db)

    asA.MustInsertMovie(t, "Movie One")
    asB.MustInsertMovie(t, "Movie Two")

    // A sees only A's row
    rows := asA.MustListMovies(t)
    require.Len(t, rows, 1)
    require.Equal(t, "Movie One", rows[0].Title)

    // B sees only B's row
    rows = asB.MustListMovies(t)
    require.Len(t, rows, 1)
    require.Equal(t, "Movie Two", rows[0].Title)

    // BYPASSRLS sees both
    asSys := testdb.AsSystem(db)
    rows = asSys.MustListMovies(t)
    require.Len(t, rows, 2)
}
```

If this test does not exist for a new tenant-scoped table, CI rejects the PR.

### 13.2 Tenant fixtures

```go
// internal/testdb/fixtures.go
type Tenant struct {
    OrgID    uuid.UUID
    OwnerID  uuid.UUID
    GroupID  uuid.UUID
}

// NewOrg creates a fresh tenant + default group + owner user. Returns
// a fully-formed Tenant for use in service-level tests.
func NewOrg(t *testing.T, db *DB, code string) *Tenant { ... }

// AsTenant returns a function that opens a tx scoped to that tenant.
// Use as: client := testdb.AsTenant(tenant)(db)
```

### 13.3 Parallel tests are safe

RLS isolation between tenants means parallel tests (`t.Parallel()`) running in different orgs cannot interfere — even on a shared DB. This is an underrated benefit of RLS for testing.

### 13.4 The "missing tenant" test

Every endpoint should have one test asserting it rejects requests with no tenant context:

```go
func TestMoviesCreate_RequiresTenant(t *testing.T) {
    h := newHandler(t /* no RequireTenant middleware */)
    req := httptest.NewRequest("POST", "/movies", body)
    rr := httptest.NewRecorder()
    h.ServeHTTP(rr, req)
    require.Equal(t, http.StatusBadRequest, rr.Code)
}
```

This catches the worst-class bug: an endpoint mounted without `RequireTenant`.

---

## 14. Code organization

```text
backend/
├── cmd/
│   ├── api/main.go          ← HTTP, RLS pool only
│   ├── worker/main.go       ← Asynq, RLS pool only
│   └── sysjobs/main.go      ← BYPASSRLS pool — cross-tenant batch
│
├── internal/
│   ├── auth/                ← identity (JWT, OIDC, refresh, TOTP)
│   ├── tenant/              ← tenant.Context, MustEqual, helpers
│   ├── rbac/                ← engine, permission, role, cache
│   ├── middleware/          ← RequireAuth, RequireTenant, RequireStepUp,
│   │                          RequirePermission, ratelimit, logger
│   ├── repository/          ← sqlc-generated; one Querier per domain
│   ├── sysrepository/       ← BYPASSRLS — imported only from cmd/sysjobs
│   ├── service/
│   │   ├── movies/          ← business logic, tenant-aware
│   │   ├── music/
│   │   ├── stories/
│   │   ├── groups/
│   │   ├── policies/
│   │   └── auth/
│   ├── storage/             ← S3 client with tenant-prefixed keys
│   ├── jobs/                ← Asynq client + middleware (TenantMiddleware)
│   ├── worker/              ← task handlers (transcode, thumbnail, notify)
│   ├── audit/               ← audit logger
│   ├── notifications/       ← in-app + Web Push
│   ├── cache/               ← Redis wrapper with TenantKey helper
│   └── handler/             ← HTTP handlers; thin — delegate to service
│
├── db/
│   ├── migrations/
│   └── queries/
└── go.mod
```

### Allowed-import rules

| Package | May import | Must NOT import |
|---------|------------|-----------------|
| `internal/sysrepository` | `internal/repository` types only | anything from `service`, `handler`, `middleware` |
| `internal/handler` | `service`, `middleware`, `auth`, `tenant`, `audit` | `repository` directly |
| `internal/service/*` | `repository`, `cache`, `storage`, `jobs`, `audit`, `tenant` | `sysrepository`, `handler`, `middleware` |
| `internal/repository` | sqlc-generated only | `service`, `tenant` |
| `cmd/api` | `service`, `handler`, `middleware`, `repository`, `auth`, `tenant` | `sysrepository` |
| `cmd/sysjobs` | `sysrepository`, `service` (with care) | nothing forbids; this is the escape hatch |

Use `golangci-lint`'s `depguard` linter to enforce.

---

## 15. Anti-patterns (do not do)

- **Passing `orgID` as a parameter** alongside ctx. There is one tenant per ctx; if you need a different one, that's a `cmd/sysjobs/` job.
- **Bypassing RLS in the API process.** Tempted to optimize a join? Push the work to a denormalized read model instead.
- **Sharing a cache key across tenants.** Even a "harmless" cache like config: include the org_id in the key.
- **Fan-out from one user request to multiple tenants.** That's a system job. The `RequireTenant` middleware exists to make this hard.
- **Logging without tenant tag.** Logs without `org_id` are unsearchable in incident response.
- **Creating a `*pgx.Conn` directly in a service.** Always go through `repository.Repo.WithTx(ctx)`.
- **Swallowing the panic from `tenant.Must`.** It's there to fail loud.
- **Using `BYPASSRLS` in CI integration tests.** They should run as a regular tenant; if you need cross-tenant, write a `cmd/sysjobs/` test instead.
- **Encoding tenant in URL path AND header.** Pick one (we use JWT claim + URL parameter validation via `tenant.MustEqual`).

---

## 16. Implementation milestones

These build on the phases in [archivetech.md §7](archivetech.md):

### M0 — Tenant primitives  *(blocks Phase 1)*

- Migration 0003 (organizations + memberships).
- `internal/tenant/` package: Context, helpers.
- `internal/middleware/tenant.go`: `RequireTenant` with `BeginTenantScope`.
- `internal/repository/db.go`: tenant-scoped tx wrapper.
- Update `cmd/api/main.go` to mount `RequireTenant` after `RequireAuth`.
- One end-to-end test: GET `/auth/me` returns user's orgs; `POST /auth/switch-tenant` mints a new token; subsequent request lands in the new tenant context.

### M1 — RLS rollout

- Migration 0009 (RLS enable on every tenant-scoped table).
- `cmd/sysjobs/` skeleton with `BYPASSRLS` pool.
- `internal/sysrepository/` (system-only Querier).
- RLS test fixtures + the `TestRLS_*_Isolation` family across every domain table.
- CI gate: PR that adds a tenant table without an RLS test fails.

### M2 — Job, cache, storage tenancy

- Asynq `TenantMiddleware`; update `transcode`/`thumbnail` payloads to require `organization_id`.
- `internal/cache.TenantKey` helper + lint rule against raw Redis Set/Get.
- `internal/storage` wrapper enforces `org/<id>/` prefix invariant.

### M3 — Lifecycle + observability

- Onboarding endpoint + seed script.
- Soft delete + hard delete cron in `cmd/sysjobs/`.
- Export job.
- Logger middleware decorates with `org_id` + `user_id`.
- Tracing attributes propagated through Asynq payload.

### M4 — Per-tenant config

- `organization_settings` table.
- Settings cache (1-min TTL) inside `RequireTenant`.
- Feature-flag helper `tenant.HasFeature`.
- Per-tenant rate limit middleware on top of per-IP.

### M5 — Enterprise tier tooling

- Per-tenant Asynq queues for SLA isolation.
- Bucket-per-tenant option in storage.
- Per-tenant Meilisearch indices (when search lifts to Meilisearch).

Each milestone is independently shippable. M0–M2 are mandatory before opening to any external tenant. M3+ unblock specific business requirements as they arrive.
