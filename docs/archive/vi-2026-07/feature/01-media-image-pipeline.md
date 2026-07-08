# 01 — Media: Asset kind Ảnh + Hoàn thiện Pipeline

**Module:** `media` (đã build + wire) · **Effort:** ~vài ngày · **Phụ thuộc:** không.
**Mở khóa:** trang comic (spec 02), avatar, photos, đính kèm hóa đơn cho finance.

## Phát biểu vấn đề

Schema media đã cho phép kind `image`, nhưng pipeline worker chỉ xử lý video;
`worker.HandleThumbnail` vẫn là stub; chưa có `DELETE /assets`, và danh sách asset
duy nhất là cái nhỏ trên `/upload`. Hỗ trợ ảnh là **nút cổ chai chung** cho trục
giải trí (comic = trình đọc chuỗi ảnh) và cho nhiều mục P2 rải rác trong backlog
(avatar upload, photos).

## Mục tiêu

- Ảnh upload qua flow sẵn có đạt trạng thái `ready` kèm các bản dẫn xuất (derivatives).
- Asset video có poster thumbnail thật (xóa stub).
- User xóa được asset và storage được dọn thật sự.
- Có trang thư viện media ngoài `/upload`.

## Không phải mục tiêu

- HLS đa rendition, kiểm soát truy cập playback, presigned direct upload — vẫn là
  các mục backlog riêng (missing-features §2), không đụng ở đây.
- Kind audio (P2, xem Yêu cầu).
- Bất kỳ *chỉnh sửa* ảnh nào (crop/xoay) — bên tiêu thụ tự crop phía client trước
  khi upload nếu cần.

## User story

- Là creator comic, tôi upload ảnh trang và nhận derivatives tối ưu web để reader
  tải nhanh trên mobile.
- Là chủ sở hữu asset, tôi xóa nó và nó biến mất khỏi danh sách, playback lẫn
  object storage, để ổ VPS / hóa đơn R2 không phình mãi.
- Là user, tôi duyệt mọi thứ đã upload trong một trang thư viện, lọc theo kind
  và trạng thái.

## Yêu cầu

### P0 — bắt buộc

1. **Nạp ảnh**: upload chấp nhận `image/jpeg|png|webp`; task worker
   (`media:process_image`) sinh derivatives — bộ đề xuất: `thumb` (~320w),
   `medium` (~1280w), cộng bản gốc; xóa EXIF; đánh dấu asset `ready`.
   - [ ] Cho một JPEG 12 MP, khi worker xong, asset ở trạng thái `ready`
         với 2 derivatives + bản gốc trong storage.
   - [ ] Dữ liệu GPS/vị trí EXIF vắng mặt ở mọi biến thể lưu trữ.
   - [ ] File hỏng/quá cỡ đánh dấu asset `failed` kèm thông báo lỗi,
         không bao giờ crash worker.
2. **Thumbnail video**: hiện thực `HandleThumbnail` — trích một frame (ví dụ tại
   10% thời lượng) bằng ffmpeg, lưu làm poster của asset.
   - [ ] Mọi video transcode mới hiện poster trong thư viện và trên Vidstack.
3. **Xóa**: `DELETE /api/v1/assets/{id}` — permission `media:asset:delete:own`
   (admin wildcard bao phần còn lại); xóa row DB và dọn mọi storage key
   (bản gốc, renditions, variants).
   - [ ] Sau khi xóa, URL HLS và mọi URL variant trả 404/403.
   - [ ] Xóa idempotent (gọi lần hai → 404, không 500).
4. **Trang thư viện**: `/library/media` — lưới poster/thumb, lọc kind + trạng thái,
   phân trang; hành động: mở, xóa.

### P1 — nên có

5. Đổi tên / sửa metadata (`PATCH /assets/{id}`: title).
6. **Phát `media:asset_ready`** lên bus khi asset bất kỳ đạt `ready`
   (nhảy từ P3 lên — đây là nguồn phát event dòng-đời đầu tiên).

### P2 — cân nhắc tương lai (thiết kế sẵn, chưa build)

7. Kind audio (schema đã cho phép).
8. Upload hàng loạt (đa file / zip) — spec 02 có bên tiêu thụ cụ thể cho zip.

## Phác thảo data model

Bảng mới (migration số trống kế tiếp, `000N_media_variants`):

```
media_asset_variants(
  id uuid pk, asset_id uuid → media_assets (cùng module: FK OK),
  variant text check in ('thumb','medium','original','poster'),
  storage_key text, width int, height int, size_bytes bigint,
  created_at timestamptz
)
```

Poster video là một variant `poster` trên asset video — một cơ chế cho cả hai kind.

## Phác thảo API (thêm vào `shared/openapi.yaml`)

```
DELETE /api/v1/assets/{id}
PATCH  /api/v1/assets/{id}            {title}
GET    /api/v1/assets?kind=&status=&page=   (mở rộng list sẵn có)
```

## Câu hỏi mở

- **(engineering, không chặn)** Sinh derivative bằng ffmpeg (đã có trong image
  worker, không thêm dep) hay thư viện imaging Go (resize nét hơn, thêm dep).
  Khuyến nghị: ffmpeg trước; đổi sau, giấu sau task handler.
- **(engineering, không chặn)** Kích thước/độ phân giải tối đa chấp nhận cho ảnh
  (đề xuất 50 MB / 12k px, validate lúc upload).
