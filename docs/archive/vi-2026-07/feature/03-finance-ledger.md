# 03 — Sổ chi tiêu / Finance Ledger (module `bank`, scope ledger)

**Module:** `bank` (tên đã đặt chỗ trong diagrams/MODULES; chưa có code).
**Tham chiếu:** [feature.md §8](../feature.md) — spec này hiện thực **một tập con** (lõi §8.1–8.2).
**Phụ thuộc:** không phụ thuộc cứng; đính kèm hóa đơn tái dùng spec 01.
**Yêu cầu:** ADR-08 (spec 00) — amend ADR-01, vốn đang hoãn "bank" nguyên khối.

## Phát biểu vấn đề

Mặt tiền bạc của life OS. Scope chốt trong brainstorm: **sổ chi tiêu cá nhân tầm
Money Lover** — đa tài khoản, giao dịch nhập tay, danh mục, ngân sách tháng,
chuyển khoản nội bộ — với schema **sẵn sàng cho import ngay từ ngày đầu** dù chính
tính năng import sao kê được hoãn (TCB xuất PDF → cần OCR; xem
[04-deferred.md](04-deferred.md)).

Insight then chốt từ brainstorm: một ledger nhập tay self-hosted **không giữ
credential ngân hàng nào**, nên luật "MFA trước bank" (D-27/D-28) **không** chặn
scope này. TOTP trở thành điều kiện mở khóa cho *tích hợp bank thật* về sau.

## Mục tiêu

- Dev ghi chép chi tiêu hằng ngày trên ≥3 tài khoản thật (vd TCB, tiền mặt, Momo)
  trọn một tháng mà không vướng tường.
- Chuyển khoản giữa các tài khoản của mình không bao giờ làm méo báo cáo thu/chi.
- Mỗi giao dịch phát một event lên bus — mặt tiền bạc nuôi dòng đời từ ngày đầu.
- Schema hấp thụ được import sao kê tương lai **không cần migration phá vỡ**.

## Không phải mục tiêu (kèm lý do)

- **Import sao kê (mọi định dạng)** — hoãn; TCB=PDF cần OCR (xem 04). Chỉ chuẩn bị schema.
- **Nợ (debts), cho vay (loans), đầu tư, mục tiêu tiết kiệm** (§8.3–8.5) — các vòng
  lặp riêng; ledger trước.
- **Split, giao dịch định kỳ, tag** — P2 bên dưới; vòng lõi chạy được không cần chúng.
- **Báo cáo đa tiền tệ có FX** — mỗi tài khoản một currency; tổng chéo tiền tệ nằm
  ngoài (chưa có hạ tầng tỷ giá ở v1). User một-tiền-tệ (VND) không bị ảnh hưởng.
- **Kết nối bank thật / TOTP** — rõ ràng là bước mở khóa *kế tiếp*, không thuộc spec này.

## User story

- Là user, tôi tạo tài khoản nhiều loại (tiền mặt, thanh toán, ví điện tử, thẻ tín
  dụng) với số dư đầu kỳ, để sổ phản chiếu thực tế.
- Là user, tôi ghi một khoản chi trong vài giây — số tiền, tài khoản, danh mục,
  ghi chú — vì friction giết thói quen ghi chép hằng ngày.
- Là user, tôi ghi một lần chuyển TCB → Momo, nó thay đổi cả hai số dư mà không
  hiện thành thu hay chi.
- Là user, tôi đặt ngân sách tháng theo danh mục và thấy tiến độ so với nó.
- Là user, tôi lưu trữ (archive) một tài khoản đã đóng mà không mất lịch sử.

## Yêu cầu

### P0 — bắt buộc

1. **Tài khoản**: loại (cash|checking|savings|credit_card|ewallet|other), currency
   (mặc định VND), số dư đầu kỳ, active/archived. Số dư là **giá trị dẫn xuất**
   (đầu kỳ + Σ giao dịch), không bao giờ là cột mutable lưu sẵn.
   - [ ] Archive ẩn tài khoản khỏi picker nhưng giữ nguyên lịch sử và báo cáo.
2. **Giao dịch (nhập tay)**: số tiền (BIGINT đơn vị nhỏ nhất — với VND = 1 đồng;
   hiển thị qua helper Money, không bao giờ float), chiều debit|credit, tài khoản,
   danh mục, occurred_at, ghi chú. Cột sẵn-sàng-import có mặt từ migration #1:
   `description_raw text null`, `import_batch_id uuid null`, `dedup_hash text null`.
   - [ ] Tạo/sửa/xóa giao dịch cập nhật đúng số dư dẫn xuất của tài khoản.
   - [ ] Form nhập ≤ 4 trường bắt buộc; picker danh mục mặc định mục dùng gần nhất.
