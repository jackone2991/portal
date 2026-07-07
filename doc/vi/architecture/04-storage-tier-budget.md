# ADR-04: Tầng lưu trữ — chỉ-R2 cho v1; hoãn MinIO origin tới phase đa khu vực

**Trạng thái:** Được chấp nhận (xem Cập nhật 2026-06-06, 2026-07-06)
**Ngày:** 2026-05-24
**Người quyết định:** kirito
**Ảnh hưởng:** [docker-compose.yml](../../../docker-compose.yml), `backend/internal/platform/storage/`, [diagrams.md §1] (system landscape)

## Cập nhật (2026-06-06) — dev cục bộ chạy MinIO trên một thư mục local; chỉ-R2 vẫn áp dụng cho các môi trường đã triển khai

Quyết định chỉ-R2 bên dưới vẫn đứng vững **cho các môi trường đã triển khai** (staging/prod): không có tầng MinIO origin, không có replication. Với **phát triển cục bộ**, `docker-compose.yml` cho dev vẫn giữ MinIO làm origin tương thích S3, **gắn với thư mục local `./data/minio`** (một bind-mount, không phải named volume), kèm một `minio-setup` (`mc`) chạy một lần để tạo bucket media.

Lý do: luồng upload media dùng **presigned URL** (trình duyệt PUT thẳng tới store). Một driver local-filesystem thuần túy không thể phát hành presigned URL, điều này sẽ buộc phải có một đường upload thứ hai, chỉ-dành-cho-dev (client → API → disk) và khiến dev lệch khỏi prod. MinIO nói S3, nên dev giữ đúng luồng presigned và **go-live chỉ là một thay đổi `.env`** — trỏ lại `S3_ENDPOINT` + `S3_ACCESS_KEY`/`S3_SECRET_KEY` sang R2 và đặt `S3_USE_PATH_STYLE=false`. Không có khác biệt code; `platform/storage/` vẫn là một S3 client duy nhất.

Tóm lại: **dev = MinIO (thư mục local) · prod = R2.** Điều này *tinh chỉnh*, không đảo ngược, quyết định bên dưới. Hạng mục hành động 1–2 được thay thế tương ứng: MinIO chỉ bị gỡ khỏi overlay prod (`docker-compose.prod.yml`); base dev vẫn giữ nó trên bind-mount.

## Cập nhật (2026-07-06) — đã triển khai; các sai khác so với nội dung quyết định

- **Đã triển khai:** `backend/internal/platform/storage/` là một S3 client aws-sdk-go-v2 duy nhất (`BaseEndpoint` + `UsePathStyle`, có test trong `s3_test.go`); `.env.example` mang khối `S3_*` (tên biến là `S3_ACCESS_KEY`/`S3_SECRET_KEY`).
- **Đường upload:** `POST /api/v1/assets` trả về một presigned PUT (đường prod); dev còn dùng thêm một `PUT /api/v1/assets/{id}/source` đi qua proxy của API — câu "API không bao giờ giữ byte upload" chỉ đúng cho đường presigned.
- **Phát lại:** v1 phục vụ HLS qua proxy công khai của API `GET /api/v1/assets/{id}/hls/*`, không trực tiếp từ R2/edge; fetch trực tiếp từ edge vẫn là mục tiêu cho deployed-prod.
- **Object key như-đã-triển-khai:** `uploads/<id>/original<ext>` và `hls/<assetID>` — tiền tố tenant `org/<tid>` trong mục 4 của phần Quyết định bị hoãn cho tới khi tenancy được đưa vào.
- Tham chiếu tới Authentik trong mục 3 của "Vì sao MinIO từng tồn tại" đã bị bãi bỏ ([ADR-06](./06-local-auth-model.md)).

## Bối cảnh

`diagrams.md` §1 vẽ một kiến trúc lưu trữ hai tầng:

- **MinIO origin** chạy trên VPS, giữ các byte chính tắc (`org/<tid>/assets/source/<id>.mp4`, `org/<tid>/assets/hls/<id>/`).
- **Cloudflare R2 edge** đứng phía trước, với origin-pull khi cache miss. Replication liên tục qua `mc admin replicate` giữ cho R2 luôn "hot".

Đây là kiến trúc đúng đắn cho một SaaS self-hosted đa khu vực với yêu cầu chủ quyền dữ liệu (data-sovereignty) nghiêm ngặt (một số operator muốn giữ byte chính tắc trong lãnh thổ pháp lý của họ). Đây là **kiến trúc sai cho v1**, và có lẽ là kiến trúc sai cho bất kỳ triển khai một-VPS nào.

### Chi phí của hai tầng lưu trữ trên một VPS duy nhất

