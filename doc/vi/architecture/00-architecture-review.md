# Đánh Giá Kiến Trúc — Portal (Tháng 5 năm 2026)

**Trạng thái:** Được chấp nhận
**Ngày:** 2026-05-24
**Những người đánh giá:** kirito (nhà phát triển duy nhất / người đánh giá duy nhất)
**Phạm vi:** Toàn bộ ngăn xếp — mọi khu vực trong biểu mẫu (xác thực, tenant/RLS, phương tiện, các mô-đun miền, lưu trữ/CDN, công việc, cơ sở dữ liệu, giao diện trước, OpenAPI, ngách cắt ngang).

Tài liệu này là điểm vào cho bộ ADR. Nó ghi lại **những phát hiện** của cuộc đánh giá; các ADR riêng lẻ sau đó đề xuất các hành động khắc phục.

---

## 1. Tóm tắt điều hành

Tập hợp thiết kế (CLAUDE.md + MODULES.md + feature.md + diagrams.md) là **không bình thường hoạt động** cho một dự án ở giai đoạn mã này. 40 quyết định được ghi lại với lý do; ranh giới mô-đun được viết ra và có thể thực thi depguard; hợp đồng OpenAPI được đặt tên là nguồn sự thật; mô-đun xác thực thực sự được suy nghĩ kỹ lưỡng (phát hiện tái sử dụng mã thông báo làm mới, trình khớp quyền thất bại, hai kênh thu hồi).

Cùng một sự trưởng thành đó cũng là rủi ro lớn nhất của dự án. Phạm vi được mô tả trong `feature.md` là **công việc nhiều năm cho một nhóm nhỏ**, không phải công việc 2 tuần cho một người duy nhất. Hầu hết các quyết định kỹ thuật chịu tải được mô tả đều chính xác về mặt trừu tượng nhưng trở thành trách nhiệp ở quy mô nhóm và timeline đã nêu:

- Một monolith mô-đun với ranh giới được thực thi bởi depguard là phù hợp cho một nhóm 4 người và sai với một nhà phát triển duy nhất cố gắng phát hành trong 2 tuần — ranh giới được phát hiện trong quá trình khám phá không nên được thực thi trong CI cho đến khi ít nhất một nhị phân chạy.
- Đường dẫn phương tiện (FFmpeg + Asynq + MinIO + R2 + hạn ngạch transcode) là phù hợp cho SaaS đa người dùng và quá kỹ thuật cho v1 cần *một* tải lên hoạt động để chứng minh kiến trúc.
- Hai thông số kỹ thuật RBAC cạnh tranh (phân cấp vai trò trong CLAUDE.md/feature.md, các gói chính sách trong archivetech.md) không thể cả hai đều đúng. Mã đã tồn tại cho một trong số chúng.
- Mục tiêu triển khai diagrams.md vẽ (Postgres + PgBouncer + Dragonfly + MinIO + Authentik + ngăn xếp khả năng quan sát 5 dịch vụ + mediamtx + LiveKit + Mailpit) không phù hợp trên một VPS duy nhất $30–60 khi tính đến lượng RAM của Authentik và CPU bùng nổ của FFmpeg.

Điều hữu ích nhất mà cuộc đánh giá này có thể làm là **làm cho bộ cắt v1 rõ ràng** để 2 tuần tiếp theo không được dành để đọc 40 quyết định theo thứ tự tuyến tính. Xem [ADR-01](./01-v1-scope-cut.md).

---

## 2. Những gì chịu tải và chính xác

Những lựa chọn này sẽ sống sót sau bất kỳ lần cắt v1 nào. Đừng đặt câu hỏi về chúng giữa cuộc phát hành:

