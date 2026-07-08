# Đặc tả Feature — Pivot Life-OS (2026-07-07)

Bộ đặc tả feature sinh ra từ phiên brainstorm ngày 2026-07-07. Chúng ghi lại
**bước ngoặt định vị**: Portal không phải Facebook clone đo bằng thước feature-parity —
nó là **"hệ điều hành cuộc sống" self-hosted**: một danh tính số với các mặt
tiền bạc, thời gian, học tập, xã hội, giải trí. Hai trục xây đầu tiên: **tiền bạc**
và **giải trí (video / ảnh / comic)**.

Các đặc tả này nằm hạ nguồn của [feature.md](../feature.md) (decision log gốc)
và thay thế *thứ tự* (không thay nội dung) của
[missing-features.md — Suggested next order](../missing-features.md).
Vị trí trong repo: `doc/vi/feature/` với mirror `doc/en/feature/` (quy tắc song ngữ).

## Thứ tự build

| # | Đặc tả | Module | Trạng thái | Vì sao thứ tự này |
|---|--------|--------|------------|-------------------|
| 00 | [Pivot Life-OS](00-life-os-pivot.md) | — (định vị) | Cần ADR-08 | Khung cho mọi thứ bên dưới |
| 01 | [Pipeline ảnh cho media](01-media-image-pipeline.md) | `media` | Chưa bắt đầu | Nút cổ chai chung: mở khóa comic, avatar, photos, hóa đơn |
| 02 | [Vertical Comic](02-comic-vertical.md) | `comic` | Skeleton | Vertical media → domain đầu tiên; chứng minh pattern |
| 03 | [Sổ chi tiêu (Finance ledger)](03-finance-ledger.md) | `bank` | Chưa có code | Domain "đời sống" đầu tiên; tầm Money Lover, schema sẵn sàng cho import |
| 04 | [Hoãn / parking lot](04-deferred.md) | — | — | Những gì chủ động gác lại, kèm điều kiện quay lại |

## Quy ước ràng buộc mọi đặc tả

- Migration lấy **số thứ tự trống kế tiếp** theo dạng `000N_<module-sở-hữu>_<mô-tả>.up/down.sql`
  (0003–0007 đã dùng đến `0007_media_assets` — kiểm tra lại repo trước khi viết).
- Truy cập chéo module **chỉ** qua package `api/` của module kia; ghép nối chéo module
  qua Asynq event `<module>:<event>`. Không JOIN hay FK chéo module.
- Mọi kiểm tra quyền đi qua RBAC engine / `RequirePermission`
  (grammar `<resource>:<action>[:<scope>]`).
- `shared/openapi.yaml` là hợp đồng API; lưu ý §9 của missing-features.md — các
  path auth trong spec phải sửa trước hoặc song song với endpoint mới.
- Mỗi tài liệu ở đây có mirror `en/`; giữ cặp đồng bộ.
