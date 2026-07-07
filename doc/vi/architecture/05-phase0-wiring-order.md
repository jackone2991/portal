# ADR-05: Thứ tự nối dây (wiring) Phase 0 — đường găng tới một bản demo chạy được

**Trạng thái:** Được chấp nhận (Accepted) (đã thực thi; Milestone 0.4 bị thay thế bởi [ADR-06](./06-local-auth-model.md); Milestone 0.5 bị thay thế một phần — xem ghi chú Cập nhật)
**Ngày:** 2026-05-24
**Người quyết định:** kirito
**Ảnh hưởng:** [cmd/api/main.go](../../../backend/cmd/api/main.go), [cmd/worker/main.go](../../../backend/cmd/worker/main.go), [backend/internal/modules/account/module.go](../../../backend/internal/modules/account/module.go), [backend/sqlc.yaml](../../../backend/sqlc.yaml), [backend/db/migrations/](../../../backend/db/migrations/)

## Cập nhật (2026-07-06) — đã thực thi; giữ lại làm hồ sơ lịch sử

- Tất cả các milestone 0.1–0.6 đã hoàn tất; vòng lặp demo v1 (đăng nhập → tải lên → transcode → phát HLS → đăng xuất có thể thu hồi) đã khép kín và được commit. Trạng thái trực tiếp: [MILESTONE_CHECKS.md](../../../MILESTONE_CHECKS.md).
- Milestone 0.4 (OIDC/Authentik) thay vào đó được triển khai thành **xác thực mật khẩu local** theo [ADR-06](./06-local-auth-model.md); Authentik đã bị gỡ khỏi code và compose.
- Route refresh-and-return của Milestone 0.5 đã được thay bằng silent refresh phía client `SessionKeeper` (theo interval + focus, throttle đa-tab); middleware Next.js chặn dựa trên cookie `portal_session`.
- Các migration đã lên thành `0001_platform_init` … `0007_media_assets` (đã áp dụng v7) — các bảng media được đưa vào `0007`, mở rộng kế hoạch 5-file ở Milestone 0.1.
- CI lên đầy đủ hơn kế hoạch (backend go build/vet/test `-race` + sqlc-drift; frontend `next build`), nhưng job openapi chỉ kiểm tra spec có hợp lệ về cú pháp (well-formed) — chưa có kiểm tra openapi-drift.

## Bối cảnh

CLAUDE.md nêu rõ điểm nghẽn:

> `cmd/api/main.go` vẫn còn comment `TODO: mount OpenAPI-generated handlers` và chưa gọi `account.New(...)` hay `MountHTTP` của bất kỳ module nào. Module account tự lắp ráp handler của nó bên trong `backend/internal/modules/account/module.go`; binary API chỉ đơn giản là chưa được dạy cách khởi tạo nó. Việc nối dây bị hoãn cho tới khi các repository adapter được đưa vào.

> Các thư mục `internal/modules/*/repository/` đã tồn tại nhưng còn rỗng. Các interface mà module account tiêu thụ (`AuthSnapshotFetcher`, `RefreshStore`, `PermissionFetcher`, `EventStore`, `UserUpserter`) cần các adapter bọc quanh code do sqlc sinh ra ngay khi `make sqlc` chạy.

Mọi hạng mục bàn giao của v1 đều phụ thuộc vào việc lấp khoảng trống này. Sprint 2 tuần không thể chịu nổi một trình tự sai — chẳng hạn, làm lại migration sau khi sqlc đã sinh code sẽ tốn phần còn lại của cả một ngày.

Bản cắt-giảm v1 của [ADR-01](./01-v1-scope-cut.md) giữ lại 8 hạng mục Phase 0. ADR này sắp xếp chúng theo thứ tự thực thi.

## Quyết định

**Phase 0 được triển khai trong 5 milestone theo trình tự nghiêm ngặt, trải trong Ngày 1–6 của sprint, trước khi bất kỳ công việc tính năng nào bắt đầu.** Mỗi milestone kết thúc bằng một check cụ thể mà developer có thể chạy.

### Milestone 0.1 — Rà soát cây migration (Ngày 1, ~4 giờ)

Trước khi sqlc chạy và đóng băng schema, tách `0001` theo [D-18]:

