"use client";

import { Icon } from "../ui/Icon";

/**
 * Floating FAB column for a post card — port of Olympus
 * `.control-block-button.post-control-button` (Newsfeed.html 2777-2791): filled
 * `var(--tpl-muted)` discs (~34px), white icon, active → `var(--tpl-accent)`.
 *
 * Positioned absolutely so the column sits half-outside the card's right edge
 * (`translate-x-1/2`); render it inside a `relative` parent. Hidden below `sm`.
 *
 * The newsfeed reference shows three actions — like / comment / share. The
 * award (trophy) disc only appears when `onAward` is given, which is the
 * profile-page variant. A disc with no handler renders as design chrome: there
 * is no social layer yet, so it is disabled and says so on hover rather than
 * pretending to be a button that does nothing.
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
      {onAward && <Fab label="Award" icon="trophy-icon" onClick={onAward} />}
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
  const inert = !onClick;
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={inert}
      aria-label={label}
      aria-pressed={active}
      title={inert ? `${label} — arrives with the social layer` : label}
      className="grid h-[34px] w-[34px] place-items-center rounded-full text-white shadow-sm transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-60"
      style={{ background: active ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={16} />
    </button>
  );
}
