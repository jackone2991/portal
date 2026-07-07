# Portal vs. Facebook — So sánh chức năng

**Đã xác minh lần cuối:** 2026-07-06 — phản ánh vòng demo v1 đã khép lại (auth mật khẩu local + upload→transcode→phát HLS); xem [MILESTONE_CHECKS.md](../../MILESTONE_CHECKS.md) để theo dõi trạng thái cập nhật liên tục.

Tầng social của Portal so với Facebook, theo từng nhóm tính năng. Đi kèm
[missing-features.md](missing-features.md) (so với đặc tả của chính Portal); doc này
lấy **Facebook làm thước đo** vì UI là bản port template Olympus (giống Facebook).

**Trạng thái:** ✅ chạy thật · ◐ chỉ UI (có màn hình, không backend) · ○ chưa làm · ⛔ ngoài scope Portal.

> **Kết luận một dòng:** Portal hiện là **vỏ UI hình-Facebook với đúng một năng lực
> thật — pipeline video (upload → transcode → phát HLS)**. Auth cơ bản chạy. Gần như
> mọi *backend social* (bài viết, bạn bè, tin nhắn, thông báo, group, tìm kiếm) chỉ là
> **UI dữ liệu mẫu**. Độ phủ so với sản phẩm social của Facebook còn thấp; slice
> video/"Watch" là điểm sáng chạy thật.

---

## Tổng quan độ phủ

| Nhóm Facebook | Portal | Ghi chú |
|---|---|---|
| Đăng ký / đăng nhập | ◐ | chỉ email+password; thiếu phone/Google/Apple, 2FA, khôi phục |
| Profile & timeline | ○ | chưa có trang profile; avatar chỉ chữ cái |
| Newsfeed & đăng bài | ◐ | composer post vào state cục bộ; chưa có API/ranking |
| Reaction / bình luận / share | ○ | số đếm tĩnh |
| Stories (24h) | ○ | — |
| Reels (video ngắn) | ○ | — |
| Video / Watch | ✅ | **upload → HLS → phát chạy** (1 rendition) |
| Ảnh & album | ○ | pipeline mới xử lý video |
| Bạn bè / social graph | ◐ | request/suggestion/group là mẫu |
| Follow (bất đối xứng) | ○ | — |
| Messenger (chat/gọi) | ◐ | thanh chat + dropdown tin nhắn trang trí |
| Group / cộng đồng | ○ | chỉ có mục menu |
| Pages | ○ | "Pages You May Like" là mẫu |
| Sự kiện / sinh nhật | ◐ | widget tĩnh |
| Thông báo | ◐ | dropdown chuông/activity hard-code |
| Tìm kiếm | ◐ | có ô nhập, không backend/kết quả |
| Marketplace | ⛔ | đã hoãn (ADR-01) |
| Riêng tư & cài đặt | ○ | không có |
| Kiểm duyệt & an toàn | ○ | không có |
| Kiếm tiền / quảng cáo | ⛔ | đã hoãn |
| App mobile / i18n / dark mode | ○ | chỉ web, chỉ tiếng Anh, chỉ light theme |

---

## 1. Tài khoản, đăng nhập & định danh
- ◐ Đăng ký/đăng nhập email+password, remember-me, khoá brute-force, phiên qua JWT+refresh.
- ○ **Thiếu so với FB:** đăng nhập bằng số điện thoại, **Login với Google/Apple**, **xác thực hai yếu tố (2FA)**, khôi phục tài khoản, xác minh email/phone, quản lý thiết bị tin cậy, UI "đăng xuất mọi phiên", tên/username, tick xanh xác minh.

## 2. Profile & timeline
- ○ Không có profile ngoài `display_name` + avatar chữ cái.
- ○ **Thiếu so với FB:** trang profile (timeline), **upload avatar + ảnh bìa**, bio/intro, about (công việc/học vấn/nơi ở/liên hệ), life events, featured, tab bạn bè/ảnh/video, activity log, nút follow/message trên profile.

