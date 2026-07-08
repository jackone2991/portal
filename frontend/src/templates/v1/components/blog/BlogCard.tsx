import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { Card } from "../widget/WidgetCard";

/**
 * Blog grid card — React + Tailwind port of the Olympus `article.hentry.blog-post`
 * (template-main/social/social/"Blog V1 Grid.html", ~2640-2692).
 *
 * Server component (no interactivity): a gradient thumbnail standing in for the
 * post cover (no image assets in v1), an optional coloured category chip overlaid
 * top-left, title + excerpt, an author (Avatar + name) + date byline, and a footer
 * reaction row (heart + likes, speech-balloon + comments). Drop several into a
 * responsive grid to reproduce the blog index.
 */

/** Deterministic two-stop gradient from a seed string (self-contained; no images). */
function seedGradient(seed: string): string {
  const hue = [...seed].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return `linear-gradient(135deg, hsl(${hue} 70% 60%), hsl(${(hue + 45) % 360} 65% 45%))`;
}

export function BlogCard({
  title,
  excerpt,
  author,
  date,
  category,
  categoryColor = "var(--tpl-blue)",
  likes = 0,
  comments = 0,
  cover,
}: {
  title: string;
  excerpt: string;
  author: string;
  date: string;
  /** Optional pill label overlaid on the thumbnail (e.g. "THE COMMUNITY"). */
  category?: string;
  /** CSS colour for the category pill; defaults to the blue token. */
  categoryColor?: string;
  likes?: number;
  comments?: number;
  /** Seed for the deterministic gradient thumbnail; falls back to `title`. */
  cover?: string;
}) {
  return (
    <Card as="article" className="overflow-hidden">
      {/* Gradient cover placeholder with optional category chip */}
      <div
        className="relative aspect-[370/261] w-full"
        style={{ background: seedGradient(cover ?? title) }}
      >
        {category && (
          <span
            className="absolute left-4 top-4 rounded-md px-3 py-1 text-[11px] font-bold uppercase tracking-wide text-white shadow-sm"
            style={{ background: categoryColor }}
          >
            {category}
          </span>
        )}
      </div>

      <div className="p-5">
        <h3 className="text-lg font-bold leading-snug" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </h3>
        <p className="mt-2 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
          {excerpt}
        </p>

        <div className="mt-4 flex items-center gap-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
          <Avatar name={author} size={28} />
          <span>
            by{" "}
            <b style={{ color: "var(--tpl-heading)" }}>{author}</b>
          </span>
          <span aria-hidden>·</span>
          <time>{date}</time>
        </div>

        <footer
          className="mt-4 flex items-center gap-5 border-t pt-3 text-sm"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          <span className="inline-flex items-center gap-1.5" style={{ color: "var(--tpl-muted)" }}>
            <Icon name="heart-icon" size={16} />
            <span className="text-xs">{likes}</span>
          </span>
          <span className="inline-flex items-center gap-1.5" style={{ color: "var(--tpl-muted)" }}>
            <Icon name="speech-balloon-icon" size={16} />
            <span className="text-xs">{comments}</span>
          </span>
        </footer>
      </div>
    </Card>
  );
}
