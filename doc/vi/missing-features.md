# Chức năng còn thiếu — Phân tích khoảng trống / Backlog

**Đã xác minh lần cuối:** 2026-07-06 — snapshot chụp sau khi vòng demo v1 đã khép lại; xem [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) để theo dõi trạng thái cập nhật liên tục.

Những gì đã dựng so với đặc tả ([feature.md](feature.md)) mô tả. Đây là **backlog sau khi
khép vòng demo v1** (đăng nhập → upload → transcode → phát HLS → đăng xuất). Dùng để
chọn việc tiếp theo.

**Ký hiệu:** ✅ xong · ◐ một phần (chỉ UI hoặc chỉ schema) · ○ chưa làm · ⛔ hoãn (ngoài scope v1, [ADR-01](architecture/01-v1-scope-cut.md)).
**Ưu tiên:** `P1` = bước kế tiếp rõ ràng / mở khoá màn hình đã có · `P2` = sớm · `P3` = sau.

> Chủ đề lặp lại: **vỏ UI Olympus đã dựng, nhưng phần lớn là dữ liệu mẫu + thao tác
> chỉ chạy cục bộ.** Việc đáng làm nhất là nối các màn hình sẵn có vào backend thật
> (bài viết, bạn bè, tin nhắn, thông báo, tìm kiếm).

---

## 0. Nền tảng — cái đang chạy thật
- ✅ Auth mật khẩu local (login/register/refresh/logout, remember-me, khoá brute-force, RBAC engine, audit).
- ✅ Media slice: upload → `ffmpeg` HLS → `ready` → phát Vidstack (`/upload`).
- ✅ Object storage (`platform/storage`, MinIO/R2), worker Asynq, Postgres+PgBouncer+Dragonfly, Traefik, CI + test.
- ✅ Vỏ UI Olympus light (header, menu trái, panel bạn bè phải, newsfeed, dropdown profile/thông báo).

---

## 1. Account & Auth — module đã dựng, còn thiếu chức năng
Lõi auth đã chắc; đây là các chức năng account xung quanh (nhiều cái đã có UI).
- ○ **P1** Đặt lại mật khẩu (`forgot` / `reset` bằng token gửi email dùng-một-lần) — cần module notification. Hiện: chỉ admin/CLI.
- ○ **P1** Đổi mật khẩu (đã đăng nhập) — có mục menu "Profile Settings", chưa có endpoint.
- ○ **P2** Trang phiên/thiết bị — `GET /me/sessions` + thu hồi phiên cụ thể (query `ListActiveRefreshTokensForUser` đã có; thiếu handler/UI).
- ○ **P2** MFA / TOTP enrol + step-up (`/auth/totp/*`) — đã spec ([D-27]/[D-28]), chưa có code. Cần trước module bank.
- ○ **P2** Xác minh email khi đăng ký.
- ○ **P3** "Login with Google" (social login) — nay do Portal tự quản (ADR-06), trước đây chỉ là one-liner gọi IdP.
- ○ **P2** Profile: các trường hồ sơ thật của `users` (upload avatar, bio, sửa locale/timezone) + trang Profile. Hiện avatar chỉ là chữ cái đầu.
- ○ **P2** Admin: UI + endpoint liệt kê/vô hiệu hoá user, gán role (`users:*`, `rbac:role:*` đã có quyền; thiếu handler/trang).

## 2. Media — slice đã dựng, khoảng trống
- ○ **P1** Worker thumbnail — `worker.HandleThumbnail` vẫn là stub (trích 1 frame → upload → lưu).
- ○ **P1** Quản lý asset: `DELETE /assets/{id}` (+ xoá storage), đổi tên/sửa metadata; một **trang thư viện** media (danh sách đầy đủ hơn list nhỏ ở `/upload`).
- ○ **P2** HLS đa độ phân giải (240/480/720/1080) + master playlist — hiện chỉ 1 rendition.
- ○ **P2** Kiểm soát truy cập phát — HLS hiện **public**; thêm URL phát ký ngắn hạn hoặc visibility từng asset.
- ○ **P2** Presigned direct upload cho prod (browser→bucket): thêm `PublicEndpoint` browser truy cập được (route Traefik cho S3 API của MinIO / host R2). Dev dùng đường proxy qua API.
- ○ **P3** Phát event `media:asset_ready` + subscriber (nối vào các vertical bên dưới).
- ○ **P3** Loại asset audio/image (schema cho phép `audio`/`image`; pipeline mới xử lý video).

## 3. Tầng social — UI đã dựng, thiếu backend (khoảng trống lớn nhất)
Mỗi mục ở đây đều **đã có màn hình với dữ liệu mẫu**; không cái nào có backend. Xem [feature.md §9](feature.md).
- ○ **P1** **API bài viết / newsfeed** — composer chỉ post vào state cục bộ. Cần bảng `posts` + endpoint tạo/list/feed + nối composer & feed của `HomeView`.
- ○ **P1** **Bình luận, like/reaction, share** bài viết — số đếm đang tĩnh.
- ○ **P1** **Đồ thị bạn bè** — lời mời kết bạn (dropdown header), chấp nhận/từ chối, danh sách bạn, "Friend Suggestions", nhóm bạn (Close Friends/Family/Uncategorized). Tất cả là mẫu.
- ○ **P1** **Thông báo (thật)** — dropdown chuông + "Activity Feed" đang hard-code; cần kho thông báo + `GET /me/notifications` + SSE/poll + web-push.
- ○ **P1** **Nhắn tin / chat** — thanh "Olympus Chat" + dropdown tin nhắn chỉ trang trí; cần conversation/message + realtime.
- ○ **P1** **Tìm kiếm** — ô "Search here people or pages…" + "Find Friends" chưa có backend/trang kết quả.
- ○ **P2** **Cộng đồng / Favourite Pages** — menu "Fav Pages Feed" + widget "Pages You May Like"; chưa có entity pages.
- ○ **P2** **Sự kiện / sinh nhật / lịch** — menu "Calendar and Events" / "Friends Birthdays" + thẻ Birthday + widget lịch đang tĩnh.
- ○ **P2** **Widget thời tiết** — tĩnh; nối API thời tiết (hoặc bỏ khỏi v1).
- ○ **P3** Trang profile (about/photos/videos/friends), stories (24h ephemeral), follow graph, hashtag/mention, bookmark, xếp hạng feed, moderation — đều thuộc §9, chưa làm.

