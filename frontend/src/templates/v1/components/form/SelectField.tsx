"use client";

import { useState, type CSSProperties } from "react";
import { Icon } from "../ui/Icon";

export type SelectOption = string | { value: string; label: string };

/**
 * Floating-label native `<select>` — the Olympus `.form-group.label-floating
 * .is-select` control. A select always carries a value, so the label stays in
 * the floated slot; it recolours to the accent on focus alongside the growing
 * underline. Accepts either a plain `string[]` or `{value,label}[]` options.
 */
export function SelectField({
  label,
  options,
  value,
  onChange,
  disabled = false,
}: {
  label: string;
  options: SelectOption[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
}) {
  const [focused, setFocused] = useState(false);
  const normalized = options.map((o) =>
    typeof o === "string" ? { value: o, label: o } : o,
  );

  const labelStyle: CSSProperties = {
    transform: "translateY(-1.25rem) scale(0.8)",
    color: focused ? "var(--tpl-accent)" : "var(--tpl-muted)",
  };

  return (
    <div className="relative pt-5" style={{ opacity: disabled ? 0.55 : 1 }}>
      <label
        className="pointer-events-none absolute left-0 top-5 origin-left text-sm transition-all duration-200 ease-out"
        style={labelStyle}
      >
        {label}
      </label>
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onFocus={() => setFocused(true)}
        onBlur={() => setFocused(false)}
        disabled={disabled}
        className="w-full appearance-none bg-transparent pb-1.5 pr-7 text-sm outline-none disabled:cursor-not-allowed"
        style={{ color: "var(--tpl-text)" }}
      >
        {normalized.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
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
      <span
        className="pointer-events-none absolute bottom-2 right-0"
        style={{ color: focused ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
      >
        <Icon name="dropdown-arrow-icon" size={11} />
      </span>
    </div>
  );
}