```
0001_platform_init.up.sql          extensions (uuid-ossp, citext, pg_trgm), common helper functions
0002_account_users.up.sql          users (no role col; +locale +timezone +token_version +disabled_at)
0003_account_rbac.up.sql           roles, role_parents, permissions, role_permissions, user_roles,
                                   user_oidc_roles (per [D-26])
0004_account_sessions.up.sql       refresh_tokens (with parent_id, replaced_by_id, revoked_at)
0005_platform_audit.up.sql         audit_log (moved from account; per [D-25])
```

Mỗi `up.sql` có một `down.sql` tương ứng. Bảng `assets` (từng nằm trong `0001` cũ) và mọi bảng media đều bị hoãn — chúng không lên trong Milestone 0.x.

**Check:** `make migrate && make migrate-down && make migrate` chạy sạch. Ghi lại đây là tiêu chí nghiệm thu (acceptance) v1 cho migration.

### Milestone 0.2 — Sinh code sqlc + repository adapter (Ngày 2, ~6 giờ)

1. Chạy `make sqlc` cho block account. Code sinh ra nằm trong `backend/internal/modules/account/repository/`.
2. Viết các adapter hiện thực hóa các interface mà account đã khai báo sẵn:
   - `AuthSnapshotFetcher` — bọc `GetUserAuthSnapshot` (trả về `id`, `token_version`, `disabled_at`).
   - `RefreshStore` — bọc `InsertRefreshToken`, `GetRefreshToken`, `RevokeRefreshTokenChain` (recursive CTE để phát hiện đánh cắp token).
   - `PermissionFetcher` — bọc `GetEffectivePermissions` (duyệt đệ quy tổ tiên vai trò).
   - `EventStore` — bọc `InsertAuditEvent`. Giờ nằm trong `platform/audit/`, không phải `account/audit/` (theo Milestone 0.1 / [D-25]).
   - `UserUpserter` — bọc `UpsertOidcUser`, `SyncOidcRoles`. *(Cập nhật 2026-07-06: upsert OIDC đã bị loại bỏ theo [ADR-06](./06-local-auth-model.md); được thay bằng các query local-auth. Adapter đã lên cho account và media.)*

Adapter là ánh xạ 1:1 với các hàm do sqlc sinh ra; không chứa business logic. Chúng nằm trong `backend/internal/modules/account/repository/adapter.go` (một file, sắp theo bảng chữ cái).

**Check:** `go build ./...` thành công trên toàn bộ package. Không còn comment `// TODO: adapter` nào trong account.

### Milestone 0.3 — Khởi tạo module account trong `cmd/api/main.go` (Ngày 3, ~6 giờ)

Trình tự nối dây trong `cmd/api/main.go`:

```go
func main() {
    cfg := config.MustLoad()                                    // env loader
    logger := platformlog.New(cfg.Env)                          // stdout JSON in v1
    pgPool := db.MustOpen(ctx, cfg.DatabaseURL)                 // pgxpool
    cache := cache.NewDragonfly(cfg.RedisURL)                   // Dragonfly client
    asynqClient := jobs.NewClient(cfg.RedisURL)                 // Asynq producer

    auditLogger := audit.NewLogger(pgPool, logger)              // platform/audit (per [D-25])

    accountMod, err := account.New(account.Deps{
        DB:                 pgPool,
        Cache:              cache,
        Audit:              auditLogger,
        OIDCIssuerURL:      cfg.OIDCIssuerURL,
        OIDCClientID:       cfg.OIDCClientID,
        OIDCClientSecret:   cfg.OIDCClientSecret,
        OIDCRedirectURL:    cfg.OIDCRedirectURL,
        JWTSigningKeys:     cfg.JWTSigningKeys,                 // comma-separated, rotating kid
        CookieDomain:       cfg.CookieDomain,
        CookieSecure:       cfg.CookieSecure,
        BootstrapAdminSubs: cfg.BootstrapAdminOIDCSubjects,     // per [D-26]
        OIDCGroupRoleMap:   cfg.OIDCGroupRoleMap,
    })
    must(err)

    r := chi.NewRouter()
    r.Use(middleware.RealIP, middleware.RequestID, middleware.Recoverer)
    r.Use(middleware.Timeout(30 * time.Second))
    r.Use(corsMiddleware(cfg))                                  // configured per env
    r.Use(ratelimit.Middleware(cache))

    r.Route("/api/v1", func(r chi.Router) {
        r.Get("/healthz", healthz(pgPool, cache))
        accountMod.MountHTTP(r)                                 // mounts /auth/*, /me/*
        // v1 stops here. Future modules append their MountHTTP under r.
    })

    server := &http.Server{Addr: ":8080", Handler: r}
    logger.Info("api listening", "addr", server.Addr)
    must(server.ListenAndServe())
}
```