| Quyết định | Tại sao nó tồn tại |
| --- | --- |
| **Monolith mô-đun Go với `cmd/{api,worker,sysjobs}`** | Câu trả lời đúng cho quy mô nhóm, mục tiêu triển khai và phạm vi tính năng. Microservices ở 1 nhà phát triển / 1 VPS là sai lầm; một nhị phân Go phẳng sẽ làm cho chia tách xã hội/ngân hàng/phương tiện trong tương lai trở nên khó khăn. |
| **OpenAPI là nguồn sự thật** (`shared/openapi.yaml` → oapi-codegen + openapi-typescript) | Spec-first là điều duy nhất giữ cho hợp đồng Go ↔ TS trung thực dưới sự phát triển duy nhất. Bỏ qua điều này rò rỉ các loại và phá vỡ giao diện trước sau. [D-29] |
| **Postgres 17 + RLS để đa người dùng** | RLS đang thực hiện công việc thực tế là lá chắn cuối cùng phía sau lọc cấp xử lý. Đi bộ CTE đệ quy quyền được thiết kế chính xác. |
| **Asynq cho hàng đợi công việc** | Lựa chọn đúng — Go-native, Redis/Dragonfly-compatible, được cung cấp với hàng đợi chữ không phân phối. BullMQ sẽ buộc một người lao động Node; xây dựng một người là sử dụng tệ hơn của 2 tuần. |
| **Dragonfly thay vì Redis** | Cùng một giao thức, RAM thấp hơn, một quá trình duy nhất. Không có lý do để không. |
| **Authentik cho OIDC** | Thêm RAM, nhưng thay thế là tự làm lưu trữ mật khẩu + luồng đặt lại + vòng lặp email — đó là 3 ngày bạn không có. Authentik + OIDC + Mã thông báo làm mới được quản lý Portal là sự chia sẻ công việc phù hợp. |
| **Vidstack cho phát lại HLS** | Vidstack trên Next.js là con đường ít kháng cự nhất; thay thế (Shaka, hls.js trực tiếp) là dây nối nhiều hơn. |
| **Tách biệt `cmd/sysjobs` + khóa BYPASSRLS của sysrepository** | Đây là một quyết định bạn KHÔNG nên hoãn ngay cả ở tốc độ phát triển solo. Bypass RLS là loại điều đó chuyển thành rò rỉ dữ liệu đa người dùng. Quy tắc depguard tự trả tiền cho chính nó lần đầu tiên bạn quên. |

## 3. Những gì gặp rủi ro

Đây là những lựa chọn có khả năng tốn thời gian hoặc thiêu đốt ngân sách dưới các ràng buộc đã nêu:

### 3.1 Sự phân chia RBAC (ưu tiên cao nhất)

`archivetech.md` §1 nói "thông số kỹ thuật thắng, điều chỉnh mã, không phải cách khác" và sau đó mô tả một **mô hình RBAC hoàn toàn khác** từ mô hình trong CLAUDE.md và `feature.md`:

| Khía cạnh | CLAUDE.md / feature.md (xây dựng) | archivetech.md (specced) |
| --- | --- | --- |
| Đơn vị cấp chính | Vai trò (có phân cấp) | Chính sách (gói quyền có thể tái sử dụng) |
| Phân cấp | `guest → user → creator → editor → moderator → admin → superadmin` qua `roles.parent_id` | Chuỗi cha `User Group` qua `user_groups.parent_id` |
| Các khoản cấp cho mỗi người dùng | Bảng `user_roles` | Bảng `user_policy_attachments` |
| Cổng quyền | Không có (chỉ khớp RBAC) | **Quyền được cấp theo tệp** — cấp phép tải lên được yêu cầu để kích hoạt cấp |
| Giải quyết xung đột | Lần trận đầu tiên thắng trên bộ cấp | Deny-wins (ngữ nghĩa AWS/OPA) |
| Khóa bộ nhớ đệm quyền | `rbac:perms:<userID>:v<N>` | Hình dạng giống nhau, nhưng mỗi `(user_id, token_version)` |

Cả hai thông số kỹ thuật không thể cùng tồn tại. Mã hiện tại triển khai mô hình phân cấp vai trò. Mệnh lệnh spec-wins của `archivetech.md` không thể thực thi cho đến khi ai đó quyết định thông số kỹ thuật nào là chính thức. Quyết định hoãn lại để [ADR-02](./02-rbac-model-reconciliation.md); khuyến nghị ở đó là **giữ phân cấp vai trò làm nguyên thủy cấp v1** và thêm các gói chính sách làm lớp Phase-2 **trên đầu của** vai trò, không thay vào đó.

### 3.2 Sự không phù hợp phạm vi so với đường băng

`feature.md` Phase 0 một mình có 14 deliverable và sẽ mất 1 dev ~ 1 tuần nếu mọi thứ diễn ra tốt. Giai đoạn 1–12 là nhiều năm công việc. Ngân sách 2 tuần có nghĩa là **chọn một bộ pha duy nhất** và cam kết. Xem [ADR-01](./01-v1-scope-cut.md); lần cắt được đề xuất là *Phase 0 + một lát cắt dọc của Phase 2 (một đường cơ sở tải lên video)* và không gì khác.

### 3.3 Ngân sách RAM trên một VPS duy nhất

