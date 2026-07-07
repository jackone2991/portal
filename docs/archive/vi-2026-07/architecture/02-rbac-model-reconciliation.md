# ADR-02: Hòa giải RBAC — phân cấp vai trò (đã dựng) và gói chính sách (đã đặc tả)

**Trạng thái:** Được chấp nhận
**Ngày:** 2026-05-24
**Người quyết định:** kirito
**Thay thế:** Không áp dụng (quyết định đầu tiên trong mảng này)
**Ảnh hưởng:** [feature.md D-26], [archivetech.md §2, §3.3]

> **Cập nhật (2026-07-06):** Đã chấp nhận và ra mắt — v1 chạy trên mô hình phân cấp vai trò đúng như quyết định ở đây; tầng policy-bundle/user-group và file-gating vẫn còn hoãn lại.
> - [ADR-06](./06-local-auth-model.md) (2026-07-05) đã gỡ Authentik: input `user_oidc_roles` (đồng bộ từ nhóm Authentik) trong Spec A và số hạng `user_roles ∪ user_oidc_roles` ở bước 1 của quy tắc hợp thành đều bị gỡ bỏ theo (bảng đó đã bị xoá bởi migration `0006`); quyền hiệu lực giờ chỉ đến từ `user_roles` + tổ tiên vai trò. Mọi cơ chế khác của Spec A (ngữ pháp, thu hồi hai-kênh, cache key `rbac:perms:<userID>:v<N>`) không đổi.
> - `rbac.Matches` từ đó đã được sửa để một grant hành-động-wildcard như `movies:*` bao phủ mọi scope kể cả `:own` — nhất quán với, chứ không mâu thuẫn với, ADR này.

## Bối cảnh

Dự án có **hai spec kiểm soát truy cập mâu thuẫn nhau**, và code chỉ tồn tại cho một trong hai.

### Spec A — Phân cấp vai trò (CLAUDE.md + feature.md + code thực tế)

- Ngữ pháp permission: `<resource>:<action>[:<scope>]` với wildcard `*` / `:any` / `:own`.
- Grant chảy qua **role**. `roles.parent_id` tạo một phân cấp dạng adjacency-list: `guest → user → creator → editor → moderator → admin → superadmin`.
- Tập quyền hiệu lực = duyệt tổ tiên role qua recursive CTE, hợp với `user_roles` được gán trực tiếp, hợp với `user_oidc_roles` (đồng bộ từ nhóm Authentik). [D-26]
- Triển khai tại `backend/internal/modules/account/rbac/`.
- Thu hồi hai-kênh: `users.token_version` (logout-all tức thì) + `refresh_tokens.revoked_at` (thu hồi chuỗi).
- Cache key `rbac:perms:<userID>:v<N>` namespace theo `token_version`.

### Spec B — Gói chính sách (archivetech.md)

- Cùng ngữ pháp permission ở tầng lá.
- Grant chảy qua **policy** (gói có tên, tái sử dụng được như "Radiologist", "Read-Only Auditor"). Policy gắn vào **user group** hoặc trực tiếp vào **user**.
- User group tạo phân cấp riêng (`user_groups.parent_id`); một user thừa hưởng mọi policy active gắn vào bất kỳ group tổ tiên nào cộng với policy riêng-per-user của họ.
- **Permission file-gated**: một số permission trong policy cần một file đã upload (license, chứng chỉ) để có hiệu lực. Có hàng đợi review admin. File hết hạn → permission âm thầm biến mất.
- Giải quyết xung đột: **deny-thắng** (ngữ nghĩa AWS IAM / OPA).
- archivetech.md §1 tuyên bố "spec thắng, điều chỉnh code, không phải ngược lại" — nghĩa là nếu spec này được chấp nhận, code phân-cấp-vai-trò hiện tại là sai.

### Tại sao đây là một xung đột thật

