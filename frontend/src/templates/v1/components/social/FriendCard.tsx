import { Avatar } from "../ui/Avatar";
import { ControlBlockButtons } from "../profile/ControlBlockButtons";

type Stats = { friends: number; photos: number; videos: number };

/** Deterministic cover gradient derived from the name (no image assets in v1). */
function coverGradient(seed: string) {
  const hue = [...seed].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return `linear-gradient(135deg, hsl(${hue} 70% 60%), hsl(${(hue + 55) % 360} 62% 46%))`;
}

/**
 * A friend card — React port of the Olympus `.friend-item`
 * (Profile Page - Friends): a gradient cover strip, an overlapping Avatar,
 * name + country, a Friends/Photos/Videos stats row, and the add / message
 * ControlBlockButtons.
 */
export function FriendCard({
  name,
  country,
  stats,
}: {
  name: string;
  country?: string;
  stats?: Stats;
}) {
  return (
    <div
      className="overflow-hidden rounded-xl shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <div className="h-24" style={{ background: coverGradient(name) }} />

      <div className="px-4 pb-4">
        <div className="flex items-end gap-3">
          <Avatar name={name} size={72} className="-mt-9 shrink-0 ring-4 ring-white" />
          <div className="min-w-0 flex-1 pb-1">
            <p className="truncate text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
              {name}
            </p>
            {country && (
              <p className="truncate text-sm" style={{ color: "var(--tpl-muted)" }}>
                {country}
              </p>
            )}
          </div>
        </div>

        {stats && (
          <div className="mt-4 grid grid-cols-3 text-center">
            {(
              [
                ["Friends", stats.friends],
                ["Photos", stats.photos],
                ["Videos", stats.videos],
              ] as const
            ).map(([label, count]) => (
              <div key={label}>
                <div className="text-base font-bold" style={{ color: "var(--tpl-heading)" }}>
                  {count}
                </div>
                <div className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                  {label}
                </div>
              </div>
            ))}
          </div>
        )}

        <ControlBlockButtons
          className="mt-4 justify-center"
          size={36}
          buttons={[
            { icon: "happy-face-icon", label: "Add friend", color: "var(--tpl-blue)" },
            { icon: "speech-balloon-icon", label: "Message", color: "var(--tpl-accent)" },
          ]}
        />
      </div>
    </div>
  );
}
