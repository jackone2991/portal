# ADR-03: Cấu trúc tô-pô một-VPS-duy-nhất và giới hạn compose-profile cho v1

**Trạng thái:** Được chấp nhận (xem Cập nhật 2026-07-06)
**Ngày:** 2026-05-24
**Người quyết định:** kirito
**Ảnh hưởng:** [docker-compose.yml](../../../docker-compose.yml), [Makefile](../../../Makefile), [feature.md D-8 (observability)], [feature.md D-36/D-39 (live + calls)]

## Cập nhật (2026-07-06) — Authentik đã bị gỡ bỏ (ADR-06); bộ dịch vụ như-đã-triển-khai

Quyết định cốt lõi vẫn đứng vững: một VPS duy nhất (CCX23), các profile `observability`/`live`/`calls` vẫn tắt. Nhưng [ADR-06](./06-local-auth-model.md) (2026-07-05) đã thay OIDC bằng xác thực mật khẩu local, nên `authentik-server`/`authentik-worker`/`mailpit` **chưa bao giờ được triển khai** — mọi nhắc đến Authentik/Mailpit/OIDC bên dưới đều mang tính lịch sử. Bộ compose như-đã-triển-khai: `traefik`, `postgres`, `pgbouncer`, `dragonfly` (chạy với `--default_lua_flags=allow-undeclared-keys` cho Asynq), `minio` + `minio-setup` (chỉ-dev, theo [ADR-04 cập nhật 2026-06-06](./04-storage-tier-budget.md)), `api`, `worker`, `frontend`. Khi Authentik không còn, ngưỡng RAM tối thiểu thấp hơn ~1 GB so với phân tích bên dưới; kích cỡ CCX23 không đổi (dư phòng nhiều hơn). Phân chia lưu trữ như-đã-triển-khai: không có volume Authentik; upload dev nằm trong MinIO trên bind-mount `./data/minio` — direct-to-R2 chỉ áp dụng cho các môi trường đã triển khai (ADR-04).

## Bối cảnh

`docker-compose.yml` hiện đang khởi động Traefik + Postgres + PgBouncer + Dragonfly + MinIO + API + Worker + Frontend. Toàn bộ corpus tài liệu ngụ ý các dịch vụ khác sẽ được thêm dần: Authentik (OIDC), Mailpit (email dev), một stack khả năng quan sát 5-dịch-vụ ([D-8]: Loki, Promtail, Prometheus, Tempo, Grafana, GlitchTip), mediamtx (live ingest, [D-36]), LiveKit (group calls, [D-39]), và các worker bị ràng buộc FFmpeg dưới tải bùng nổ.

Giới hạn ràng buộc là **một VPS duy nhất, ≤ $100/tháng**. Với mức giá VPS hợp lý, điều này quy đổi thành:

| Tier | Ví dụ (Hetzner) | vCPU | RAM | Disk | ~$/tháng |
| --- | --- | --- | --- | --- | --- |
| Tối thiểu | CCX13 | 2 dedicated | 8 GB | 80 GB SSD | ~$13 |
| Khuyến nghị cho v1 | CCX23 | 4 dedicated | 16 GB | 160 GB SSD | ~$30 |
| Có dư phòng | CCX33 | 8 dedicated | 32 GB | 240 GB SSD | ~$60 |
| Trần | CCX43 | 16 dedicated | 64 GB | 360 GB SSD | ~$120 |

Lưu trữ Cloudflare R2 khoảng $0.015/GB-tháng, không mất phí egress trong mạng Cloudflare. 100 GB HLS lưu trữ = $1.50/tháng; băng thông tới người xem miễn phí ở edge. Vì vậy ngân sách hạ tầng chủ yếu bị chi phối bởi chính bản thân VPS.

Khi cộng dồn Authentik (~1 GB resident), Postgres (~500 MB shared_buffers + work), Dragonfly (~256 MB được cấp phát, tăng theo cache), MinIO (~150 MB), Traefik (~50 MB), API + Worker (~300 MB tổng khi idle), Frontend Next.js SSR (~250 MB), và một đợt bùng nổ transcode (FFmpeg có thể tăng vọt 1–2 GB với nội dung 1080p+), **ngưỡng sàn là ~3.5 GB RAM khi idle, ~6 GB khi transcode**. CCX13 (8 GB) quá chật; CCX23 (16 GB) là tier đúng cho v1.

