# Authoration — Xác thực, Phân quyền, và Multi-Tenancy

> Đặc tả bảo mật chính tắc cho Portal. Bao gồm identity (authn), quyết định permission (authz), và cô lập tenant (data segregation).
>
> **Tài liệu đi kèm:**
> - [archivetech.md](archivetech.md) — roadmap chức năng đầy đủ (UI, module, phasing)
> - [CLAUDE.md](../../CLAUDE.md) — quyết định kiến trúc + working agreement
>
> Khi tài liệu này xung đột với code, tài liệu thắng. Cập nhật doc như một phần cùng change-set, không bao giờ sau đó.

---

## 0. Decision log

Câu trả lời đã chốt cho các open question nêu trong `archivetech.md §9`:

| # | Câu hỏi | Quyết định |
|---|----------|----------|
| 1 | UX xoá group | **TOTP step-up confirm**: user phải nhập code hiện tại từ Google/Microsoft Authenticator (hoặc tương đương đã enroll) trước khi op huỷ diệt chạy. Áp dụng mọi op hard-delete. |
| 2 | Label "User Role" | **Chỉ cosmetic.** Permission đến từ policy attach vào group + user. Label là metadata. |
| 3 | Policy per-user | **Additive.** Không có deny rule trong v1. |
| 4 | Hết hạn permission file-gated | **Auto cut-off** khi expiry + audit event. Mở lại là hành động admin thủ công (re-review file). |
| 5 | Policy thay đổi giữa chừng | **Invalidate tức thì** qua bump `token_version` + cache key roll-forward. User bị ảnh hưởng nhận in-app + push notification. |
| — | Multi-tenancy | **Shared DB + Row-Level Security (RLS)** keyed theo `tenant_id`, switching tenant yêu cầu TOTP. Schema-per-tenant defer sang "enterprise tier" sau. |

---

## 1. Kiến trúc bảo mật ba lớp

Mỗi request đi qua ba lớp enforce độc lập. Mỗi lớp trả lời đúng một câu hỏi; không overlap.

```text
                ┌──────────────────────────────────────┐
   Request ──►  │  L1 — IDENTITY                       │  Principal này là ai?
                │  (auth middleware: JWT + DB snapshot)│  → auth.Identity trong ctx
                └──────────────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │  L2 — TENANT                         │  Họ đang hành động trong
                │  (tenant middleware: org binding)    │  organization nào? Set DB
                │                                      │  session var. → tenant.Context
                └──────────────────────────────────────┘
                                │
                                ▼
                ┌──────────────────────────────────────┐
                │  L3 — AUTHORIZATION                  │  Trong tenant đó, họ có thể
                │  (rbac.Engine: policy resolution)    │  thực hiện hành động này trên
                │                                      │  resource này? → allow / deny
                └──────────────────────────────────────┘
                                │
                                ▼
                          Handler chạy.
                          Postgres enforce RLS dùng app.current_tenant.
```

### Vì sao ba lớp, không phải một

- Chỉ L1 thì leak data: đã xác thực ≠ đã phân quyền cho data của *tenant này*.
- Chỉ L1 + L3 thì brittle: một permission check sót trong handler là leak data tenant. RLS ở L2 là **defense in depth ở database** — kể cả khi query quên `WHERE tenant_id = $1`, Postgres từ chối.
- Thứ tự quan trọng: L2 phụ thuộc L1 (cần user đã verify để biết họ thuộc tenant nào); L3 phụ thuộc L2 (effective perm khác nhau per tenant với user thuộc nhiều).

---

## 2. Lớp Identity (authentication)

### 2.1 OIDC login flow  *([BUILT])*

Authentik là IdP. Không có local password auth. Flow:

1. `GET /auth/login` — server tạo `state` + `nonce`, set cookie `portal_oidc` ngắn hạn signed bind cả hai, redirect tới IdP.
2. IdP xác thực user (mọi factor đã cấu hình upstream — gồm cả MFA của Authentik).
3. `GET /auth/callback` — server validate state (CSRF) và nonce (chống ID-token replay), exchange code, upsert user.
4. Server phát access + refresh token, set cookie, redirect tới URL post-login.

