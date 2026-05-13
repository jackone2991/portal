# ArchiveTech — Đặc tả chức năng hệ thống

> Tài liệu đặc tả tính năng chính thức cho hệ sinh thái Portal + nền tảng đa phương tiện.
> Bắt nguồn từ các UI mock trong `template-main/portal/document/anh{1,2,3}.png`
> và từ các quyết định kiến trúc đã ghi trong [CLAUDE.md](../../CLAUDE.md).
>
> **Status legend** dùng xuyên suốt tài liệu:
>
> - **[BUILT]**  — code đã có trong `backend/`
> - **[PARTIAL]** — đã làm một phần; schema hoặc interface đã có nhưng chưa wire
> - **[PLANNED]** — đã thiết kế nhưng chưa có code
>
> Khi spec xung đột với code hiện tại, **spec thắng** — điều chỉnh code, không phải ngược lại. Cập nhật tài liệu này khi quyết định thay đổi.

---

## 1. Tầm nhìn

Một nền tảng media self-hosted với **hệ thống kiểm soát truy cập phân cấp, fine-grained**, đủ tốt cho triển khai cấp tổ chức (phòng khám, studio, archive) — không chỉ tài khoản người dùng cá nhân. Ba trụ cột:

1. **Domain media** — Movies, Music, Stories. Pipeline upload → transcode → stream.
2. **Kiểm soát truy cập tổ chức** — User Group, User, User Role, Policy, Permission file-gated.
3. **Tính toàn vẹn vận hành** — audit trail đầy đủ, revoke tức thì, OIDC SSO, không có secret hard-coded.

Các mock thể hiện ArchiveTech là một hệ thống **policy-driven, group-scoped** — không phải hệ thống role phẳng. Mô hình dữ liệu bên dưới phản ánh điều đó.

---

## 2. Mô hình kiểm soát truy cập cốt lõi

Các mock giới thiệu bốn entity tương tác với nhau:

```text
                 ┌────────────┐
                 │ User Group │  phân cấp (parent_id), có thể duplicate
                 └─────┬──────┘
              members  │  có policies
                ┌──────┴──────┐
                ▼             ▼
            ┌──────┐     ┌────────┐
            │ User │     │ Policy │  bundle permission tái sử dụng
            └──┬───┘     └───┬────┘
   has policies│             │ has permissions
               ▼             ▼
            ┌──────┐     ┌────────────┐
            │Policy│ ◄── │ Permission │  atomic; một số là file-gated
            └──────┘     └────────────┘
```

### 2.1 Định nghĩa

| Thuật ngữ | Định nghĩa | Cardinality |
|------|-----------|-------------|
| **User Group** | Container tổ chức. Có thể là parent của các group khác (phòng ban → team → squad). Chứa user, định nghĩa policy set riêng. Source-of-truth cho "ai có thể hành động trong scope này". | Phân cấp |
| **User** | Principal đã được xác thực. Thuộc ≥1 User Group. Có thể mang **per-user policy** override hoặc mở rộng group policy. | Many-to-many với group |
| **User Role** | Một label bên trong User Group (Manager, Junior, Reviewer). Hiện trình bày như một role group-scoped. **Ghi chú implementation**: model như một bundle Policy gắn vào quan hệ user-trong-group, không phải table role độc lập — tránh ambiguity "role global vs scoped". | Group-scoped |
| **Policy** | Một tập Permission có tên, tái sử dụng được ("Radiologist", "Read-Only Auditor"). Kích hoạt/tắt độc lập với grant. Có thể attach vào User Group hoặc User. | 1 policy → nhiều perm |
| **Permission** | Token hành động nguyên tử: `<resource>:<action>[:<scope>]`. Một số permission là **file-gated**: cần một file (license, certificate, signed agreement) đã upload để có hiệu lực. | Đơn vị nhỏ nhất |

### 2.2 Giải quyết effective-permission

Tính effective set của user theo thứ tự, sau đó **deduplicate**:

1. Walk theo nhánh tổ tiên User Group (group → parent → root).
2. Với mỗi group trên đường đi, collect mọi policy **active** đã attach vào đó.
3. Thêm mọi policy **active** attach trực tiếp vào user.
4. Với mỗi policy, expand thành các permission, **lọc bỏ permission file-gated mà file required đang thiếu hoặc hết hạn**.
5. Apply rule wildcard / scope từ [permission.go](../../backend/internal/rbac/permission.go).

