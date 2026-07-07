# ADR-01: Bộ cắt phạm vi v1 — cái gì vừa trong 2 tuần / 1 dev / $100/tháng / 1 VPS

**Trạng thái:** Được chấp nhận
**Ngày:** 2026-05-24
**Người quyết định:** kirito

> **Cập nhật (2026-07-06):** v1 đã ra mắt — vòng lặp demo bên dưới đã khép kín và được commit; `MILESTONE_CHECKS.md` ở gốc repo là bộ theo dõi trạng thái đang sống. Các sai khác so với bản kế hoạch (as-built deltas):
> - Bước 1 đã bị thay thế bởi [ADR-06](./06-local-auth-model.md): đăng nhập là xác thực mật khẩu local do Portal tự quản lý (`POST /api/v1/auth/login`); Authentik/OIDC đã bị gỡ khỏi code và compose. Các hạng mục deliverable đặc-thù-OIDC bên dưới (đồng bộ nhóm `user_oidc_roles`, các claim `amr`/`acr`/`auth_time`) cũng bị gỡ bỏ theo.
> - Bước 4: dev lưu vào MinIO; R2 chỉ áp dụng cho các môi trường đã triển khai — xem mục Cập nhật của ADR-04 (2026-06-06).
> - Route refresh-and-return ở [D-34] đã được thay bằng cổng middleware `portal_session` + refresh ngầm phía client `SessionKeeper`.
> - CI đã lên trong Phase 6 kèm `sqlc-drift`; kiểm tra lệch (drift) handler openapi vẫn còn để ngỏ — `shared/openapi.yaml` hiện đang lệch (thiếu `/auth/register`, còn sót `/auth/callback` đã cũ).
> - Các route as-built được mount dưới `/api/v1` trơn, không có tiền tố `/t/{tenant}`; hợp đồng URL tenant vẫn hoãn sang Phase 1.

## Bối cảnh

`feature.md` mô tả 12 phase và 40 quyết định đã chốt, bao trùm danh tính, đa-tenant, media, bốn vertical nội dung, tài chính cá nhân, thông báo, mạng xã hội, tìm kiếm, trang marketing, mạng xã hội nâng cao (reels/live/audio room), kinh tế sáng tạo (creator economy), marketplace, và an toàn ML. Từng quyết định riêng lẻ đều hợp lý; gộp lại chúng mô tả một nền tảng mà một đội nhỏ phải mất một năm hoặc hơn mới ra mắt được.

Khung ràng buộc đã nêu là:

- 1 developer
- 2 tuần để ra v1
- ngân sách hạ tầng ≤ $100/tháng
- một VPS duy nhất

Một phiên bản Portal cố gắng tôn trọng mọi deliverable của Phase 0 trong 2 tuần sẽ hết thời gian ở khoảng bước 8 (trong tổng 14 bước) của Phase 0 và không ra mắt được gì. Một phiên bản chọn một lát cắt (slice) mạch lạc duy nhất, ra mắt nó, và coi phần còn lại là backlog thì có thể tạo ra một artefact *chạy được* vào cuối sprint.

ADR này ghi rõ lát cắt đó thành một quyết định, chứ không phải một sự trôi dạt (drift).

## Quyết định

**v1 ra mắt Phase 0 (nối dây nền tảng) cộng với một lát cắt dọc (vertical slice) của Phase 2 (một luồng happy-path tải lên video) và không gì khác.** Mọi thứ ở Phase 1, 3–12 đều bị hoãn lại. Thứ tự phase trong `feature.md` không đổi; phạm vi của cái gọi là "v1" là thứ duy nhất mà ADR này dịch chuyển.

Cụ thể, v1 = bản demo nhỏ nhất chứng minh kiến trúc hoạt động đầu-cuối:

1. Người dùng đăng nhập qua Authentik (OIDC).
2. Họ vào trang chủ Next.js ở trạng thái đã xác thực.
3. Họ tải lên một file mp4 qua UI.
4. File tải lên được lưu vào R2 (xem [ADR-04](./04-storage-tier-budget.md)).
5. Worker nhận task transcode, tạo ra một HLS ladder, và cập nhật `assets.status = ready`.
6. Người dùng phát lại video trên trình duyệt bằng Vidstack.
7. Họ đăng xuất; phiên có thể bị thu hồi qua cơ chế hai-kênh đã có.

Đó là toàn bộ vòng lặp demo v1. Không tenant. Không CRUD movies/music/stories/comics. Không bank. Không mạng xã hội. Không thông báo. Không mediamtx. Không LiveKit. Không observability stack. Không file-gated permission. Không policy bundle. Không marketplace.

## Các phương án đã cân nhắc

### Phương án A — Tôn trọng Phase 0 trọn vẹn, hoãn mọi thứ khác

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Cao — riêng Phase 0 đã có 14 deliverable |
| Chi phí | $30–60/tháng |
| Khả năng mở rộng | Không áp dụng (chỉ là nền tảng) |
| Mức quen thuộc của team | Dev đơn độc đã quen stack này |

