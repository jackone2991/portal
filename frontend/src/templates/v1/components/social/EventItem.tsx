import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/** Overlapping row of attendee avatars, with a `+N` overflow chip. */
function AttendeeStack({ names, max = 5 }: { names: string[]; max?: number }) {
  const shown = names.slice(0, max);
  const extra = names.length - shown.length;
  return (
    <span className="flex items-center">
      {shown.map((n, i) => (
        <span key={n} className="rounded-full ring-2 ring-white" style={{ marginLeft: i ? -8 : 0 }}>
          <Avatar name={n} size={28} />
        </span>
      ))}
      {extra > 0 && (
        <span
          className="grid h-7 w-7 place-items-center rounded-full text-[11px] font-semibold text-white ring-2 ring-white"
          style={{ marginLeft: -8, background: "var(--tpl-blue)" }}
        >
          +{extra}
        </span>
      )}
    </span>
  );
}

/**
 * An upcoming-event row — React port of the Olympus `.event-item`
 * (Favorit Page - Events 2615-2670): an accent date badge, the title, host,
 * place (small-pin-icon), stacked attendee avatars, and a Join button.
 */
export function EventItem({
  day,
  month,
  title,
  host,
  place,
  attendees,
  onJoin,
}: {
  day: number | string;
  month: string;
  title: string;
  host?: string;
  place?: string;
  attendees?: string[];
  onJoin?: () => void;
}) {
  return (
    <div
      className="flex flex-wrap items-center gap-4 rounded-xl p-4 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Date badge */}
      <div
        className="grid h-16 w-16 shrink-0 place-content-center place-items-center rounded-lg text-white"
        style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
      >
        <span className="text-2xl font-bold leading-none">{day}</span>
        <span className="mt-1 text-[11px] font-semibold uppercase tracking-wide">{month}</span>
      </div>

      {/* Details */}
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </p>
        <div
          className="mt-0.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs"
          style={{ color: "var(--tpl-muted)" }}
        >
          {host && <span className="truncate">Hosted by {host}</span>}
          {place && (
            <span className="inline-flex items-center gap-1">
              <Icon name="small-pin-icon" size={12} />
              {place}
            </span>
          )}
        </div>
        {attendees && attendees.length > 0 && (
          <div className="mt-2">
            <AttendeeStack names={attendees} />
          </div>
        )}
      </div>

      <button
        type="button"
        onClick={onJoin}
        className="shrink-0 rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90"
        style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
      >
        Join
      </button>
    </div>
  );
}