Đây không phải hai góc nhìn của cùng một mô hình. Chúng là hai mô hình khác nhau với entity chính khác nhau (role so với policy), khái niệm nhóm khác nhau (system role trong spec A không phải cùng thứ với user group trong spec B), luồng invalidate cache khác nhau (file-gating trong B không có tương đương trong A), và ngữ nghĩa audit khác nhau (spec B log các sự kiện "permission mất hiệu lực" mà spec A không có khái niệm tương ứng).

Bạn không thể ship cả hai nguyên trạng. Bạn có thể ship một, ship cả hai xếp chồng, hoặc ship một bây giờ và migrate sau. Chi phí khác nhau.

## Quyết định

**Với v1, giữ phân cấp vai trò (Spec A) làm nguyên thủy (primitive) cấp grant. Định hình lại gói chính sách (Spec B) thành một *tầng xếp chồng lên trên* role, hoãn sang một phase tương lai. Permission file-gated vẫn là tính năng Phase-3+ (theo đúng phân kỳ của chính archivetech.md), không phải v1.**

Cụ thể:

1. **Spec A là chính tắc cho v1.** Code phân-cấp-vai-trò hiện tại được giữ nguyên. `users.token_version` vẫn là kênh thu hồi. Việc duyệt quyền-hiệu-lực bằng recursive-CTE không đổi.
2. **User Group của Spec B trở thành một module tương lai**, không phải một cách đổi tên của `roles`. Gọi nó là `usergroup` (hoặc gộp vào một module `organization`/`tenant` tương lai — xem [D-24]) để hai phân cấp không đụng nhau về từ vựng.
3. **Policy của Spec B xếp chồng lên trên.** Khi module policy ra mắt, một policy sẽ mở rộng thành một tập grant `(role | permission)`; việc duyệt quyền-hiệu-lực có thêm bước "policy gắn vào user/group này" *trước* bước hợp role.
4. **File-gating trở thành một bộ lọc hiệu-lực-permission** ở cuối chuỗi giải quyết. Matcher hiện tại vẫn chỉ-grant; lọc hiệu lực là một stage riêng loại bỏ các permission mà file bắt buộc bị thiếu/hết hạn/bị từ chối.
5. **Precedence deny-thắng được giữ chỗ.** Spec A chỉ-grant và hiện chưa hỗ trợ rule deny. Giữ chỗ hợp đồng này — khi deny tường minh lên, thứ tự là "bất kỳ đường dẫn deny nào cũng thắng". archivetech.md §2.3 đã cam kết hợp đồng này; ghi lại ngay bây giờ dù chưa có code nào triển khai deny.

Với chính v1, không có cơ chế policy/group nào tồn tại. Sprint 2 tuần chỉ ship auth phân-cấp-vai-trò vốn đã hoạt động. Mục đích của ADR này là **ngăn hai spec bị triển khai đồng thời và không tương thích nhau**, và giữ đường mở để thêm policy chồng lên sau.

## Các phương án đã cân nhắc

### Phương án A — Migrate sang Spec B; viết lại module role

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Cao — viết lại mọi test chạm-vào-auth, migration 0002+, RBAC engine, middleware |
| Chi phí | 4–7 ngày công dev-đơn-độc trước khi v1 ra mắt |
| Khả năng mở rộng | Spec B có thể lập luận là mô hình dài hạn linh hoạt hơn |
| Mức quen thuộc của team | Đất mới; deny-thắng của Spec B chưa quen |

**Ưu điểm:** Điều khoản "spec thắng" của archivetech.md §1 được tôn trọng. Permission file-gated là công dân hạng nhất.
**Nhược điểm:** Đốt 30–50% sprint v1 vào một cuộc viết lại. Vứt bỏ code đang chạy với test coverage. Thứ đầu tiên demo v1 chứng minh là luồng auth — làm nó bất ổn ở tuần 1 làm bất ổn mọi thứ.

