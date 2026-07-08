"use client";

import { useState, type ChangeEvent, type CSSProperties } from "react";
import { Icon } from "../ui/Icon";

export type FormFieldType = "text" | "email" | "password" | "date";

/**
 * Material floating-label text input — the standalone, richer sibling of the
 * `Field` used inside the popups. Ports the Olympus `.form-group.label-floating`
 * behaviour: the label sits in the input as a placeholder and animates up when
 * the field is focused or filled, while a bottom accent underline grows on focus.
 *
 * Controlled (`value` + `onChange`) or uncontrolled (omit both) — an internal
 * value mirror keeps the floating label correct either way.
 */
export function FormField({
  label,
  value,
  onChange,
  type = "text",
  placeholder,
  icon,
  disabled = false,
}: {
  label: string;
  value?: string;
  onChange?: (value: string) => void;
  type?: FormFieldType;
  placeholder?: string;
  /** Sprite name (without the `olymp-` prefix) rendered at the right edge. */
  icon?: string;
  disabled?: boolean;
}) {
  const [focused, setFocused] = useState(false);
  const [internal, setInternal] = useState(value ?? "");
  const controlled = value !== undefined;
  const current = controlled ? value : internal;
  const filled = (current ?? "").length > 0;
  // Native date inputs paint their own placeholder (mm/dd/yyyy), so the label
  // must live in the floated slot at all times to avoid overlapping it.
  const floated = focused || filled || type === "date";

  function handleChange(e: ChangeEvent<HTMLInputElement>) {
    if (!controlled) setInternal(e.target.value);
    onChange?.(e.target.value);
  }

  const labelStyle: CSSProperties = floated
    ? {
        transform: "translateY(-1.25rem) scale(0.8)",
        color: focused ? "var(--tpl-accent)" : "var(--tpl-muted)",
      }
    : { transform: "translateY(0) scale(1)", color: "var(--tpl-muted)" };

  return (
    <div className="relative pt-5" style={{ opacity: disabled ? 0.55 : 1 }}>
      <label
        className="pointer-events-none absolute left-0 top-5 origin-left text-sm transition-all duration-200 ease-out"
        style={labelStyle}
      >
        {label}
      </label>
      <input
        type={type}
        value={current}
        onChange={handleChange}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        disabled={disabled}
        placeholder={floated ? placeholder : undefined}
        className="w-full bg-transparent pb-1.5 pr-7 text-sm outline-none disabled:cursor-not-allowed"
        style={{ color: "var(--tpl-text)" }}
      />
      {/* resting underline */}
      <span
        className="absolute bottom-0 left-0 h-px w-full"
        style={{ background: "var(--tpl-border)" }}
      />
      {/* accent underline that grows out from the left on focus */}
      <span
        className="absolute bottom-0 left-0 h-0.5 w-full origin-left transition-transform duration-200 ease-out"
        style={{ background: "var(--tpl-accent)", transform: focused ? "scaleX(1)" : "scaleX(0)" }}
      />
      {icon && (
        <span
          className="pointer-events-none absolute bottom-1.5 right-0"
          style={{ color: focused ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
        >
          <Icon name={icon} size={16} />
        </span>
      )}
    </div>
  );
}