Implementation chính: [oidc.go](../../backend/internal/auth/oidc.go).

### 2.2 Token

| Token | Tuổi thọ | Storage | Thuật toán | Mục đích |
|-------|----------|---------|-----------|---------|
| Access | 5 phút | cookie `portal_access` HOẶC `Authorization: Bearer` | HS256 với `kid` xoay | Authn per-request |
| Refresh | 30 ngày | cookie `portal_refresh` (`Path=/auth`) HOẶC JSON body | random 256-bit, hash SHA-256 lúc lưu | Mint access token mới |
| Step-up (TOTP) | 5 phút | session-bound; không phải cookie riêng | n/a — flag trên session record | Authorize op huỷ diệt |

Cookie luôn: `HttpOnly; Secure; SameSite=Strict`. Implementation trong [jwt.go](../../backend/internal/auth/jwt.go) và [refresh.go](../../backend/internal/auth/refresh.go).

### 2.3 Hai kênh revoke  *([BUILT])*

Cần cả hai; một mình không đủ.

- **`users.token_version`** — bump nó và mọi access token đang tồn tại fail check DB snapshot trong `RequireAuth`. Instant logout-all.
- **`refresh_tokens.revoked_at`** — revoke phía refresh-token. Chain rotation (`parent_id`/`replaced_by_id`) tuyến tính; trình một token đã rotate revoke **toàn bộ** chain (forward + backward qua recursive CTE) và emit event `auth.refresh.reuse_detected`. Phát hiện trộm.

### 2.4 TOTP / 2FA  *([PLANNED])*

Theo decision-log #1, mọi action admin huỷ diệt yêu cầu confirm TOTP fresh. Implementation:

- **Enrolment**: `POST /auth/totp/enroll` trả về secret base32 + URI provisioning (`otpauth://`). User scan bằng Google/Microsoft/Authy/v.v. Confirm bằng một code hợp lệ → `users.totp_enrolled_at = now()`.
- **Verify**: `POST /auth/totp/verify` nhận code 6 số. Time window: ±1 step (30 s) để absorb clock drift. So sánh **constant-time**.
- **Recovery code**: 10 code dùng một lần, hash (Argon2id). Tạo lúc enrolment; tạo lại theo demand. Mỗi code đã dùng được đánh dấu used ngay lập tức.
- **Step-up flow**: endpoint huỷ diệt yêu cầu header `X-Step-Up-Token: <code>` HOẶC session đã marked `stepped_up_at < 5 phút trước`. Middleware từ chối với `403 step_up_required` nếu không. Frontend prompt code, gọi `POST /auth/totp/verify?intent=step-up`, sau đó retry request gốc.
- **Re-enrolment**: yêu cầu code hiện tại HOẶC recovery code. Admin không thể reset TOTP của user (sẽ phá ý nghĩa); user mất device phải dùng recovery code.

#### Schema delta cho TOTP

```sql
ALTER TABLE users
    ADD COLUMN totp_secret_enc       BYTEA,         -- ciphertext AES-GCM
    ADD COLUMN totp_enrolled_at      TIMESTAMPTZ,
    ADD COLUMN totp_last_verified_at TIMESTAMPTZ;

CREATE TABLE totp_recovery_codes (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id       UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    code_hash     BYTEA NOT NULL,                   -- Argon2id của plaintext
    used_at       TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX totp_recovery_codes_user_idx ON totp_recovery_codes(user_id);
```

TOTP secret được mã hoá bằng key cấp process derive từ env `TOTP_KMS_KEY` (tách khỏi key JWT — blast radius khác). Chỉ decrypt khi verify.

### 2.5 Quản lý session

- `GET /me/sessions` — list refresh token đang active với IP + user-agent (đã hỗ trợ bởi `ListActiveRefreshTokensForUser`).
- `DELETE /me/sessions/{id}` — revoke session cụ thể. Hữu ích khi "tôi để laptop ở văn phòng".
- `POST /auth/logout-all` — revoke mọi refresh + bump `token_version`. Dùng sau khi nghi ngờ compromise.