Nếu bạn mang lên `docker-compose.yml` cộng với thêm **Authentik** (~1 GB), **hồ sơ khả năng quan sát** (~1.1 GB trên Loki/Prometheus/Tempo/Grafana/GlitchTip), **mediamtx + LiveKit** (~500 MB + bùng nổ) và FFmpeg worker (1–2 GB trong quá trình transcode), bạn vượt quá 4 GB trước khi API xử lý một yêu cầu. Một VPS $60/mo hợp lý (8 vCPU / 32 GB) bao gồm nó; VPS $30/mo (4 vCPU / 16 GB) không sau khi Authentik ở trong hỗn hợp.

[ADR-03](./03-single-vps-topology.md) đề xuất bộ dịch vụ v1 và cấu hình hồ sơ nào ở ngoài.

### 3.4 Độ phức tạp lớp lưu trữ

`docker-compose.yml` chạy MinIO bên trong VPS. Các biểu đồ ngụ ý MinIO là *origin* và R2 là *edge*, với sao chép liên tục. Đó là hai hệ thống lưu trữ, hai bộ thông tin xác thực, giám sát sao chép và 100–500 GB đĩa được đính kèm VPS trước khi bất kỳ tải lên người dùng nào.

Đối với v1, câu trả lời đơn giản hơn là **R2 chỉ** — nó tương thích S3, chi phí ~$0.015/GB-month cho lưu trữ và không có phí egress bên trong cạnh Cloudflare. MinIO trở thành bổ sung Phase-2 nếu một người tự lưu trữ muốn chạy hoàn toàn mà không Cloudflare. Xem [ADR-04](./04-storage-tier-budget.md).

### 3.5 Khoảng trống dây là bộ chặn thực tế

CLAUDE.md nói ra lớn: *"`cmd/api/main.go` vẫn có nhận xét `TODO: mount OpenAPI-generated handlers` và không gọi `account.New(...)` hay `MountHTTP` của mô-đun nào."* Mọi tính năng hạ lưu đều được cổng sau điều này. [ADR-05](./05-phase0-wiring-order.md) trình tự perevClosures.

## 4. Cái gì yên tĩnh là tốt

Những lựa chọn này có được những bài viết dài trong tập hợp nhưng không cần ADR — chúng đã là câu trả lời đúng và không có gì trong bộ ràng buộc thay đổi điều đó:

- **Tenant qua tiền tố URL `/t/{tenant}/...` + tenant `me` tổng hợp** [D-23] — thực tế, thân thiện với RLS, có thể chia sẻ liên kết.
- **Phản hồi Vấn đề RFC 7807 với `type` URI khóa i18n** [D-7] — giải quyết vấn đề i18n ở nguồn.
- **Tiền như `numeric(20,8)` + shopspring/decimal + loại giá trị Tiền** [D-14] — ngân hàng đủ xa để lý thuyết ngay bây giờ, nhưng lý thuyết là đúng.
- **Di chuyển sản xuất chuyển tiếp chỉ + mẫu hợp đồng dữ liệu mở rộng** [D-12] — kỷ luật bắt buộc khi dữ liệu tồn tại; rẻ để cam kết ngay bây giờ.
- **`platform/audit/` ngách cắt ngang + `<module>.<resource>.<action>` taxonomy sự kiện** [D-25] — tốt nhất nỗ lực không chặn là sự đánh đổi đúng.

## 5. Cái gì nên được hoãn lại hoàn toàn

Đây là rõ ràng **cắt phạm vi** cho cửa sổ 2 tuần. Mỗi là một tính năng hoàn chỉnh sẽ xứng đáng với ADR của riêng nó nếu được cố gắng; khuyến nghị là KHÔNG cố gắng chúng trong v1. Các tài liệu tham khảo trong ngoặc đơn là các phần `feature.md`.

| Khu vực tính năng | Hoãn lại vì |
| --- | --- |
| Mô-đun Ngân hàng (§8, Phase 5a-i) | 9 giai đoạn phụ. Cần xác thực step-up, thực thi MFA, sổ cái kép, tỷ giá FX, chia sẻ hộ gia đình. Nhiều tháng công việc. |
| Mô-đun Xã hội (§9 cơ bản, Phase 7) | Bài đăng + nguồn cấp + phản ứng + DM + cộng đồng + kiểm soát quyền riêng tư là một phần tư của công việc ngay cả trước §9.12+ tính năng nâng cao. |
| Xã hội nâng cao (§9.13–9.37, Phase 10) | Cuộn phim, truyền trực tiếp, phòng âm thanh, bỏ phiếu, karma, wikis — mỗi là một dự án bên. |
| Kinh tế người sáng tạo (§10, Phase 11) | Mẹo + subs + paywall + payouts phụ thuộc vào ngân hàng được vận chuyển trước. |
| Thị trường (§11, Phase 12) | Mô-đun cầu nối spanning xã hội + ngân hàng; cả hai phải được vận chuyển trước. |
| An toàn ML (§12, Phase 12) | NSFW + CSAM + bộ phân loại độc tính yêu cầu cơ sở hạ tầng mô hình. Hàng đợi báo cáo thủ công là câu trả lời v1. |
| LiveKit + mediamtx (§9.25, Phase 10/12) | ~500 MB + bùng nổ CPU + mạng phức tạp. Ngoài v1. Soạn thảo hồ sơ `--calls` và `--profile live` đã cổng chúng; giữ các hồ sơ bị vô hiệu hóa. |
| Ngăn xếp khả năng quan sát (Phase 1, D-8) | Loki + Prometheus + Tempo + Grafana + GlitchTip là 5 dịch vụ. Chạy với nhật ký JSON stdout trong v1; thêm ngăn xếp khi lưu lượng biện minh cho nó. Giữ `--profile observability` bị vô hiệu hóa. |

