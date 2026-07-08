import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { ControlBlockButtons } from "./ControlBlockButtons";

const DEFAULT_TABS = ["Timeline", "About", "Friends", "Photos", "Videos"];

/** Deterministic cover gradient derived from the name (no image assets in v1). */
function coverGradient(seed: string) {
  const hue = [...seed].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return `linear-gradient(135deg, hsl(${hue} 70% 60%), hsl(${(hue + 55) % 360} 62% 46%))`;
}

/**
 * Profile page header — React port of the Olympus `.top-header`
 * (Profile Page 2918-3007): a gradient cover strip with an "Update Cover"
 * affordance, an overlapping Avatar, name + location, a horizontal tab menu,
 * and the ControlBlockButtons (add friend / message / settings).
 *
 * `activeTab` / `onTab` are controlled by the parent; leave them unset for a
 * static header.
 */
export function ProfileHeader({
  name,
  location,
  tabs = DEFAULT_TABS,
  activeTab,
  onTab,
  onUpdateCover,
}: {
  name: string;
  location?: string;
  tabs?: string[];
  activeTab?: string;
  onTab?: (tab: string) => void;
  onUpdateCover?: () => void;
}) {
  return (
    <div
      className="overflow-hidden rounded-xl shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Cover strip */}
      <div className="relative h-40 sm:h-52" style={{ background: coverGradient(name) }}>
        <button
          type="button"
          onClick={onUpdateCover}
          className="absolute right-4 top-4 inline-flex items-center gap-2 rounded-md bg-black/30 px-3 py-1.5 text-xs font-semibold text-white backdrop-blur-sm transition hover:bg-black/45"
        >
          <Icon name="camera-icon" size={16} />
          <span>Update Cover</span>
        </button>
      </div>

      {/* Author + controls */}
      <div className="px-5 pb-1 sm:px-6">
        <div className="flex flex-wrap items-end gap-4">
          <Avatar name={name} size={104} className="-mt-14 shrink-0 ring-4 ring-white" />
          <div className="min-w-0 flex-1 pb-1">
            <h1 className="truncate text-xl font-bold" style={{ color: "var(--tpl-heading)" }}>
              {name}
            </h1>
            {location && (
              <p
                className="mt-1 inline-flex items-center gap-1.5 text-sm"
                style={{ color: "var(--tpl-muted)" }}
              >
                <Icon name="small-pin-icon" size={14} />
                {location}
              </p>
            )}
          </div>
          <ControlBlockButtons
            className="pb-1"
            buttons={[
              { icon: "happy-face-icon", label: "Add friend", color: "var(--tpl-blue)" },
              { icon: "speech-balloon-icon", label: "Message", color: "var(--tpl-accent)" },
              { icon: "three-dots-icon", label: "Settings" },
            ]}
          />
        </div>

        {/* Tab menu */}
        <nav
          className="mt-4 flex flex-wrap border-t"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          {tabs.map((tab) => {
            const active = tab === activeTab;
            return (
              <button
                key={tab}
                type="button"
                onClick={() => onTab?.(tab)}
                className="px-4 py-3 text-sm font-semibold transition"
                style={
                  active
                    ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                    : { color: "var(--tpl-muted)" }
                }
              >
                {tab}
              </button>
            );
          })}
        </nav>
      </div>
    </div>
  );
}
