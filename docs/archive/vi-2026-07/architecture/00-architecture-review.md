# Đánh Giá Kiến Trúc — Portal (Tháng 5 năm 2026)

**Trạng thái:** Được chấp nhận
**Ngày:** 2026-05-24
**Những người đánh giá:** kirito (nhà phát triển duy nhất / người đánh giá duy nhất)
**Phạm vi:** Toàn bộ stack — mọi khu vực trên biểu mẫu (xác thực, tenant/RLS, phương tiện, các mô-đun miền, lưu trữ/CDN, jobs, cơ sở dữ liệu, frontend, OpenAPI, các mối quan tâm xuyên suốt).

> **Cập nhật (2026-07-06):** Các hạng mục hành động của bài đánh giá này đều đã được thực thi — ADR-01 đến ADR-05 đã ở trạng thái Được chấp nhận và đã triển khai xong, vòng lặp demo v1 đã khép kín (xem `MILESTONE_CHECKS.md`). Một sự tán thành ở §2 sau đó đã bị đảo ngược: [ADR-06](./06-local-auth-model.md) (2026-07-05) đã loại bỏ Authentik/OIDC để chuyển sang xác thực mật khẩu local do Portal tự quản lý (Argon2id); cơ chế refresh-token và thu hồi không thay đổi. Nội dung bên dưới được giữ nguyên như đã viết vào 2026-05-24, kèm các ghi chú có ngày tháng đánh dấu những phát hiện đã bị thay thế.

Tài liệu này là điểm vào cho bộ ADR. Nó ghi lại **những phát hiện** của cuộc đánh giá; các ADR riêng lẻ tiếp theo đề xuất các hành động khắc phục.

---

## 1. Tóm tắt điều hành

Kho tài liệu thiết kế (CLAUDE.md + MODULES.md + feature.md + diagrams.md) **trưởng thành một cách bất thường** so với giai đoạn hiện tại của mã nguồn dự án. 40 quyết định được ghi lại kèm lý do; ranh giới mô-đun được viết rõ và có thể thực thi bằng depguard; hợp đồng OpenAPI được chỉ định là nguồn chân lý; mô-đun xác thực thực sự được suy nghĩ thấu đáo (phát hiện tái sử dụng refresh-token, bộ khớp quyền fail-closed, thu hồi hai kênh).

Chính sự trưởng thành đó cũng là rủi ro lớn nhất của dự án. Phạm vi được mô tả trong `feature.md` là **công việc nhiều năm cho một đội nhỏ**, không phải công việc 2 tuần cho một người. Hầu hết các quyết định kỹ thuật chịu tải đều đúng đắn về mặt trừu tượng nhưng trở thành gánh nặng ở quy mô đội và mốc thời gian đã nêu:

- Một modular monolith với ranh giới được thực thi bằng depguard là đúng đắn cho một đội 4 người và sai lầm cho một nhà phát triển đơn lẻ đang cố gắng ra mắt trong 2 tuần — ranh giới được phát hiện trong quá trình khám phá không nên được thực thi trong CI cho tới khi ít nhất một binary đang chạy.
- Pipeline phương tiện (FFmpeg + Asynq + MinIO + R2 + hạn ngạch transcode) là đúng đắn cho một SaaS đa tenant và quá mức cần thiết cho một v1 chỉ cần *một* lượt tải lên hoạt động để chứng minh kiến trúc.
- Hai đặc tả RBAC cạnh tranh nhau (phân cấp vai trò trong CLAUDE.md/feature.md, gói chính sách trong archivetech.md) không thể cùng đúng. Mã nguồn đã tồn tại cho một trong hai.
- Mục tiêu triển khai mà diagrams.md vẽ ra (Postgres + PgBouncer + Dragonfly + MinIO + Authentik + stack khả năng quan sát 5 dịch vụ + mediamtx + LiveKit + Mailpit) không vừa với một VPS $30–60 duy nhất một khi tính đến dung lượng RAM của Authentik và CPU bùng nổ của FFmpeg.

Điều hữu ích nhất mà bài đánh giá này có thể làm là **làm rõ ràng bộ cắt v1** để 2 tuần tiếp theo không bị dành để đọc 40 quyết định theo thứ tự tuyến tính. Xem [ADR-01](./01-v1-scope-cut.md).

---

## 2. Cái gì chịu tải và đúng đắn