*(Bản phác thảo lịch sử — như đã triển khai thực tế, các field OIDC trong `Deps` đã biến mất theo [ADR-06](./06-local-auth-model.md), và `media.New(...)` cũng được khởi tạo và mount dưới `/api/v1`.)*

Hình dạng tương tự áp dụng cho `cmd/worker/main.go` với `accountMod.RegisterTasks(asynqMux)` — account không có task Asynq nào trong v1, nên lời gọi này là no-op, nhưng khung nối dây đã có sẵn.

**Check:** `make up && go run ./cmd/api` (hoặc `make dev`) khởi động được. `curl http://localhost:8080/api/v1/healthz` trả về 200 với `{"status":"ok","db":true,"cache":true}`.

### Milestone 0.4 — OIDC đầu-cuối (Ngày 4, ~6 giờ)

> **Đã bị thay thế (2026-07-05) bởi [ADR-06](./06-local-auth-model.md)** — milestone auth đã được triển khai dưới dạng xác thực mật khẩu local; Authentik đã bị gỡ khỏi code và compose. Giữ lại để lưu lịch sử.

Với Authentik chạy trong compose (theo [ADR-03](./03-single-vps-topology.md)), cái bắt tay (handshake) OIDC từ `diagrams.md` §5 phải hoạt động:

1. Cấu hình provider Authentik cho Portal: client ID + secret, redirect URI `https://${APP_DOMAIN}/api/v1/auth/callback`, cho phép scope `openid profile email groups`.
2. Set `OIDC_GROUP_ROLE_MAP=portal-admins:admin` và tạo group `portal-admins` trong Authentik.
3. Tạo user của riêng bạn trong Authentik, thêm vào `portal-admins`, set `BOOTSTRAP_ADMIN_OIDC_SUBJECTS=<your-sub>`.
4. Luồng trình duyệt: truy cập `${APP_DOMAIN}` → frontend redirect tới `/api/v1/auth/login` → 302 tới Authentik → đăng nhập → callback → tạo dòng `users`, điền `user_oidc_roles`, set cookie access + refresh → redirect về `/`.
5. `curl -b cookies.txt https://${APP_DOMAIN}/api/v1/me` trả về payload user.
6. `curl -b cookies.txt -X POST https://${APP_DOMAIN}/api/v1/auth/logout-all` tăng `token_version`; lần gọi `/me` tiếp theo trả về 401.

**Check:** 6 bước trên hoạt động mà không cần SQL thủ công.

### Milestone 0.5 — Frontend server-only API client + bàn giao xác thực RSC (Ngày 5–6, ~10 giờ)

> **Bị thay thế một phần (2026-07-06)** — mục 3 (route refresh-and-return) đã được thay bằng silent refresh phía client `SessionKeeper`; middleware Next.js chặn dựa trên cookie `portal_session`. Link sign-in của mục 4 đã trở thành form `/login` thật ([ADR-06](./06-local-auth-model.md)).

Theo [D-34]:

1. Tạo `frontend/src/lib/api-server.ts` với `import "server-only"`. Bọc `fetch` để đọc `cookies()` và chèn `Cookie:` vào các lời gọi API đi ra.
2. Tạo `frontend/src/lib/api-client.ts` (không có guard server-only) cho các fetch từ client-component; dùng `credentials: 'include'`.
3. Tạo `frontend/src/app/auth/refresh-and-return/route.ts` — nhận query param `return_to=<path>`, gọi `/api/v1/auth/refresh`, redirect tới `return_to`.
4. Tạo nút `<Sign in>` trên trang index, trỏ tới `/api/v1/auth/login`.
5. Tạo `/account/page.tsx` (RSC) gọi `/api/v1/me` và render user. Khi gặp 401, throw `redirect('/auth/refresh-and-return?return_to=/account')`.