| Thành phần chi phí | MinIO + R2 (sơ đồ hiện tại) | Chỉ-R2 (đề xuất) |
| --- | --- | --- |
| Disk VPS cho asset | 100–500 GB ($5–25/tháng disk thêm trên Hetzner) | 0 GB |
| Dịch vụ MinIO | ~150 MB RAM | 0 MB |
| Giám sát replication | dashboard health + lag của `mc admin replicate` | Không có — R2 là bản sao duy nhất |
| Chiến lược backup | Backup MinIO (rclone sang S3 thứ hai) VÀ backup R2 | R2 chính là backup; export hàng tuần sang cold S3 nếu muốn thận trọng hơn |
| Bề mặt vận hành | Hai bộ credential, hai URL base, hai cấu hình CORS | Mỗi thứ một cái |
| Kịch bản recovery | Cả hai tầng có thể lệch nhau; cần playbook reconciliation | Một nguồn chân lý duy nhất |

### Vì sao MinIO từng tồn tại trong thiết kế

Ba lý do chính đáng, không lý do nào áp dụng cho v1:

1. **Chủ quyền dữ liệu (data sovereignty)** — một số operator về mặt pháp lý không thể gửi dữ liệu người dùng cho Cloudflare. v1 chỉ có một operator (bạn) và được host tại một site Hetzner vốn dĩ đã không tuân thủ chủ quyền cho nhiều lãnh thổ pháp lý.
2. **Trần chi phí dưới băng thông khổng lồ** — ở mức egress rất cao (>10 TB/tháng), tự chạy origin riêng với một CDN rẻ hơn có thể đánh bại R2. Egress của demo v1 chỉ tính bằng MB.
3. **Vận hành air-gapped** — một số self-hoster không thể chạm tới internet công cộng từ VPS. Không phải v1; VPS vốn đã cần internet cho luồng OIDC của Authentik, image pull, và DNS. *(Authentik từ đó đã bị gỡ bỏ — ADR-06; image pull và DNS vẫn áp dụng.)*

### Chỉ-R2 mang lại gì

R2 tương thích S3: cùng `s3.Client` từ AWS SDK, cùng luồng presigned-URL, cùng lifecycle rule. Nó *đồng thời* cũng là một CDN edge — fetch từ trình duyệt chạm thẳng vào PoP của Cloudflare, không cần origin-pull. Egress miễn phí trong mạng Cloudflare (bao gồm cả fetch từ trình duyệt qua DNS Cloudflare).

Giá (tháng 5/2026, mức giá công khai của Cloudflare):

- Lưu trữ: $0.015/GB-tháng (10 GB free tier)
- Thao tác Class A (PUT, POST, COPY): $4.50/triệu (1 triệu miễn phí)
- Thao tác Class B (GET, HEAD): $0.36/triệu (10 triệu miễn phí)
- **Egress: $0 ra internet qua Cloudflare**

Một demo v1 với 5 GB asset lưu trữ và 100 nghìn request/tháng tốn khoảng $0.

## Quyết định

**v1 dùng Cloudflare R2 làm tầng lưu trữ duy nhất. MinIO bị gỡ khỏi `docker-compose.yml` cho v1.** Abstraction `platform/storage/` vẫn là một interface S3 chung chung (vốn đã vậy), nên đưa MinIO origin trở lại ở một phase tương lai chỉ là một thay đổi cấu hình, không phải thay đổi code.

Cụ thể:

1. Go S3 client trỏ tới `https://<account>.r2.cloudflarestorage.com` thay vì `minio:9000`. Cùng lời gọi SDK.
2. Upload đi thẳng từ API tới R2 qua presigned URL — trình duyệt PUT tới R2, API chỉ ký.
3. Worker transcode đọc/ghi R2. Input/output của FFmpeg dùng streaming kiểu `s3fs` qua SDK hoặc qua file local tạm trong `/tmp` (tmpfs-backed).
4. R2 bucket được tiền tố theo tenant đúng như spec đã định: `org/<tid>/assets/source/`, `org/<tid>/assets/hls/`. Cơ chế tiền tố này không phụ thuộc bucket.
5. **Không replication.** R2 là nguồn chân lý trong v1. Một job export hàng tuần (Asynq cron) copy sang một bucket R2 thứ hai (hoặc S3 off-Cloudflare) cho disaster recovery; triển khai ở Phase 0.5 nếu có dữ liệu bên ngoài xuất hiện.
6. **CORS trên R2** phải cho phép origin của frontend (`https://${APP_DOMAIN}`) để browser PUT trực tiếp; ghi vào deployment guide.

## Các phương án đã cân nhắc

