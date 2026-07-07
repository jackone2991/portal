# ADR-07: Mô hình Multi-tenancy & Row-Level Security (Phase 1 — hoãn khỏi v1)

**Trạng thái:** Đề xuất — **đã hoãn**. Phase 1, KHÔNG thuộc scope v1 ([ADR-01](./01-v1-scope-cut.md)). Đây là design/plan để thực thi khi bắt đầu Phase 1; **chưa có code cho tới lúc đó.**
**Ngày:** 2026-07-07
**Người quyết định:** kirito
**Liên quan:** [ADR-01](./01-v1-scope-cut.md) (cắt v1) · [ADR-02](./02-rbac-model-reconciliation.md) (RBAC) · [ADR-03](./03-single-vps-topology.md) (single-VPS / PgBouncer) · [feature.md §2 + §18 Phase 1](../feature.md) · [D-23] [D-24] [D-25]

## Bối cảnh

`feature.md §2` và roadmap Phase 1 muốn multi-tenancy — `organizations` + `memberships` — với **Row-Level Security (RLS) của Postgres làm phòng-thủ-theo-tầng** (L2 trong authoration.md): kể cả query quên `WHERE tenant_id = ?` cũng không được rò dữ liệu chéo tenant. v1 đã hoãn tất cả — hiện **không có cột `tenant_id` ở đâu cả**, và app thực chất là single-user.

Ba lực định hình thiết kế:

1. **Thế bảo mật.** Điểm cốt lõi của RLS: *cơ sở dữ liệu*, không phải handler, mới là chốt chặn cuối. Chỉ scope ở tầng app thì chỉ cách một filter bị quên là rò rỉ.
2. **Ràng buộc PgBouncer (chịu lực chính).** Prod pool qua **PgBouncer chế độ transaction** ([ADR-03](./03-single-vps-topology.md)). RLS cần một biến session per-request; transaction-mode dùng lại một connection cho nhiều transaction, nên kiểu ngây thơ "`SET app.tenant_id` một lần mỗi connection" sẽ rò tenant của request này sang request kế. (Dev v1 né bằng cách connect **thẳng tới `postgres:5432`** — xem `.env` — vì prepared-statement cache của pgx cũng xung đột với PgBouncer transaction-mode. Phase 1 phải giải quyết dứt khoát.)
3. **Dữ liệu cá nhân vs. org.** Phần lớn dữ liệu Portal là *cá nhân* (upload, feed, bank của một user); chỉ một phần chia sẻ trong org. Một **tenant cá nhân tổng hợp (synthetic)** cho mỗi user để mọi hàng đều mang `tenant_id` và giữ một đường code duy nhất cho cả hai.

## Quyết định

Dùng **RLS của PostgreSQL khoá theo một GUC per-request, đặt bằng `SET LOCAL` bên trong một transaction per-request, trên một role ứng dụng không-sở-hữu-bảng với `FORCE ROW LEVEL SECURITY`.** Mô hình tenancy = `organizations` (có cột phân loại `kind` `'org' | 'household' | 'personal'`) + `organization_memberships`; mỗi user có một org **personal** tổng hợp. Chạy batch cross-tenant bằng một role **`BYPASSRLS`** riêng, cô lập vào `cmd/sysjobs` bằng depguard.

> **Tên GUC:** skeleton tenant hiện tham chiếu `app.current_tenant`; roadmap Phase 1 viết `app.tenant_id`. **Chọn một và dùng nhất quán** — ADR này chuẩn hoá về **`app.current_tenant`** (khớp comment skeleton đã ship). Sửa `app.tenant_id` trong `feature.md §18` cho khớp khi Phase 1 xong.

### 1. Mô hình dữ liệu

- `organizations(id, kind, slug, name, owner_id → users(id), created_at, updated_at)` — `kind ∈ {'org','household','personal'}` **ngay từ đầu** [D-24]; thêm household sau không được migrate một bảng đã có data.
- `organization_memberships(org_id, user_id, role, granted_at, ...)` — user ↔ tenant với role có phạm vi. Độ chi tiết role khác theo kind: org → phân cấp RBAC đầy đủ; household → `owner` + `member` (cap mềm ~6); personal → một `owner`.
- **Mỗi user được cấp một org `personal` lúc đăng ký** (`kind='personal'`, `owner_id=user`, một membership owner). Route cá nhân (`/t/me/...`) phân giải `me` → id org đó.
- **`users` vẫn GLOBAL** — một người là một danh tính xuyên các org (authoration.md). Không có `tenant_id` trên `users`.
- **Bảng tenant-scoped** mang `tenant_id UUID NOT NULL REFERENCES organizations(id)`: các bảng domain tương lai (movie/music/story/comic), bank, social — **và `media.assets` được thêm `tenant_id`** (một upload thuộc về context tenant lúc tạo; `me` cho cá nhân). Bảng RBAC (`roles`, `permissions`) vẫn global; `user_roles` trở thành scoped theo membership (xem §4).

### 2. Thực thi RLS (mỗi bảng tenant-scoped)