**Check:** user chưa xác thực bấm Sign in, đến trang Authentik, đăng nhập, quay về `/account`, thấy email của mình. Refresh sau khi access token hết hạn kích hoạt luồng refresh-and-return đúng một lần và quay lại `/account`.

### Milestone 0.6 — Đặt trước tên gọi (naming) + CI tối thiểu (song song, ~3 giờ)

Những việc này diễn ra song song nhưng không chặn (gate) các milestone ở trên:

- Thêm ghi chú đặt trước prefix `notify:*` cho Asynq vào `backend/MODULES.md` §5.2 (theo [D-1]).
- Thêm một `.github/workflows/ci.yml` tối thiểu chỉ với hai job: `sqlc-drift` (`make sqlc && git diff --exit-code`) và `openapi-drift` (`make openapi && git diff --exit-code`). Bỏ qua lint/test/security cho v1. Chỉ riêng việc phát hiện drift cũng đã bắt được lớp bug tốn kém nhất.
- Thêm dòng comment header `# v1 scope: ADR-01` vào `cmd/api/main.go`.

## Các phương án đã cân nhắc

### Phương án A — sqlc trước, rồi mới đến migration *(bị từ chối)*

Sinh code dựa trên một schema chưa được rà soát. Tách migration sau đó buộc phải sinh lại code, có thể đổi tên hàm/kiểu, làm hỏng adapter giữa sprint. Thứ tự quan trọng — migration là hợp đồng (contract) mà sqlc đọc.

### Phương án B — Khởi tạo module trước khi adapter sqlc tồn tại *(bị từ chối)*

Constructor `New(Deps)` của module account yêu cầu các adapter làm input. Việc stub chúng bằng no-op để cho binary chạy được rất hấp dẫn nhưng tạo ra một lượt "khởi tạo rồi nối dây lại" (double-pass). Tệ hơn, nó che giấu các bug về hình dạng adapter (sai lệch interface giữa những gì sqlc sinh ra và những gì module account cần) đằng sau một build xanh (green build).

### Phương án C — Bỏ qua Authentik trong v1, tự viết tay xác thực mật khẩu *(bị từ chối)*

Tiết kiệm ~1 GB RAM và 1 ngày cấu hình Authentik. Tốn 3 ngày cho lưu trữ mật khẩu + luồng reset + template email + logic khóa tài khoản + recovery code. Lỗ ròng; bề mặt auth chính là nơi các regression bảo mật tốn kém nhất. Đưa Authentik vào compose là quyết định đúng cho v1 dù nó nặng.

*(Cập nhật 2026-07-05: đảo ngược — [ADR-06](./06-local-auth-model.md) áp dụng xác thực mật khẩu local vì lý do UX/quyền sở hữu; máy móc token/RBAC được tái dùng nguyên trạng.)*

### Phương án D — Hoãn việc tách migration; đổi tên bên trong một mega-migration duy nhất *(bị từ chối)*

Hấp dẫn vì chưa có dữ liệu production. Không tốn gì bây giờ, nhưng áp một khoản "thuế nhận thức" vĩnh viễn kiểu "migration này thực ra là ba migration". Việc tách chỉ rẻ *trước khi* sqlc chạy trên nó. Sau đó, nó đắt. Trả cái giá rẻ. [D-18]

## Phân tích đánh đổi

Thứ tự milestone 0.1 → 0.2 → 0.3 là không thể thương lượng: mỗi cái phụ thuộc vào cái trước (migration → sqlc → adapter → khởi tạo module). Milestone 0.4 (OIDC) và 0.5 (auth RSC frontend) gần như song song — OIDC kết thúc ở mức "có thể curl /me với cookie", frontend bắt đầu ở mức "trình duyệt làm điều mà curl vừa làm". Bạn có thể đảo thứ tự, nhưng làm OIDC trước cho bạn một backend chạy được trước khi đụng tới frontend, dễ debug hơn vì mỗi lúc chỉ có vấn đề ở một chỗ.

Milestone 0.6 (naming + CI) đủ nhỏ để chèn vào bất kỳ khoảng trống nào, nhưng làm nó trước Milestone 0.4 nghĩa là các check drift bắt được mọi regression sqlc hoặc openapi do 0.1–0.3 gây ra trước khi chúng dồn lại.