Những lựa chọn này sẽ sống sót qua bất kỳ bộ cắt v1 nào. Đừng nghi ngờ chúng giữa sprint:

| Quyết định | Tại sao nó đứng vững |
| --- | --- |
| **Modular monolith Go với `cmd/{api,worker,sysjobs}`** | Câu trả lời đúng cho quy mô đội, mục tiêu triển khai, và độ rộng tính năng. Microservices ở quy mô 1 dev / 1 VPS là sai lầm nghiêm trọng; một binary Go phẳng sẽ khiến việc tách social/bank/media sau này trở nên đau đớn. |
| **OpenAPI là nguồn chân lý** (`shared/openapi.yaml` → oapi-codegen + openapi-typescript) | Spec-first là điều duy nhất giữ cho hợp đồng Go ↔ TS trung thực dưới sự phát triển một mình. Bỏ qua điều này sẽ rò rỉ kiểu dữ liệu và phá vỡ frontend về sau. [D-29] |
| **Postgres 17 + RLS cho đa tenant** | RLS đang làm công việc thực sự như tuyến phòng thủ cuối cùng phía sau việc lọc ở cấp handler. Việc duyệt quyền bằng CTE đệ quy được thiết kế chính xác. |
| **Asynq cho hàng đợi job** | Lựa chọn đúng — Go-native, tương thích Redis/Dragonfly, có sẵn hàng đợi dead-letter. BullMQ sẽ buộc phải có một worker Node; xây dựng một cái là cách dùng 2 tuần tệ hơn. |
| **Dragonfly thay vì Redis** | Cùng giao thức, RAM thấp hơn, một tiến trình duy nhất. Không có lý do gì để không dùng. |
| **Authentik cho OIDC** | Tốn thêm RAM, nhưng phương án thay thế là tự xây dựng lưu trữ mật khẩu + luồng reset + vòng lặp email — đó là 3 ngày bạn không có. Authentik + OIDC + refresh token do Portal quản lý là sự phân chia công việc hợp lý. |
| **Vidstack cho phát lại HLS** | Vidstack trên Next.js là con đường ít trở ngại nhất; phương án thay thế (Shaka, hls.js trực tiếp) đòi hỏi nối dây nhiều hơn. |
| **Tách `cmd/sysjobs` + khóa chặt BYPASSRLS của sysrepository** | Đây là một quyết định bạn KHÔNG nên trì hoãn kể cả ở tốc độ phát triển một mình. Bypass RLS là kiểu thứ dễ biến thành rò rỉ dữ liệu đa tenant. Quy tắc depguard sẽ tự trả giá trị của nó ngay lần đầu tiên bạn quên. |

> **Cập nhật (2026-07-06):** hàng Authentik ở trên đã bị thay thế bởi [ADR-06](./06-local-auth-model.md) — Authentik đã bị loại bỏ và Portal giờ tự quản lý xác thực mật khẩu local (Argon2id). Cơ chế refresh-token và thu hồi mà hàng đó tán thành vẫn không đổi và vẫn đang được sử dụng.

## 3. Cái gì gặp rủi ro

Đây là những lựa chọn có khả năng cao nhất sẽ tốn thời gian hoặc đốt ngân sách dưới các ràng buộc đã nêu:

### 3.1 Sự phân rẽ RBAC (ưu tiên cao nhất)

`archivetech.md` §1 nói "đặc tả thắng, điều chỉnh mã, không phải ngược lại" và sau đó mô tả một **mô hình RBAC hoàn toàn khác** so với mô hình trong CLAUDE.md và `feature.md`:

| Khía cạnh | CLAUDE.md / feature.md (đã xây dựng) | archivetech.md (đã đặc tả) |
| --- | --- | --- |
| Đơn vị cấp quyền chính | Vai trò (có phân cấp) | Chính sách (gói quyền tái sử dụng) |
| Phân cấp | `guest → user → creator → editor → moderator → admin → superadmin` qua `roles.parent_id` | Chuỗi cha `User Group` qua `user_groups.parent_id` |
| Cấp quyền theo người dùng | Bảng `user_roles` | Bảng `user_policy_attachments` |
| Cổng kiểm soát quyền | Không có (chỉ khớp RBAC) | **Quyền được cấp theo tệp** — cần giấy phép tải lên để kích hoạt việc cấp quyền |
| Giải quyết xung đột | Khớp đầu tiên thắng trên bộ cấp quyền | Deny-wins (ngữ nghĩa kiểu AWS/OPA) |
| Khóa cache quyền | `rbac:perms:<userID>:v<N>` | Cùng hình dạng, nhưng theo từng `(user_id, token_version)` |

