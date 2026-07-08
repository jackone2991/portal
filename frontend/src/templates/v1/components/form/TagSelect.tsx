"use client";

import { useState, type KeyboardEvent } from "react";
import { Icon } from "../ui/Icon";

/**
 * Multi-value tag / chip input. Type a value and press Enter (or comma) to add a
 * chip; each chip carries a `little-delete` X to remove it; Backspace on an empty
 * field removes the last chip. When `suggestions` are supplied, the still-unused
 * matches surface as clickable pills below the field while it is focused.
 *
 * Fully controlled — `values` is the source of truth, `onChange` receives the
 * next array on every add/remove.
 */
export function TagSelect({
  label,
  values,
  onChange,
  suggestions = [],
  placeholder,
}: {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
  suggestions?: string[];
  placeholder?: string;
}) {
  const [draft, setDraft] = useState("");
  const [focused, setFocused] = useState(false);

  function addTag(raw: string) {
    const tag = raw.trim();
    setDraft("");
    if (!tag) return;
    if (values.some((v) => v.toLowerCase() === tag.toLowerCase())) return;
    onChange([...values, tag]);
  }

  function removeTag(tag: string) {
    onChange(values.filter((v) => v !== tag));
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault();
      addTag(draft);
    } else if (e.key === "Backspace" && draft === "") {
      const last = values[values.length - 1];
      if (last !== undefined) removeTag(last);
    }
  }

  const available = suggestions.filter(
    (s) =>
      !values.some((v) => v.toLowerCase() === s.toLowerCase()) &&
      (draft === "" || s.toLowerCase().includes(draft.toLowerCase())),
  );

  return (
    <div>
      <span
        className="mb-1.5 block text-xs font-semibold uppercase tracking-wide"
        style={{ color: "var(--tpl-muted)" }}
      >
        {label}
      </span>
      <div
        className="flex flex-wrap items-center gap-2 rounded-lg border px-2.5 py-2 transition"
        style={{ borderColor: focused ? "var(--tpl-accent)" : "var(--tpl-border)" }}
      >
        {values.map((tag) => (
          <span
            key={tag}
            className="inline-flex items-center gap-1.5 rounded-full px-2.5 py-1 text-xs font-medium"
            style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-text)" }}
          >
            {tag}
            <button
              type="button"
              onClick={() => removeTag(tag)}
              aria-label={`Remove ${tag}`}
              className="inline-flex transition hover:text-[var(--tpl-accent)]"
              style={{ color: "var(--tpl-muted)" }}
            >
              <Icon name="little-delete" size={8} />
            </button>
          </span>
        ))}
        <input
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={onKeyDown}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          placeholder={values.length === 0 ? placeholder : undefined}
          className="min-w-[6rem] flex-1 bg-transparent py-0.5 text-sm outline-none"
          style={{ color: "var(--tpl-text)" }}
        />
      </div>
      {focused && available.length > 0 && (
        <div className="mt-1.5 flex flex-wrap gap-1.5">
          {available.slice(0, 8).map((s) => (
            <button
              key={s}
              type="button"
              // mouseDown (not click) so the pick registers before the input's
              // blur tears the suggestion list down.
              onMouseDown={(e) => {
                e.preventDefault();
                addTag(s);
              }}
              className="rounded-full border px-2.5 py-1 text-xs transition hover:border-[var(--tpl-accent)] hover:text-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              + {s}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
