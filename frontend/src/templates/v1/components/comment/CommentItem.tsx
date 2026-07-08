import type { ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * A single comment — port of Olympus `.comment-item` (Post Versions.html
 * 2677-2782). Avatar · author · time · options menu, body text, a heart like
 * count and a Reply link. `replies` renders one nested level as a
 * `ul.children` of nested comments.
 */
export interface CommentItemProps {
  author: string;
  time: ReactNode;
  text: ReactNode;
  likes?: number;
  replies?: CommentItemProps[];
}

export function CommentItem({ author, time, text, likes = 0, replies }: CommentItemProps) {
  const hasChildren = !!replies && replies.length > 0;
  return (
    <li className={hasChildren ? "comment-item has-children" : "comment-item"}>
      <div className="flex items-center gap-3">
        <Avatar name={author} size={36} />
        <div className="min-w-0">
          <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
            {author}
          </p>
          <time className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            {time}
          </time>
        </div>
        <button
          type="button"
          className="ml-auto text-[var(--tpl-muted)]"
          aria-label="Comment options"
        >
          <Icon name="three-dots-icon" size={16} />
        </button>
      </div>

      <p className="mt-2 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
        {text}
      </p>

      <div className="mt-2 flex items-center gap-4 text-xs">
        <span
          className="inline-flex items-center gap-1.5"
          style={{ color: "var(--tpl-muted)" }}
        >
          <Icon name="heart-icon" size={14} />
          <span>{likes}</span>
        </span>
        <a
          href="#"
          className="font-medium hover:underline"
          style={{ color: "var(--tpl-accent)" }}
        >
          Reply
        </a>
      </div>

      {hasChildren && (
        <ul className="children mt-4 space-y-4 pl-6">
          {replies!.map((r, i) => (
            <CommentItem
              key={`${r.author}-${i}`}
              author={r.author}
              time={r.time}
              text={r.text}
              likes={r.likes}
              /* one nesting level only — drop any grandchildren */
            />
          ))}
        </ul>
      )}
    </li>
  );
}