Cả hai đặc tả không thể cùng tồn tại. Mã nguồn hiện tại triển khai mô hình phân cấp vai trò. Điều khoản đặc-tả-thắng của `archivetech.md` không thể thực thi được cho tới khi ai đó quyết định đặc tả nào là chính thức. Quyết định được hoãn lại sang [ADR-02](./02-rbac-model-reconciliation.md); khuyến nghị ở đó là **giữ phân cấp vai trò làm nguyên thủy cấp quyền của v1** và thêm gói chính sách như một lớp Phase-2 **chồng lên trên** vai trò, chứ không thay thế chúng.

> **Cập nhật (2026-07-06):** đã giải quyết — [ADR-02](./02-rbac-model-reconciliation.md) đã ở trạng thái Được chấp nhận; phân cấp vai trò là chính thức cho v1 và gói chính sách sẽ chồng lên trên sau này. Điều khoản đặc-tả-thắng của `archivetech.md` bị bỏ qua đối với v1.

### 3.2 Sự không khớp giữa phạm vi và đường băng

Chỉ riêng Phase 0 của `feature.md` đã có 14 deliverable và sẽ mất khoảng 1 tuần cho 1 dev nếu mọi thứ suôn sẻ. Phase 1–12 là nhiều năm công việc. Ngân sách 2 tuần có nghĩa là phải **chọn một bộ phase duy nhất** và cam kết theo nó. Xem [ADR-01](./01-v1-scope-cut.md); bộ cắt được khuyến nghị là *Phase 0 + một lát cắt dọc của Phase 2 (một happy path tải lên video)* và không gì khác.

### 3.3 Ngân sách RAM trên một VPS duy nhất

Nếu bạn khởi động `docker-compose.yml` cộng thêm **Authentik** (~1 GB), **profile khả năng quan sát** (~1.1 GB trải trên Loki/Prometheus/Tempo/Grafana/GlitchTip), **mediamtx + LiveKit** (~500 MB + bùng nổ), và worker FFmpeg (1–2 GB khi transcode), bạn sẽ vượt quá 4 GB trước cả khi API xử lý một request. Một VPS $60/tháng hợp lý (8 vCPU / 32 GB) đủ sức chứa; một VPS $30/tháng (4 vCPU / 16 GB) thì không, một khi Authentik nằm trong hỗn hợp đó.

[ADR-03](./03-single-vps-topology.md) đề xuất bộ dịch vụ v1 và những cờ profile nào nên giữ tắt.

> **Cập nhật (2026-07-06):** Authentik đã bị loại bỏ bởi [ADR-06](./06-local-auth-model.md), lấy dung lượng RAM của nó ra khỏi phép tính; stack v1 đã ra mắt gồm 8 dịch vụ (postgres, pgbouncer, dragonfly, minio, traefik, api, worker, frontend).

### 3.4 Độ phức tạp của tầng lưu trữ

`docker-compose.yml` chạy MinIO bên trong VPS. Các sơ đồ ngụ ý MinIO là *origin* và R2 là *edge*, với việc sao chép liên tục. Đó là hai hệ thống lưu trữ, hai bộ thông tin xác thực, giám sát sao chép, và 100–500 GB ổ đĩa gắn kèm VPS trước cả khi có bất kỳ lượt tải lên nào của người dùng.

Với v1, câu trả lời đơn giản hơn là **chỉ dùng R2** — nó tương thích S3, chi phí lưu trữ ~$0.015/GB-tháng và không tính phí egress bên trong edge của Cloudflare. MinIO trở thành một bổ sung ở Phase-2 nếu người tự lưu trữ muốn chạy hoàn toàn không cần Cloudflare. Xem [ADR-04](./04-storage-tier-budget.md).

> **Cập nhật (2026-07-06):** đã được áp dụng với một điều chỉnh — xem cập nhật ngày 2026-06-06 của ADR-04. Chỉ-R2 áp dụng cho các môi trường đã triển khai; dev cục bộ dùng MinIO đứng sau cùng một S3 client duy nhất.

### 3.5 Khoảng trống nối dây là điểm chặn thực sự