### 2.6 Map failure mode

Middleware emit generic `401 unauthorized` cho mọi authn failure. Lý do thực ở audit, không phải response body. Tránh oracle về token state.

| Lỗi nội bộ | HTTP | Audit action |
|----------------|------|--------------|
| `ErrTokenInvalid`     | 401 | `auth.token.invalid` |
| `ErrTokenExpired`     | 401 | (bỏ qua — quá ồn) |
| `ErrTokenRevoked`     | 401 | `auth.token.revoked` |
| `ErrUserDisabled`     | 401 | `auth.disabled_user_attempt` |
| `ErrTokenReused`      | 401 | `auth.refresh.reuse_detected` (NGHIÊM TRỌNG) |
| Step-up thiếu/hết hạn | 403 | `auth.stepup.required` |
| TOTP sai            | 401 | `auth.totp.invalid` |

---

## 3. Lớp Tenant (cô lập dữ liệu)

### 3.1 Mô hình Tenant

**Tenant** là ranh giới cô lập dữ liệu top-level. Trong Portal, Tenant ≡ một `organization`. Công ty, bệnh viện, studio, archive — mỗi cái một organization, cô lập data cứng.

```text
                 ┌──────────────────┐
                 │   Organization   │  ◄── Ranh giới Tenant. RLS enforce.
                 └────────┬─────────┘
        sub-orgs (opt.)   │
                          ▼
                 ┌──────────────────┐
                 │ Sub-organization │  phân cấp (parent_org_id), cùng tenant
                 └────────┬─────────┘
                          ▼
                 ┌──────────────────┐
                 │   User Group     │  xem archivetech.md §3.1
                 └────────┬─────────┘
                          ▼
                 ┌──────────────────┐
                 │      User        │
                 └──────────────────┘
```

Một user có thể là member của nhiều organization (vd: auditor freelance làm với nhiều phòng khám). Mỗi membership có role/policy riêng. Organization active cho session là phần của JWT và chọn lúc login hoặc qua switch tường minh.

### 3.2 Schema cho tenancy

```sql
CREATE TABLE organizations (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    code            TEXT UNIQUE NOT NULL,           -- slug ngắn vd 'acme-clinic'
    name            TEXT NOT NULL,
    parent_org_id   UUID REFERENCES organizations(id) ON DELETE RESTRICT,
    tier            TEXT NOT NULL DEFAULT 'standard', -- 'standard' | 'enterprise'
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE organization_memberships (
    id                UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id   UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id           UUID NOT NULL REFERENCES users(id)         ON DELETE CASCADE,
    is_default        BOOLEAN NOT NULL DEFAULT false, -- chọn lúc login nếu không tường minh
    invited_by        UUID REFERENCES users(id) ON DELETE SET NULL,
    joined_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (organization_id, user_id)
);
CREATE INDEX organization_memberships_user_idx ON organization_memberships(user_id);
```

### 3.3 Truyền `tenant_id`

Mỗi table tenant-scoped mang `organization_id` và RLS policy tương ứng.

| Table | Column `organization_id` | RLS enforce |
|-------|-------------------------|--------------|
| `user_groups` | YES | YES |
| `user_group_members` | inherit qua group | YES |
| `policies` | YES (policy system-wide dùng NULL — special case) | YES |
| `group_policy_attachments` | inherit qua group | YES |
| `user_policy_attachments` | YES (denormalize cho RLS) | YES |
| `assets` | YES | YES |
| `movies`, `music`, `stories` | YES | YES |
| `audit_log` | YES (NULL cho event system) | YES |
| `users` | NO — identity global | n/a (read scope qua join membership) |
| `refresh_tokens` | NO — bound vào user, không tenant | n/a |
| `organizations`, `organization_memberships` | n/a | policy đặc biệt |

**Vì sao `users` global**: một người là một người across org. Email và OIDC subject identify họ một lần. Truy cập của họ trong một org cụ thể được mediate qua `organization_memberships`. Denormalize `organization_id` lên users sẽ buộc unique user per org — sai shape.