3. **Chuyển khoản**: mô hình **cặp debit+credit chung `transfer_id`**;
   loại khỏi báo cáo thu/chi; tính vào số dư từng tài khoản.
   Sửa/xóa một vế thì sửa/xóa cả cặp một cách nguyên tử.
   - [ ] Chuyển 5.000.000 VND thay đổi cả hai số dư và làm tổng thu/chi tháng
         xê dịch đúng bằng 0.
4. **Danh mục**: phân cấp (2 tầng là đủ), seed mặc định hợp ngữ cảnh Việt
   (Ăn uống, Di chuyển, Hóa đơn, Lương…), user mở rộng được.
5. **Ngân sách tháng**: số tiền theo danh mục theo tháng; dashboard hiện đã-chi/ngân-sách.
6. **Dashboard**: số dư từng tài khoản, thu/chi tháng hiện tại, thanh ngân sách.
7. **Event**: phát `bank:transaction_created` (và `bank:transaction_deleted`) lên
   bus từ ngày đầu.
8. **RBAC**: toàn bộ dữ liệu scope chặt theo chủ sở hữu (`bank:transaction:*:own`
   v.v.); không có đọc-chéo-user ở bất kỳ mức quyền nào ngoài admin wildcard tường minh.
9. **Bảng `import_batches` tồn tại sẵn** (id, source, file_name, created_at, status)
   — rỗng cho tới khi tính năng import ra đời; `import_batch_id` tham chiếu nó.
   Rẻ bây giờ, đau đớn nếu retrofit.

### P1 — nên có

10. Đính kèm hóa đơn vào giao dịch (image asset qua `mediaapi` — spec 01).
11. Trang báo cáo tháng đơn giản (phân rã theo danh mục, so tháng trước).
12. Event `bank:budget_exceeded` khi một danh mục vượt 100%.

### P2 — cân nhắc tương lai (bảo hiểm kiến trúc)

13. Split (một giao dịch, nhiều danh mục) — giữ cửa mở: query báo cáo nên aggregate
    qua join danh mục, đừng giả định mãi mãi chỉ có 1 cột category.
14. Giao dịch định kỳ (sinh draft để user xác nhận).
15. Tag.
16. **Import sao kê** (xem 04): CSV/xlsx generic + template column-mapping dạng
    dữ-liệu-không-phải-code; preset theo bank; dedup qua `dedup_hash`
    (tài khoản + ngày + số tiền + ref); rollback theo batch qua `import_batch_id`.

## Phác thảo data model (migration số trống kế tiếp, `000N_bank_*`)

```
bank_accounts(id, user_id, name, type, currency char(3) default 'VND',
              opening_balance bigint, archived bool, created_at)
bank_categories(id, user_id null /* null = seed mặc định */, parent_id null,
                name, kind check in ('income','expense'))
bank_transactions(id, user_id, account_id fk, category_id fk null,
                  amount bigint check (amount > 0), direction check in ('debit','credit'),
                  transfer_id uuid null, occurred_at date, note text,
                  description_raw text null, import_batch_id uuid null,
                  dedup_hash text null, created_at, updated_at)
  -- partial unique index trên (account_id, dedup_hash) where dedup_hash is not null
bank_budgets(id, user_id, category_id fk, month date /* mùng 1 của tháng */,
             amount bigint, unique(user_id, category_id, month))
bank_import_batches(id, user_id, source, file_name, status, created_at)
```

Chuyển khoản mang `category_id = null` và `transfer_id = uuid chung` trên cả hai vế.

## Phác thảo API

```
GET/POST           /api/v1/bank/accounts        PATCH /accounts/{id} (archive)
GET/POST           /api/v1/bank/transactions    ?account=&month=&category=
PATCH/DELETE       /api/v1/bank/transactions/{id}
POST               /api/v1/bank/transfers        {from,to,amount,occurred_at,note}
GET/POST           /api/v1/bank/categories
GET/PUT            /api/v1/bank/budgets          ?month=
GET                /api/v1/bank/dashboard        ?month=
```

## Tín hiệu thành công (metrics trung thực với n=1)

- Leading: dev ghi giao dịch ≥20 trên 30 ngày đầu (bài test friction).
- Lagging: số dư cuối tháng đối soát khớp tài khoản thật trong sai số làm tròn;
  nếu đối soát đau đớn, mô hình transfer/sửa-xóa đang sai — sửa trước khi thêm feature.

## Câu hỏi mở

- **(product, không chặn)** Tài khoản thẻ tín dụng: v1 coi như tài khoản thường
  với số dư âm; logic chu kỳ sao kê (ngày đáo hạn) để tương lai.
- **(engineering, chặn)** Bề mặt frontend: route group `(bank)` mới — các
  màn/template Olympus nào map vào (frontend.md Phase 5 liệt kê `<MoneyDisplay />`,
  `<MoneyInput />`)? Quyết trước khi build dashboard.
- **(product, không chặn)** Dòng đời hiện số tiền, hay chỉ "đã ghi 3 giao dịch hôm
  nay" (riêng tư khi nhìn màn hình)? Quyết khi dòng đời ra đời.