CLAUDE.md nói thẳng ra: *"`cmd/api/main.go` vẫn còn comment `TODO: mount OpenAPI-generated handlers` và chưa gọi `account.New(...)` hay `MountHTTP` của bất kỳ mô-đun nào."* Mọi tính năng ở hạ nguồn đều bị chặn bởi điều này. [ADR-05](./05-phase0-wiring-order.md) sắp xếp trình tự để đóng khoảng trống đó.

> **Cập nhật (2026-07-06):** đã đóng — ADR-05 đã được thực thi đúng thứ tự; `cmd/api/main.go` giờ đã khởi tạo các mô-đun account và media, và `/api/v1/healthz` trả về 200. Xem `MILESTONE_CHECKS.md`.

## 4. Cái gì âm thầm ổn

Những lựa chọn này có bài viết dài trong kho tài liệu nhưng không cần ADR riêng — chúng đã là câu trả lời đúng và không có gì trong khung ràng buộc thay đổi điều đó:

- **Tenant qua tiền tố URL `/t/{tenant}/...` + tenant tổng hợp `me`** [D-23] — thực dụng, thân thiện với RLS, có thể chia sẻ qua link.
- **Phản hồi RFC 7807 Problem với URI `type` dạng khóa i18n** [D-7] — giải quyết vấn đề i18n ngay tại nguồn.
- **Tiền dưới dạng `numeric(20,8)` + shopspring/decimal + kiểu giá trị Money** [D-14] — module bank còn xa nên đây là lý thuyết vào lúc này, nhưng lý thuyết đó đúng.
- **Migration production chỉ tiến-tới + mẫu hình expand-migrate-data-contract** [D-12] — kỷ luật bắt buộc một khi đã có dữ liệu; rẻ để cam kết ngay từ bây giờ.
- **`platform/audit/` xuyên suốt + phân loại sự kiện `<module>.<resource>.<action>`** [D-25] — best-effort không chặn là sự đánh đổi đúng đắn.

## 5. Cái gì nên bị hoãn lại hoàn toàn

Đây là những **cắt giảm phạm vi** rõ ràng cho khung 2 tuần. Mỗi mục là một tính năng hoàn chỉnh xứng đáng có ADR riêng nếu được thực hiện; khuyến nghị là KHÔNG thực hiện chúng trong v1. Các tham chiếu trong ngoặc là tới các phần của `feature.md`.

| Khu vực tính năng | Hoãn lại vì |
| --- | --- |
| Mô-đun Bank (§8, Phase 5a-i) | 9 sub-phase. Cần step-up auth, thực thi MFA, sổ cái kép (double-entry ledger), tỷ giá FX, chia sẻ hộ gia đình. Nhiều tháng công việc. |
| Mô-đun Social (§9 cơ bản, Phase 7) | Bài đăng + feed + reaction + DM + cộng đồng + kiểm soát quyền riêng tư đã chiếm một phần tư khối lượng công việc, còn chưa tính tới các tính năng nâng cao ở §9.12+. |
| Social nâng cao (§9.13–9.37, Phase 10) | Reels, live streaming, phòng audio, bình chọn, karma, wiki — mỗi thứ là một dự án phụ riêng. |
| Kinh tế nhà sáng tạo (§10, Phase 11) | Tip + subscription + paywall + payout phụ thuộc vào việc module bank phải ra mắt trước. |
| Marketplace (§11, Phase 12) | Mô-đun cầu nối trải rộng qua social + bank; cả hai phải ra mắt trước. |
| An toàn ML (§12, Phase 12) | Bộ phân loại NSFW + CSAM + độc hại cần hạ tầng model. Hàng đợi báo cáo thủ công là câu trả lời cho v1. |
| LiveKit + mediamtx (§9.25, Phase 10/12) | ~500 MB + CPU bùng nổ + networking phức tạp. Không nằm trong v1. Compose profile `--calls` và `--profile live` đã chặn sẵn chúng; giữ các profile này ở trạng thái tắt. |
| Stack khả năng quan sát (Phase 1, D-8) | Loki + Prometheus + Tempo + Grafana + GlitchTip là 5 dịch vụ. Chạy với log JSON qua stdout trong v1; thêm stack này khi lưu lượng truy cập đủ lớn để biện minh cho nó. Giữ `--profile observability` ở trạng thái tắt. |

[ADR-01](./01-v1-scope-cut.md) nhắc lại bộ cắt này một cách chính thức.