### 3.4 RLS PostgreSQL — cơ chế enforce

Với mỗi table tenant-scoped:

```sql
ALTER TABLE assets ENABLE ROW LEVEL SECURITY;

CREATE POLICY assets_tenant_isolation ON assets
    USING (organization_id = current_setting('app.current_tenant')::uuid);

CREATE POLICY assets_tenant_insert ON assets
    FOR INSERT
    WITH CHECK (organization_id = current_setting('app.current_tenant')::uuid);
```

Middleware app set `app.current_tenant` per request, **trước khi** query nào chạy:

```go
// pseudocode trong tenant middleware
conn.Exec(ctx, "SELECT set_config('app.current_tenant', $1, true)", tenantID)
```

`set_config(..., true)` làm setting transaction-local — không leak across pooled connection. Kết hợp với **PgBouncer ở chế độ transaction pooling**, an toàn.

#### Bypass cho operation system

Một số ít job background (gom audit cross-tenant, billing rollup) cần đọc across tenant. Họ dùng role chuyên dụng `portal_system` đã set `BYPASSRLS`, VÀ các op đó sống trong binary Go riêng (`cmd/sysjobs/`) không bao giờ phục vụ user traffic. **API server không bao giờ connect bằng role này.**

### 3.5 Switching tenant

User có nhiều membership chọn một lúc login (default-selected nếu `is_default=true`). Để switch:

```text
POST /auth/switch-tenant
  body: { "organization_id": "..." }
```

Server validate membership, **yêu cầu TOTP fresh nếu user đã enroll**, mint access token mới với claim `org_id` mới, và bump `token_version` cho token *trước đó* (để không thể dùng access tenant cũ sau switching).

### 3.6 Administrator cross-tenant

Role `superadmin` là *cấp system*, không phải cấp tenant. Sống trong "system organization" ảo (`organization_id = NULL` cho match policy system). Cụ thể:

- User với `superadmin` có thể switch vào org bất kỳ qua `POST /auth/switch-tenant` mà không cần là member, **với TOTP step-up luôn bắt buộc**.
- Session của họ flag `system_impersonation = true`; mọi action audit với cả identity của họ và tenant đang impersonate.
- Tenant admin không thể tự grant `superadmin`. Bootstrap yêu cầu `cmd/admin grant-superadmin` (CLI), bản thân nó yêu cầu runtime secret không có sẵn cho API process.

---

## 4. Lớp Authorization

### 4.1 Tóm tắt mô hình kiểm soát truy cập

(Chi tiết trong `archivetech.md §2`. Lặp ở đây như đơn vị quyết định của tài liệu này.)

```text
        Group hierarchy             Policies (bundle reusable)
        bên trong một Org            attach vào Group HOẶC User
              │                                  │
              └──────────────┬───────────────────┘
                             ▼
                    Effective permission set
                     cho (user, organization)
                             │
                             ▼
              rbac.Engine.Authorize(...)  ← điểm quyết định duy nhất
```

### 4.2 Resolve effective permission

Với mỗi `(user_id, organization_id)`, tính tập theo thứ tự, **per request, cached**:

1. Tìm `organization_membership` của user cho tenant này. Nếu không có → deny mọi thứ.
2. Tìm mọi User Group user thuộc về (qua `user_group_members`).
3. Với mỗi group, walk chain parent (group → parent group → root). Policy của mỗi ancestor cũng áp dụng.
4. Collect mọi policy **active** đã attach vào group trên đường đi (`group_policy_attachments` JOIN `policies` on `is_active = true`).
5. Thêm mọi policy **active** attach trực tiếp vào user (`user_policy_attachments` scope cùng org).
6. Expand mỗi policy → permission (`policy_permissions`). Với permission `requires_file = true`, bỏ trừ khi user có row `user_permission_files` tương ứng với `status = 'approved'` và `expires_at > now()`.
7. Apply rule wildcard / scope từ [permission.go](../../backend/internal/rbac/permission.go).

