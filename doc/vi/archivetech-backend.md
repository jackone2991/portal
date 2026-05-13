# ArchiveTech — Kiến trúc Backend cho Multi-Tenancy

> Pattern engineering để chạy backend Go của Portal trong môi trường multi-tenant. **Mô hình bảo mật** ở [authoration.md](authoration.md); tài liệu này về **kiến trúc code** implement nó an toàn xuyên HTTP request, background job, storage, cache, và observability.
>
> Đọc sau [authoration.md](authoration.md) §3 (lớp tenant). Tài liệu này giả định bạn đã nội-hoá: PostgreSQL RLS là sàn enforce, `app.current_tenant` là DB session variable per-request, organization là ranh giới cô lập dữ liệu.

---

## 1. Quy tắc duy nhất

> **Mọi đoạn code touch dữ liệu tenant phải derive tenant từ `context.Context`. Không bao giờ từ tham số hàm, không bao giờ từ struct field, không bao giờ từ global.**

Vì sao một quy tắc, nghiêm như vậy: ngay khi tenant ID có thể đến từ hai nơi, code drift và một bug rồi sẽ chọn nhầm. Single source = không ambiguity, dễ audit. Diagnostic stack kiểu "show tôi mọi thứ chạy cho tenant X hôm qua" gói gọn về một field tracing.

Hệ quả: một function signature kiểu `ListMovies(ctx, orgID)` là **smell**. Shape đúng là `ListMovies(ctx)`, với `orgID` resolve nội bộ qua `tenant.OrgFromContext(ctx)`.

---

## 2. Flow tenant layered

```text
   ┌──────────────────────────────────────────────────┐
   │ HTTP request                                     │
   │   • RequireAuth → Identity (user, token_version) │
   │   • RequireTenant → tenant.Context (org_id)      │
   │     ↳ mở DB tx, SET LOCAL app.current_tenant     │
   │     ↳ bind tenant.Context vào ctx                │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Handler  ← nhận ctx với auth + tenant            │
   │   service.Movies.Create(ctx, input)              │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Service  ← logic nghiệp vụ, không gọi DB         │
   │   repo := r.WithTx(ctx)   ← tx đã mở             │
   │   repo.Movies.Insert(ctx, ...)                   │
   │   r.Audit.Write(ctx, ...)                        │
   │   r.Storage.Put(ctx, key, ...)                   │
   │   r.Jobs.Enqueue(ctx, asynq.NewTranscodeTask())  │
   └──────────┬───────────────────────────────────────┘
              ▼
   ┌──────────────────────────────────────────────────┐
   │ Repo (sqlc) ← chạy trên tx của request;          │
   │   RLS filter mọi row theo app.current_tenant     │
   └──────────────────────────────────────────────────┘

   Worker process — consumer Asynq — đối xứng:
   ┌──────────────────────────────────────────────────┐
   │ Task delivered với org_id trong payload          │
   │   • TenantTaskMiddleware → mở tx, SET LOCAL      │
   │     app.current_tenant, bind tenant.Context      │
   │   • Task handler chạy, gọi service y hệt         │
   └──────────────────────────────────────────────────┘
```

Đối xứng giữa API và worker là chủ ý: cùng service code chạy ở cả hai, với cùng shape context. Middleware là thứ duy nhất khác.

---

## 3. Package tenant context

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
    OrgCode string  // human-readable; hữu ích trong log
    // Thêm tier, feature flag, v.v. ở đây khi mô hình lớn ra.
    // Giữ struct này read-only.
}

type ctxKey struct{}

func With(ctx context.Context, t Context) context.Context {
    return context.WithValue(ctx, ctxKey{}, t)
}

func From(ctx context.Context) (Context, bool) {
    t, ok := ctx.Value(ctxKey{}).(Context)
    return t, ok
}

// Must dùng cho code path sau khi RequireTenant đã chạy. Panic nếu thiếu —
// surface bug routing nhanh.
func Must(ctx context.Context) Context {
    t, ok := From(ctx)
    if !ok {
        panic("tenant: context missing — wire RequireTenant middleware")
    }
    return t
}

// OrgID là shortcut common-case.
func OrgID(ctx context.Context) uuid.UUID { return Must(ctx).OrgID }