## 6. Những phát hiện mà kho tài liệu chưa đề cập

Một vài điều không có trong các tài liệu đầu vào và nên được để ý:

- **Không đề cập tới việc tinh chỉnh connection pool cơ sở dữ liệu** giữa chế độ transaction-pool của PgBouncer và `pgx`. Asynq, API, và worker cùng chia sẻ một cluster Postgres; nếu không cẩn thận về kích thước pool, worker sẽ khiến API bị đói tài nguyên trong lúc transcode bùng nổ.
- **`OIDC_GROUP_ROLE_MAP` được cấu hình qua biến môi trường** [D-26]. Một lỗi gõ nhầm trong production sẽ âm thầm tước quyền của một admin ở lần refresh tiếp theo. Nên thêm kiểm tra tại thời điểm khởi động để gắn cờ các nhóm Authentik chưa được ánh xạ nhưng có mặt trong danh sách admin khởi tạo. *(Cập nhật 2026-07-06: đã lỗi thời — ADR-06 đã loại bỏ OIDC/Authentik; không còn tồn tại việc ánh xạ nhóm-vai trò nào nữa.)*
- **Chuỗi xoay vòng refresh-token phát ra một sự kiện chain-revoke khi bị tái sử dụng**, nhưng kho tài liệu không định nghĩa cái gì sẽ *tiêu thụ* sự kiện đó. Tối thiểu, chain-revoke nên gửi một email bảo mật cho người dùng; module notification thuộc Phase 6, nên với v1 thì hoặc là ghi log thật rõ ràng, hoặc gửi một email tự viết tay từ auth handler.
- **`disabled_at` trên bảng users được kiểm tra trong middleware**, nhưng không có job theo lịch nào để thu hồi các refresh token đang hoạt động khi một người dùng bị vô hiệu hóa. Các refresh token hiện có của một người dùng bị vô hiệu hóa vẫn tiếp tục xoay vòng cho tới khi hết hạn. Đáng để thêm một dòng `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = ?` cạnh handler vô hiệu hóa.
- **Cookie SameSite=Strict ở frontend yêu cầu Next.js và API phải chia sẻ chung một domain có thể đăng ký (registrable domain)** [D-34]. Định tuyến theo path trên một domain duy nhất qua Traefik hoạt động tốt, nhưng đây là một ràng buộc triển khai không hiển nhiên cần có trong tài liệu vận hành ngay từ ngày đầu — dễ bị cấu hình sai trong môi trường phát triển với `localhost:3000` + `localhost:8080` (đó là hai origin *khác nhau* xét theo mục đích của SameSite).

## 7. Tóm tắt khuyến nghị

> **Cập nhật (2026-07-06):** cả năm khuyến nghị đều đã được thực thi; vòng lặp demo v1 (đăng nhập local → tải lên → phát lại HLS → đăng xuất có thể thu hồi) đã khép kín và được commit. ADR-06 sau đó đã thay thế mô hình xác thực. `MILESTONE_CHECKS.md` là bộ theo dõi trạng thái đang sống.

Năm ADR cụ thể theo sau đây. Theo thứ tự ưu tiên của sprint:

1. **[ADR-05](./05-phase0-wiring-order.md)** — đóng khoảng trống nối dây (Ngày 1–3). Không gì khác quan trọng cho tới khi `make up && make dev && curl /api/v1/healthz` trả về 200 từ một mô-đun đã được *khởi tạo*.
2. **[ADR-01](./01-v1-scope-cut.md)** — thống nhất bộ cắt v1 để bạn không theo phản xạ mà đụng tới module bank ở tuần 2.
3. **[ADR-02](./02-rbac-model-reconciliation.md)** — chọn một mô hình RBAC và viết nó ra trước khi bất kỳ UI admin nào được ra mắt.
4. **[ADR-03](./03-single-vps-topology.md)** — chốt bộ profile compose và kích thước VPS để script deploy không lỡ ra mắt với `--profile observability` bị bật nhầm.
5. **[ADR-04](./04-storage-tier-budget.md)** — quyết định MinIO+R2 hay chỉ-R2 trước khi handler tải lên được viết; quyết định này định hình interface lưu trữ.

Xem sơ đồ cảnh quan hệ thống tại [`diagrams/system-landscape.md`](./diagrams/system-landscape.md) để có bức tranh theo phạm vi v1.
