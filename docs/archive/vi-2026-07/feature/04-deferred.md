# 04 — Hoãn / Parking Lot

Những gì được chủ động gác lại trong brainstorm 2026-07-07. Mỗi mục ghi **lý do**
và **điều kiện quay lại** — cú hích đưa nó trở lại bàn. File này tồn tại để khỏi
tranh luận lại các quyết định này mỗi phiên làm việc.

| Mục | Vì sao hoãn | Điều kiện quay lại |
|-----|-------------|---------------------|
| **Import sao kê** (bank) | TCB xuất **PDF** → import tử tế nghĩa là parse PDF/OCR, hố effort cho v1. Schema đã sẵn sàng import (spec 03 P0.2/P0.9) nên chờ không mất gì. | User lấy được CSV/xlsx từ bank đang dùng, **hoặc** chấp nhận build đường CSV generic + column-mapping trước, PDF sau. Thiết kế đã chốt trước: template mapping là dữ-liệu-không-phải-code, `dedup_hash`, rollback theo batch. |
| **TOTP / MFA / step-up** (D-27/D-28) | Từng chặn "bank"; scope ledger không giữ credential ngân hàng nên cổng chặn không áp dụng. | Khoảnh khắc xuất hiện credential/API sync bank thật hoặc bất kỳ feature *chuyển* tiền nào. TOTP khi đó là nhiệm vụ mở khóa có tên, không phải P2 lơ lửng. |
| **Module notifications** (`notify:*`) | Lý do P1 cũ (password reset) giả định sản phẩm đa-user. Vẫn là **xương sống dòng đời** — hoãn ngắn thôi. | Ngay sau khi spec 01–03 hạ cánh: tiêu thụ `media:asset_ready`, `bank:transaction_created`, `comic:chapter_published` vào stream in-app. Email/web-push theo sau. |
| **Password reset, xác minh email** | Chưa có user thứ hai thật; account seed bằng admin/CLI. | User ngoài thật đầu tiên, hoặc module notifications ra đời (cái nào trước). |
| **Friend graph / messenger / tìm người** | Bẫy feature-parity khi n=1 user; life OS bắt đầu từ một người. UI shell giữ nguyên. | Có user thứ hai thật trên một instance (vd gia đình). Quay lại qua "chia sẻ cho thành viên hộ", không phải parity Facebook đầy đủ. |
| **Vertical Movie** | Playback video đã chạy trên `/upload`; catalog thêm ít giá trị hơn việc mở khóa hai domain mới (comic, finance). | Sau khi spec 02 chứng minh pattern vertical — movie thành bài copy-pattern. |
| **Vertical Music / Story** | Cùng pattern, ưu tiên user nêu thấp hơn comic. | Sau movie, hoặc khi user yêu cầu. |
| **HLS đa rendition, ACL playback, presigned direct upload** | Chu kỳ này không đụng; một user trên LAN/VPS chịu được single rendition + HLS gần-như-public ngắn hạn. | ACL playback quay lại **cùng bất kỳ user thứ hai nào**; renditions khi playback mobile/từ xa bị giật. |
| **Domain thời gian (calendar/tasks)** | Từng là domain đời sống rẻ nhất; user chọn tiền bạc + giải trí trước. | Sau spec 03 — nhiều khả năng là mặt đời sống kế tiếp, wire các widget calendar/sinh nhật đã build sẵn. |
| **Nợ / cho vay / đầu tư** (feature.md §8.3–8.5) | Lõi ledger trước; đây là các vòng lặp riêng với model riêng. | Ledger đối soát sạch ≥1 tháng (tín hiệu thành công trong spec 03). |
| **Multi-tenancy + RLS, bank-thật, marketplace, creator economy, observability, LiveKit** | Không đổi so với [ADR-01](architecture/01-v1-scope-cut.md). | Theo ADR-01; ADR-08 không đụng các mục này. |
