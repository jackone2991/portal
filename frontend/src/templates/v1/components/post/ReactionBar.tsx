import type { ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Post footer reaction row — port of Olympus `.post-additional-info`
 * (Newsfeed.html 2720-2772), matched to HomeView's inline footer.
 *
 * Left: heart + like count · overlapping liker avatars · "X liked this" label.
 * Right: comment count · share count. Pass `liked` to tint the heart with the
 * accent. When `likedByLabel` is omitted a default is derived from `likedBy`.
 */
export interface ReactionBarProps {
  likes: number;
  likedBy: string[];
  likedByLabel?: ReactNode;
  comments: number;
  shares: number;
  liked?: boolean;
  className?: string;
}

export function ReactionBar({
  likes,
  likedBy,
  likedByLabel,
  comments,
  shares,
  liked,
  className = "",
}: ReactionBarProps) {
  return (
    <footer className={`mt-4 flex items-center gap-3 text-sm ${className}`}>
      <span
        className="inline-flex items-center gap-2"
        style={{ color: liked ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
      >
        <Icon name="heart-icon" size={16} />
        <span>{likes}</span>
      </span>

      {likedBy.length > 0 && <StackedAvatars names={likedBy} />}

      <span className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
        {likedByLabel ?? <DefaultLabel likedBy={likedBy} likes={likes} />}
      </span>

      <span
        className="ml-auto inline-flex items-center gap-4"
        style={{ color: "var(--tpl-muted)" }}
      >
        <span className="inline-flex items-center gap-1.5">
          <Icon name="speech-balloon-icon" size={16} />
          <span className="text-xs">{comments}</span>
        </span>
        <span className="inline-flex items-center gap-1.5">
          <Icon name="share-icon" size={16} />
          <span className="text-xs">{shares}</span>
        </span>
      </span>
    </footer>
  );
}

/** Overlapping liker avatars — Avatar size 22, -8px overlap, white ring. */
function StackedAvatars({ names }: { names: string[] }) {
  return (
    <span className="flex">
      {names.map((n, i) => (
        <span
          key={n}
          className="rounded-full ring-2 ring-white"
          style={{ marginLeft: i ? -8 : 0 }}
        >
          <Avatar name={n} size={22} />
        </span>
      ))}
    </span>
  );
}

function DefaultLabel({ likedBy, likes }: { likedBy: string[]; likes: number }) {
  const lead = likedBy[0];
  if (!lead) {
    return <span>Be the first to like this</span>;
  }
  const first = lead.split(/\s+/)[0];
  const more = Math.max(0, likes - 1);
  return (
    <>
      <b>{first}</b>
      {more > 0 ? (
        <>
          {" "}
          and <b>{more} more</b>
        </>
      ) : null}{" "}
      liked this
    </>
  );
}