Cache theo `(user_id, token_version)` trong Redis. Bump `users.token_version` là kênh invalidation chính tắc.

### 2.3 Xung đột & precedence

- **Deny chưa nằm trong scope.** Policy chỉ grant. Nếu hai đường dẫn vừa deny vừa allow cùng một permission, allow thắng. Giữ logic dễ suy luận; xem xét lại nếu/khi compliance khắt khe đòi hỏi.
- **Per-user policy là additive**, không phải override. Nếu user ở trong group có `movies:read` và có personal policy `movies:write:own`, họ có cả hai.
- **File-gated permission biến mất silently** khi file hết hạn. Audit log ghi lại thời điểm permission mất hiệu lực.

---

## 3. Danh mục module

Tag module map đến các màn hình trong `anh1/2/3.png`.

### 3.1 Module: Quản lý User Group  *(anh1, anh3)*

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Liệt kê user group (grid view, search, phân trang) | [PLANNED] | Trang trên của anh1 |
| Tạo user group (modal, selector parent group) | [PLANNED] | Modal trong anh3; auto-set `parent_id` từ view hiện tại |
| Mở profile group (overview + members + policies) | [PLANNED] | Trang giữa anh1 |
| Sửa description + metadata group | [PLANNED] | |
| **Xoá group** — cascade xuống các group con *(theo anh1: "deleting child groups eradicated")* | [PLANNED] | Hard delete; cảnh báo và yêu cầu gõ code group; audit event bắt buộc |
| **Duplicate group** | [PLANNED] | Deep-copy: tạo group mới dưới cùng parent + clone các attachment policy + (tuỳ chọn) clone member |
| Attach / detach Policy vào group (modal search với preview) | [PLANNED] | Flow modal search trong anh3 |
| Hiển thị policy inherited (badge read-only từ ancestor group) | [PLANNED] | Quan trọng để hiểu effective perm |

Schema delta (đã planned):

```sql
CREATE TABLE user_groups (
    id          UUID PRIMARY KEY,
    code        TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    description TEXT,
    parent_id   UUID REFERENCES user_groups(id) ON DELETE CASCADE,
    -- field soft cho flow duplicate
    cloned_from UUID REFERENCES user_groups(id) ON DELETE SET NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE user_group_members (
    group_id    UUID REFERENCES user_groups(id) ON DELETE CASCADE,
    user_id     UUID REFERENCES users(id)       ON DELETE CASCADE,
    role_label  TEXT,                  -- 'Manager', 'Junior', v.v. (nullable)
    joined_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, user_id)
);
```

### 3.2 Module: Quản lý User  *(anh1, anh3)*

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Trang profile per-user (gần như "super-page" của group profile) | [PLANNED] | Trang dưới của anh1 |
| Bảng policy inline cho user (grant/revoke policy cá nhân) | [PLANNED] | Bảng độc quyền theo anh1 |
| Tạo user bên trong một group (modal: name, email, profile type) | [PLANNED] | Modal create trong anh3 |
| Di chuyển user giữa các group | [PLANNED] | Cần bump `token_version` để buộc re-resolve |
| Disable / enable user | [BUILT] | `users.disabled_at` + query `DisableUser`/`EnableUser` |
| Xoá user (cascade xuống policy) | [PLANNED] | |
| Liệt kê effective permission của user (debug view) | [PLANNED] | Quan trọng cho support; hiển thị chuỗi nguồn policy |

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

### 3.3 Module: Quản lý Policy  *(anh2)*

Tab "Policies" là một catalog phẳng các bundle permission tái sử dụng.

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Danh sách policy (card với preview checkbox) | [PLANNED] | anh2 trên |
| Tạo policy (name + description + functionality blurb) | [PLANNED] | "+ CREATE NEW POLICY" |
| Activate / deactivate policy global | [PLANNED] | Policy disabled bị bỏ qua khi resolve effective-perm |
| Trang chi tiết policy (functionality + danh sách permission) | [PLANNED] | anh2 giữa |
| Thêm permission vào policy | [PLANNED] | "+ ADD NEW PERMISSION" — row inline xuất hiện (anh2 dưới) |
| Xoá permission khỏi policy | [PLANNED] | |
| Xoá policy (kèm cascade audit) | [PLANNED] | Từ chối nếu đã attach ở đâu đó; buộc user detach trước |
| Duplicate policy | [PLANNED] | Cùng shape như duplicate group |