Nếu stack khả năng quan sát ([D-8]) được bật, cộng thêm ~1.1 GB. Nếu LiveKit + mediamtx + coturn được bật, cộng thêm ~500 MB idle cộng với CPU/băng thông bùng nổ. CCX23 không thể host đồng thời cả observability VÀ live streaming VÀ một đợt bùng nổ transcode.

## Quyết định

**v1 chạy trên Hetzner CCX23 (4 vCPU / 16 GB / 160 GB) hoặc tương đương (~$30/tháng). Các compose profile sau đây bị *tắt* rõ ràng cho v1:**

- `--profile observability` (Loki/Prometheus/Tempo/Grafana/GlitchTip) — hoãn đến khi lưu lượng truy cập đủ lớn để biện minh. v1 chạy với log JSON qua stdout.
- `--profile live` (mediamtx) — live streaming là Phase 10. Không phải v1.
- `--profile calls` (LiveKit + coturn) — voice/video là Phase 12. Không phải v1.

**Bộ dịch vụ v1 là:**

| Dịch vụ | Vai trò | RAM (idle) | Ghi chú |
| --- | --- | --- | --- |
| `traefik` | TLS terminator + reverse proxy | ~50 MB | Edge đơn; định tuyến theo Host + path |
| `postgres` | Cơ sở dữ liệu | ~500 MB | shared_buffers được tune ở mức 25% RAM (4 GB) |
| `pgbouncer` | Connection pool | ~30 MB | Chế độ transaction-pool |
| `dragonfly` | Cache tương thích Redis + broker Asynq | ~256 MB | Giới hạn bằng `--maxmemory` (xem hạng mục hành động) |
| `api` | Go HTTP server (`cmd/api`) | ~150 MB | Một replica duy nhất |
| `worker` | Asynq consumer (`cmd/worker`) | ~150 MB idle, 1–2 GB khi transcode | TRANSCODE_CONCURRENCY=1 trong v1 |
| `frontend` | Next.js SSR | ~250 MB | Một replica duy nhất |
| `authentik-server` | OIDC IdP | ~700 MB | Bổ sung mới cho v1 |
| `authentik-worker` | Tác vụ nền của Authentik | ~300 MB | Bắt buộc bởi Authentik |
| `mailpit` | SMTP dev (email đặt-lại-mật-khẩu của Authentik) | ~30 MB | Thay bằng SMTP thật ở prod |

Ngưỡng sàn ~2.4 GB khi idle, ~4 GB khi tải. Dư phòng trên VPS 16 GB là đủ rộng rãi cho v1 và để lại chỗ thêm profile observability ở Phase 0.5 mà không cần resize.

**Cloudflare R2** là phụ thuộc duy nhất nằm ngoài VPS (storage origin; xem [ADR-04](./04-storage-tier-budget.md)). DNS qua Cloudflare được giả định (free tier là đủ).

**Storage** trên chính VPS được chia: dữ liệu Postgres + dữ liệu Authentik + snapshot Dragonfly trên một volume; upload bỏ qua MinIO và đi thẳng tới R2 — tiết kiệm phần disk mà lẽ ra phải giữ các asset được nhân bản.

## Các phương án đã cân nhắc

### Phương án A — CCX13 (2 vCPU / 8 GB) ở mức ~$13/tháng

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | Tốt nhất — dưới $20/tháng |
| Dư phòng | Không có — Authentik + Postgres + đợt bùng nổ transcode sẽ OOM |
| Khả năng chịu tương lai | Buộc phải migrate sang VPS lớn hơn trong vài tháng |

**Ưu điểm:** Rẻ nhất có thể. Phù hợp với người dùng nghiệp dư không bao giờ transcode >720p.
**Nhược điểm:** Riêng Authentik đã chiếm 1 GB resident; một lượt transcode 1080p là kernel sẽ kill thứ gì đó. Không khả thi cho demo 7 bước.

