# ADR-06: Xác thực bằng mật khẩu local — Portal tự giữ thông tin đăng nhập (bỏ Authentik khỏi luồng login)

**Trạng thái:** Được chấp nhận (Accepted)
**Ngày:** 2026-07-05
**Người quyết định:** kirito
**Thay thế:** quyết định đăng nhập-OIDC trong [ADR-05](./05-phase0-wiring-order.md) (Milestone 0.4) và câu "Không có mật khẩu local. OIDC qua Authentik" trong [CLAUDE.md](../../../CLAUDE.md) (module account).

## Cập nhật (2026-07-06) — đã triển khai

- Tất cả endpoint đã lên: `POST /auth/login {email, password, remember}` (rate-limit + khóa tài khoản chống brute-force qua Redis), `POST /auth/register` (trả về 201, không tạo session — user quay lại `/login`), cùng các endpoint không đổi `/auth/refresh`, `/auth/logout`, `/auth/logout-all`, `/auth/me`. Argon2id ở mức 64 MB / t=3 / p=2, định dạng PHC.
- Các sai khác so với spec bên dưới: cờ `remember` chọn giữa cookie bền vững (persistent, 24h) và cookie phiên (session); một cookie thứ ba `portal_session` (Path=/, marker được cổng middleware của Next.js đọc) đi kèm `portal_access`/`portal_refresh`; `.env` hiện tại set `ACCESS_TOKEN_TTL=5m` và `REFRESH_TOKEN_TTL=24h` (không phải 30d).
- Code OIDC, các service compose của Authentik, blueprint, và cấu hình `OIDC_*` đã bị xóa hoàn toàn.
- Drift còn sót lại: `shared/openapi.yaml` vẫn liệt kê `/auth/callback` đã bị loại bỏ và còn thiếu `/auth/register`.

## Bối cảnh

Thiết kế OIDC-qua-Authentik (đã dựng và nối dây trong sprint Phase-0) xác thực người dùng **tại IdP**: trình duyệt bị chuyển hướng từ Portal sang Authentik, người dùng nhập thông tin đăng nhập trên trang của Authentik, rồi Authentik trả về một authorization code để API đổi lấy danh tính người dùng.

Kiến trúc đó sạch sẽ, nhưng phát sinh **một phản đối về trải nghiệm người dùng**: đăng nhập luôn rời khỏi domain Portal để sang `auth.portal.localhost`, và form đăng nhập Portal mà chúng ta dựng chỉ mang tính trang trí (chỉ nút SSO hoạt động) — nên người dùng thấy "hai form". Việc gắn thương hiệu (brand) cho Authentik và chuyển hướng thẳng tới IdP ("Mức 2") đã giảm còn một form có thương hiệu, nhưng màn hình đăng nhập vẫn **do Authentik phục vụ và lưu trữ**, không phải Portal.

Chủ sản phẩm muốn form đăng nhập nằm **ngay trên Portal**, và Portal tự xác minh mật khẩu — không chuyển hướng, không dịch vụ danh tính riêng trong luồng login. ADR này ghi lại quyết định đó cùng kiến trúc của nó.

> Đây là một sự đảo ngược quyết định trước. [ADR-05](./05-phase0-wiring-order.md) chọn OIDC và [ADR-02](./02-rbac-model-reconciliation.md) giả định vai trò được đồng bộ từ Authentik. **Toàn bộ máy móc token, refresh, RBAC, thu hồi (revocation) và audit KHÔNG bị ảnh hưởng** — chỉ *cửa trước* (cách người dùng chứng minh danh tính) thay đổi. Xem §"Cái gì được tái dùng".

## Quyết định

**Portal xác thực người dùng cục bộ dựa trên chính bảng `users` của mình với mật khẩu đã băm. Authentik bị loại khỏi luồng login.** Form đăng nhập do frontend Portal phục vụ; API xác minh mật khẩu và phát hành cùng bộ access + refresh token mà hệ thống vốn đã dùng.