## 4. Các vertical domain — mới là skeleton
`movie` / `music` / `story` / `comic` chỉ có `module.go` + stub `api/` (không query, handler, migration, hay UI thật). `/library/comic` và `/library/novel/[id]` render view placeholder.
- ○ **P2** **Movies** ([feature.md §4]): schema `movies` + CRUD + trang list/detail, phát nối với media asset, luồng publish.
- ○ **P2** **Music** (§5): track/playlist + player (menu "Music & Playlists").
- ○ **P2** **Stories** (§6): chapter/reader (view novel detail đang skeleton).
- ○ **P2** **Comics** (§7): page/reader (view comic index đang skeleton).
- Mỗi cái cần: migration (`000N_<name>_…`), `query/`, repository, service/handler, `MountHTTP`, và view frontend thật. Tất cả phụ thuộc **media** cho asset (đã có sẵn).

## 5. Module Notifications (`notify:*`) — chưa làm
- ○ **P1** Module mới sở hữu task `notify:*` đã reserve ([MODULES.md §5.2](../../backend/MODULES.md)): email (SMTP/provider), web-push, in-app. Mở khoá đặt lại mật khẩu, gửi lời mời/thông báo, cảnh báo refresh-reuse. `account` đã stub `RegisterTasks` cho nó.

## 6. Multi-tenancy & RLS — hoãn (⛔ với v1)
- ⛔ Module `tenant` (organization, membership), bootstrap **RLS** Postgres, `cmd/sysjobs` (BYPASSRLS). Mới skeleton; đã cắt khỏi v1 ([ADR-01](architecture/01-v1-scope-cut.md), [feature.md §2]). Xem lại nếu cần multi-org.

## 7. Platform / Ops
- ○ **P2** Nối IP rate-limiter sẵn có (`platform/middleware`) vào `/auth/*` ở router (đã dựng, chưa dùng).
- ○ **P3** Stack observability (metrics/logs/traces) — cắt khỏi v1 (`--profile observability`), stack 5 service trong đặc tả dài hạn.
- ○ **P3** Binary `cmd/sysjobs` (batch cross-tenant) — dự kiến, chưa có.
- ○ **P3** Backup / retention (dump Postgres, lifecycle R2), health/readiness ngoài `/healthz`.

## 8. Frontend — trang & nối dữ liệu
- ◐ **P1** Biến thao tác placeholder thành thật: dropdown profile (Profile Settings, Create Fav Page, status), "Settings"/"⋯" ở thông báo/bạn bè, các mục menu trái chưa có route (Friend Groups, Weather App, Community Badges, Account Stats, Manage Widgets).
- ○ **P1** Ô **search** header + trang kết quả; trang **Find Friends**.
- ○ **P2** Trang còn thiếu: Profile, Account Settings, Messages, Friends, Events, Communities, Notifications, kết quả Search, trang Library detail thật.
- ○ **P3** Thay toàn bộ **dữ liệu mẫu** UI bằng gọi API khi các module trên xong.
- ○ **P3** Bundle hls.js (Vidstack đang tải từ CDN → cần internet để phát).
- ○ **P3** Test frontend (chưa có); rà a11y cho các dropdown/menu.

## 9. API contract (OpenAPI)
- ○ **P2** `shared/openapi.yaml` tồn tại nhưng các stub generated (`internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`) **chưa được generate/commit** và handler đang viết tay — hơn nữa bản thân spec cũng đã trôi (drift): vẫn còn liệt kê `/auth/callback` đã retire (đã gỡ bởi ADR-06) và thiếu `/auth/register`. Dù quyết định theo hướng nào, các path auth cũng cần cập nhật trước tiên. Quyết định: dùng `oapi-codegen`/`openapi-typescript` (rồi bật openapi-drift đầy đủ trong CI), hoặc bỏ spec làm nguồn sự thật.

## 10. Các module lớn đã hoãn (⛔ ngoài v1)
Đã spec trong feature.md, cắt bởi [ADR-01]:
- ⛔ **Bank / Tài chính cá nhân** (§8) — tài khoản, giao dịch, ngân sách, net worth; cần MFA/step-up trước.
- ⛔ **Creator economy & monetisation** (§10), **Marketplace / commerce** (§11).
- ⛔ **Social nâng cao** (§9.13–9.37): reels, live stream, audio room, karma, wiki, AMA, verification…
- ⛔ **ML safety / dashboard trust & safety** (§12.3), **microsite công ty** (§13).

---

## Thứ tự đề xuất tiếp theo (P1)
1. **Module Notifications** (`notify:*` + email) — mở khoá đặt lại mật khẩu và mọi thông báo social.
2. **Bài viết + bình luận/like** — làm newsfeed thành thật (màn hình chủ lực).
3. **Đồ thị bạn bè** — mời/chấp nhận kết bạn + danh sách bạn (nối dropdown header + panel phải).
4. **Tìm kiếm** — người/trang, ô header + trang kết quả.
5. **Vertical domain đầu tiên (Movies)** — chứng minh mẫu media→domain đầu-cuối.
6. **Media**: worker thumbnail + xoá + trang thư viện.
