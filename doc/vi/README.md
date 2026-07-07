# Portal — Mục lục tài liệu

Cây thư mục này là **kho thiết kế** (design corpus) của Portal: đặc tả, hồ sơ quyết định, sơ đồ và
phân tích. Tình trạng triển khai thực tế nằm ở [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md);
hướng dẫn vận hành và quy ước của repo nằm ở [CLAUDE.md](../../CLAUDE.md); hợp đồng module backend
là [backend/MODULES.md](../../backend/MODULES.md).

## Bắt đầu từ đây

Người đóng góp mới? Hãy đọc theo thứ tự sau:

1. [architecture/README.md](architecture/README.md) — Mục lục ADR và ràng buộc khung v1 (1 developer · 2 tuần · ≤ $100/tháng · 1 VPS).
2. [architecture/01-v1-scope-cut.md](architecture/01-v1-scope-cut.md) — v1 thực sự là gì (và những gì nó không phải).
3. [architecture/06-local-auth-model.md](architecture/06-local-auth-model.md) — mô hình xác thực hiện tại (mật khẩu nội bộ; đã loại bỏ Authentik/OIDC).
4. [feature.md](feature.md) — nhật ký quyết định chuẩn `D-1`…`D-40` và bảng kê đầy đủ tính năng.
5. [diagrams.md](diagrams.md) — bản đồ kiến trúc trực quan.
6. [backend/MODULES.md](../../backend/MODULES.md) — ranh giới module trước khi viết bất kỳ code backend nào.

## Tất cả tài liệu

| Tệp | Vai trò | Trạng thái | Mô tả ngắn |
| --- | --- | --- | --- |
| [feature.md](feature.md) | Nhật ký quyết định | Hiện hành | Bảng kê tính năng theo từng module, lộ trình theo giai đoạn, câu hỏi còn mở, và các quyết định `D-1`…`D-40`. |
| [checklist.md](checklist.md) | Checklist | Hiện hành | Checklist công việc toàn dự án: mọi deliverable Phase 0–12 dạng checkbox, gắn ID `D-N` và trạng thái hiện tại. |
| [diagrams.md](diagrams.md) | Sơ đồ | Hiện hành | Bản đồ Mermaid: toàn cảnh hệ thống, ranh giới module, luồng request/đăng nhập, pipeline media, lộ trình. |
| [authoration.md](authoration.md) | Đặc tả | Hiện hành | Đặc tả bảo mật: xác thực (authentication), token, thu hồi (revocation), phân quyền (RBAC/RLS), mô hình đe dọa (threat model). |
| [frontend.md](frontend.md) | Đặc tả | Hiện hành | Frontend Next.js 15: lớp template có phiên bản, routing, state, bàn giao xác thực (auth handoff), bảng kê trang. |
| [archivetech.md](archivetech.md) | Đặc tả | Thiết kế hoãn lại (sau v1) | Kiểm soát truy cập theo policy-bundle / user-group / file-gated — được xếp lớp trên nền role theo ADR-02. |
| [archivetech-backend.md](archivetech-backend.md) | Đặc tả | Thiết kế hoãn lại (sau v1) | Backend Go multi-tenant: giao dịch giới hạn theo RLS (RLS-scoped transactions), tenancy xuyên suốt Asynq/storage/cache, các mốc M0–M5. |
| [facebook-comparison.md](facebook-comparison.md) | Phân tích | Ảnh chụp nhanh (2026-07-06) | So sánh từng tính năng giữa Portal và Facebook (thước đo cho UI Olympus). |
| [missing-features.md](missing-features.md) | Phân tích | Ảnh chụp nhanh (2026-07-06) | Phân tích khoảng trống sau v1 đối chiếu với feature.md — backlog để chọn hạng mục xây dựng tiếp theo. |
| [architecture/README.md](architecture/README.md) | Mục lục | Mục lục | Mục lục bộ ADR, quy ước biên soạn, và khung ràng buộc 1 dev / 2 tuần / $100/tháng / 1 VPS. |
| [architecture/00-architecture-review.md](architecture/00-architecture-review.md) | Đánh giá / phân tích | ADR đã chấp thuận | Đánh giá kiến trúc đề ngày (2026-05-24) với các phát hiện là động lực cho ADR 01–05. |
| [architecture/01-v1-scope-cut.md](architecture/01-v1-scope-cut.md) | ADR | ADR đã chấp thuận | v1 = đấu nối Phase 0 + một happy path upload video; bank/social/marketplace bị hoãn lại. |
| [architecture/02-rbac-model-reconciliation.md](architecture/02-rbac-model-reconciliation.md) | ADR | ADR đã chấp thuận | Phân cấp role (role hierarchy) là chuẩn; các policy bundle của archivetech trở thành lớp bổ sung sau này. |
| [architecture/03-single-vps-topology.md](architecture/03-single-vps-topology.md) | ADR | ADR đã chấp thuận | Khung hosting một VPS: tập dịch vụ v1, các compose profile bị vô hiệu hóa, ngân sách RAM/chi phí. |
| [architecture/04-storage-tier-budget.md](architecture/04-storage-tier-budget.md) | ADR | ADR đã chấp thuận | Lưu trữ chỉ dùng R2 trong môi trường triển khai; MinIO bind-mount cho dev cục bộ (cập nhật 2026-06-06). |
| [architecture/05-phase0-wiring-order.md](architecture/05-phase0-wiring-order.md) | ADR | ADR đã chấp thuận | Thứ tự nghiêm ngặt để lấp khoảng trống đấu nối Phase 0: migrations → sqlc → adapters → wiring → auth → frontend. |
| [architecture/06-local-auth-model.md](architecture/06-local-auth-model.md) | ADR | ADR đã chấp thuận | Portal tự quản lý credential (Argon2id); loại bỏ Authentik/OIDC; tái sử dụng cơ chế token/RBAC/revocation. |
| [architecture/diagrams/system-landscape.md](architecture/diagrams/system-landscape.md) | Sơ đồ | Hiện hành (giới hạn phạm vi v1) | Những gì deploy thực sự khởi chạy, cùng các happy path đăng nhập và upload → transcode → playback. |
| [architecture/diagrams/request-flow.md](architecture/diagrams/request-flow.md) | Sơ đồ | Hiện hành (giới hạn phạm vi v1) | Trình tự request đã xác thực đi qua `RequireAuth` → `RequirePermission` → handler. |

---

Đây là bản dịch tiếng Việt (Vietnamese mirror) của `doc/en/`, được đồng bộ tính đến ngày 2026-07-06.
Nếu có nội dung nào có vẻ lỗi thời so với bản gốc, `doc/en/` là bản tham chiếu chuẩn.
