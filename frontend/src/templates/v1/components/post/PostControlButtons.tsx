"use client";

import { Icon } from "../ui/Icon";

/**
 * Floating FAB column for a post card — port of Olympus
 * `.control-block-button.post-control-button` (Newsfeed.html 2777-2791), matched
 * to HomeView's inline `QuickFab` styling: filled `var(--tpl-muted)` discs
 * (~34px), white icon, active → `var(--tpl-accent)`.
 *
 * Positioned absolutely so the column sits half-outside the card's right edge
 * (`translate-x-1/2`); render it inside a `relative` parent. Hidden below `sm`.
 * Four actions: award (trophy), like, comment, share.
 */
export interface PostControlButtonsProps {
  liked?: boolean;
  onLike?: () => void;
  onAward?: () => void;
  onComment?: () => void;
  onShare?: () => void;
  className?: string;
}

export function PostControlButtons({
  liked,
  onLike,
  onAward,
  onComment,
  onShare,
  className = "",
}: PostControlButtonsProps) {
  return (
    <div
      className={`absolute right-0 top-6 hidden translate-x-1/2 flex-col gap-2 sm:flex ${className}`}
    >
      <Fab label="Award" icon="trophy-icon" onClick={onAward} />
      <Fab label="Like" icon="like-post-icon" active={liked} onClick={onLike} />
      <Fab label="Comment" icon="comments-post-icon" onClick={onComment} />
      <Fab label="Share" icon="share-icon" onClick={onShare} />
    </div>
  );
}

function Fab({
  label,
  icon,
  active,
  onClick,
}: {
  label: string;
  icon: string;
  active?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      aria-pressed={active}
      className="grid h-[34px] w-[34px] place-items-center rounded-full text-white shadow-sm transition hover:opacity-90"
      style={{ background: active ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={16} />
    </button>
  );
}
