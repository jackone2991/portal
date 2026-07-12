"use client";

// Money display/input for the bank ledger (SPEC-03 §8). The wire carries integer
// minor units (D-41); these components own the display-layer string handling
// (VND thousands separators, "1.500.000").

import { formatVND, parseVND } from "@/lib/bank";

export function MoneyDisplay({
  amount,
  currency,
  className,
  signed,
}: {
  amount: number;
  currency?: string;
  className?: string;
  /** Colour negative red / positive green and show an explicit + sign. */
  signed?: boolean;
}) {
  const tone = signed ? (amount < 0 ? "text-red-400" : amount > 0 ? "text-green-400" : "") : "";
  const prefix = signed && amount > 0 ? "+" : "";
  return (
    <span className={`tabular-nums ${tone} ${className ?? ""}`}>
      {prefix}
      {formatVND(amount)}
      {currency ? <span className="ml-1 text-xs opacity-60">{currency}</span> : null}
    </span>
  );
}

export function MoneyInput({
  value,
  onChange,
  placeholder,
  className,
  id,
}: {
  value: number;
  onChange: (minor: number) => void;
  placeholder?: string;
  className?: string;
  id?: string;
}) {
  return (
    <input
      id={id}
      type="text"
      inputMode="numeric"
      value={value === 0 ? "" : formatVND(value)}
      placeholder={placeholder ?? "0"}
      onChange={(e) => onChange(parseVND(e.target.value))}
      className={
        className ??
        "w-full rounded-md bg-gray-800 border border-gray-700 px-3 py-2 text-right tabular-nums text-white focus:border-blue-500 focus:outline-none"
      }
    />
  );
}