Cache trong Redis dưới key `rbac:perms:<userID>:<orgID>:v<token_version>`. TTL 5 phút. **Bump `token_version` là kênh invalidation chính tắc duy nhất.**

### 4.3 Versioning policy + notify user

Theo decision-log #5, khi policy đột biến:

1. Handler đột biến update `policies` / `policy_permissions`.
2. Nó tính **tập user bị ảnh hưởng** bằng cách join `policies → group_policy_attachments → user_group_members → users` (transitively lên trên hierarchy group) và `user_policy_attachments → users`.
3. Với mỗi user bị ảnh hưởng, bump `users.token_version`. Điều này invalidate access token của họ ở request kế tiếp và roll cache key forward.
4. Enqueue task Asynq `notify:policy_changed` per user bị ảnh hưởng. Worker notification:
    - Ghi row notification in-app (table `notifications` — sẽ định nghĩa).
    - Fire Web Push nếu user đã subscribe (key VAPID per `web_push_subscriptions`).
    - Với thay đổi high-impact (grant/revoke permission mới mà user đang dùng tích cực), cũng ghi audit event tham chiếu actor.

```text
Policy P thay đổi
   │
   ├─► RLS-isolated tìm user bị ảnh hưởng (trong org sở hữu)
   ├─► Bump token_version cho mỗi
   ├─► Với mỗi: enqueue notify:policy_changed
   │        └─► row notification in-app
   │        └─► web push nếu subscribed
   └─► audit event: rbac.policy.updated
```

### 4.4 Permission file-gated — auto cut-off + audit

Theo decision-log #4. Chạy như task Asynq định kỳ (mỗi 5 phút):

```text
  cron: rbac:expire_files
   │
   ├─► SELECT user_permission_files
   │     WHERE status='approved' AND expires_at < now()
   │
   ├─► Với mỗi row:
   │     UPDATE ... SET status='expired'
   │     bump users.token_version
   │     audit: rbac.file.expired
   │     enqueue notify:perm_lost
```

Mở lại = admin re-review (hoặc user re-upload), file đi qua `pending → approved` lại. Row expired cũ ở lại history; không bao giờ xoá.

### 4.5 Xoá group với TOTP step-up

Theo decision-log #1. Endpoint `DELETE /admin/groups/{id}`:

1. Yêu cầu permission `rbac:role:write` (hoặc perm quản lý group tương đương) — authz tiêu chuẩn.
2. Thêm yêu cầu step-up: hoặc header `X-Step-Up-Token: <6 số>` HOẶC session flag được set trong 5 phút gần nhất.
3. On success: cascade xoá con (theo `archivetech.md`), bump `token_version` cho mọi member của mọi group bị ảnh hưởng, audit `rbac.group.deleted` với field metadata liệt kê mọi group con đã cascade.
4. Nếu actor thiếu enrolment TOTP, endpoint trả về `403 totp_required` và frontend redirect tới `/account/security` để enroll.

Pattern này (`requireStepUp`) bọc mọi op huỷ diệt khác:
`DELETE /admin/policies/{id}`, `DELETE /admin/users/{id}`, `POST /auth/logout-all`, `POST /auth/switch-tenant` (khi org nguồn có perm elevated), `cmd/admin grant-superadmin`.

---

## 5. Quan tâm cross-cutting

### 5.1 Audit  *([BUILT] core; UI [PLANNED])*

Mọi event nhạy cảm bảo mật được ghi vào `audit_log` (append-only). Xem [audit/logger.go](../../backend/internal/audit/logger.go). Action code dotted, vd `auth.login`, `rbac.policy.updated`, `tenant.switched`, `auth.totp.verified`. **Failure ồn ào nhưng non-blocking** cho user request.

Thêm cho multi-tenancy: mỗi row audit mang `organization_id` (NULL cho event system). Migration delta:

```sql
ALTER TABLE audit_log
    ADD COLUMN organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL;
CREATE INDEX audit_log_org_idx ON audit_log(organization_id, occurred_at DESC);
```