### Phương án B — Giữ Spec A; xếp Spec B chồng lên trên ở một phase tương lai *(được chọn)*

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Thấp cho v1; trung bình cho việc xếp chồng sau này |
| Chi phí | 0 ngày bây giờ; ~1 tuần khi thêm policy |
| Khả năng mở rộng | Tốt-nhất-của-cả-hai — role cho truy cập thô, policy cho fine-grain cấp tổ chức |
| Mức quen thuộc của team | Code hiện tại được giữ nguyên; không xáo trộn auth |

**Ưu điểm:** v1 ra mắt với auth vốn đã hoạt động. Xếp policy chồng lên trên role là một pattern đã được kiểm chứng (AWS IAM có cả hai); việc duyệt quyền-hiệu-lực chỉ thêm một bước hợp nữa. File-gating khớp gọn như một bộ lọc stage-cuối.
**Nhược điểm:** Hai phân cấp khái niệm cho việc quản lý grant (role + policy + user group). Operator phải học cả hai. Lời hứa "spec thắng" trong archivetech.md bị *làm nhẹ đi*, không phải được tôn trọng — cần một ADR tường minh để ghi lại sự thay đổi ý định này.

### Phương án C — Lai ngay bây giờ: giữ role, thêm policy trong v1

| Khía cạnh | Đánh giá |
| --- | --- |
| Độ phức tạp | Trung bình-cao — hai bảng mới, code giải quyết mới, admin UI mới |
| Chi phí | 3–5 ngày của sprint v1 |
| Khả năng mở rộng | Giống Phương án B về dài hạn |
| Mức quen thuộc của team | Hỗn hợp |

**Ưu điểm:** Tránh khoản nợ tương lai "chúng ta đã nói sẽ thêm policy".
**Nhược điểm:** Chen chúc v1 với các tính năng ngoài-demo. Admin UI cho policy không demo được trong happy-path 7-bước. Scope creep thuần tuý, đi ngược [ADR-01](./01-v1-scope-cut.md).

## Phân tích đánh đổi

Lập luận mạnh nhất của Phương án A là điều khoản "spec thắng"; lập luận yếu nhất của nó là spec mà nó đang tôn trọng (archivetech.md) tự nó là một bản phác thảo 6-màn-hình từ `template-main/portal/document/anh{1,2,3}.png` không có code nào phía sau. Spec chưa có tư cách để ghi đè code đang chạy.

Lập luận mạnh nhất của Phương án C là "làm đúng ngay từ đầu"; lập luận yếu nhất của nó là "đúng" ở đây nghĩa là "policy + role + group + file-gating + hàng đợi review" — năm khái niệm dồn vào một sprint vốn đã có 8 deliverable. Bề mặt auth trở nên giòn (brittle) đúng lúc demo cần nó ổn định.

Lập luận mạnh nhất của Phương án B là trình tự — làm cho v1 demo được, rồi thêm các tính năng cấp-tổ-chức khi có một operator thật sự yêu cầu. Lập luận yếu nhất của nó là chi phí khái niệm: bất kỳ ai đọc cả hai spec đều phải tự ghép role + policy + group + file-gating vào một mô hình trong đầu. Việc của ADR này là làm cho sự ghép nối đó tường minh để tương-lai-của-bạn không phải reverse-engineer nó.

Quy tắc hợp thành (composition rule), viết ra một lần cho rõ ràng:

```
effective_permissions(user, tenant):
    1. roles_user_holds = recursive_walk(user_roles ∪ user_oidc_roles)
    2. policies_user_holds = ∪{
           policies_attached(user),
           policies_attached(group) for group in walk(user_groups(user))
       }
    3. grants = ∪{
           permissions(role) for role in roles_user_holds,
           permissions(policy) for policy in policies_user_holds
       }
    4. effective = filter(grants, where file_gate_satisfied(grant, user))
    5. if any deny grant in policies_user_holds matches the required code:
           return DENY
    6. return effective
```

