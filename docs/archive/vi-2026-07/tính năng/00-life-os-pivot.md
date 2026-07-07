# 00 — Pivot Life-OS (định vị)

**Trạng thái:** đã thống nhất trong brainstorm 2026-07-07 · cần **ADR-08** để có hiệu lực.
**Chủ trì:** product (solo dev).

## Phát biểu vấn đề

Các bản phân tích gap của Portal ([missing-features.md](../missing-features.md),
[facebook-comparison.md](../facebook-comparison.md)) đo sản phẩm bằng thước Facebook.
Giá trị của Facebook dựa trên network effect; Portal là self-hosted, single-VPS,
bắt đầu từ **một user**. Đuổi theo friend graph / messenger / tìm người trước tiên
là cái bẫy feature-parity: effort cao, giá trị gần bằng 0 khi n=1 user.

## Quyết định (sẽ phê chuẩn thành ADR-08)

Portal là một **hệ điều hành cuộc sống self-hosted**: một danh tính số ("một cá thể
con người") với các mặt — **tiền bạc, thời gian, học tập, xã hội, giải trí**. Hai
tài sản kiến trúc đã có sẵn trở thành chất keo khiến "tất cả trong một nền tảng"
thắng "mỗi domain một app xịn":

1. **Event bus** (hard rule: module chỉ ghép nối qua Asynq `<module>:<event>`).
   Mọi domain đời sống phát event; UI newsfeed sẵn có trở thành **dòng đời** của
   user ("hôm nay chi 500k", "3 ngày nữa sinh nhật mẹ", "đọc xong chương 3"),
   không phải timeline xã hội.
2. **Một identity + RBAC** xuyên suốt mọi domain thay vì 5 account 5 app.

Hai trục đầu: **tiền bạc** (spec 03) và **giải trí** (spec 01–02).

## Mục tiêu

- Mỗi module domain mới phát ít nhất một event vào bus ngay từ ngày đầu.
- Hai domain đời sống đầu tiên (finance, comic) dùng được end-to-end bởi một user
  thật (chính dev).
- Backlog được xếp hạng lại cho khớp định vị (xem Hệ quả).

## Không phải mục tiêu

- Xây module notifications *ngay bây giờ* (nó là xương sống của dòng đời và sẽ đến
  ngay sau spec 01–03; xem [04-deferred.md](04-deferred.md)).
- Bất kỳ feature xã hội đa-user nào làm động lực ưu tiên (friend graph, messenger,
  tìm người đều tụt hạng — quay lại khi có user thứ hai thật).
- Đổi tên hay làm lại UI shell Olympus; shell được tái sử dụng, chỉ nguồn dữ liệu
  đổi ý nghĩa.

## Hệ quả lên backlog

- **Tụt khỏi P1:** friend graph, messaging, tìm người, password reset qua email
  (lý do P1 cũ — "unblock một surface đã ship" — giả định một sản phẩm production
  đa-user chưa tồn tại; user đang được seed bằng admin/CLI).
- **Thăng hạng / đổi động cơ:** notifications & event stream (với tư cách xương
  sống dòng đời, không phải ống nước cho password reset); emit `media:asset_ready`
  nhảy từ P3 lên.
- **Tách scope:** "bank" tách thành **finance ledger** (vào scope ngay, spec 03) và
  **tích hợp bank thật** (vẫn hoãn; cần TOTP/step-up trước).
- **Posts** đổi nghĩa: post type thật đầu tiên là **journal / sự kiện đời sống của
  chính user**, không phải status cho bạn bè.

## Việc cần làm

- [ ] Viết **ADR-08** (context → decision → options → trade-offs → consequences →
      action items), amend [ADR-01](architecture/01-v1-scope-cut.md): định vị
      life-OS; finance ledger vào scope; "bank thật" vẫn hoãn.
- [ ] Cập nhật "Suggested next order" trong `missing-features.md` trỏ về folder này.
- [ ] Giữ mirror `doc/en/` đồng bộ cho mọi file bị chạm.

## Câu hỏi mở

- **(product, không chặn)** "Dòng đời" thay thế newsfeed ở `/` hay sống song song
  như một tab? Quyết khi hai nguồn phát event đầu tiên đã tồn tại.
- **(product, không chặn)** Domain thời gian (calendar/tasks) từng là domain đời
  sống rẻ nhất nhưng được chủ động xếp sau tiền bạc + giải trí — xem lại sau spec 03.