Ngân sách tổng cho Phase 0: ~35 giờ, ~Ngày 1–6 của sprint. Điều đó để lại Ngày 7–10 cho vertical slice Phase 2 (upload → transcode → playback), Ngày 11–12 cho bugfix + script deploy, Ngày 13–14 làm buffer.

## Hệ quả

**Cái gì dễ hơn:**

- Cơn hoảng loạn "nối dây cho xong" kết thúc vào Ngày 3. Từ đó trở đi, mọi tính năng gắn vào một khung (scaffold) đã chạy được.
- Demo 7 bước từ [ADR-01](./01-v1-scope-cut.md) trở nên tầm thường về mặt kiến trúc: 6 trong 7 bước nằm ở Milestone 0.4–0.5; bước thứ bảy (logout) nằm ở Milestone 0.4.
- Các module tương lai (movie, music, v.v.) gắn vào `r.Route("/api/v1", ...)` y hệt như account đã làm — sao chép pattern.

**Cái gì khó hơn:**

- Việc rà soát migration ở Ngày 1 *cảm giác* như một khoản thuế khi mục tiêu là ship một bản demo. Tuy nhiên đây là thứ tốn kém nhất nếu hoãn lại trên con đường này, nên ADR này yêu cầu developer làm việc nhàm chán trước.
- Cấu hình Authentik là ẩn số. Dành thêm thời gian ở Ngày 4 nếu bạn chưa từng làm việc này; công thức (recipe) provider OIDC của Authentik đã công bố khá đơn giản nhưng giả định Authentik có thể truy cập được từ trình duyệt (routing theo hostname của Traefik phải hoạt động cho cả Portal LẪN Authentik).

**Cái gì cần xem lại:**

- Các phần frontend của Milestone 0.5 là mức tối thiểu để demo. Tài liệu ranh giới Zustand/TanStack/RHF đầy đủ ([D-32]) và cây quyết định RSC ([D-33]) bị hoãn theo [ADR-01](./01-v1-scope-cut.md); thêm chúng vào Phase 0.5.
- Các job CI bị bỏ qua (lint, test, security, build multi-arch) nên lên trong Phase 0.5 trước khi bất kỳ user bên ngoài nào chạm vào hệ thống.
- Nếu Authentik gây thêm >2 ngày đau khổ khi setup, đánh giá lại Phương án C (auth tự viết tay) — nhưng chỉ khi có tổn thất runway rõ ràng. Đừng đánh giá lại giữa sprint; hoàn thành OIDC rồi rút kinh nghiệm.

## Hạng mục hành động

1. [x] Mở 5 issue milestone trong tracker phản chiếu §1–§5 ở trên; đóng từng cái khi check của nó pass.
2. [x] Ngày 0 (lập kế hoạch): viết ra danh sách env var (`DATABASE_URL`, `REDIS_URL`, `S3_*`, `OIDC_*`, `JWT_SIGNING_KEYS`, `COOKIE_*`, `OIDC_GROUP_ROLE_MAP`, `BOOTSTRAP_ADMIN_OIDC_SUBJECTS`) và điền `.env.example` để Ngày 1 không bị kẹt vì thiếu credential.
3. [x] Sáng Ngày 1: viết ra lệnh milestone-check cho từng milestone vào một scratchpad `MILESTONE_CHECKS.md`; tick từng cái khi hoàn thành. Kiềm chế ý muốn nhảy sang milestone tiếp theo trước khi check của milestone trước pass.
4. [x] Ngày 4 (Authentik): chặn hẳn một buổi chiều trọn vẹn. Cấu hình lần đầu của Authentik là giờ rủi ro cao nhất trong sprint.
5. [x] Cuối Milestone 0.5: chạy trọn vẹn demo 7 bước từ [ADR-01](./01-v1-scope-cut.md) §Decision. Nếu nó chạy được, bạn đang đúng tiến độ cho v1.

*(2026-07-06: tất cả hạng mục đã hoàn tất — phần Authentik/OIDC của mục 2 và 4 đã bị bỏ theo [ADR-06](./06-local-auth-model.md); trạng thái trực tiếp nằm ở [MILESTONE_CHECKS.md](../../../MILESTONE_CHECKS.md).)*
