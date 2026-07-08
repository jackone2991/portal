import { Icon } from "../ui/Icon";
import { CommentItem, type CommentItemProps } from "./CommentItem";

/**
 * A comment list — port of Olympus `ul.comments-list` (Post Versions.html
 * 2676-2793). Renders {@link CommentItem}s (each with up to one nested reply
 * level) followed by a "View more comments" affordance.
 */
export interface CommentThreadProps {
  comments: CommentItemProps[];
  className?: string;
}

export function CommentThread({ comments, className = "" }: CommentThreadProps) {
  return (
    <div className={className}>
      <ul className="comments-list space-y-5">
        {comments.map((c, i) => (
          <CommentItem
            key={`${c.author}-${i}`}
            author={c.author}
            time={c.time}
            text={c.text}
            likes={c.likes}
            replies={c.replies}
          />
        ))}
      </ul>

      <a
        href="#"
        className="more-comments mt-4 inline-flex items-center gap-1.5 text-xs font-semibold hover:underline"
        style={{ color: "var(--tpl-muted)" }}
      >
        View more comments
        <Icon name="plus-icon" size={10} />
      </a>
    </div>
  );
}