// MustEqual assert hai mảnh code đồng thuận về tenant active.
// Dùng phòng thủ khi input mang org_id phải match session —
// vd: URL parameter hoặc payload job. Panic on mismatch là
// an toàn hơn silent accept.
func MustEqual(ctx context.Context, candidate uuid.UUID) {
    if t := Must(ctx); t.OrgID != candidate {
        panic(fmt.Sprintf("tenant mismatch: ctx=%s arg=%s", t.OrgID, candidate))
    }
}
```

Hai helper (`MustEqual`, `Must`) đều panic on misuse. Đó là chủ ý — đây là lỗi lập trình, không phải lỗi runtime. Một panic ở request boundary surface trong log và CI sớm hơn một write nhầm-tenant silent.

---

## 4. Chiến lược kết nối Database

### 4.1 PgBouncer + RLS tương tác

Portal pool qua PgBouncer trong **chế độ transaction pooling**. RLS dùng Postgres session variable, mà PgBouncer sẽ xử lý sai nếu set bằng `SET ...` thường — nhưng `SET LOCAL` và `set_config(..., true)` là scope transaction, vậy nên release cùng connection.

Quy tắc cứng: **mỗi request tenant-scoped chạy bên trong một Postgres transaction duy nhất**, kể cả read-only. Guard tenant set ở đầu transaction.

```go
// internal/repository/db.go
type DB struct {
    pool *pgxpool.Pool
}

// BeginTenantScope khởi tạo tx và pin app.current_tenant trong lifetime của nó.
// Trả về cleanup func commit-or-rollback tuỳ vào lỗi
// lưu trong *errPtr (pattern Go cổ điển deferred-tx).
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

Middleware dùng cái này; service code dùng `pgx.Tx` kết quả gián tiếp qua interface `Querier` sqlc generate.

### 4.2 Indirection Querier

sqlc emit type `*Queries` per-table và interface `Querier`. Lớp repo của Portal thêm một helper:

```go
// internal/repository/repo.go
type Repo struct {
    db *DB
    // Querier per-domain. Tất cả nhận DBTX (pool, tx, hoặc conn).
    // Luôn pass tx của request.
    Movies   *movies.Queries
    Music    *music.Queries
    Stories  *stories.Queries
    Assets   *assets.Queries
    RBAC     *rbac.Queries
    Auth     *auth.Queries
}

// WithTx trả về Repo bound vào transaction của request. Service code
// được một cái này và không bao giờ thấy *pgxpool.Pool trực tiếp.
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

Handler **không** gọi `WithTx` trực tiếp — tenant middleware đã đặt tx trong context. Service kéo ra:

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

Shape này giữ contract của service sạch trong khi đảm bảo mọi DB call land trên transaction scope của request.

### 4.3 Role system cho work cross-tenant

Một `pgxpool.Pool` **riêng** connect như `portal_system` (với `BYPASSRLS`). Chỉ wire vào `cmd/sysjobs/` — binary API và worker không import nó. Enforce bằng layout package:

```
backend/
  cmd/api/main.go      ← import internal/repository (pool request-scoped)
  cmd/worker/main.go   ← import internal/repository
  cmd/sysjobs/main.go  ← import internal/repository VÀ internal/sysrepository
                          (pool BYPASSRLS — nơi duy nhất reference nó)