### Phương án B — CCX23 (4 vCPU / 16 GB) ở mức ~$30/tháng  *(được chọn cho v1)*

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | $30/tháng để lại $70 ngân sách cho R2, DNS, các tier trả phí tương lai |
| Dư phòng | Thoải mái khi idle; chịu được một lượt transcode đồng thời |
| Khả năng chịu tương lai | Có thể thêm profile observability mà không resize; live streaming sẽ buộc phải resize |

**Ưu điểm:** Kích cỡ vừa đúng cho v1 + mở rộng Phase 0.5. Nâng cấp tại chỗ lên CCX33 rẻ nếu cần.
**Nhược điểm:** Không thể chạy nhiều lượt transcode đồng thời; `TRANSCODE_CONCURRENCY=1` là giới hạn cứng.

### Phương án C — CCX33 (8 vCPU / 32 GB) ở mức ~$60/tháng

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | $60/tháng + ~$20 R2/Cloudflare = ~$80; vẫn dưới ngân sách |
| Dư phòng | Thoải mái với observability + 2-3 lượt transcode đồng thời |
| Khả năng chịu tương lai | Đủ dùng đến Phase 5 (bank) trước khi cần resize |

**Ưu điểm:** Rất nhiều chỗ trống; không cần resize cho tới Phase 7 (social).
**Nhược điểm:** Trả tiền cho dung lượng mà v1 không dùng tới. Bắt đầu nhỏ hơn; nâng cấp tại chỗ khi cần.

### Phương án D — Tách thành hai VPS rẻ (một cho app, một cho DB/storage)

| Khía cạnh | Đánh giá |
| --- | --- |
| Chi phí | ~$26 (2 × CCX13) |
| Dư phòng | DB trên máy riêng; app trên máy còn lại |
| Độ phức tạp vận hành | Cao hơn — mạng riêng, cert, monitoring trên hai host |

**Ưu điểm:** Rẻ hơn CCX23 khoảng ~$4.
**Nhược điểm:** Vi phạm ràng buộc "một VPS duy nhất" đã nêu ngay từ đầu. Thêm độ phức tạp vận hành để đổi lấy khoản tiết kiệm không đáng kể. Bỏ qua.

## Phân tích đánh đổi

Câu hỏi then chốt là **áp lực bộ nhớ từ Authentik cộng với một đợt bùng nổ transcode**. Không có Authentik, một VPS 8 GB là đủ. Có Authentik, 16 GB là ngưỡng sàn. Phương án thay thế (bỏ Authentik để đổi lấy một kho mật khẩu local tự viết) đánh đổi ~1 GB RAM lấy 3 ngày công của một dev đơn lẻ để viết lưu trữ mật khẩu + luồng reset + template email + logic khóa tài khoản; thời gian đó có giá trị hơn RAM.

> **Cập nhật (2026-07-06):** [ADR-06](./06-local-auth-model.md) đã đảo ngược sự đánh đổi này — Portal giờ tự giữ credential (xác thực mật khẩu local Argon2id) và Authentik đã bị gỡ khỏi stack. Phân tích áp lực bộ nhớ ở trên không còn ràng buộc việc định cỡ VPS nữa.

Việc Cloudflare R2 giúp tiết kiệm disk VPS là quyết định lớn thứ hai. Lưu asset cục bộ trên VPS nghĩa là phải cấp phát ≥240 GB cho bất kỳ thư viện có ý nghĩa nào, điều này buộc phải dùng tối thiểu CCX33 và một chiến lược backup (R2 replication hoặc rsync). Gửi upload thẳng tới R2 né được cả hai — xem [ADR-04](./04-storage-tier-budget.md).

Tắt profile observability cho v1 là quyết định rẻ nhất trong ADR này. Loki + Prometheus + Tempo + Grafana + GlitchTip tốn 5 dịch vụ và ~1.1 GB cho telemetry mà chẳng ai đọc trong tuần đầu. `docker compose logs api worker` đã đủ cho vòng lặp demo.

## Hệ quả

**Cái gì dễ hơn**

