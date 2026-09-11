import { MoneyDisplay } from "portal-frontend";

// Bank-ledger money display (SPEC-03 §8). The wire carries integer minor units
// (D-41) and this formats them VND-style with thousands separators. `signed`
// switches on the ledger colouring: red for money out, green for money in, and
// an explicit + on positives.

const row = (label: string, children: React.ReactNode) => (
  <div className="flex items-baseline justify-between gap-6 py-1.5">
    <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>{label}</span>
    <span className="text-lg font-semibold" style={{ color: "var(--tpl-heading)" }}>{children}</span>
  </div>
);

const frame = (children: React.ReactNode) => (
  <div
    data-template="v1"
    className="rounded-xl p-4"
    style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)", minWidth: 300 }}
  >
    {children}
  </div>
);

export const Plain = () =>
  frame(
    <>
      {row("Số dư", <MoneyDisplay amount={1500000} />)}
      {row("Hạn mức", <MoneyDisplay amount={12000000} />)}
      {row("Bằng không", <MoneyDisplay amount={0} />)}
    </>,
  );

export const Signed = () =>
  frame(
    <>
      {row("Lương tháng 8", <MoneyDisplay amount={24000000} signed />)}
      {row("Tiền chợ", <MoneyDisplay amount={-320000} signed />)}
      {row("Không đổi", <MoneyDisplay amount={0} signed />)}
    </>,
  );

export const WithCurrency = () =>
  frame(
    <>
      {row("Tài khoản chính", <MoneyDisplay amount={8750000} currency="VND" />)}
      {row("Tiết kiệm", <MoneyDisplay amount={150000000} currency="VND" />)}
    </>,
  );
