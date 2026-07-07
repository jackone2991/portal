# Portal — Đánh Giá Kiến Trúc & Các ADR

> **Trạng thái (2026-07-06):** Bộ cắt v1 được định nghĩa bởi bộ ADR này đã ra mắt — vòng lặp demo (đăng nhập bằng mật khẩu local → trang chủ Next.js đã xác thực → tải lên mp4 → worker transcode HLS → phát lại Vidstack → đăng xuất có thể thu hồi) đã khép kín và được commit. ADR-06 đã thay thế đăng nhập OIDC/Authentik bằng xác thực mật khẩu do Portal tự quản lý. `MILESTONE_CHECKS.md` ở gốc repo là bộ theo dõi trạng thái đang sống.

Đánh giá đang sống về kiến trúc Portal so với các ràng buộc đã nêu. Mỗi ADR theo hình mẫu chuẩn *bối cảnh → quyết định → phương án → phân tích đánh đổi → hệ quả → hạng mục hành động*.

## Bộ tài liệu này được tạo ra như thế nào

Các ADR này được viết như một **lượt đánh giá** trên kho tài liệu thiết kế hiện có, không phải viết từ đầu. Đầu vào gồm:

- [`CLAUDE.md`](../../../CLAUDE.md) — README chính thức của dự án và các quy ước theo từng mô-đun.
- [`backend/MODULES.md`](../../../backend/MODULES.md) — hợp đồng ranh giới mô-đun.
- `feature.md` — bản đặc tả tính năng đầy đủ với 40 quyết định được ghi lại (`D-1` … `D-40`) trên 12 phase.
- `diagrams.md` — sơ đồ Mermaid cảnh quan hệ thống, bản đồ mô-đun, luồng request, chuỗi OIDC, pipeline tài sản, lộ trình *(chuỗi đăng nhập OIDC sau đó đã bị loại bỏ bởi ADR-06)*.
- `archivetech.md` — một tầm nhìn RBAC cạnh tranh (gói chính sách + quyền được cấp theo tệp) xung đột với `feature.md` và `CLAUDE.md`.
- `docker-compose.yml`, `Makefile`, `shared/openapi.yaml`, `traefik/*` — hạ tầng thực tế.

## Ràng buộc khung sườn

Mỗi ADR dưới đây được đánh giá dựa trên khung ràng buộc cứng sau:

| Ràng buộc | Giá trị |
| --- | --- |
| Đội ngũ | **1 nhà phát triển** |
| Thời gian tới v1 | **2 tuần** |
| Ngân sách hạ tầng | **≤ $100 / tháng** |
| Mục tiêu triển khai | **Một VPS duy nhất** |
| Hình thức | Tự lưu trữ (self-hosted), thân thiện mã nguồn mở |

Kho tài liệu thiết kế hiện có không giả định rõ ràng bất kỳ điều nào trong số này. Hầu hết 40 quyết định là hợp lý về mặt trừu tượng — nhưng một số **không tương thích về phạm vi** với khung ràng buộc và phải được hoãn lại. Hai ADR đầu tiên nói rõ điều đó; phần còn lại đứng vững hay không tùy thuộc vào việc chúng có sống sót qua lần cắt hay không.

## Chỉ mục

| ADR | Trạng thái | Chủ đề |
| --- | --- | --- |
| [00-architecture-review](./00-architecture-review.md) | Được chấp nhận | Đánh giá điều hành — cái gì chịu tải, cái gì gặp rủi ro, thiết kế xung đột với chính nó ở đâu |
| [01-v1-scope-cut](./01-v1-scope-cut.md) | Được chấp nhận | Cắt 12 phase của feature.md xuống một v1 vừa với 2 tuần / 1 dev / $100 / 1 VPS |
| [02-rbac-model-reconciliation](./02-rbac-model-reconciliation.md) | Được chấp nhận | Hòa giải RBAC gói-chính-sách của archivetech.md với RBAC phân-cấp-vai-trò của feature.md/CLAUDE.md |
| [03-single-vps-topology](./03-single-vps-topology.md) | Được chấp nhận | Bộ dịch vụ compose, ngân sách tài nguyên, và những gì cần tắt cho v1 |
| [04-storage-tier-budget](./04-storage-tier-budget.md) | Được chấp nhận | MinIO origin + R2 edge so với chỉ-R2 so với chỉ-MinIO dưới trần ngân sách (cập nhật 2026-06-06: chỉ-R2 = các môi trường đã triển khai; dev dùng MinIO) |
| [05-phase0-wiring-order](./05-phase0-wiring-order.md) | Được chấp nhận | Thứ tự đường găng để đóng khoảng trống nối dây Phase 0 trong `cmd/api/main.go` (đã thực thi; khoảng trống đã đóng) |
| [06-local-auth-model](./06-local-auth-model.md) | Được chấp nhận | Chuyển đăng nhập từ OIDC/Authentik sang xác thực mật khẩu local do Portal tự quản lý (thay thế đăng nhập OIDC của ADR-05; đã thực thi 2026-07-05, Authentik đã bị gỡ bỏ) |

## Sơ đồ

- [`diagrams/system-landscape.md`](./diagrams/system-landscape.md) — cảnh quan theo phạm vi v1 (thưa hơn phiên bản dài hạn của `diagrams.md`).
- [`diagrams/request-flow.md`](./diagrams/request-flow.md) — chuỗi middleware của request đã xác thực với bộ cắt v1 được áp dụng.

## Quy ước

- Tên file ADR: `NN-kebab-case-subject.md`. Số thứ tự tuần tự và **không bao giờ được tái sử dụng** một khi đã xuất bản.
- Trạng thái chuyển theo trình tự Proposed (Đề xuất) → Accepted (Được chấp nhận) → Deprecated (Không dùng nữa). Để sửa đổi một ADR đã Được chấp nhận, hãy viết một ADR mới **thay thế** ADR cũ và cập nhật trạng thái của ADR cũ. Đừng sửa lịch sử.
- Dùng Mermaid cho sơ đồ (khớp quy ước dự án trong `diagrams.md`); GitHub render chúng natively.
- Tham chiếu chéo các quyết định trong `feature.md` bằng id `D-N` của chúng bất cứ khi nào ADR đang nhắc lại hoặc tinh chỉnh một quyết định đã chốt.