Schema delta:

```sql
CREATE TABLE policies (
    id           UUID PRIMARY KEY,
    code         TEXT UNIQUE NOT NULL,
    name         TEXT NOT NULL,
    description  TEXT,
    functionality TEXT,                -- đoạn blurb dài trong anh2
    is_active    BOOLEAN NOT NULL DEFAULT true,
    is_system    BOOLEAN NOT NULL DEFAULT false,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE policy_permissions (
    policy_id     UUID REFERENCES policies(id)    ON DELETE CASCADE,
    permission_id UUID REFERENCES permissions(id) ON DELETE CASCADE,
    -- permission file-gated: cần file đã upload để enable
    requires_file BOOLEAN NOT NULL DEFAULT false,
    file_label    TEXT,                -- 'Radiologist license', 'NDA', v.v.
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

### 3.4 Module: Quản lý Permission *(anh2)*

Đơn vị nguyên tử. Hầu hết được seed; ít khi user chỉnh sửa.

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Catalog permission (read-only với hầu hết user) | [BUILT] | Migration 0002 đã seed 36 permission |
| Tạo permission custom (chỉ admin) | [BUILT] | Query `CreatePermission` đã có |
| **Hạ tầng permission file-gated** | [PLANNED] | Mới: table `user_permission_files` theo dõi user nào đã upload file nào cho permission nào |
| Verify file đã upload (queue review thủ công) | [PLANNED] | Admin phải duyệt upload trước khi perm file-gated kích hoạt |
| Hết hạn file & nhắc renewal | [PLANNED] | Cron job; emit audit event khi perm mất hiệu lực |

Schema delta cho permission file-gated:

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

### 3.5 Module: Authentication  *(không có mock — backend-only)*

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| OIDC login (Authentik) với state + nonce | [BUILT] | [oidc.go](../../backend/internal/auth/oidc.go) |
| Access token (HS256, `kid` xoay, 5 phút) | [BUILT] | [jwt.go](../../backend/internal/auth/jwt.go) |
| Refresh token rotation + reuse detection | [BUILT] | [refresh.go](../../backend/internal/auth/refresh.go) |
| Logout / logout-all | [BUILT] | [auth.go handler](../../backend/internal/handler/auth.go) |
| Danh sách session per user (devices + revoke) | [PARTIAL] | Query đã có (`ListActiveRefreshTokensForUser`); UI [PLANNED] |
| Wire issuer + verifier + handler trong `cmd/api/main.go` | [PLANNED] | Block bởi `make sqlc` |
| Repository adapter (UserUpserter, RefreshStore, v.v.) | [PLANNED] | Bao bọc code sqlc-generated |

### 3.6 Module: Search & Discovery *(anh3)*

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Modal search policy với preview tập permission | [PLANNED] | anh3 dưới — search theo name; modal preview perm trước khi commit |
| Search user (theo email, theo group) | [PLANNED] | |
| Search audit log (theo actor, action, time range) | [PLANNED] | Query đã có (`ListAuditEvents`) |
| Global search bar (header trong mọi screenshot) | [PLANNED] | Cross-entity: groups, policies, users — TanStack Query + debounce |

### 3.7 Module: Audit & Compliance

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Audit log table append-only | [BUILT] | Migration 0002 |
| Audit logger best-effort writes | [BUILT] | [audit/logger.go](../../backend/internal/audit/logger.go) |
| UI viewer audit (table + filters) | [PLANNED] | Hạn chế với `audit:read` |
| Export range audit (CSV/JSON) | [PLANNED] | Job async — range lớn không nên block API |
| Retention policy & archival vào cold storage | [PLANNED] | Bucket archive Cloudflare R2 |

### 3.8 Module: Domain Media (movies / music / stories)

Out of scope cho redesign access-control nhưng liệt kê cho đầy đủ; chức năng từ [README.md](README.md) gốc (đã xoá) được chuyển tiếp.

| Tính năng | Trạng thái | Ghi chú |
|---------|--------|-------|
| Upload asset (S3 multipart với presigned URL) | [PARTIAL] | OpenAPI đã định nghĩa; handler [PLANNED] |
| Worker transcode (FFmpeg → HLS) | [PARTIAL] | Stub trong [transcode.go](../../backend/internal/worker/transcode.go) |
| Worker thumbnail | [PARTIAL] | Stub trong [thumbnail.go](../../backend/internal/worker/thumbnail.go) |
| CRUD Movies / Music / Stories | [PLANNED] | Package domain dưới `internal/domain/` |
| Tích hợp Vidstack player trên frontend | [PLANNED] | |
| Comment, rating, watchlist | [PLANNED] | Permission-gated qua `comments:write` / `comments:delete:*` |
| Search xuyên content (Postgres FTS → Meilisearch) | [PLANNED] | |

---

## 4. Danh mục page UI

Map screenshot sang React Server Component / Page trên frontend.

| Page | Mock | Path (planned) |
|------|------|----------------|
| Danh sách User Group | anh1 trên | `app/admin/groups/page.tsx` |
| Profile User Group | anh1 giữa | `app/admin/groups/[id]/page.tsx` |
| Profile User (admin view) | anh1 dưới | `app/admin/users/[id]/page.tsx` |
| Modal Create User Group | anh3 | `components/admin/CreateGroupDialog.tsx` |
| Modal Create User Profile | anh3 | `components/admin/CreateUserDialog.tsx` |
| Danh sách Policy | anh2 trên | `app/admin/policies/page.tsx` |
| Chi tiết Policy | anh2 giữa | `app/admin/policies/[id]/page.tsx` |
| Row inline thêm permission | anh2 dưới | inline state trên policy detail |
| Modal search Policy | anh3 dưới | `components/admin/PolicySearchDialog.tsx` |
| Viewer audit log | (không có mock) | `app/admin/audit/page.tsx` |
| Quản lý session/device | (không có mock) | `app/account/sessions/page.tsx` |

Tất cả admin route đi qua middleware `RequirePermission` trên backend; frontend thêm vào đó ẩn UI affordance mà user không có permission code tương ứng. Server luôn là source-of-truth.

---

## 5. API surface delta

Ngoài các auth endpoint đã có trong [shared/openapi.yaml](../../shared/openapi.yaml):

```text
POST   /admin/groups                    create
GET    /admin/groups                    list
GET    /admin/groups/{id}               profile (members, policy đã attach, policy inherited)
PATCH  /admin/groups/{id}               edit
DELETE /admin/groups/{id}               delete (cascade)
POST   /admin/groups/{id}/duplicate     deep-copy
POST   /admin/groups/{id}/members       thêm user
DELETE /admin/groups/{id}/members/{u}   xoá user
POST   /admin/groups/{id}/policies      attach policy
DELETE /admin/groups/{id}/policies/{p}  detach policy

