import type { ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { ReactionBar } from "./ReactionBar";
import { PostControlButtons } from "./PostControlButtons";

/**
 * Full post card — port of Olympus `.ui-block .hentry.post` (Newsfeed.html
 * 2666-2793; media variants from Post Versions.html), built on HomeView's inline
 * `PostCard`. Header (avatar · author · optional action · time · options menu),
 * body text, optional media slot, then the {@link ReactionBar} footer and the
 * half-outside {@link PostControlButtons} FAB column.
 *
 * Presentational: interactivity lives in the client-side FAB column, so this
 * stays render-only and forwards `liked` / `onToggleLike` down to it.
 */
export interface PostMedia {
  type: "video" | "photo";
  title?: ReactNode;
  desc?: ReactNode;
  source?: ReactNode;
}

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
        <button
          type="button"
          className="ml-auto pr-8 text-[var(--tpl-muted)]"
          aria-label="Post options"
        >
          <Icon name="three-dots-icon" size={18} />
        </button>
      </header>

      <p className="mt-3 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
        {text}
      </p>

      {media && <MediaCard media={media} />}

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
  if (media.type === "photo") return <PhotoCard media={media} />;
  return <VideoCard media={media} />;
}

/** Video link card — mirrors HomeView's inline `VideoCard`. */
function VideoCard({ media }: { media: PostMedia }) {
  return (
    <div
      className="mt-3 flex gap-4 overflow-hidden rounded-lg border"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <div
        className="relative grid h-40 w-44 shrink-0 place-items-center"
        style={{ background: "linear-gradient(135deg,#c78a5b,#7c5240)" }}
      >
        <span
          className="grid h-14 w-14 place-items-center rounded-full text-white shadow"
          style={{ background: "var(--tpl-accent)" }}
        >
          <Icon name="play-icon" size={20} />
        </span>
      </div>
      <div className="min-w-0 py-4 pr-4">
        {media.title && (
          <p className="text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
            {media.title}
          </p>
        )}
        {media.desc && (
          <p className="mt-1 text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
            {media.desc}
          </p>
        )}
        {media.source && (
          <p
            className="mt-3 text-[11px] font-semibold tracking-wide"
            style={{ color: "var(--tpl-muted)" }}
          >
            {media.source}
          </p>
        )}
      </div>
    </div>
  );
}

/** Photo card — self-contained gradient thumbnail placeholder (no image assets). */
function PhotoCard({ media }: { media: PostMedia }) {
  return (
    <div
      className="mt-3 overflow-hidden rounded-lg border"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <div
        className="grid h-64 place-items-center"
        style={{ background: "linear-gradient(135deg,#6d4bb8,#8a63d2)" }}
      >
        <span className="text-white/70">
          <Icon name="photos-icon" size={40} />
        </span>
      </div>
      {(media.title || media.desc) && (
        <div className="p-4">
          {media.title && (
            <p className="text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
              {media.title}
            </p>
          )}
          {media.desc && (
            <p className="mt-1 text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
              {media.desc}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