```

Thêm check kiểu `go vet` (hoặc `golangci-lint forbidigo`) cấm import `internal/sysrepository` từ bất kỳ package nào ngoài `cmd/sysjobs/`.

---

## 5. Convention service layer

### 5.1 Shape struct Service

Mỗi domain có một struct `Service`. Dependency (repo, audit, storage, jobs) inject một lần lúc construction. Method nhận `(ctx, input)` và không bao giờ nhận raw ID override context.

```go
// internal/service/groups/service.go
type Service struct {
    repo    *repository.Repo
    audit   *audit.Logger
    jobs    *jobs.Client
    cache   *rbac.CachedLoader
}
```

Method theo pattern nhất quán:

1. Lấy tenant + identity từ ctx.
2. Authorize (gọi engine).
3. Logic nghiệp vụ.
4. Audit.
5. Invalidate cache nếu áp dụng.

Anti-pattern: một method service nhận tham số `orgID` "cho op cross-tenant". Nếu bạn thực sự cần cross-tenant, nó sống trong `cmd/sysjobs/`, không bao giờ trong lớp service API.

### 5.2 Lỗi tenant-aware

Lỗi trả handler không được leak data từ tenant khác. Một "not found" cho một tenant phải trông giống "tồn tại nhưng ở tenant khác" — đều 404.

```go
// internal/service/movies/errors.go
var (
    ErrNotFound      = errors.New("movies: not found")              // 404
    ErrAlreadyExists = errors.New("movies: already exists")         // 409
    ErrFileTooLarge  = errors.New("movies: file exceeds quota")     // 413
)
```

Repository đảm bảo lookup theo ID là RLS-scoped; service dịch `ErrNoRows` của pgx thành `ErrNotFound`. Không có đường "cross-tenant exists".

---

## 6. Background job (Asynq)

### 6.1 Tenant trong mỗi payload

Mỗi task payload mang organization ID. Không ngoại lệ.

```go
// internal/worker/transcode.go (cập nhật cho multi-tenant)
type TranscodePayload struct {
    OrganizationID uuid.UUID `json:"organization_id"`  // YÊU CẦU
    AssetID        uuid.UUID `json:"asset_id"`
    SourceKey      string    `json:"source_key"`
    OutputKey      string    `json:"output_key"`
    Variants       []string  `json:"variants,omitempty"`
}

func NewTranscodeTask(ctx context.Context, p TranscodePayload) (*asynq.Task, error) {
    if p.OrganizationID == uuid.Nil {
        p.OrganizationID = tenant.OrgID(ctx)  // default an toàn — cùng tenant
    }
    body, err := json.Marshal(p)
    if err != nil {
        return nil, err
    }
    return asynq.NewTask(TaskTypeTranscode, body, asynq.Queue("transcode")), nil
}
```

`NewTranscodeTask(ctx, p)` đọc tenant active từ ctx nếu không set tường minh. Caller `Enqueue` không thể vô tình drop tenant — constructor từ chối (`uuid.Nil` post-fill là bug lập trình, panic chấp nhận được).

### 6.2 Middleware phía worker

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

Bây giờ cùng `service.Movies.Update(ctx, ...)` hoạt động y hệt khi gọi từ HTTP handler hay từ job — cả hai đều có tx tenant-scoped trong context.

### 6.3 Scheduling per-queue hay per-tenant?

Default: **priority per-queue**, không phải per-tenant. Queue transcode mang job từ mọi tenant cân bằng đều, với priority theo loại job (transcode 5, thumbnail 3, default 1).

Cho tenant enterprise-tier cần cô lập SLA (không noisy-neighbor): cấp một pool worker chuyên dụng chỉ consume từ `transcode-tenant-<orgID>`. Helper `Enqueue` chọn queue đúng theo tier của org:

```go
queue := "transcode"
if tier == TierEnterprise {
    queue = fmt.Sprintf("transcode-tenant-%s", orgID)
}
asynq.Queue(queue)
```

Đừng làm từ ngày đầu. Thêm queue routing chỉ khi customer trả tiền trigger yêu cầu.

---

## 7. Object storage (S3 / MinIO / R2)

### 7.1 Phân vùng key

Mọi key bắt đầu với `org/<org_id>/`. Không ngoại lệ, kể cả upload system (dùng `org/system/...`).

```go
// internal/storage/keys.go
func AssetSourceKey(orgID, assetID uuid.UUID, ext string) string {
    return fmt.Sprintf("org/%s/assets/source/%s.%s", orgID, assetID, ext)
}
func AssetHLSPrefix(orgID, assetID uuid.UUID) string {
    return fmt.Sprintf("org/%s/assets/hls/%s/", orgID, assetID)
}
```

Lợi ích:

- Duyệt bucket tự nhiên segmented (hữu ích cho support).
- Ranh giới backup align với prefix; restore một tenant là `aws s3 sync s3://portal/org/<id>/ ...`.
- Lifecycle rule per-tenant trở nên đơn giản (vd "xoá upload hết hạn của tenant X").
- Migration tương lai sang bucket-per-tenant là một rename, không phải re-architect.