**Ưu điểm:** Phase 0 là sprint "đúng-theo-spec"; mọi mảnh được dựng ở đây đều sinh lời ở mọi phase sau.
**Nhược điểm:** 14 deliverable trong 2 tuần cho 1 dev là ~2 giờ mỗi cái kể cả testing — phi thực tế khi vài hạng mục (CI workflow, tài liệu convention frontend, audit migration, retrofit RFC 7807, observability stack) là việc nhiều-giờ. Kết quả nhiều khả năng nhất là "Phase 0 làm dở dang, không có demo chạy được".

### Phương án B — Tối thiểu Phase 0 + lát cắt dọc Phase 2 *(được chọn)*

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Trung bình — cắt Phase 0 từ 14 mục xuống ~8 |
| Chi phí | $30–60/tháng |
| Khả năng mở rộng | Demo một-người-dùng; multi-tenant hoãn lại |
| Mức quen thuộc của team | Dev đơn độc đã quen stack này |

**Ưu điểm:** Cho ra một demo chạy được vào cuối sprint. Buộc khoảng trống nối dây phải đóng lại vào Ngày 3. Phơi bày các bug tích hợp (cờ cookie, CORS, hình dạng handler oapi-codegen, chữ ký adapter sqlc) — đó mới là rủi ro thật.
**Nhược điểm:** Bỏ qua audit migration ([D-18]), observability stack ([D-8]), tài liệu convention frontend ([D-32]/[D-33]), CI workflow ([D-9]), retrofit schema cross-module OpenAPI ([D-29]). Tất cả đều phải lên sau; một số sẽ khó retrofit.

### Phương án C — Bỏ qua Phase 0, tự viết tay một lớp auth mỏng + demo media

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Thấp cho v1 |
| Chi phí | $30/tháng |
| Khả năng mở rộng | Dùng-rồi-bỏ — sẽ cần viết lại toàn bộ |
| Mức quen thuộc của team | Dev đơn độc đã quen stack này |

**Ưu điểm:** Đường nhanh nhất tới một demo chạy được.
**Nhược điểm:** Vứt bỏ module account đã có sẵn (đã được viết), spec OpenAPI, kỷ luật ranh giới module, và bố cục modular monolith. Tạo nợ kỹ thuật mà phần còn lại của năm phải trả dần. Chỉ đúng nếu v1 là một *prototype dùng-rồi-bỏ*; nếu nó là hạt giống của sản phẩm thật, đây là lựa chọn sai.

## Phân tích đánh đổi

Chế độ thất bại của Phương án A là "không có demo cuối sprint, nhiều phần nối dây dở dang". Chế độ thất bại của Phương án C là "demo chạy được nhưng không mở rộng được". Chế độ thất bại của Phương án B là "demo chạy được, thiếu vài tiện ích Phase 0 cần một sprint Phase 0.5". Trong ba phương án, chế độ thất bại của Phương án B là rẻ nhất để khắc phục: các phần thiếu (CI workflow, tài liệu convention frontend, profile observability) đều có thể thêm trong một sprint nửa-ngày mà không đụng vào code ứng dụng.

Lát cắt còn lại ~8 deliverable của Phase 0 là:

| Deliverable Phase 0 (từ feature.md) | Có trong v1? | Lý do |
| --- | --- | --- |
| Nối dây `cmd/api/main.go` | **Có** | Điểm nghẽn thật sự. |
| `make sqlc` cho block account + chốt quyết định | **Có** | Bắt buộc trước khi adapter compile được. |
| Adapter repository cho các interface account | **Có** | Bắt buộc để construct module. |
| Audit migration `0001` (tách thành 0001/0002/0003/0005) | **Có** | Rẻ để làm *ngay bây giờ* trước khi có dữ liệu; không thể làm sau. [D-18] |
| Cột `users.locale` + `users.timezone` | **Có** | Thêm một dòng trong lúc audit; frontend cần ngay từ ngày đầu. |
| Chuyển `audit/` → `platform/audit/` + đổi tên event | **Có** | Rẻ trong lúc audit migration; đắt sau khi audit_log đã có dữ liệu. [D-25] |
| Đưa claim `amr`/`acr`/`auth_time` vào context | **Có** | Chỉ đổi một file; cho phép [D-27]/[D-28] lên sau mà không phải viết lại middleware. |
| Bảng `user_oidc_roles` | **Có** | Lên trong `0003_account_rbac`; đồng bộ nhóm OIDC ghi vào đó ở lần đăng nhập đầu tiên. [D-26] |
| Áp dụng `Problem` theo RFC 7807 trong OpenAPI | **Một phần** | Thêm schema; retrofit handler khi chúng được viết, không làm một lượt quét toàn bộ. |
| Giữ chỗ tiền tố Asynq `notify:*` | **Có** | Chỉ là tài liệu; một dòng trong MODULES.md §5.2. |
| Schema cross-module OpenAPI (Money, PaginatedResult, TenantContext, ContinuingItem) | **Không** | Money/Continue/TenantContext chưa cần đến khi bank/Phase 4/tenant ra mắt. Thêm khi thật sự cần. |
| Versioning URL + tài liệu RFC 9745 | **Không** | `/api/v1/` đã có sẵn; tài liệu là việc giấy tờ có thể lên vào tuần 3. |
| Client API chỉ-phía-server của frontend + route refresh-and-return | **Có** | Không có cái này, trang RSC không thể xác thực với API. [D-34] |
| Tài liệu convention frontend (ranh giới Zustand/TanStack/RHF) | **Không** | Dev đơn độc; tài liệu cho một độc giả duy nhất là giấy tờ thừa. Thêm khi có contributor thứ hai. [D-32]/[D-33] |
| CI workflow (lint + test + drift + roundtrip + build + security) | **Một phần** | Chỉ ship kiểm tra drift (`sqlc-drift`, `openapi-drift`). Bỏ qua build multi-arch, security scan, integration matrix đến tuần 3. [D-9] |

