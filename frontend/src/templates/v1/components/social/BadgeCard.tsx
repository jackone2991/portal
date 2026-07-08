import { Icon } from "../ui/Icon";

/**
 * A community-badge card — React port of the Olympus `.birthday-item.badges`
 * (Community Badges 2563-2579): a badge icon disc with an optional count label,
 * a title + description, and an accent progress meter.
 *
 * `progress` (0-100) is clamped; omit it to hide the meter.
 */
export function BadgeCard({
  icon,
  count,
  title,
  description,
  progress,
}: {
  /** Sprite icon id (without the `olymp-` prefix), e.g. "badge-icon". */
  icon: string;
  count?: number;
  title: string;
  description?: string;
  progress?: number;
}) {
  const pct = progress == null ? null : Math.max(0, Math.min(100, progress));
  return (
    <div
      className="flex items-center gap-4 rounded-xl p-4 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Badge disc */}
      <div className="relative shrink-0">
        <div
          className="grid h-14 w-14 place-items-center rounded-full text-white shadow-sm"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          <Icon name={icon} size={24} />
        </div>
        {count != null && (
          <span
            className="absolute -right-1 -top-1 grid h-5 min-w-5 place-items-center rounded-full px-1 text-[11px] font-bold text-white ring-2 ring-white"
            style={{ background: "var(--tpl-blue)" }}
          >
            {count}
          </span>
        )}
      </div>

      {/* Text + meter */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </p>
        {description && (
          <p className="mt-0.5 text-xs leading-snug" style={{ color: "var(--tpl-muted)" }}>
            {description}
          </p>
        )}
        {pct != null && (
          <div
            className="mt-3 h-2 overflow-hidden rounded-full"
            style={{ background: "var(--tpl-surface-2)" }}
            role="progressbar"
            aria-valuenow={pct}
            aria-valuemin={0}
            aria-valuemax={100}
          >
            <span
              className="block h-full rounded-full"
              style={{
                width: `${pct}%`,
                background: "linear-gradient(90deg, var(--tpl-accent), var(--tpl-accent-2))",
              }}
            />
          </div>
        )}
      </div>
    </div>
  );
}
