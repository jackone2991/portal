"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Journal composer — faithful port of the Olympus `.news-feed-form` create-post
 * box (social/social/Newsfeed.html 2542-2660): Status / Multimedia / Blog tabs, an
 * avatar + "Share what you are thinking here…" textarea, and ONE add-options row
 * with the photo / tag / location icons on the left and the Post Status + Preview
 * buttons on the right. Fully controlled/presentational — the caller (HomeView)
 * owns the draft + the TanStack mutation (D-32); this only renders + calls
 * `onSubmit`. Multimedia/Blog tabs and the icon buttons are inert UI chrome
 * (photo attachments are P1.5); only the Status text path is wired. Backdating +
 * mood live on the /calendar note form, not here (matches the design).
 */
const TABS = [
  { key: "status", label: "Status", icon: "status-icon" },
  { key: "media", label: "Multimedia", icon: "multimedia-icon" },
  { key: "blog", label: "Blog Post", icon: "blog-icon" },
];

export interface ComposerProps {
  displayName: string;
  bodyMd: string;
  onBodyMdChange: (value: string) => void;
  onSubmit: () => void;
  submitting?: boolean;
  error?: string | null;
  className?: string;
}

export function Composer({
  displayName,
  bodyMd,
  onBodyMdChange,
  onSubmit,
  submitting = false,
  error = null,
  className = "",
}: ComposerProps) {
  const [preview, setPreview] = useState(false);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!bodyMd.trim() || submitting) return;
    onSubmit();
  }

  return (
    <div
      className={`overflow-hidden rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Nav tabs */}
      <div className="flex border-b" style={{ borderColor: "var(--tpl-border)" }}>
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            className="flex items-center gap-2 px-5 py-3.5 text-sm font-semibold transition"
            style={
              t.key === "status"
                ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                : { color: "var(--tpl-muted)" }
            }
            aria-current={t.key === "status" ? "true" : undefined}
          >
            <Icon name={t.icon} size={16} />
            <span className="hidden sm:inline">{t.label}</span>
          </button>
        ))}
      </div>

      <form onSubmit={submit} className="p-4">
        {error && (
          <p
            role="alert"
            className="mb-3 rounded-lg border px-3 py-2 text-sm"
            style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
          >
            {error}
          </p>
        )}

        <div className="flex gap-3">
          <Avatar name={displayName} size={40} />
          {preview ? (
            <div className="min-h-[3rem] w-full whitespace-pre-wrap pt-2 text-sm" style={{ color: "var(--tpl-text)" }}>
              {bodyMd.trim() ? bodyMd : <span style={{ color: "var(--tpl-muted)" }}>Chưa có nội dung để xem trước.</span>}
            </div>
          ) : (
            <textarea
              value={bodyMd}
              onChange={(e) => onBodyMdChange(e.target.value)}
              rows={2}
              placeholder="Share what you are thinking here..."
              className="min-h-[3rem] w-full resize-none border-0 bg-transparent pt-2 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
              style={{ color: "var(--tpl-text)" }}
            />
          )}
        </div>

        {/* add-options-message: icons left, Post Status + Preview right */}
        <div className="mt-3 flex items-center gap-1 border-t pt-3" style={{ borderColor: "var(--tpl-border)" }}>
          <IconBtn label="Add photos (coming soon)" icon="camera-icon" disabled />
          <IconBtn label="Tag friends (coming soon)" icon="computer-icon" disabled />
          <IconBtn label="Add location (coming soon)" icon="small-pin-icon" disabled />

          <div className="ml-auto flex items-center gap-2">
            <button
              type="submit"
              disabled={!bodyMd.trim() || submitting}
              className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              {submitting ? "Posting…" : "Post Status"}
            </button>
            <button
              type="button"
              onClick={() => setPreview((p) => !p)}
              className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", background: "transparent", color: "var(--tpl-muted)" }}
            >
              {preview ? "Edit" : "Preview"}
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}

function IconBtn({ label, icon, disabled }: { label: string; icon: string; disabled?: boolean }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      disabled={disabled}
      className="grid h-9 w-9 place-items-center rounded-lg transition hover:bg-[var(--tpl-surface-2)] disabled:cursor-not-allowed disabled:opacity-40"
      style={{ color: "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={18} />
    </button>
  );
}