### Phương án A — MinIO origin + R2 edge với replication (sơ đồ hiện tại)

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | $5–25/tháng disk VPS thêm + lưu trữ R2 |
| Độ phức tạp vận hành | Cao — hai hệ thống, replication, recovery hai nguồn |
| Độ trễ | Fetch đầu tiên chậm (origin pull); các lần sau nhanh |
| Bề mặt lỗi | Disk MinIO, độ trễ replication, R2 outage — đều tách biệt |

**Ưu điểm:** Hoàn chỉnh nhất về kiến trúc; khớp với tầm nhìn dài hạn.
**Nhược điểm:** Hai bản của mọi thứ cho một hệ thống chỉ có một user.

### Phương án B — Chỉ-R2  *(được chọn cho v1)*

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | ~$1–5/tháng cho dữ liệu ở quy mô v1 |
| Độ phức tạp vận hành | Thấp — một bộ credential, một URL |
| Độ trễ | Luôn nhanh nhờ edge (PoP gần người xem nhất) |
| Bề mặt lỗi | R2 outage là toàn phần — chấp nhận điều đó cho v1 |

**Ưu điểm:** Rẻ nhất, đơn giản nhất, nhanh nhất cho người xem. Loại bỏ RAM + disk của MinIO khỏi ngân sách VPS.
**Nhược điểm:** Một điểm lỗi duy nhất (R2). Không có câu chuyện chủ quyền dữ liệu. Egress tới đích không-phải-Cloudflare (vd `wget` từ ngoài mạng Cloudflare) bị tính phí.

### Phương án C — Chỉ-MinIO (không R2)

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | Chỉ disk VPS (~$25/tháng cho 240 GB trên Hetzner BX-class) |
| Độ phức tạp vận hành | Thấp — một hệ thống |
| Độ trễ | Fetch từ trình duyệt chạm thẳng VPS; chậm về mặt địa lý |
| Bề mặt lỗi | VPS = SPoF cho cả compute lẫn storage |

**Ưu điểm:** Hoàn toàn tự chứa; không phụ thuộc bên ngoài.
**Nhược điểm:** Phát lại HLS trên trình duyệt fetch segment HLS từ VPS — mỗi người xem đặt tải đọc lên cùng cỗ máy đang transcode. Không scale được quá vài người xem đồng thời. Đánh đổi sai cho media.

### Phương án D — Backblaze B2 thay vì R2

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | $0.006/GB-tháng lưu trữ; $0.01/GB egress |
| Độ phức tạp vận hành | Giống R2 |
| Độ trễ | Không có CDN gốc — phải ghép với Cloudflare bandwidth-alliance (egress miễn phí sang Cloudflare) |
| Bề mặt lỗi | Tương tự R2 |

**Ưu điểm:** Lưu trữ per-GB rẻ hơn.
**Nhược điểm:** Không thiết lập bandwidth-alliance thì phí egress cộng dồn. Có bandwidth-alliance thì quay lại thành "lưu ở B2, phục vụ qua Cloudflare", tức là R2 nhưng thêm bước. Bỏ qua.

## Phân tích đánh đổi

Sự đánh đổi quyết định là "đơn giản vận hành ngay bây giờ so với hoàn chỉnh kiến trúc sau này". MinIO + R2 là kiến trúc đích đúng đắn nếu và khi Portal phục vụ nhiều khu vực hoặc một operator nhạy cảm về chủ quyền. Với v1, đó là hai hệ thống làm việc mà một hệ thống đã làm tốt như nhau.

Rủi ro của chỉ-R2 là **R2 outage = không phát lại được**. Cloudflare R2 từng có sự cố kéo dài nhiều giờ vào tháng 2/2024; sự cố tiếp theo sẽ xảy ra. Giảm thiểu:

- Export hàng tuần giữa các bucket (deliverable của Phase 0.5) nghĩa là dữ liệu không *mất* ngay cả khi R2 gặp lỗi thảm khốc, chỉ tạm thời không truy cập được.
- Với vòng lặp demo của v1, R2 không sẵn sàng chỉ suy giảm xuống thành "video không phát được"; trạng thái database không bị ảnh hưởng. Bề mặt phụ thuộc nhỏ.
- Triển khai production (sau v1) có thể thêm lại MinIO như một *failover origin* — một worker pull từ MinIO khi R2 trả về 5xx — mà không cần đổi storage interface. Đây là trạng thái tương lai từ các sơ đồ, bị hoãn lại.

Chi phí của việc KHÔNG gỡ MinIO khỏi v1 là cụ thể: ~$15/tháng disk + 150 MB RAM + 2 giờ setup của operator + gánh nặng nhận thức liên tục kiểu "replication có khỏe không". Chi phí của việc gỡ nó là bằng không — kiến trúc dài hạn có thể quay lại khi nó xứng đáng có chỗ đứng.

## Hệ quả

**Cái gì dễ hơn**