- Một **role app `portal_app`** riêng (`NOSUPERUSER NOBYPASSRLS`) mà API/worker connect vào. **Then chốt:** superuser *và chủ sở hữu bảng* bỏ qua RLS trừ khi có `FORCE` — nên role app **không được** sở hữu bảng (sở hữu bằng role migration/admin, chạy dưới `portal_app`).
- Mỗi bảng tenant-scoped, **trong chính migration tạo nó**:
  ```sql
  ALTER TABLE movies ENABLE ROW LEVEL SECURITY;
  ALTER TABLE movies FORCE  ROW LEVEL SECURITY;      -- áp dụng cả với owner
  CREATE POLICY tenant_isolation ON movies
    USING      (tenant_id = current_setting('app.current_tenant')::uuid)
    WITH CHECK (tenant_id = current_setting('app.current_tenant')::uuid);
  ```
  `USING` lọc read/update/delete; `WITH CHECK` chặn ghi một hàng vào tenant *khác*.
- **Fail closed:** `current_setting('app.current_tenant')` khi chưa set GUC sẽ báo lỗi — query quên mở tenant scope thì *lỗi* thay vì rò. Chỉ dùng `current_setting(..., true)` (trả NULL) ở nơi đã xử lý rõ "không tenant ⇒ deny".

### 3. Chiến lược connection (mấu chốt)

RLS-per-request dưới PgBouncer transaction pooling:

- **`SET LOCAL app.current_tenant = $1` bên trong một transaction.** `SET LOCAL` có phạm vi transaction, reset khi `COMMIT`/`ROLLBACK`, nên connection pool không bao giờ mang tenant request này sang request kế. `SET` thường (phạm vi session) là **sai** dưới transaction pooling.
- **Mọi request tenant-scoped chạy trong một transaction.** `platform/db.BeginTenantScope(ctx, tenantID)` mở `BEGIN; SET LOCAL app.current_tenant = $1;`, trao tx cho các query của request, rồi `COMMIT` (tự `ROLLBACK` khi handler lỗi).
- **pgx ⨯ PgBouncer transaction mode** xung đột với prepared-statement cache mặc định của pgx → đặt `DefaultQueryExecMode = QueryExecModeExec` (hoặc `SimpleProtocol`) / tắt statement cache của pool. (Xung đột này là lý do dev connect thẳng hiện nay.) Phase 1 chọn một:
  - **(a) Qua PgBouncer (khuyến nghị)** — transaction mode + simple/exec protocol + `SET LOCAL` per request. Giữ được pool.
  - **(b) Thẳng-tới-Postgres (dự phòng)** — giữ `postgres:5432`, bỏ PgBouncer cho app; chỉ chấp nhận khi số connection còn thấp dưới `max_connections`.

### 4. Phân giải tenant & RBAC

- URL: `/api/v1/t/{tenant}/...`; `{tenant}` = slug org hoặc chữ `me` [D-23].
- **Thứ tự middleware:** `RequireAuth` → **`RequireTenant`** (phân giải `{tenant}` → tenant_id; `me` → org personal của caller; **kiểm tra caller có membership**, không thì 403) → `BeginTenantScope(ctx, tenant_id)` cho tx của request → handler module.
- **Triển khai single-tenant** map `/api/v1/...` (không `/t/`) sang một tenant mặc định qua Traefik/middleware — trường hợp phổ biến không xấu đi.
- **RBAC hợp thành theo tenant:** quyền hiệu lực tính **trong membership đang active** (admin ở org A, member ở org B). `user_roles` thành `(user_id, org_id, role_id)`; key cache RBAC thêm tenant ([ADR-02](./02-rbac-model-reconciliation.md)). Catalog role/permission vẫn global.

### 5. Batch cross-tenant (BYPASSRLS)

- `cmd/sysjobs` connect bằng một **role `portal_sys` riêng (`BYPASSRLS`)** cho bảo trì cross-tenant (purge, migration, báo cáo tổng hợp) qua `internal/sysrepository`. **depguard chặn mọi package khác import `sysrepository`** — một đường BYPASSRLS gọi được từ API sẽ vô hiệu hoá toàn bộ RLS (CLAUDE.md).

## Mô hình kiến trúc — đường đi request

```mermaid
sequenceDiagram
    actor U as Client
    participant MW as Middleware API
    participant DB as Postgres (role portal_app, RLS FORCE)

    U->>MW: GET /api/v1/t/acme/movies  (cookie)
    MW->>MW: RequireAuth → danh tính
    MW->>DB: RequireTenant: phân giải slug 'acme' + kiểm tra membership
    MW->>DB: BeginTenantScope: BEGIN; SET LOCAL app.current_tenant = '<acme-id>'
    MW->>DB: SELECT * FROM movies         (không cần WHERE tenant_id)
    Note over DB: Policy RLS lọc theo tenant_id = current_setting('app.current_tenant')
    DB-->>MW: chỉ các hàng của acme
    MW->>DB: COMMIT   (SET LOCAL bị bỏ; connection an toàn để tái dùng)
    MW-->>U: 200
    Note over MW,DB: Filter bị quên vẫn không rò — DB tự enforce.
```

