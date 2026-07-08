import { Icon } from "../ui/Icon";
import { OLYMP_VIEWBOX } from "../footers/SvgSprite";

export type ControlButton = {
  /** Sprite icon id (without the `olymp-` prefix), e.g. "happy-face-icon". */
  icon: string;
  /** Accessible label / tooltip (also used as the React key). */
  label: string;
  /** Disc background; defaults to Olympus' muted grey (`bg-primary`). */
  color?: string;
  onClick?: () => void;
};

/**
 * A row of circular action buttons — the Olympus `.control-block-button`
 * (Profile Page 2965-2995): coloured discs each showing one sprite icon.
 * Reused by ProfileHeader, FriendCard and FriendRequestItem.
 *
 * The per-button icon is clamped to fit its disc so wide sprites (notably
 * `three-dots-icon`, a 128×32 glyph) don't overflow the circle.
 */
export function ControlBlockButtons({
  buttons,
  size = 40,
  className = "",
}: {
  buttons: ControlButton[];
  /** Diameter of each disc in px. */
  size?: number;
  className?: string;
}) {
  const defaultIcon = Math.round(size * 0.45);
  const inner = size * 0.52; // usable diameter inside the disc padding

  return (
    <div className={`flex items-center gap-2 ${className}`}>
      {buttons.map((b, i) => {
        const vb = OLYMP_VIEWBOX[b.icon] ?? "0 0 32 32";
        const parts = vb.split(/\s+/).map(Number);
        const aspect = (parts[2] || 1) / (parts[3] || 1);
        const iconSize = Math.min(defaultIcon, Math.round(inner / Math.max(1, aspect)));
        return (
          <button
            key={`${b.label}-${i}`}
            type="button"
            onClick={b.onClick}
            title={b.label}
            aria-label={b.label}
            className="grid shrink-0 place-items-center overflow-hidden rounded-full text-white shadow-sm transition hover:opacity-90"
            style={{ width: size, height: size, background: b.color ?? "var(--tpl-muted)" }}
          >
            <Icon name={b.icon} size={iconSize} />
          </button>
        );
      })}
    </div>
  );
}