[ADR-01](./01-v1-scope-cut.md) nêu lại cụt một cách chính thức.

## 6. Những phát hiện tập hợp không giải quyết

Một vài điều không ở trong tài liệu đầu vào và nên ở trên radar:

- **Không có đề cập đến điều chỉnh bộ kết nối cơ sở dữ liệu** giữa chế độ giao dịch PgBouncer và `pgx`. Asynq, API và worker chia sẻ một cụm Postgres; mà không có sắp xếp kích thước hồ bơi cẩn thận, người lao động làm đói API trong quá trình bùng nổ transcode.
- **`OIDC_GROUP_ROLE_MAP` được cấu hình env** [D-26]. Một lỗi đánh máy trong sản xuất lặng lẽ tước một quản trị viên của các đặc quyền của họ vào lần làm mới tiếp theo. Thêm xác thực thời gian khởi động mà cờ nhóm Authentik không được ánh xạ hiện diện trong danh sách quản trị viên khởi động.
- **Chuỗi quay mã thông báo làm mới phát hành sự kiện chain-revoke khi tái sử dụng**, nhưng tập hợp không xác định những gì *tiêu thụ* sự kiện đó. Tối thiểu, chain-revoke nên gửi email bảo mật cho người dùng; mô-đun thông báo là Phase 6, vì vậy đối với v1 hoặc nhật ký lớn hoặc gửi email tự làm từ trình xử lý xác thực.
- **`disabled_at` trên người dùng được kiểm tra trong middleware**, nhưng không có công việc được lên lịch để thu hồi mã thông báo làm mới hoạt động khi người dùng bị vô hiệu hóa. Các mã thông báo làm mới hiện tại của người dùng bị vô hiệu hóa vẫn xoay cho đến khi hết hạn. Đáng một dòng `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = ?` tiếp theo để vô hiệu hóa trình xử lý.
- **Cookie SameSite=Strict giao diện trước yêu cầu Next.js và API để chia sẻ miền có thể đăng ký được** [D-34]. Định tuyến đơn miền dựa trên đường dẫn qua Traefik hoạt động, nhưng nó là ràng buộc triển khai không rõ ràng cần phải có trong tài liệu toán tử ngày đầu tiên — dễ dàng cấu hình sai trong sự phát triển với `localhost:3000` + `localhost:8080` (những là *different* gốc cho mục đích SameSite).

## 7. Tóm tắt khuyến nghị

Năm ADR cụ thể theo sau. Theo thứ tự ưu tiên sprint:

1. **[ADR-05](./05-phase0-wiring-order.md)** — đóng khoảng trống dây (Ngày 1–3). Không có gì khác quan trọng cho đến `make up && make dev && curl /api/v1/healthz` trả về 200 từ mô-đun *constructed*.
2. **[ADR-01](./01-v1-scope-cut.md)** — đồng ý lên v1 cắt để bạn không phản xạ vươn tới mô-đun ngân hàng trong tuần 2.
3. **[ADR-02](./02-rbac-model-reconciliation.md)** — chọn một mô hình RBAC và viết xuống trước khi bất kỳ UI quản trị nào vận chuyển.
4. **[ADR-03](./03-single-vps-topology.md)** — khóa trong bộ hồ sơ soạn thảo và kích thước VPS để tập lệnh triển khai không vận chuyển với `--profile observability` được bật ngẫu nhiên.
5. **[ADR-04](./04-storage-tier-budget.md)** — quyết định MinIO+R2 so với R2-only trước khi trình xử lý tải lên được viết; hình dạng cuộc gọi giao diện lưu trữ.

Xem sơ đồ cảnh quan hệ thống ở [`diagrams/system-landscape.md`](./diagrams/system-landscape.md) cho hình ảnh v1-scoped.