### 7.2 Wrapper client storage

Wrapper S3 client nhận ctx và prefix tự động:

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
        // Caller đã namespace; verify nó match tenant active.
        prefix := fmt.Sprintf("org/%s/", org)
        if !strings.HasPrefix(key, prefix) {
            panic(fmt.Sprintf("storage: key %q crosses tenant %s", key, org))
        }
        return key
    }
    return fmt.Sprintf("org/%s/%s", org, key)
}
```

Panic on mismatch là cùng tư thế phòng thủ như `tenant.MustEqual` — bẫy bật to khi misuse.

### 7.3 Presigned URL

Presigned URL sign đúng một key, scope vào đúng một tenant. Generate qua cùng wrapper để invariant prefix giữ.

Cho upload từ browser qua S3 multipart, API generate presigned URL per-part **chỉ** cho prefix của tenant active. User không bao giờ có thể nhận presigned URL cho key của tenant khác, kể cả guess.

### 7.4 Chiến lược bucket

| Tier | Chiến lược |
|------|----------|
| Tenant standard | Một bucket `portal-media`, prefix `org/<id>/` |
| Tenant enterprise | Bucket chuyên dụng `portal-media-<id>` với credentials IAM riêng (defer — enable khi hợp đồng enterprise đầu tiên close) |

Archive audit log là bucket riêng (`portal-audit-archive`) với **policy bucket immutable** (object-lock retention). Credential app bị compromise không thể xoá log.

---

## 8. Caching (Redis / DragonflyDB)

### 8.1 Convention naming key

```text
<resource>:<org_id>:<rest>
```

Luôn luôn, không ngoại lệ. Ví dụ:

- `rbac:perms:<org_id>:<user_id>:v<token_version>`
- `policy:detail:<org_id>:<policy_id>`
- `movie:list:<org_id>:page<n>`
- `session:<user_id>` ← global (identity user, không phải tenant)

Đoạn `<org_id>` biến invalidation per-tenant thành một `SCAN MATCH <resource>:<org_id>:*`. Không có nó, "evict mọi thứ cho tenant X" là không thể nếu không full flush.

### 8.2 Helper cache-aside

```go
// internal/cache/keys.go
func TenantKey(ctx context.Context, parts ...string) string {
    org := tenant.OrgID(ctx)
    return fmt.Sprintf("%s:%s:%s", parts[0], org, strings.Join(parts[1:], ":"))
}
```

Builder không nhận `ctx` không được phép. Rule linter (golangci-lint `forbidigo`):

```yaml
forbidigo:
  forbid:
    - p: 'redis\.NewClient\(\)\.Set\('
      msg: "use cache.SetTenant; never call redis client directly with raw keys"
```

### 8.3 Cache cross-tenant

Cache global hợp pháp duy nhất là identity (refresh token theo hash, metadata OIDC provider, profile user theo id). Key này tường minh bỏ `<org_id>` và đi qua checklist code-review: "Cái này thật sự độc lập tenant?"

---

## 9. Search

### 9.1 Default: Postgres FTS

Cột `tsvector` tenant-scoped + RLS = cô lập miễn phí. Không cần config thêm.

```sql
-- trong 0004_user_groups.up.sql hoặc nơi movies sống
ALTER TABLE movies
    ADD COLUMN search_tsv tsvector
    GENERATED ALWAYS AS (
        setweight(to_tsvector('simple', coalesce(unaccent(title), '')), 'A') ||
        setweight(to_tsvector('simple', coalesce(unaccent(description), '')), 'B')
    ) STORED;

