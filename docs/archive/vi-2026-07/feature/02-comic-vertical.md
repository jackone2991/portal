# 02 — Vertical Comic (end-to-end)

**Module:** `comic` (skeleton: `module.go` + stub `api/`) · **Phụ thuộc:** spec 01 (kind ảnh).
**Tham chiếu:** [feature.md §7](../feature.md). Thay thế các view placeholder `/library/comic`.

## Phát biểu vấn đề

Cả bốn vertical domain đều là skeleton. Comic được chọn trước vì (a) là domain
trục giải trí user yêu cầu, (b) là bằng chứng rẻ nhất cho pattern
**media → domain vertical** (reader comic là trình xem chuỗi ảnh có thứ tự trên
nền spec 01), và (c) frontend đã render sẵn view thư viện placeholder để thay.

## Mục tiêu

- Một comic thật đọc được end-to-end: tạo → upload trang → publish → đọc →
  nhớ tiến độ.
- Module là hiện thực tham chiếu cho pattern vertical
  (migration → `query/` → repository → service/handler → `MountHTTP` → view thật),
  để movie/music/story copy sau.

## Không phải mục tiêu

- Comment/đánh giá comic (lớp xã hội, để sau).
- Theo dõi/subscribe series, thông báo chương mới (cần module notification).
- Import từ nguồn ngoài / scraping.
- Chế độ đọc từng-trang và hai-trang là P1, không phải P0 (cuộn dọc trước —
  chế độ thống trị kiểu webtoon và dễ build nhất).

## User story

- Là creator, tôi tạo comic, thêm chương, upload ảnh trang theo thứ tự, và publish,
  để reader thấy nó trong thư viện.
- Là reader, tôi mở một comic đã publish, cuộn qua một chương, và khi quay lại sau
  tôi tiếp tục đúng chỗ đã dừng.
- Là creator, bản draft chưa publish của tôi vô hình với user khác.

## Yêu cầu

### P0 — bắt buộc

1. **Entity + CRUD**: comic (title, mô tả, cover asset, status draft|published),
   chương có thứ tự, trang có thứ tự (mỗi trang = một image asset ID lấy qua
   `mediaapi` — không FK/JOIN chéo module, validate lúc ghi).
   - [ ] Tạo trang với asset ID không tồn tại hoặc không phải ảnh bị từ chối (422).
   - [ ] Xóa chương sắp xếp lại sạch sẽ; thứ tự trang ổn định và tường minh
         (`sort_order` int, cho phép khoảng trống).
2. **RBAC**: `comics:create` / `comics:update:own` / `comics:publish:own` cho role
   creator; `comics:read:published` cho user đã đăng nhập. Wildcard (`comics:*`)
   bao mọi scope theo grammar sẵn có. Mọi check qua `RequirePermission`.
   - [ ] Không phải creator → 403 khi tạo; reader → 404/403 với draft của người khác.
3. **Reader (cuộn dọc)**: view chương stream trang từ trên xuống dùng variant
   `medium`, lazy-load kèm preload 2–3 trang kế; điều hướng chương trước/sau.
   - [ ] Trang đầu hiện < 2s trên kết nối bình thường (dùng variant, không bao giờ bản gốc).
4. **Tiến độ đọc**: theo user × comic — chương + chỉ số trang cuối; upsert debounce
   từ reader; "Đọc tiếp" hiện trên trang chi tiết comic.
   - [ ] Mở lại comic sau khi đọc tới ch.2 tr.14, đáp xuống trong phạm vi 1 trang quanh đó.
5. **Trang thư viện + chi tiết**: `/library/comic` liệt kê comic đã publish (cover,
   title, số chương); trang chi tiết hiện chương + nút đọc tiếp. Thay placeholders.

### P1 — nên có

6. Chế độ đọc từng-trang và hai-trang (toggle lưu theo user).
7. **Upload zip**: upload một `.zip` ảnh → worker giải nén, tạo page asset theo thứ
   tự tên file (tiêu thụ P2 bulk-upload của spec 01; đây là use case cụ thể của nó).
8. Bookmark (theo user, theo trang).
9. Phát `comic:chapter_published` lên bus (nguồn dòng-đời #2).

### P2 — cân nhắc tương lai

10. Workflow publish có duyệt (soi gương module story khi nó ra đời).
11. Hỗ trợ chiều đọc RTL (manga).

## Phác thảo data model (migration số trống kế tiếp, `000N_comic_*`)

```
comics(id, owner_user_id, title, description, cover_asset_id uuid,
       status text check in ('draft','published'), created_at, updated_at)
comic_chapters(id, comic_id fk, title, sort_order int, created_at)
comic_pages(id, chapter_id fk, asset_id uuid /* media, không FK */, sort_order int)
comic_reading_progress(user_id, comic_id, chapter_id, page_index int,
                       updated_at, pk(user_id, comic_id))
```

## Phác thảo API

```
GET    /api/v1/comics                      danh sách đã publish (phân trang)
POST   /api/v1/comics                      creator
GET    /api/v1/comics/{id}                 chi tiết + chương
PATCH  /api/v1/comics/{id}                 cập nhật / publish
POST   /api/v1/comics/{id}/chapters
POST   /api/v1/chapters/{id}/pages         [{asset_id, sort_order}]
GET    /api/v1/chapters/{id}/pages         payload reader (URL variant)
PUT    /api/v1/comics/{id}/progress        {chapter_id, page_index}
```

## Câu hỏi mở

- **(product, không chặn)** "Published" nghĩa là hiển thị với *mọi* user đã đăng
  nhập, hay cần visibility theo từng comic ngay v1? Khuyến nghị: mọi-user-đã-đăng-nhập
  ở v1; visibility scoping đến cùng lớp social/privacy.
- **(engineering, chặn cho P1.7)** Trần dung lượng zip & sandbox giải nén trong
  worker (chống zip-bomb) — quyết trước khi build upload zip.
