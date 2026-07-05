import type { CSSProperties } from "react";
import { OLYMP_VIEWBOX } from "../footers/SvgSprite";

/**
 * Renders an Olympus sprite icon by `<use>`-referencing the symbol injected by
 * <SvgSprite/>. `name` is the id without the `olymp-` prefix (e.g. "heart-icon").
 * Height = `size`; width is derived from the symbol's native viewBox so the icon
 * keeps its aspect ratio. Colour follows `currentColor`.
 */
export function Icon({
  name,
  size = 20,
  className,
  style,
}: {
  name: string;
  size?: number;
  className?: string;
  style?: CSSProperties;
}) {
  const vb = OLYMP_VIEWBOX[name] ?? "0 0 32 32";
  const parts = vb.split(/\s+/).map(Number);
  const w = parts[2] || 1;
  const h = parts[3] || 1;
  const width = Math.round((w / h) * size);
  return (
    <svg
      width={width}
      height={size}
      viewBox={vb}
      fill="currentColor"
      aria-hidden
      className={className}
      style={style}
    >
      <use href={`#olymp-${name}`} />
    </svg>
  );
}