Với Phase 2, lát cắt v1 là: một mức ưu tiên queue, chỉ libx264, không có đường hardware encoder, không có quota theo per-user/per-tenant, không backpressure, không có UI dead-letter queue (transcode lỗi thì log to và operator tự mò ra bằng tay). Luồng happy-path duy nhất chứng minh kiến trúc; phần trau chuốt lên ở Phase 2.5.

## Hệ quả

**Cái gì dễ hơn:**

- Sprint 2 tuần có một tiêu chí thành công duy nhất, dễ chứng minh: demo cuối sprint chạy đủ 7 bước ở trên. Dễ test, dễ biết khi nào xong.
- Khoảng trống nối dây (điểm nghẽn thật sự) được đóng lại trong Ngày 1–3 vì mọi thứ khác đều phụ thuộc vào nó.
- Tải nhận thức (cognitive load) của dev đơn độc giảm xuống — chỉ những module chạm vào vòng demo mới cần hiểu sâu trong tuần 1.

**Cái gì khó hơn:**

- Hoãn audit migration *sẽ* rẻ hơn nếu làm sau, nhưng làm nó trong v1 (thay vì đẩy sang Phase 0.5) tốn một ngày. Đáng.
- Bỏ qua tài liệu convention frontend nghĩa là contributor đầu tiên (bất kể là ai) đến mà không có hợp đồng ranh-giới-state. Chấp nhận điều này — tài liệu có thể lên khi cần.
- Bỏ qua observability stack nghĩa là demo v1 chạy "mù". Với demo một-người-dùng thì ổn; khi triển khai production nên thêm `--profile observability` trước khi có người dùng bên ngoài.
- Bỏ qua multi-tenant nghĩa là tiền tố URL `/t/{tenant}/...` không được thực thi trong v1. Hoãn hợp đồng URL đến Phase 1; ĐỪNG hard-code đường dẫn demo v1 theo cách không tương thích với tiền tố đó về sau. Chỉ dùng `/api/v1/healthz`; các route được bảo vệ nên đã nằm dưới `/t/me/api/v1/...` kể cả khi tenant `me` bị hard-code.

**Cái gì cần xem lại:**

- Sau khi v1 ra mắt, chạy một sprint Phase 0.5 để đóng các mục Phase 0 đã bỏ qua (CI workflow đầy đủ, tài liệu convention frontend, thiết lập profile observability) trước khi Phase 1 (tenancy) bắt đầu.
- Nhặt lại Phase 1 (tenancy + RLS) làm sprint tiếp theo; tenant tổng hợp `me` được mang tiếp không đổi. [D-23]/[D-24]
- Cuộc chia rẽ RBAC ([ADR-02](./02-rbac-model-reconciliation.md)) nên được giải quyết TRƯỚC Phase 1 để migration role + policy lên trong một lượt.

## Hạng mục hành động

1. [x] Ghim ADR này (`Accepted`) trước khi viết bất kỳ code nào cho sprint 2 tuần. *(xong 2026-07-06 — trạng thái đã chuyển; v1 được xây dựng theo lát cắt này)*
2. [ ] Mở một issue/todo theo dõi với demo 7-bước làm tiêu chí chấp nhận nghĩa đen. → tiêu chí chấp nhận được theo dõi trong `MILESTONE_CHECKS.md`
3. [ ] Thêm nhãn/section `v1-out-of-scope` trong issue tracker cho mọi thứ ở §3 của bài đánh giá điều hành — giữ công việc bị hoãn hiển thị mà không mời gọi scope creep.
4. [ ] Thêm `# v1 scope: see doc/en/architecture/01-v1-scope-cut.md` như một comment ở đầu `cmd/api/main.go` để tương-lai-của-bạn không lỡ với tới một module ngoài-v1.
5. [ ] Cuối sprint, viết một retrospective một-trang "cái gì đã cắt, cái gì đã ship, cái gì đã học"; đưa mọi mục bị-cắt-nhưng-cần vào backlog Phase 0.5.