## 3. Newsfeed & đăng bài
- ◐ Composer có tab Status/Media/Blog + feed — nhưng đăng chỉ prepend vào **state React cục bộ**, feed là mẫu.
- ○ **Thiếu so với FB:** lưu bài + API feed, **chọn đối tượng** (public/friends/only-me/custom), bài ảnh/video/link/GIF/poll, **feeling/activity**, **check-in/vị trí**, tag bạn, màu nền, **xếp hạng feed**, ẩn/snooze/"see first", sửa/xoá, nháp & lên lịch.

## 4. Reaction, bình luận & share
- ○ Số like + comment/share tĩnh; không lưu tương tác.
- ○ **Thiếu so với FB:** **6 reaction** (like/love/haha/wow/sad/angry), **bình luận phân cấp** + reaction cho comment, **share/quote-share** lên feed/group/tin nhắn của mình, save/bookmark, react trên bình luận, mention trong comment.

## 5. Stories & Reels
- ○ Cả hai đều chưa có.
- ○ **Thiếu so với FB:** **Stories** (ảnh/video tạm thời 24h, người xem, trả lời, reaction, highlight), **Reels** (feed video dọc ngắn, audio, remix).

## 6. Video / "Watch"  ← điểm mạnh của Portal
- ✅ Upload trực tiếp → worker `ffmpeg` → **VOD HLS** → **phát Vidstack** ở `/upload`; metadata ffprobe.
- ○ **Thiếu so với FB:** ladder đa bitrate + master playlist, **thumbnail/preview**, feed/khám phá Watch, reaction/bình luận video, lượt xem, phụ đề, **live streaming**, playlist, kiểm soát tải, kiếm tiền.

## 7. Ảnh & album
- ○ Chưa có pipeline ảnh (schema cho `image` nhưng chưa dùng).
- ○ **Thiếu so với FB:** upload ảnh, **album/carousel**, tag ảnh, gợi ý tag kiểu nhận diện khuôn mặt, EXIF/ngày, lightbox, lịch sử ảnh bìa/đại diện.

## 8. Bạn bè & social graph
- ◐ Dropdown **lời mời kết bạn** ở header, "Friend Suggestions", panel **nhóm bạn** bên phải (Close Friends/Family/Uncategorized) — tất cả **dữ liệu mẫu**; accept/decline chỉ đổi state cục bộ.
- ○ **Thiếu so với FB:** bảng friendship + endpoint request/accept/decline/unfriend, **People You May Know** (bạn chung), **block**, danh sách bạn tùy chỉnh, xem bạn chung, phạm vi riêng tư "friends".

## 9. Follow (bất đối xứng)
- ○ Chưa có (FB có follow song song với friendship cho người nổi tiếng/creator).
- ○ **Thiếu so với FB:** follow/unfollow, đếm follower/following, "see first", follow công khai cho page/creator.

## 10. Messenger (chat & gọi)
- ◐ Thanh "Olympus Chat" + dropdown tin nhắn trang trí; có modal stub `ChatResponsive`.
- ○ **Thiếu so với FB:** lưu conversation + message, **chat 1:1 & nhóm**, realtime (WS/SSE), đính kèm media/voice/file, **read receipt + typing + active status**, reaction tin nhắn, message request, **gọi voice/video**, tùy chọn mã hoá đầu-cuối, thu hồi/sửa.

## 11. Group / cộng đồng
- ○ Mục menu "Friend Groups" chỉ để điều hướng; không có entity group.
- ○ **Thiếu so với FB:** tạo/tham gia group, feed group, **vai trò (admin/moderator)**, duyệt thành viên, nội quy, sự kiện group, ghim bài, thảo luận vs feed, quyền riêng tư (public/private/hidden).

## 12. Pages
- ○ "Pages You May Like" + "Fav Pages Feed" là mẫu.
- ○ **Thiếu so với FB:** entity page (tạo/follow/like), vai trò page, đăng bài dưới tên page, insight/analytics, danh mục, đánh giá.

## 13. Sự kiện, sinh nhật & lịch
- ◐ Thẻ Birthday, widget lịch, menu "Calendar and Events"/"Friends Birthdays" — **đều tĩnh**.
- ○ **Thiếu so với FB:** sự kiện (tạo/RSVP/mời/lặp lại), feed sự kiện, **nhắc sinh nhật**, tích hợp lịch.