CREATE INDEX movies_search_tsv_idx ON movies USING GIN (search_tsv);
```

### 9.2 Migration sang Meilisearch

Khi/nếu FTS không đủ:

- Một index Meilisearch **per tenant** — `movies-<org_id>`. Cô lập sạch nhất; Meilisearch hỗ trợ native.
- Reindex lúc policy thay đổi là work per-tenant; không touch tenant khác.
- Search key (API key Meilisearch) per-tenant, tạo lúc onboard tenant, lưu encrypted.

Đừng làm ngày đầu. Postgres FTS ổn cho đến khi search relevance kém competitors đáng kể.

---

## 10. Config per-tenant

Một số tenant cần limit, feature flag, integration khác nhau.

```sql
CREATE TABLE organization_settings (
    organization_id     UUID PRIMARY KEY REFERENCES organizations(id) ON DELETE CASCADE,
    max_storage_bytes   BIGINT,                       -- NULL = không giới hạn
    max_users           INTEGER,                      -- NULL = không giới hạn
    max_uploads_per_min INTEGER NOT NULL DEFAULT 30,
    feature_flags       JSONB NOT NULL DEFAULT '{}'::jsonb,
    custom_branding     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Load một lần per request bên trong middleware `RequireTenant` (cache ~1 phút). Feature flag đọc qua:

```go
if tenant.HasFeature(ctx, "live_streaming") {
    ...
}
```

Rate limit enforce trong middleware trên đỉnh limit per-IP:

```go
// internal/middleware/tenant_ratelimit.go
// Token bucket per (org_id, action). Limit đến từ organization_settings.
```

---

## 11. Lifecycle tenant

### 11.1 Onboarding

`POST /admin/organizations` (chỉ system-superadmin) chạy trong một transaction:

1. Insert vào `organizations`.
2. Tạo User Group mặc định (`<org_code>-default`) dưới org.
3. Attach policy seed `Owner` vào group (perm admin scope vào org).
4. Tạo row `organization_memberships` cho user mời như owner.
5. Audit `tenant.organization.created`.

### 11.2 Suspension

Toggle `organizations.is_active = false`. Middleware `RequireTenant` từ chối request cho org inactive (trả 503 `tenant_suspended`). Session đang tồn tại KHÔNG auto-revoke — khi user retry với token, middleware đẩy họ ra. Mềm hơn mass logout.

### 11.3 Xoá cứng

Hai-pha:

1. **Soft delete** (`deleted_at = now()`). Mọi truy cập API bị chặn. Data giữ 30 ngày. Restore được bằng một `UPDATE`.
2. **Hard delete** (Asynq cron job sau 30 ngày):
   - Xoá S3 object dưới `org/<id>/`.
   - DELETE từ mọi table tenant-scoped (RLS-bypass — `cmd/sysjobs/`).
   - Verify không còn row nào reference org_id.
   - Insert event `tenant.organization.purged` cuối vào bucket archive audit (table audit bản thân đã đi).
   - Email confirm cho contact billing cuối được biết của org.

### 11.4 Export

`POST /admin/organizations/{id}/export` (admin org HOẶC system-superadmin) enqueue một job:

- Stream mọi table tenant-scoped sang một file ND-JSON per table.
- Copy S3 object dưới `org/<id>/` sang bucket export một-lần-dùng.
- Generate URL download signed (hợp lệ 7 ngày).
- Notify admin yêu cầu khi xong.

Đây là cái GDPR / churn customer / due diligence yêu cầu. Build trong Phase 1; rẻ hơn retrofit.

---

## 12. Observability

### 12.1 Logging

Mỗi dòng log có field `org_id` và `user_id`. Middleware logger đọc ctx và decorate:

```go
// internal/middleware/logger.go
zerolog.Ctx(ctx).With().
    Str("org_id", t.OrgID.String()).
    Str("user_id", id.UserID.String()).
    Str("request_id", requestID).
    Logger()
```

Background job làm tương tự trong `TenantMiddleware`.

### 12.2 Metrics

Label Prometheus với `org_id` hấp dẫn nhưng **nổ cardinality**. Chiến lược hai-tier:

- Metric top-K: bucket tenant lớn riêng từng cái, mọi cái khác vào `org_id="other"`. Refresh K hàng ngày.
- Detail per-tenant: khi cần, query audit log + counter DB, không phải Prometheus.

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

Span OpenTelemetry luôn set `tenant.id` và `tenant.code` như attribute. Trace cross-service (API → worker qua Asynq) propagate context trace trong payload task cạnh `organization_id`.

```go
type TaskHeader struct {
    OrganizationID uuid.UUID `json:"organization_id"`
    TraceID        string    `json:"trace_id,omitempty"`
    SpanID         string    `json:"span_id,omitempty"`
}
```

---

## 13. Testing

### 13.1 Discipline test RLS

Mỗi PR touch table tenant-scoped bao gồm test parametric:

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

    // A chỉ thấy row của A
    rows := asA.MustListMovies(t)
    require.Len(t, rows, 1)
    require.Equal(t, "Movie One", rows[0].Title)

    // B chỉ thấy row của B
    rows = asB.MustListMovies(t)
    require.Len(t, rows, 1)
    require.Equal(t, "Movie Two", rows[0].Title)

    // BYPASSRLS thấy cả hai
    asSys := testdb.AsSystem(db)
    rows = asSys.MustListMovies(t)
    require.Len(t, rows, 2)
}
```

Nếu test này không tồn tại cho table tenant-scoped mới, CI reject PR.

### 13.2 Fixture tenant

```go
// internal/testdb/fixtures.go
type Tenant struct {
    OrgID    uuid.UUID
    OwnerID  uuid.UUID
    GroupID  uuid.UUID
}