Cụ thể:

1. `users` được thêm cột `password_hash` (Argon2id). Không bao giờ lưu plaintext.
2. `POST /api/v1/auth/login {email, password}` xác minh hash và, khi thành công, phát hành access JWT + refresh token **đang có sẵn** và set các cookie **đang có sẵn**. Nó thay thế cả redirect OIDC `/auth/login` **lẫn** `/auth/callback`.
3. Trang `/login` của Portal trở thành form **thật** (email + password) gửi tới endpoint đó. `middleware` của frontend chặn khách vãng lai về `/login` (Portal), không phải về Authentik.
4. Tạo tài khoản qua `POST /api/v1/auth/register {email, password, display_name}` (hoặc do admin tạo) — đường upsert-từ-OIDC bị bỏ.
5. Đặt lại mật khẩu (`forgot`/`reset` với token gửi qua email) được thêm khi module thông báo (notification) xuất hiện; trước đó, dùng admin-set hoặc reset qua CLI.
6. **Authentik bị bỏ khỏi dev stack** (giải phóng ~1 GB RAM + Postgres của nó). Blueprint OIDC provider, `auth/oidc.go`, handler `/auth/callback`, đồng bộ `user_oidc_roles`, và cấu hình `OIDC_*` trở thành mã chết và được gỡ bỏ.

## Mô hình kiến trúc

### Luồng đăng nhập (Luồng B)

```mermaid
sequenceDiagram
    actor U as Người dùng (Trình duyệt)
    participant F as Frontend<br/>portal.localhost
    participant A as API<br/>api.portal.localhost
    participant DB as Postgres

    U->>F: GET / (chưa có cookie phiên)
    F-->>U: 307 → /login
    U->>F: GET /login (form Portal thật)
    Note over U: ★ nhập email + mật khẩu NGAY TRÊN PORTAL ★
    U->>A: POST /api/v1/auth/login {email, password}
    A->>DB: SELECT user theo email
    A->>A: argon2 xác minh mật khẩu; kiểm tra disabled_at
    A->>DB: INSERT refresh_token (băm sha256)
    A-->>U: Set-Cookie portal_access (JWT) + portal_refresh; 200
    U->>F: GET / (kèm cookie)
    F-->>U: trang chủ (đã xác thực)
    Note over A,DB: Không có Authentik ở bất kỳ đâu trong luồng.
```

So với luồng OIDC (đã bỏ): trình duyệt phải vòng qua `auth.portal.localhost`, API không bao giờ thấy mật khẩu, và danh tính quay về dưới dạng ID token. Ở đây mật khẩu được gửi thẳng tới API và đối chiếu với `users.password_hash`.

### Cái gì thay đổi

| Tầng | OIDC (hiện tại, đã bỏ) | Local auth (ADR này) |
| --- | --- | --- |
| Nơi lưu credential | Authentik | `users.password_hash` (Argon2id) trong Postgres của Portal |
| Màn hình đăng nhập | Trang flow Authentik (`auth.portal.localhost`) | Form `/login` của Portal (`portal.localhost`) |
| API `/auth/login` | 302 redirect tới IdP + đổi code ở `/auth/callback` | `POST {email,password}` → xác minh → phát token |
| Tạo user | `UpsertUserFromOIDC` ở callback | `POST /auth/register` (hoặc admin) |
| Cấu hình | `OIDC_ISSUER/CLIENT_ID/SECRET/REDIRECT_URL` | không (đã gỡ) |
| Hạ tầng thêm | authentik-server + worker + Postgres của nó + blueprint | **không** |
| Ai thấy mật khẩu | chỉ Authentik | API Portal (thoáng qua, rồi băm rồi bỏ) |

### Cái gì được tái dùng (không đổi)

Những phần khó, nhạy cảm về bảo mật của module account **không đổi** — chính vì thế mà việc chuyển đổi được cô lập gọn:

