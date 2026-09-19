"use client";

import type { ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { ReactionBar } from "./ReactionBar";
import { PostControlButtons } from "./PostControlButtons";
import { AttachmentGallery } from "../journal/AttachmentGallery";

/**
 * Full post card — port of Olympus `.ui-block .hentry.post` (Newsfeed.html
 * 2666-2793; media variants from Post Versions.html). Header (avatar · author ·
 * optional action · time · options menu), body text, optional media slot — a
 * link/video preview card, or the Entry's photos via {@link AttachmentGallery}
 * — then the {@link ReactionBar} footer and the half-outside
 * {@link PostControlButtons} FAB column.
 *
 * Presentational: interactivity lives in the client-side FAB column and in the
 * `menu` slot the caller passes, so this stays render-only.
 */
export interface PostLinkMedia {
  /** `video` adds the play overlay; `link` is the same card without it. */
  type: "video" | "link";
  title?: ReactNode;
  desc?: ReactNode;
  /** The Olympus `.link-site` line — a bare host, rendered uppercase. */
  source?: ReactNode;
  /** Makes the whole card a link out. */
  href?: string;
  /**
   * Seed for the thumbnail's gradient. A shared link has no crawler and no
   * og:image behind it, so its thumbnail is a deterministic placeholder — same
   * host, same colours — rather than a fake screenshot.
   */
  seed?: string;
}

export interface PostPhotoMedia {
  type: "photos";
  /** The Entry's Attachments in display order — drawn by {@link AttachmentGallery}. */
  assetIds: string[];
}

export type PostMedia = PostLinkMedia | PostPhotoMedia;

export interface PostProps {
  author: string;
  time: ReactNode;
  action?: ReactNode;
  text: ReactNode;
  media?: PostMedia;
  likes: number;
  likedBy: string[];
  likedByLabel?: ReactNode;
  comments: number;
  shares: number;
  liked?: boolean;
  onToggleLike?: () => void;
  /** Attached location — rendered as a pin chip under the body. */
  location?: { name: string; href: string };
  /** Replaces the inert three-dots button (e.g. a real Edit/Delete dropdown). */
  menu?: ReactNode;
  className?: string;
}

export function Post({
  author,
  time,
  action,
  text,
  media,
  likes,
  likedBy,
  likedByLabel,
  comments,
  shares,
  liked,
  onToggleLike,
  location,
  menu,
  className = "",
}: PostProps) {
  return (
    <article
      className={`relative rounded-xl p-5 shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <PostControlButtons liked={liked} onLike={onToggleLike} />

      <header className="flex items-center gap-3">
        <Avatar name={author} size={40} />
        <div className="min-w-0">
          <p className="truncate text-sm">
            <span className="font-semibold" style={{ color: "var(--tpl-heading)" }}>
              {author}
            </span>{" "}
            {action && <span style={{ color: "var(--tpl-muted)" }}>{action}</span>}
          </p>
          <time className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            {time}
          </time>
        </div>
        <div className="ml-auto pr-8">
          {menu ?? (
            <button
              type="button"
              className="text-[var(--tpl-muted)]"
              aria-label="Post options"
            >
              <Icon name="three-dots-icon" size={6} />
            </button>
          )}
        </div>
      </header>

      {text && (
        <div className="mt-3 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
          {text}
        </div>
      )}

      {media && <MediaCard media={media} />}

      {location && (
        <a
          href={location.href}
          target="_blank"
          rel="noreferrer noopener"
          className="mt-3 inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium transition hover:opacity-90"
          style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-text)" }}
        >
          <span style={{ color: "var(--tpl-accent)" }}>
            <Icon name="small-pin-icon" size={12} />
          </span>
          <span className="max-w-[18rem] truncate">{location.name}</span>
        </a>
      )}

      <ReactionBar
        likes={likes}
        likedBy={likedBy}
        likedByLabel={likedByLabel}
        comments={comments}
        shares={shares}
        liked={liked}
      />
    </article>
  );
}

/* ── Media variants ───────────────────────────────────────────────── */

function MediaCard({ media }: { media: PostMedia }) {
  if (media.type === "photos") return <AttachmentGallery assetIds={media.assetIds} />;
  return <LinkCard media={media} />;
}

/**
 * Link / video card — Olympus `.post-video`: square thumb on the left, title +
 * excerpt + source host on the right. `type: "video"` overlays the play button.
 */
function LinkCard({ media }: { media: PostLinkMedia }) {
  const inner = (
    <>
      <div
        className="relative grid h-40 w-44 shrink-0 place-items-center"
        style={{ background: gradientOf(media.seed ?? String(media.title ?? "")) }}
      >
        {media.type === "video" ? (
          <span
            className="grid h-14 w-14 place-items-center rounded-full text-white shadow"
            style={{ background: "var(--tpl-accent)" }}
          >
            <Icon name="play-icon" size={20} />
          </span>
        ) : (
          <span className="text-white/70">
            <Icon name="albums-icon" size={34} />
          </span>
        )}
      </div>
      <div className="min-w-0 py-4 pr-4">
        {media.title && (
          <p
            className="truncate text-base font-semibold"
            style={{ color: "var(--tpl-heading)" }}
          >
            {media.title}
          </p>
        )}
        {media.desc && (
          <p
            className="mt-1 line-clamp-3 text-xs leading-relaxed break-words"
            style={{ color: "var(--tpl-muted)" }}
          >
            {media.desc}
          </p>
        )}
        {media.source && (
          <p
            className="mt-3 text-[11px] font-semibold uppercase tracking-wide"
            style={{ color: "var(--tpl-muted)" }}
          >
            {media.source}
          </p>
        )}
      </div>
    </>
  );

  const cls = "mt-3 flex gap-4 overflow-hidden rounded-lg border transition";
  const style = { borderColor: "var(--tpl-border)" };

  return media.href ? (
    <a
      href={media.href}
      target="_blank"
      rel="noreferrer noopener"
      className={`${cls} hover:shadow-sm`}
      style={style}
    >
      {inner}
    </a>
  ) : (
    <div className={cls} style={style}>
      {inner}
    </div>
  );
}

/** Same seed → same thumbnail, so one host keeps one colour across the feed. */
function gradientOf(seed: string): string {
  const hue = [...seed].reduce((a, c) => a + c.charCodeAt(0), 0) % 360;
  return `linear-gradient(135deg, hsl(${hue} 45% 52%), hsl(${(hue + 35) % 360} 45% 38%))`;
}
