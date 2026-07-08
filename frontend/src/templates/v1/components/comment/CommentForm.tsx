"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Comment composer — port of Olympus `form.comment-form` (Post Versions.html
 * 2799-2845). Avatar + textarea with an inline camera (photo) button, then a
 * "Post Comment" submit and a Cancel that clears the draft. Submitting a
 * non-empty draft calls `onSubmit` and clears the field.
 */
export interface CommentFormProps {
  displayName: string;
  onSubmit: (text: string) => void;
  className?: string;
}

export function CommentForm({ displayName, onSubmit, className = "" }: CommentFormProps) {
  const [text, setText] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    const t = text.trim();
    if (!t) return;
    onSubmit(t);
    setText("");
  }

  return (
    <form onSubmit={submit} className={`flex items-start gap-3 ${className}`}>
      <Avatar name={displayName} size={36} />

      <div className="min-w-0 flex-1">
        <div
          className="relative rounded-md border"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={2}
            placeholder="Add a comment..."
            className="min-h-[2.75rem] w-full resize-none border-0 bg-transparent py-2 pl-3 pr-10 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
            style={{ color: "var(--tpl-text)" }}
          />
          <button
            type="button"
            title="Add photo"
            aria-label="Add photo"
            className="absolute right-2 top-2 grid h-7 w-7 place-items-center rounded-md transition hover:bg-[var(--tpl-surface-2)]"
            style={{ color: "var(--tpl-muted)" }}
          >
            <Icon name="camera-icon" size={16} />
          </button>
        </div>

        <div className="mt-2 flex items-center gap-2">
          <button
            type="submit"
            disabled={!text.trim()}
            className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            Post Comment
          </button>
          <button
            type="button"
            onClick={() => setText("")}
            className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Cancel
          </button>
        </div>
      </div>
    </form>
  );
}