- Script deploy chỉ là *một* lệnh gọi `docker compose up -d` duy nhất; không cần nhớ profile flag nào.
- Trần chi phí có thể dự đoán: $30/tháng VPS + ~$5/tháng R2 + Cloudflare free tier = ~$35/tháng, dưới ngân sách khá xa.
- Authentik + Mailpit có mặt trong stack ngay từ ngày đầu nghĩa là luồng OIDC có thể test end-to-end trong quá trình phát triển (không nợ kiểu "nối dây OIDC sau").

**Cái gì khó hơn**

- Không có observability — khi demo hỏng tại chỗ khách hàng, chẩn đoán duy nhất là container log. Lên lịch profile observability cho sprint Phase 0.5.
- TRANSCODE_CONCURRENCY=1 nghĩa là một video nguồn chậm có thể chặn cả queue. Chấp nhận được cho v1 (một user demo duy nhất); trở thành nút thắt cổ chai thật khi dùng multi-tenant. Phase 1 phải thêm việc nối dây quota per-tenant [D-13].
- Authentik thêm cả một database schema và bề mặt admin phải học. Công thức deploy Authentik nằm ở `docs/operations/authentik.md` (được tham chiếu trong [D-28]) — viết ít nhất một bản stub trong sprint v1 để lần deploy tiếp theo không phải là một cuộc săn lùng kho báu. *(Không còn ý nghĩa — Authentik đã bị gỡ, ADR-06.)*

**Cái gì cần xem lại**

- Khi Phase 1 đưa vào tenancy + RLS, profile observability nên vào cùng sprint đó để latency request per-tenant có thể đo được ngay từ ngày đầu [D-8].
- Khi Phase 10 đưa vào live streaming, mediamtx + các lượt transcode đồng thời sẽ đẩy VPS vượt quá 16 GB. Lên kế hoạch nâng cấp CCX33 (hoặc tách sang một VPS chuyên cho media) trước sprint đó.
- Chiến lược backup [D-10] (pgbackrest + R2 replication + Dragonfly BGSAVE) không thuộc v1, nhưng nên có trước khi bất kỳ user bên ngoài nào chạm vào hệ thống. Thêm vào Phase 0.5.

## Hạng mục hành động

1. [ ] Đặt `dragonfly` `command: ["--logtostderr", "--cluster_mode=emulated", "--maxmemory=2GB"]` trong docker-compose để giới hạn bộ nhớ trước khi nó tranh chấp với FFmpeg. **Đã thay thế một phần (2026-07-06):** command đã triển khai là `["--logtostderr", "--default_lua_flags=allow-undeclared-keys"]` (bắt buộc bởi Asynq); giới hạn `--maxmemory` vẫn còn mở.
2. [x] ~~Thêm dịch vụ `authentik-server`, `authentik-worker`, và `mailpit` vào `docker-compose.yml`. Dùng công thức đã công bố của Authentik; không gate sau profile nào (luôn bật cho v1).~~ **Đã lỗi thời theo ADR-06 (2026-07-05):** Authentik đã bị bỏ; Portal tự giữ credential.
3. [ ] Ghi chú các profile bị tắt trong `docker-compose.yml` bằng một comment một dòng: `# v1 disables --profile observability, --profile live, --profile calls — see doc/en/architecture/03-single-vps-topology.md`.
4. [ ] Thêm một target `Makefile` là `make deploy-v1` chạy `docker compose up -d` KHÔNG kèm profile flag nào — ngăn observability/live bị bật nhầm trong v1.
5. [ ] Đặt Postgres `shared_buffers = 4GB`, `effective_cache_size = 10GB`, `max_connections = 50` (PgBouncer pool ở dưới nó). Ghi vào bản stub `docs/operations/postgres-tuning.md`.
6. [ ] Trong tài liệu deployment v1 (`docs/operations/deployment.md` — hiện chưa tồn tại), ghi lại quyết định định cỡ VPS và lý do các profile bị tắt. Tham chiếu chéo tới ADR này.
7. [ ] Đặt `TRANSCODE_CONCURRENCY=1` và `MAX_CONCURRENT_TRANSCODES_PER_USER=1` trong `.env.example` cho v1; tăng sau khi quota được đưa vào [D-13]. **Vẫn còn mở (2026-07-06):** `cmd/worker/main.go` hiện đang hardcode Asynq `Concurrency: 4`; chưa có env knob.
