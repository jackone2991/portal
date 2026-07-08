"use client";

/**
 * Settings row — port of the Olympus `.description-toggle` + `.togglebutton`
 * pairing. Title (and optional description) on the left, an on/off switch on the
 * right that fills with the accent when on. The switch is a proper ARIA
 * `role="switch"` button so it stays keyboard- and screen-reader-friendly.
 */
export function ToggleRow({
  title,
  description,
  checked,
  onChange,
}: {
  title: string;
  description?: string;
  checked: boolean;
  onChange: (value: boolean) => void;
}) {
  return (
    <div className="flex items-center justify-between gap-4 py-3">
      <div className="min-w-0">
        <div className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </div>
        {description && (
          <p className="mt-0.5 text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
            {description}
          </p>
        )}
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={title}
        onClick={() => onChange(!checked)}
        className="relative inline-flex h-6 w-11 shrink-0 cursor-pointer items-center rounded-full transition-colors duration-200"
        style={{
          background: checked ? "var(--tpl-accent)" : "var(--tpl-surface-2)",
          border: `1px solid ${checked ? "var(--tpl-accent)" : "var(--tpl-border)"}`,
        }}
      >
        <span
          className="inline-block h-4 w-4 rounded-full bg-white shadow transition-transform duration-200 ease-out"
          style={{ transform: checked ? "translateX(1.5rem)" : "translateX(0.25rem)" }}
        />
      </button>
    </div>
  );
}
