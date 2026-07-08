import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";

/**
 * "Friend Suggestions" rail widget — extracted from the inline
 * `FriendSuggestions` in `views/home/HomeView.tsx`. Lists people (Avatar + name
 * + meta) each with an "add" button on `var(--tpl-blue)`. `people` is optional
 * and defaults to the Olympus sample list.
 */

export type SuggestedPerson = {
  name: string;
  /** Secondary line, e.g. "8 Friends in Common". */
  meta: string;
};

const DEFAULT_PEOPLE: SuggestedPerson[] = [
  { name: "Francine Smith", meta: "8 Friends in Common" },
  { name: "Hugh Wilson", meta: "6 Friends in Common" },
  { name: "Karen Masters", meta: "6 Friends in Common" },
];

export function FriendSuggestions({
  people = DEFAULT_PEOPLE,
  title = "Friend Suggestions",
}: {
  people?: SuggestedPerson[];
  title?: string;
} = {}) {
  return (
    <WidgetCard title={title} more>
      <ul className="space-y-3">
        {people.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={40} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.meta}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto grid h-8 w-8 place-items-center rounded-md text-white transition hover:opacity-90"
              style={{ background: "var(--tpl-blue)" }}
              aria-label={`Add ${p.name}`}
            >
              <Icon name="happy-face-icon" size={16} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}