POST   /admin/policies                  create
GET    /admin/policies                  list
GET    /admin/policies/{id}             detail
PATCH  /admin/policies/{id}             edit (gồm activate/deactivate)
DELETE /admin/policies/{id}             delete (từ chối nếu đã attach)
POST   /admin/policies/{id}/duplicate
POST   /admin/policies/{id}/permissions          thêm permission
DELETE /admin/policies/{id}/permissions/{p}      xoá permission

POST   /admin/users/{id}/policies                attach policy cá nhân
DELETE /admin/users/{id}/policies/{p}            detach
GET    /admin/users/{id}/effective-permissions   debug view (hiển thị chuỗi nguồn)

POST   /me/permission-files             upload file cho perm file-gated
GET    /me/permission-files             list file của mình + status
GET    /admin/permission-files/pending  queue review (admin)
POST   /admin/permission-files/{id}/review   approve/reject

GET    /admin/audit                     phân trang, filter được
GET    /admin/audit/export              job export async
```

Yêu cầu permission per endpoint sống trong extension `x-required-permission` của OpenAPI (sẽ thêm) và được middleware `RequirePermission` enforce.

---

## 6. Data model: delta so với schema hiện tại

Bốn migration cần thiết trên `0002_rbac`:

| Migration | Mục đích |
|-----------|---------|
| `0003_user_groups.up.sql` | `user_groups`, `user_group_members`. Bump `token_version` cho user bị ảnh hưởng khi group thay đổi. |
| `0004_policies.up.sql` | `policies`, `policy_permissions`, `group_policy_attachments`, `user_policy_attachments`. |
| `0005_file_gated_permissions.up.sql` | `user_permission_files` + column workflow review. |
| `0006_effective_permissions_view.up.sql` | Materialized view (hoặc function) tính effective perm per user với file-gating apply. Refresh khi grant/revoke hoặc qua trigger. |

Table `roles` hiện tại được giữ cho **role thô cấp hệ thống** (admin, superadmin) — vẫn hữu ích cho quyết định "ai administer cả hệ thống này". Mô hình Policy nằm trên đó cho grant fine-grained, group-scoped.

---

## 7. Roadmap theo phase

Sắp xếp theo least-blocking và most-leverage:

### Phase 0 — Wire những gì đã build  *(không có tính năng mới)*

- `make sqlc` generate `internal/repository/`.
- Adapter cho `AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`.
- `cmd/api/main.go` construct `Issuer`, `Verifier`, `RefreshManager`, `rbac.Engine`, mount `/auth/*`.
- Script seed dev: 1 superadmin user, 1 default group.
- **Tiêu chí exit**: OIDC login end-to-end hoạt động với Authentik local.

### Phase 1 — Data plane Group + Policy

- Migration 0003 + 0004.
- Cập nhật RBAC engine: query effective-permission join user → groups (đệ quy) → policies → permissions.
- Cache key vẫn namespace theo `token_version`.
- API endpoint trong section 5 (groups + policies + attachments).
- **Tiêu chí exit**: một user trong group "Radiologists" inherit policy "Radiologist" của group và engine báo cáo effective set đúng.

### Phase 2 — Admin UI

- Page frontend từ section 4.
- Modal search + global search bar.
- Debug view effective-permissions (section 5: `GET /admin/users/{id}/effective-permissions`).
- **Tiêu chí exit**: admin có thể replicate mọi screen trong `anh1/2/3.png`.

### Phase 3 — Permission file-gated

- Migration 0005.
- Endpoint upload qua S3 presigned URL.
- UI queue review (admin) + email/notify khi nộp.
- Cron: hết hạn file → emit audit + invalidate cache.
- **Tiêu chí exit**: một permission yêu cầu license đã upload có hiệu lực sau khi admin review và biến mất khi hết hạn.

### Phase 4 — Audit & compliance

- Viewer audit + filter.
- Export sang bucket archive R2 (async).
- Retention policy.
- **Tiêu chí exit**: audit trail đầy đủ có thể replay cho mọi hành động user/group trong N ngày gần nhất.

### Phase 5 — Domain media (sẵn sàng parallel)

Track riêng không block 0–4. Xem package `internal/domain/{movie,music,story}/` và pipeline upload + transcode đã stub. Permission code cũng đã seed cho nó.

---

## 8. Non-goals (tạm thời)

Nói rõ để PR tương lai không drift:

- **Deny rule.** Không implement. Nếu một nhu cầu compliance thật xuất hiện, model thành enum `policy_permissions.effect` chứ không retrofit vào matcher.
- **Grant time-bounded ngoài `expires_at`.** Không có fence business-hours / geo / device.
- **Multi-tenant federated.** Tất cả group sống trong một DB. Tách tenant per DB là exercise của Phase-N.
- **Self-service password reset.** Authentik sở hữu cái này; Portal không bao giờ thấy mật khẩu.
- **App mobile native.** Chỉ PWA qua Next.js.

---

## 9. Câu hỏi mở

Cần product input, không phải code:

1. **UX confirm xoá group.** Gõ-the-code kiểu GitHub, hay modal 2-bước? Mock không show.
2. **Role-label vs Policy attach vào membership.** Mock gọi ra "User Role" bên trong group (Manager, Junior). Đó là label cosmetic thuần hay grant permission? Đề xuất: label cosmetic *thuần*; permission đến từ policy attach vào group + user. Confirm.
3. **Per-user policy — chỉ additive, hay override?** Spec default additive. Confirm hoặc escalate.
4. **Hành vi hết hạn permission file-gated.** Cut-off cứng, hay có grace period? Default: cut-off cứng + audit.
5. **Versioning policy.** Khi policy thay đổi giữa chừng, user trên session cũ thấy tập cũ đến lần refresh tiếp theo, hay tức thì? Spec default "tức thì qua cache invalidation"; confirm.