- **Access token** — JWT HS256 với `kid` xoay vòng, `token_version`, roles (`auth.Issuer`/`Verifier`).
- **Refresh token** — 256-bit, băm SHA-256 khi lưu, chuỗi xoay vòng + phát hiện tái sử dụng (`auth.RefreshManager`, bảng `refresh_tokens`).
- **Hai kênh thu hồi** — `users.token_version` (logout-all) + `refresh_tokens.revoked_at`.
- **RBAC** — phân cấp vai trò, hiệu lực quyền qua recursive-CTE, cache Redis khóa theo `token_version` (không đổi; vai trò giờ do Portal gán, không sync từ nhóm Authentik).
- **Cookie** — `portal_access` (Path=/) + `portal_refresh` (Path=/api/v1/auth), `HttpOnly Secure SameSite=Strict`, domain `portal.localhost`.
- **Audit log**, `/auth/refresh`, `/auth/logout`, `/auth/logout-all`, `/auth/me`.

Chỉ **bước chứng minh danh tính** tại `/auth/login` và việc tạo tài khoản là thay đổi.

### Trách nhiệm mới Portal phải tự gánh

Ủy thác cho Authentik từng cho ta những thứ này miễn phí; local auth nghĩa là phải tự làm:

- **Băm mật khẩu** — Argon2id (`golang.org/x/crypto/argon2`), tham số hợp lý (vd 64 MB, t=3, p=2), salt riêng từng user; so sánh thời-gian-hằng-số.
- **Chống brute-force** — rate-limit `/auth/login` theo IP + theo tài khoản; backoff mũ / khóa tạm khi thất bại lặp lại.
- **Chính sách mật khẩu** — độ dài tối thiểu / kiểm tra rò rỉ (breach) khi register + reset.
- **Đặt lại mật khẩu** — token dùng-một-lần gửi qua email (cần module notification, Phase 6) hoặc reset qua CLI/admin trước đó.
- **MFA / step-up** (sau này, cho module bank) — đăng ký TOTP + claim tương đương `acr`/`amr` phải được dựng trong Portal (trước do Authentik quản, [D-27]/[D-28]).
- **Đăng nhập mạng xã hội** ("Login with Google") — tự triển khai Google OAuth trong Portal (trước chỉ là một dòng cấu hình source ở Authentik).

## Các phương án đã cân nhắc

Ghi đầy đủ trong phần thảo luận trước ADR này; tóm tắt:

- **A — Giữ OIDC, gắn thương hiệu Authentik (Mức 1/2).** Một form đăng nhập có thương hiệu, nhưng host trên Authentik; Portal không bao giờ sở hữu credential; Google/MFA/step-up có sẵn miễn phí. *Bị từ chối* vì yêu cầu "form phải nằm trên Portal".
- **B — Xác thực mật khẩu local *(được chọn)*.** Form đăng nhập trên Portal, không redirect, Portal sở hữu credential. Portal phải tự dựng bề mặt bảo mật mà Authentik từng cung cấp.
- **C — Lai (login local, giữ Authentik cho MFA/social).** Phức tạp nhất; hai hệ danh tính phải hòa giải. *Hoãn* — xem lại nếu MFA/social trở thành bắt buộc và chỉ-local tỏ ra không đủ.

## Phân tích đánh đổi

Đánh đổi quyết định là **UX/quyền sở hữu đối lại bề-mặt-bảo-mật-bạn-phải-bảo-trì**. Authentik tồn tại chính là để sở hữu mật khẩu, MFA, khóa tài khoản, reset, và liên kết mạng xã hội — đã được kiểm nghiệm thực chiến. Đưa vào tự làm mua được một form đăng nhập native duy nhất và bỏ được ~1 GB hạ tầng, đổi lại là phải tự triển khai (và chịu trách nhiệm về) bề mặt bảo mật đó. Với v1 một-người-vận-hành, bề mặt này nhỏ; rủi ro tăng lên khi **module bank** (mà tài liệu nói cần step-up + MFA, [D-27]/[D-28]) và **social login** xuất hiện — đó vốn là lý do ban đầu chọn OIDC. ADR này chấp nhận chi phí tương lai đó để đổi lấy UX mong muốn ngay bây giờ, và để ngỏ Phương án C như một lối thoát (thêm Authentik lại thuần túy như một nhà cung cấp MFA/social, giữ mật khẩu local làm yếu tố chính).

