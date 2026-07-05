/**
 * Deterministic initials avatar — a gradient disc with the name's initials.
 * Self-contained (no image assets); the colour is derived from the name so the
 * same person is always the same hue.
 */
export function Avatar({
  name,
  size = 40,
  className = "",
}: {
  name: string;
  size?: number;
  className?: string;
}) {
  const initials = name
    .trim()
    .split(/\s+/)
    .map((w) => w[0])
    .slice(0, 2)
    .join("")
    .toUpperCase();
  const hue = [...name].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return (
    <span
      className={`inline-grid shrink-0 place-items-center rounded-full font-semibold text-white ${className}`}
      style={{
        width: size,
        height: size,
        fontSize: size * 0.38,
        background: `linear-gradient(135deg, hsl(${hue} 68% 58%), hsl(${(hue + 40) % 360} 68% 46%))`,
      }}
      aria-hidden
    >
      {initials}
    </span>
  );
}