### 5.2 Rate limiting  *([BUILT] in-memory; Redis-backed [PLANNED])*

Token bucket per IP cho `/auth/*`. Bucket nghiêm hơn per `(IP, action)` cho endpoint nhạy cảm (TOTP verify: 5/phút/IP+user, lockout 15 phút sau 5 lần fail). Implementation trong [ratelimit.go](../../backend/internal/middleware/ratelimit.go); cho production, đổi in-memory store thành `redis_rate.Limiter`.

### 5.3 Xử lý secret

- Key signing JWT: env `JWT_SIGNING_KEYS`, list xoay. Key active sign token mới; key cũ vẫn hợp lệ để verify trong rotation window.
- Key encrypt TOTP: env `TOTP_KMS_KEY` riêng. Blast radius khác với JWT.
- Client secret OIDC: `OIDC_CLIENT_SECRET` — không bao giờ log.
- Production: Doppler quản lý mọi thứ trên; deployment không bao giờ đọc `.env` từ disk.

### 5.4 Channel notification

Dùng bởi §4.3 policy-change notification và §4.4 file-expiry notification.

```sql
CREATE TABLE notifications (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    kind            TEXT NOT NULL,                -- 'policy_changed' | 'perm_lost' | ...
    title           TEXT NOT NULL,
    body            TEXT NOT NULL,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    read_at         TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX notifications_user_unread_idx
    ON notifications(user_id, created_at DESC)
    WHERE read_at IS NULL;

CREATE TABLE web_push_subscriptions (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    endpoint        TEXT NOT NULL,
    p256dh_key      TEXT NOT NULL,
    auth_key        TEXT NOT NULL,
    user_agent      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

Frontend dùng TanStack Query + channel SSE `/me/notifications/stream` để delivery real-time, fallback sang Web Push khi browser đóng (VAPID).

---

## 6. Pipeline middleware

Theo thứ tự — áp dụng cho mọi route đã authenticate:

```text
1. RealIP            ← preserve X-Forwarded-For (Traefik trusted)
2. RequestID         ← unique per request, trong log + audit
3. Recoverer         ← bắt panic, trả 500
4. Timeout(30s)      ← lifetime request giới hạn
5. CORS              ← allowlist origin từ config
6. RateLimit         ← token bucket per-IP; nghiêm hơn trên /auth/*
7. RequireAuth       ← JWT + DB snapshot; set auth.Identity vào ctx
8. RequireTenant     ← đọc org_id từ JWT; set app.current_tenant trên DB conn; set tenant.Context vào ctx
9. RequireStepUp     ← (tuỳ chọn, per-route) — verify TOTP fresh cho op huỷ diệt
10. RequirePermission ← rbac.Engine.Authorize, trả 403 on deny
11. Handler
12. AuditMiddleware  ← (deferred) ghi audit event cho op mutate
```

Route public (vd `GET /movies` cho guest) skip 7–10. Một vài route "tenant-scoped nhưng public-readable" dùng `OptionalAuth` + `RequireTenant`.

---

## 7. API surface (auth + tenant)

```text
# Authentication
GET    /auth/login                     bắt đầu flow OIDC
GET    /auth/callback                  hoàn tất OIDC; mint token
POST   /auth/refresh                   xoay refresh; mint access
POST   /auth/logout                    revoke refresh hiện tại; bump token_version
POST   /auth/logout-all                revoke mọi refresh; bump token_version  [step-up]

# 2FA (TOTP)
POST   /auth/totp/enroll               bắt đầu enrolment; trả secret + QR URI
POST   /auth/totp/verify               verify code; activate enrolment HOẶC step-up
POST   /auth/totp/recovery-codes/regen tạo lại recovery code  [step-up]
DELETE /auth/totp                      disenrol  [step-up + recovery-code]

# Tenant
GET    /me/organizations               list org user thuộc về
POST   /auth/switch-tenant             switch org active; mint token mới  [step-up nếu elevated]

# Identity
GET    /auth/me                        user hiện tại + role + org context

# Session
GET    /me/sessions                    list refresh token active (device)
DELETE /me/sessions/{id}               revoke session cụ thể  [step-up]

# Notification
GET    /me/notifications               list (phân trang, lọc unread)
POST   /me/notifications/read          đánh dấu ID đã đọc
GET    /me/notifications/stream        channel SSE cho update live
POST   /me/web-push/subscribe          register subscription push browser
DELETE /me/web-push/{id}               unsubscribe
```

OpenAPI source-of-truth tại [shared/openapi.yaml](../../shared/openapi.yaml). Mỗi endpoint annotate permission yêu cầu qua `x-required-permission` và yêu cầu step-up qua `x-step-up: true`.

---

## 8. Threat model

Cái chúng ta defend rõ ràng, và cách defend.

| Mối đe doạ | Phòng thủ |
|--------|---------|
| Access token bị trộm | TTL ngắn (5 phút) + check DB snapshot mỗi request (`token_version`) — revoke tức thì. |
| Refresh token bị trộm | Rotation per use + reuse detection burn chain. Hash khi lưu. |
| Hijack session qua XSS | Cookie `HttpOnly`; CSP enforce server-side. Không bao giờ expose token cho JS. |
| CSRF | Cookie `SameSite=Strict`. Login dùng `Lax` chỉ cho redirect IdP; callback OIDC verify state. |
| Leak data cross-tenant qua bug app | RLS enforce trong Postgres. `app.current_tenant` set transactionally per request. Role `BYPASSRLS` cô lập vào binary Go riêng. |
| Escalation quyền bởi admin tenant | `superadmin` là role system, không bao giờ grant được từ context tenant. Chỉ bootstrap CLI. |
| Brute force TOTP | Code 6 số + 5 lần thử/15-phút lockout per user; so sánh constant-time. Recovery code single-use, hash Argon2id. |
| Replay refresh token across device | Mỗi refresh token ghi IP phát hành + UA. Reuse từ fingerprint khác emit audit event severity cao hơn (vẫn revoke chain). |
| Replay OIDC ID-token | Nonce validate against cookie bound. State validate session origin (CSRF). |
| Extract TOTP secret khi at rest | Mã hoá với `TOTP_KMS_KEY` riêng; chỉ decrypt in-memory lúc verify. |
| Sửa đổi audit log | Append-only ở app layer. Retention dài hạn sang bucket archive R2 (policy bucket immutable). |
| Poisoning cache permission | Key cache Redis gồm `token_version` và `org_id`; mutation bump version → buộc re-fetch từ DB. |
| Insider có quyền write DB | Replication `audit_log` sang bucket R2 write-once (credentials riêng). Forward log out-of-band sang SIEM. |

Cái chúng ta **KHÔNG** defend (out of scope cho v1):

- Compromise của Postgres host bản thân nó.
- Compromise của IdP Authentik (chúng ta tin issuer của nó).
- Một administrator quyết tâm bên trong tenant exfiltrate data của tenant đó (sử dụng hợp pháp).
- DDoS — xử lý ở Cloudflare, không phải ở đây.

---

## 9. Roadmap migration

Số để hợp sequence migration đang có trong `backend/db/migrations/`.

| # | File | Mục đích |
|---|------|---------|
| 0001 | `init.up.sql` | [BUILT] table foundational users + assets |
| 0002 | `rbac.up.sql` | [BUILT] roles + permissions + refresh_tokens + audit_log |
| 0003 | `organizations.up.sql` | [PLANNED] organizations + organization_memberships; scaffolding RLS |
| 0004 | `user_groups.up.sql` | [PLANNED] user_groups + user_group_members (org-scoped) |
| 0005 | `policies.up.sql` | [PLANNED] policies + policy_permissions + group_policy_attachments + user_policy_attachments |
| 0006 | `file_gated_permissions.up.sql` | [PLANNED] user_permission_files + workflow review |
| 0007 | `totp.up.sql` | [PLANNED] users.totp_*, totp_recovery_codes |
| 0008 | `notifications.up.sql` | [PLANNED] notifications + web_push_subscriptions |
| 0009 | `rls_enable.up.sql` | [PLANNED] enable RLS + policy trên mọi table tenant-scoped |
| 0010 | `audit_log_org.up.sql` | [PLANNED] thêm organization_id vào audit_log |

RLS chủ ý enable trong **một migration riêng, muộn** để development sớm có thể chạy không phiền hà RLS. Production deployment phải bao gồm 0009 trước khi go live; CI gate verify nó.

---

## 10. Pointer implementation

### 10.1 Skeleton tenant middleware  *([PLANNED])*

```go
// internal/middleware/tenant.go
//
// RequireTenant resolve organization active cho request, validate
// membership của user, và bind app.current_tenant trên DB connection
// trong suốt lifetime transaction của request.
func RequireTenant(memberships MembershipFetcher, db DB) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, _ := auth.FromContext(r.Context())
            orgID := orgFromJWT(id) // claim 'org_id'
            if orgID == uuid.Nil {
                writeJSONError(w, 400, "tenant_missing", "no active organization")
                return
            }
            ok, err := memberships.IsMember(r.Context(), id.UserID, orgID)
            if err != nil || !ok {
                writeJSONError(w, 403, "tenant_denied", "not a member of this organization")
                return
            }
            // Bind RLS guard cho phần còn lại của request này.
            ctx, cleanup, err := db.BeginTenantScope(r.Context(), orgID)
            if err != nil {
                writeJSONError(w, 500, "internal", "tenant scope failed")
                return
            }
            defer cleanup()
            r = r.WithContext(tenant.WithOrg(ctx, orgID))
            next.ServeHTTP(w, r)
        })
    }
}
```

### 10.2 Skeleton step-up middleware  *([PLANNED])*

```go
// internal/middleware/stepup.go
func RequireStepUp(verifier *auth.TOTPVerifier, store StepUpStore, ttl time.Duration) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            id, _ := auth.FromContext(r.Context())
            // Path 1: header với một TOTP code hiện tại
            if code := r.Header.Get("X-Step-Up-Token"); code != "" {
                if err := verifier.Verify(r.Context(), id.UserID, code); err == nil {
                    store.MarkStepUp(r.Context(), id.TokenID, time.Now())
                    next.ServeHTTP(w, r)
                    return
                }
                writeJSONError(w, 401, "totp_invalid", "")
                return
            }
            // Path 2: session đã step-up gần đây
            if t, ok := store.LastStepUp(r.Context(), id.TokenID); ok && time.Since(t) < ttl {
                next.ServeHTTP(w, r)
                return
            }
            writeJSONError(w, 403, "step_up_required", "this action requires a fresh TOTP code")
        })
    }
}
```

### 10.3 Discipline test RLS

Mỗi PR thêm table tenant-scoped phải bao gồm integration test:

1. Insert row với `organization_id = A` trong khi `app.current_tenant = A` — thành công.
2. Switch sang `app.current_tenant = B` — `SELECT *` trả 0 row; `INSERT ... organization_id = A` bị reject.
3. Connect như `portal_system` (`BYPASSRLS`) — thấy cả hai tenant.

Đặt dưới `backend/internal/repository/rls_test.go`. Không cho CI green nếu thiếu.

---

## 11. Bảng thuật ngữ

- **Tenant** — đơn vị cô lập dữ liệu. Trong Portal, ≡ Organization.
- **Step-up auth** — confirm authenticator fresh yêu cầu cho op huỷ diệt, trên session đã hợp lệ.
- **RLS** — PostgreSQL Row-Level Security. Filter clause apply tự động lên mọi query against một table.
- **TOTP** — Time-based One-Time Password (RFC 6238). Cái Google/Microsoft Authenticator phát.
- **Effective permission set** — union dedup, file-gated, scope-aware của mọi permission grant cho user *cho một organization cụ thể*.
- **Token version (`tv`)** — counter monotonic trên `users` mà khi bump, invalidate mọi access token đang tồn tại của user đó mà không cần revoke từng token một.