// NewOrg tạo tenant tươi + group default + owner user. Trả về
// Tenant đầy đủ form để dùng trong test cấp service.
func NewOrg(t *testing.T, db *DB, code string) *Tenant { ... }

// AsTenant trả về function mở tx scope vào tenant đó.
// Dùng như: client := testdb.AsTenant(tenant)(db)
```

### 13.3 Test parallel an toàn

Cô lập RLS giữa tenant nghĩa là test parallel (`t.Parallel()`) chạy trên org khác nhau không thể can thiệp — kể cả trên DB shared. Đây là lợi ích bị đánh giá thấp của RLS cho testing.

### 13.4 Test "missing tenant"

Mỗi endpoint nên có một test assert nó reject request không có context tenant:

```go
func TestMoviesCreate_RequiresTenant(t *testing.T) {
    h := newHandler(t /* không có RequireTenant middleware */)
    req := httptest.NewRequest("POST", "/movies", body)
    rr := httptest.NewRecorder()
    h.ServeHTTP(rr, req)
    require.Equal(t, http.StatusBadRequest, rr.Code)
}
```

Cái này bắt bug tệ nhất: một endpoint mount không có `RequireTenant`.

---

## 14. Tổ chức code

```text
backend/
├── cmd/
│   ├── api/main.go          ← HTTP, chỉ pool RLS
│   ├── worker/main.go       ← Asynq, chỉ pool RLS
│   └── sysjobs/main.go      ← pool BYPASSRLS — batch cross-tenant
│
├── internal/
│   ├── auth/                ← identity (JWT, OIDC, refresh, TOTP)
│   ├── tenant/              ← tenant.Context, MustEqual, helpers
│   ├── rbac/                ← engine, permission, role, cache
│   ├── middleware/          ← RequireAuth, RequireTenant, RequireStepUp,
│   │                          RequirePermission, ratelimit, logger
│   ├── repository/          ← sqlc-generated; một Querier per domain
│   ├── sysrepository/       ← BYPASSRLS — chỉ import từ cmd/sysjobs
│   ├── service/
│   │   ├── movies/          ← logic nghiệp vụ, tenant-aware
│   │   ├── music/
│   │   ├── stories/
│   │   ├── groups/
│   │   ├── policies/
│   │   └── auth/
│   ├── storage/             ← S3 client với key prefix tenant
│   ├── jobs/                ← Asynq client + middleware (TenantMiddleware)
│   ├── worker/              ← task handler (transcode, thumbnail, notify)
│   ├── audit/               ← audit logger
│   ├── notifications/       ← in-app + Web Push
│   ├── cache/               ← Redis wrapper với TenantKey helper
│   └── handler/             ← HTTP handler; thin — delegate sang service
│
├── db/
│   ├── migrations/
│   └── queries/
└── go.mod
```

### Quy tắc allowed-import

| Package | Có thể import | KHÔNG được import |
|---------|------------|-----------------|
| `internal/sysrepository` | Chỉ type của `internal/repository` | bất cứ gì từ `service`, `handler`, `middleware` |
| `internal/handler` | `service`, `middleware`, `auth`, `tenant`, `audit` | `repository` trực tiếp |
| `internal/service/*` | `repository`, `cache`, `storage`, `jobs`, `audit`, `tenant` | `sysrepository`, `handler`, `middleware` |
| `internal/repository` | Chỉ sqlc-generated | `service`, `tenant` |
| `cmd/api` | `service`, `handler`, `middleware`, `repository`, `auth`, `tenant` | `sysrepository` |
| `cmd/sysjobs` | `sysrepository`, `service` (cẩn thận) | không cấm gì; đây là escape hatch |

Dùng linter `depguard` của `golangci-lint` để enforce.

---

## 15. Anti-pattern (không làm)

- **Truyền `orgID` như parameter** cạnh ctx. Có một tenant per ctx; nếu cần khác, đó là job `cmd/sysjobs/`.
- **Bypass RLS trong process API.** Bị cám dỗ optimize join? Đẩy work sang read model denormalize thay vì.
- **Share cache key across tenant.** Kể cả cache "vô hại" như config: include org_id trong key.
- **Fan-out từ một user request sang nhiều tenant.** Đó là system job. Middleware `RequireTenant` tồn tại để làm cái này khó.
- **Log không có tag tenant.** Log không có `org_id` là không search được khi response incident.
- **Tạo `*pgx.Conn` trực tiếp trong service.** Luôn qua `repository.Repo.WithTx(ctx)`.
- **Nuốt panic từ `tenant.Must`.** Nó ở đó để fail loud.
- **Dùng `BYPASSRLS` trong integration test CI.** Chúng nên chạy như tenant thường; nếu cần cross-tenant, viết test `cmd/sysjobs/` thay vì.
- **Encode tenant trong URL path VÀ header.** Chọn một (chúng ta dùng claim JWT + validate URL parameter qua `tenant.MustEqual`).

---

## 16. Milestone implementation

Cái này build trên các phase trong [archivetech.md §7](archivetech.md):

### M0 — Tenant primitive  *(block Phase 1)*

- Migration 0003 (organizations + memberships).
- Package `internal/tenant/`: Context, helpers.
- `internal/middleware/tenant.go`: `RequireTenant` với `BeginTenantScope`.
- `internal/repository/db.go`: wrapper tx tenant-scoped.
- Update `cmd/api/main.go` mount `RequireTenant` sau `RequireAuth`.
- Một test end-to-end: GET `/auth/me` trả về org của user; `POST /auth/switch-tenant` mint token mới; request kế tiếp land trong context tenant mới.

### M1 — Roll-out RLS

- Migration 0009 (enable RLS trên mọi table tenant-scoped).
- Skeleton `cmd/sysjobs/` với pool `BYPASSRLS`.
- `internal/sysrepository/` (Querier chỉ system).
- Fixture test RLS + family `TestRLS_*_Isolation` xuyên mọi table domain.
- Cổng CI: PR thêm table tenant không có test RLS fail.

### M2 — Tenancy job, cache, storage

- `TenantMiddleware` Asynq; update payload `transcode`/`thumbnail` yêu cầu `organization_id`.
- Helper `internal/cache.TenantKey` + lint rule chống raw Redis Set/Get.
- Wrapper `internal/storage` enforce invariant prefix `org/<id>/`.

### M3 — Lifecycle + observability

- Endpoint onboarding + script seed.
- Cron soft delete + hard delete trong `cmd/sysjobs/`.
- Job export.
- Middleware logger decorate với `org_id` + `user_id`.
- Attribute tracing propagate qua payload Asynq.

### M4 — Config per-tenant

- Table `organization_settings`.
- Cache settings (TTL 1-phút) bên trong `RequireTenant`.
- Helper feature-flag `tenant.HasFeature`.
- Middleware rate limit per-tenant trên đỉnh per-IP.

### M5 — Tooling tier enterprise

- Queue Asynq per-tenant cho cô lập SLA.
- Option bucket-per-tenant trong storage.
- Index Meilisearch per-tenant (khi search nâng lên Meilisearch).

Mỗi milestone ship độc lập được. M0–M2 bắt buộc trước khi mở cho tenant external nào. M3+ unblock yêu cầu nghiệp vụ cụ thể khi đến.