## 14. Thông báo
- ◐ Dropdown chuông + "Activity Feed" là danh sách hard-code; badge là hằng số.
- ○ **Thiếu so với FB:** kho thông báo + `GET /me/notifications`, **giao realtime** (SSE/WS), **web push + email**, cài đặt theo từng loại, mark-read/all-read lưu lại, gộp/aggregate. *(Bị chặn bởi module Notifications — xem missing-features §5.)*

## 15. Tìm kiếm
- ◐ Ô header "Search here people or pages…" + "Find Friends" — không backend.
- ○ **Thiếu so với FB:** **tìm kiếm tổng hợp** (người/bài viết/page/group/ảnh/video), typeahead, bộ lọc, tìm kiếm gần đây, kết quả xếp hạng.

## 16. Marketplace / thương mại
- ⛔ Đã hoãn ([ADR-01](architecture/01-v1-scope-cut.md), feature.md §11). FB có listing, danh mục, chat với người bán, shop.

## 17. Riêng tư, cài đặt & quyền dữ liệu
- ○ Không có gì ngoài cookie auth.
- ○ **Thiếu so với FB:** **kiểm soát đối tượng/hiển thị** cho từng bài viết & từng trường profile, block, **activity log**, **tải dữ liệu của bạn (GDPR)**, xoá tài khoản, tùy chọn quảng cáo/thông báo, UI 2FA & quản lý phiên, "ai tìm được tôi".

## 18. Kiểm duyệt, an toàn & tin cậy
- ○ Chưa có gì.
- ○ **Thiếu so với FB:** **báo cáo nội dung/người dùng**, block, ẩn, tiêu chuẩn cộng đồng + thực thi, cảnh báo nội dung, phát hiện spam/lạm dụng, luồng kháng nghị, dashboard kiểm duyệt cho admin. *(Audit log đã có ở phía auth.)*

## 19. Kiếm tiền
- ⛔ Đã hoãn (feature.md §10). FB có quảng cáo, trả tiền cho creator, stars, subscription, fundraiser.

## 20. Nền tảng, độ phủ & hoàn thiện
- ○ **Thiếu so với FB:** **app mobile** native (chỉ web; responsive mới một phần), **đa ngôn ngữ** (UI chỉ tiếng Anh; docs song ngữ), **dark mode** (chỉ light theme — Olympus dark tồn tại dưới dạng token nhưng chưa có toggle), offline/PWA, rà soát khả năng tiếp cận (accessibility), presence realtime, và các mảng rộng hơn của FB (Dating, Gaming, Jobs, Fundraisers, Memories).

---

## Nên xây gì để giống Facebook nhất, nhanh nhất
Đường rẻ nhất để đi từ vỏ hiện tại đến một "Facebook-lite khả tín". Thứ tự ở đây tối
ưu cho độ-giống-Facebook; thứ tự xây dựng chuẩn (đặt module Notifications lên đầu vì
nó mở khoá việc đặt lại mật khẩu) nằm ở [missing-features.md — Thứ tự đề xuất tiếp theo (P1)](missing-features.md#suggested-next-order-p1):

1. **Bài viết + reaction + bình luận** — biến newsfeed chủ lực thành thật. *(P1)*
2. **Đồ thị bạn bè** — request/accept/suggestion nối vào dropdown sẵn có. *(P1)*
3. **Module Notifications + realtime** — chuông/activity feed thành live. *(P1)*
4. **Messenger** — conversation + WS; thanh chat dùng được. *(P1/P2)*
5. **Trang Profile** + upload avatar/ảnh bìa (tái dùng pipeline media). *(P2)*
6. **Tìm kiếm** (người/bài/page). *(P2)*
7. **Ảnh/album** + **Stories** (tái dùng media). *(P2/P3)*
8. **Group & Pages & Events**. *(P3)*

Tất cả ở trên **đã được thiết kế sẵn trong UI** — việc cần làm là backend + nối dây,
không phải màn hình mới. Marketplace, kiếm tiền, quảng cáo, app mobile, và Dating/Gaming
**nằm ngoài scope đã tuyên bố của Portal** và không khuyến nghị cho v1.
