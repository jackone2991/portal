"use client";

/**
 * The round icon that identifies a category everywhere in the ledger — list
 * rows, the donut legend, the category picker, budget bars.
 *
 * A ledger is a list of near-identical rows that a person scans dozens of times
 * a day, and the icon is what makes a row findable without reading it. Kept as
 * one component so the chip is the same size, tint and fallback in all four
 * places; when it was inline markup they drifted.
 *
 * `icon` and `color` are both optional on the wire (see migration 0042), so the
 * fallback is not an error path — a user-made category with neither still has to
 * look deliberate. It gets the first letter of its name on a neutral chip.
 */
export function CategoryChip({
  icon,
  color,
  name,
  size = 40,
  className,
}: {
  icon?: string | null;
  color?: string | null;
  name?: string | null;
  size?: number;
  className?: string;
}) {
  const tint = color ?? "#64748b";
  return (
    <span
      aria-hidden
      className={`grid shrink-0 place-items-center rounded-full ${className ?? ""}`}
      style={{
        width: size,
        height: size,
        // A flat 18% wash of the category colour: strong enough to identify,
        // never so strong that the emoji or letter stops being legible on it.
        background: `color-mix(in srgb, ${tint} 18%, transparent)`,
        color: tint,
        fontSize: Math.round(size * 0.5),
        lineHeight: 1,
      }}
    >
      {icon ? (
        <span style={{ fontSize: Math.round(size * 0.5) }}>{icon}</span>
      ) : (
        <span style={{ fontSize: Math.round(size * 0.42), fontWeight: 700 }}>
          {(name ?? "?").trim().charAt(0).toUpperCase()}
        </span>
      )}
    </span>
  );
}

/** The same tint rule as the chip, for legend dots and bar fills. */
export function categoryTint(color?: string | null): string {
  return color ?? "#64748b";
}