## Hệ quả

**Cái gì dễ hơn**

- Một màn hình đăng nhập, do Portal phục vụ, không redirect chéo domain. Cổng `middleware` đơn giản còn "không có cookie → `/login`".
- Dev stack bỏ được authentik-server + authentik-worker + authentik-postgres + blueprint → lấy lại ~1 GB RAM, ít bộ phận chuyển động hơn, `make up` nhanh hơn.
- Không còn hack networking container→IdP (Traefik alias + `SSL_CERT_FILE`) và không lo về OIDC discovery/JWKS/tin cậy TLS.

**Cái gì khó hơn**

- Portal giờ là người giữ credential: một lỗi ở login-endpoint hoặc băm yếu là vector chiếm tài khoản trực tiếp. Rate-limit và khóa tài khoản giờ là bắt-buộc, không phải tùy-chọn.
- Đặt lại mật khẩu cần email (module notification) — trước đó, khôi phục là thủ công (admin/CLI).
- Step-up/MFA của module bank ([D-27]/[D-28]) và "Login with Google" phải tự dựng trong Portal, không cấu hình trong Authentik.

**Cái gì cần xem lại**

- [ADR-02](./02-rbac-model-reconciliation.md): `user_oidc_roles` và sync nhóm-OIDC→role ([D-26]) không còn áp dụng; vai trò chỉ do Portal gán. Cập nhật ghi chú về cách hợp thành quyền hiệu lực.
- Nếu sau này MFA/social tỏ ra nặng nề khi tự dựng, cân nhắc lại **Phương án C** (Authentik quay lại như một tầng yếu-tố-thứ-hai / social IdP đặt trên mật khẩu local).

## Hạng mục hành động (kế hoạch triển khai — tách khỏi thay đổi tài liệu này)

1. [x] Migration: thêm `users.password_hash TEXT` (+ `password_updated_at`); bỏ `user_oidc_roles` (hoặc để ngủ đông). Đưa `oidc_subject` về nullable.
2. [x] `platform/crypto` (hoặc `account/auth/password.go`): hàm băm + xác minh Argon2id.
3. [x] Query/adapter: `GetUserByEmail`, `CreateUserLocal`, `SetPassword`.
4. [x] Handler: thay `Login`/`Callback` bằng `POST /auth/login {email,password}` + `POST /auth/register`; giữ `refresh`/`logout`/`logout-all`/`me`.
5. [x] Middleware rate-limit + khóa tài khoản trên `/auth/login`.
6. [x] Frontend: form `/login` thật → `POST /auth/login`; đưa `middleware` về chặn khách về `/login`; bỏ nút SSO/Google (hoặc trỏ Google về một Google-OAuth local tương lai).
7. [x] Gỡ OIDC: `auth/oidc.go`, callback, cấu hình `OIDC_*`, dịch vụ Authentik + blueprint khỏi compose, alias Traefik + override `SSL_CERT_FILE`.
8. [ ] Tài liệu: đồng bộ mục Account của CLAUDE.md, `doc/*/authoration.md`, `doc/*/feature.md §1`, và bản mirror tiếng Việt của ADR này. *(2026-07-06: hoàn thành một phần — CLAUDE.md và bản mirror tiếng Việt đã được đồng bộ; `shared/openapi.yaml` vẫn liệt kê `/auth/callback` đã bị loại bỏ và còn thiếu `/auth/register`.)*