## Các phương án đã cân nhắc

- **A — RLS + GUC `SET LOCAL` per-request *(chọn)*.** Cô lập do DB enforce; query lỗi cũng không rò. Chi phí: mỗi request tenant-scoped là một tx; phải cẩn thận cấu hình PgBouncer/pgx.
- **B — Chỉ scope tầng app (`WHERE tenant_id = ?`), không RLS.** Đơn giản nhất, không cần GUC/tx. **Bị từ chối** — một filter bị quên = rò chéo tenant, đúng thất bại mà RLS sinh ra để chặn; không chấp nhận với bank/dữ liệu riêng tư.
- **C — Schema-per-tenant / DB-per-tenant.** Cô lập mạnh, nhưng chi phí migration/ops bùng nổ với nhiều tenant *cá nhân* nhỏ. **Bị từ chối** cho hình dạng tenant-mỗi-user của Portal.
- **D — Connection-per-tenant với `SET` session.** Cần pooling session-mode → giết hiệu quả transaction-mode của PgBouncer và bùng số connection. **Bị từ chối.**

## Phân tích đánh đổi

RLS + `FORCE` + GUC fail-closed là cô lập mạnh nhất với ít code nhất — nhưng áp hai luật mọi dev phải nhớ: **(1)** query tenant-scoped chạy *bên trong* `BeginTenantScope`, và **(2)** app connect bằng role **không-sở-hữu-bảng**. tx-per-request + simple-protocol là thuế hiệu năng/độ phức tạp có thật nhưng có giới hạn (đo bằng profile observability, mà [ADR-03] nói nên xong cùng sprint). Tenant `personal` tổng hợp đổi một hàng `organizations` mỗi user lấy một đường code duy nhất, không rẽ nhánh, cho cả cá nhân lẫn org.

## Hệ quả

- **Luật mới:** mọi migration tenant-scoped phải `ENABLE + FORCE` RLS + policy `tenant_isolation` **trong chính migration** tạo bảng — thêm vào checklist schema-ownership MODULES.md §6.
- **Retrofit:** `media.assets` thêm `tenant_id` (backfill = tenant personal của owner mỗi asset + policy). `users` không đổi (global). `user_roles` thêm `org_id`.
- **Số migration:** `0010_rls_enable` trong roadmap có trước các migration media; file thực tế tiếp từ `0007` → `0008_tenant_core`, `0009_rls_enable`.
- **Lệch tên GUC** giữa skeleton (`app.current_tenant`) và feature.md (`app.tenant_id`) phải hoà giải (ADR này chọn `app.current_tenant`).
- **Test:** một test RLS khẳng định, trên role `portal_app`, tenant B không đọc được hàng của tenant A kể cả bằng `SELECT` thô không `WHERE`.

## Kế hoạch triển khai (khi un-defer)

1. [ ] Role DB: `portal_app` (`NOBYPASSRLS`, app connect bằng role này) + `portal_sys` (`BYPASSRLS`, chỉ sysjobs). Migration + compose/env; bảng do một role admin/migration riêng sở hữu.
2. [ ] `0008_tenant_core`: `organizations`(+`kind`) + `organization_memberships`; **backfill một org `personal` + membership owner cho mọi user hiện có**.
3. [ ] `platform/db.BeginTenantScope(ctx, tenantID)` + cấu hình pool cho PgBouncer transaction mode (simple/exec protocol), hoặc ghi rõ phương án connect-thẳng dự phòng. (`platform/db/` hiện trống — pool đang tạo inline trong `cmd/api/main.go`.)
4. [ ] `0009_rls_enable`: `ENABLE + FORCE` RLS + policy `tenant_isolation` trên mọi bảng tenant-scoped; thêm `assets.tenant_id` (+ backfill + policy).
5. [ ] Module `tenant`: middleware `RequireTenant`, `GET /me/organizations`, `POST /auth/switch-tenant`, `POST/GET /admin/organizations`; wire trong `cmd/api` **trước** mọi domain module.
6. [ ] RBAC theo tenant: `user_roles(user_id, org_id, role_id)`; query quyền hiệu lực + cache key scoped theo membership active ([ADR-02]).
7. [ ] `cmd/sysjobs` + `internal/sysrepository` (BYPASSRLS) + luật depguard chặn importer khác.
8. [ ] Test cô lập RLS; mục checklist MODULES.md §6.
9. [ ] Land profile observability cùng sprint ([D-8], [ADR-03]) để thấy latency per-tenant; cập nhật trạng thái `feature.md §2/§18` + sửa `app.tenant_id`→`app.current_tenant`.

**Exit (Phase 1):** một request tới `/api/v1/t/{org}/…` được tenant-scope đầu-cuối; query thô trên role app không đọc được hàng của tenant khác; triển khai single-tenant chạy không cần prefix `/t/`; `sysjobs` (và chỉ `sysjobs`) mới cross tenant được.