Bước 1, 3, và 5 (không có deny) là những gì hiện đã tồn tại. Bước 2 và 4 là những bổ sung khi Spec B xếp chồng lên. Đường deny của bước 5 được giữ chỗ.

## Hệ quả

**Cái gì dễ hơn:**

- v1 ra mắt đúng hạn. Demo 7-bước không bị ảnh hưởng bởi quyết định này.
- Các test hiện có trong `backend/internal/modules/account/rbac/permission_test.go` vẫn xanh.
- Khi policy được thêm vào, code hiện tại không đổi — code mới hoàn toàn mang tính cộng thêm (bảng mới, stage giải quyết mới).

**Cái gì khó hơn:**

- archivetech.md cần một ghi chú đầu trang (hoặc một ADR thay thế) nói rằng phần RBAC của nó **xếp chồng lên trên**, không phải **thay thế**, phân cấp vai trò. Thiếu nó, người đọc kế tiếp sẽ thấy mâu thuẫn.
- Các contributor tương lai sẽ thấy hai khái niệm grant và cần ADR này để biết chúng hợp thành ra sao. Đảm bảo quy tắc hợp thành ở trên cũng được phản ánh trong `backend/internal/modules/account/README.md` một khi nó được viết.
- Lời hứa "deny-thắng" ràng buộc chúng ta vào một ngữ nghĩa cụ thể. Nếu một triển khai deny-tường-minh tương lai quên lời hứa đó, các hồi quy bảo mật khó-debug có thể xảy ra.

**Cái gì cần xem lại:**

- Khi admin UI từ `anh1/2/3.png` được xây, các màn hình đó dành cho **policy + group**, không phải role. Việc UI kéo các bảng của Spec B lên trước. Lên kế hoạch một sprint Policy/Group khi các mock đó lên đầu backlog (hậu-v1, có thể là Phase 1.5 hoặc công việc social-page-role của Phase 7).
- Permission file-gated cần object storage cho license đã upload + một hàng đợi review admin + kiểm tra hết hạn dựa trên cron. Những thứ này đủ độc lập để ship như một phase riêng (Phase 3 trong archivetech.md), và việc gate phase đó vào sự tồn tại của tầng policy là thứ tự đúng.
- Adjacency list phân-cấp-vai-trò chỉ có CHECK ngăn tự-tham-chiếu (self-cycle); chu trình sâu hơn được ngăn ở app layer. Khi policy và group lên, cùng một DB CHECK chỉ-tự-tham-chiếu đó sẽ không đủ — đảm bảo migration policy/group có cơ chế ngăn chu trình đúng đắn (hoặc kiểm tra app-layer được gia cố với test tường minh).

## Hạng mục hành động

1. [x] Thêm một ghi chú 5-dòng ở đầu `archivetech.md` tham chiếu ADR này và nói rõ mô hình RBAC của nó là *tầng Phase 1.5+ bị hoãn*, không phải mô hình v1. **Không** mâu thuẫn ngầm. *(xong 2026-07-06 — đã thêm banner vào archivetech.md)*
2. [ ] Thêm quy tắc hợp thành (đoạn pseudocode 6-dòng ở trên) vào `backend/internal/modules/account/README.md` (hoặc một stub nếu README chưa tồn tại) để mối quan hệ này hiển thị từ phía code.
3. [ ] Giữ chỗ rule depguard cho `internal/modules/policy/` và `internal/modules/usergroup/` để khi các module đó lên, depguard đã biết chúng tồn tại (tránh phải xáo trộn vì "module không có trong allowlist").
4. [ ] Mở một issue theo dõi "RBAC Phase 1.5: policy bundles + user groups (ADR-02 layered model)" để công việc bị hoãn hiển thị mà không cần lên lịch.
5. [ ] Sau khi v1 ra mắt, trước khi bắt đầu bất kỳ công việc admin UI nào, lên lịch sprint Policy/Group. Quy tắc hợp thành từ ADR này là hợp đồng mà sprint đó triển khai.