- `docker-compose.yml` mất đi một dịch vụ. Ngân sách VPS thoải mái hơn.
- Storage interface trong `platform/storage/` đơn giản hơn — không giám sát replication, không logic failover.
- Upload từ frontend thẳng tới R2 (presigned PUT) bỏ qua API ở data plane — API chỉ ký URL, không bao giờ giữ byte upload trong bộ nhớ.
- Người xem luôn fetch từ Cloudflare edge — ngưỡng độ trễ toàn cầu thấp mà operator không cần nỗ lực gì.

**Cái gì khó hơn**

- R2 outage = phát lại bị outage. Chấp nhận điều đó cho v1.
- Câu chuyện chủ quyền dữ liệu là "byte của bạn nằm trên Cloudflare R2 ở khu vực mặc định của họ" — nếu operator nào cần khác đi, họ phải tự thêm MinIO. Ghi rõ điều này một cách trung thực trong deployment guide v1.
- Cấu hình CORS của frontend trên bucket R2 là một thứ nữa cần làm đúng (cấu hình sai tạo ra lỗi trình duyệt khó hiểu). Ghi lại JSON chính xác trong `docs/operations/r2-setup.md`.

**Cái gì cần xem lại**

- Khi operator nhạy cảm chủ quyền đầu tiên xuất hiện, đưa MinIO trở lại như một origin cấu hình được theo từng tenant. Interface `platform/storage/` nên hỗ trợ điều này mà không cần đổi code (nó vốn đã vậy — `Endpoint` là cấu hình).
- Khi chi phí R2 hàng tháng vượt quá dòng ngân sách VPS (>~$60/tháng), đánh giá Backblaze B2 + Cloudflare bandwidth-alliance cho tầng lưu trữ và Backblaze cho origin.
- Khi đích không-phải-Cloudflare đầu tiên cần fetch asset (vd một tích hợp đối tác), phí egress-ra-internet của R2 sẽ áp dụng. Lên kế hoạch cho signed-URL + Cloudflare Worker proxy nếu đây trở thành hot path.

## Hạng mục hành động

1. [x] ~~Gỡ khối dịch vụ `minio` khỏi `docker-compose.yml` cho v1.~~ **Đã sửa đổi (Cập nhật 2026-06-06):** giữ MinIO trong compose dev, gắn với `./data/minio`; chỉ gỡ trong `docker-compose.prod.yml`.
2. [x] ~~Gỡ `volumes.minio_data` khỏi `docker-compose.yml`.~~ Đã làm theo cách khác: chuyển MinIO sang bind-mount `./data/minio` (named volume đã biến mất).
3. [x] Thêm `S3_ENDPOINT`, `S3_REGION`, `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY`, `S3_BUCKET`, `S3_USE_PATH_STYLE=false` vào `.env.example` với placeholder theo hình dạng R2. **Đã xong (2026-07-06)** — tên biến env là `S3_ACCESS_KEY`/`S3_SECRET_KEY` (không phải `*_ACCESS_KEY_ID`); giá trị mặc định theo hình dạng MinIO với giá trị R2 trong comment, `S3_USE_PATH_STYLE=true` cho dev.
4. [x] Trong `backend/internal/platform/storage/`, đảm bảo constructor của S3 client đọc endpoint + region từ config (không hard-code vào MinIO). Nếu package vẫn còn rỗng, scaffold nó thành một wrapper mỏng trên `aws-sdk-go-v2/service/s3`. **Đã xong (2026-07-06)** — wrapper aws-sdk-go-v2 đọc endpoint/region/path-style từ config.
5. [x] Trong `cmd/api`, handler upload ký PUT (`s3:PutObject`, hết hạn sau 5 phút) và trả về URL + key cho frontend; frontend upload thẳng tới R2. **Đã xong (2026-07-06)** — `POST /api/v1/assets` presign PUT; dev còn có thêm `PUT /assets/{id}/source` qua proxy API.
6. [x] Trong worker transcode, tải nguồn xuống dùng presigned GET; segment HLS được upload thẳng qua SDK. Giới hạn dung lượng `/tmp` của worker ở mức 10 GB. **Đã xong (2026-07-06)** ngoại trừ giới hạn 10 GB `/tmp` — vẫn còn mở.
7. [ ] Viết một trang `docs/operations/r2-setup.md` gồm: tạo bucket, cấu hình CORS (cho phép `${APP_DOMAIN}`), lifecycle rule (không có cho v1), cách tạo R2 token với scope `Object Read & Write`. *(`docs/operations/` chưa tồn tại.)*
8. [ ] Trong `diagrams/system-landscape.md` (bộ ADR này), sơ đồ phạm-vi-v1 đã cho thấy chỉ-R2; giữ nguyên `diagrams.md` (tầm nhìn đầy đủ) — nó thể hiện kiến trúc đích.
