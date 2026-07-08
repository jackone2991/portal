import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { WidgetCard } from "./WidgetCard";

/**
 * "Pages You May Like" rail widget — extracted from the inline `PagesWidget` in
 * `views/home/HomeView.tsx`. Lists pages (Avatar + name + category) each with a
 * star "like" button. `pages` is optional and defaults to the Olympus sample.
 */

export type SuggestedPage = {
  name: string;
  /** Category / descriptor line, e.g. "Restaurant · Bar". */
  category: string;
};

const DEFAULT_PAGES: SuggestedPage[] = [
  { name: "The Marina Bar", category: "Restaurant · Bar" },
  { name: "Tapronus Rock", category: "Rock Band" },
  { name: "Pixel Digital Design", category: "Company" },
  { name: "Thompson's Custom Clothing Boutique", category: "Clothing Store" },
];

export function PagesWidget({
  pages = DEFAULT_PAGES,
  title = "Pages You May Like",
}: {
  pages?: SuggestedPage[];
  title?: string;
} = {}) {
  return (
    <WidgetCard title={title} more>
      <ul className="space-y-3">
        {pages.map((p) => (
          <li key={p.name} className="flex items-center gap-3">
            <Avatar name={p.name} size={38} />
            <div className="min-w-0">
              <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {p.name}
              </p>
              <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                {p.category}
              </p>
            </div>
            <button
              type="button"
              className="ml-auto text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
              aria-label="Like page"
            >
              <Icon name="star-icon" size={18} />
            </button>
          </li>
        ))}
      </ul>
    </WidgetCard>
  );
}
