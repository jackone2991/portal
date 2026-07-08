"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Tabbed create-post box — port of Olympus `.create-post` (Newsfeed.html
 * 2542-2660), generalized from HomeView's inline `Composer`.
 *
 * Tabs: Status / Multimedia / Blog Post. Avatar + textarea, then a footer of
 * icon buttons (photos · tag · location) plus Preview and the accent
 * "Post Status" submit. Submitting a non-empty draft calls `onPost` and clears.
 */
const TABS = [
  { key: "status", label: "Status", icon: "status-icon" },
  { key: "media", label: "Multimedia", icon: "multimedia-icon" },
  { key: "blog", label: "Blog Post", icon: "blog-icon" },
];

export interface ComposerProps {
  displayName: string;
  onPost: (text: string) => void;
  className?: string;
}

export function Composer({ displayName, onPost, className = "" }: ComposerProps) {
  const [tab, setTab] = useState("status");
  const [text, setText] = useState("");

  function submit(e: FormEvent) {
    e.preventDefault();
    const t = text.trim();
    if (!t) return;
    onPost(t);
    setText("");
  }

  return (
    <div
      className={`overflow-hidden rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <div className="flex border-b" style={{ borderColor: "var(--tpl-border)" }}>
        {TABS.map((t) => (
          <button
            key={t.key}
            type="button"
            onClick={() => setTab(t.key)}
            className="flex items-center gap-2 px-5 py-3.5 text-sm font-semibold transition"
            style={
              tab === t.key
                ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                : { color: "var(--tpl-muted)" }
            }
          >
            <Icon name={t.icon} size={16} />
            <span className="hidden sm:inline">{t.label}</span>
          </button>
        ))}
      </div>

      <form onSubmit={submit} className="p-4">
        <div className="flex gap-3">
          <Avatar name={displayName} size={40} />
          <textarea
            value={text}
            onChange={(e) => setText(e.target.value)}
            rows={2}
            placeholder="Share what you are thinking here..."
            className="min-h-[3rem] w-full resize-none border-0 bg-transparent pt-2 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
            style={{ color: "var(--tpl-text)" }}
          />
        </div>

        <div
          className="mt-2 flex items-center gap-1 border-t pt-3"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          <IconBtn label="Add photos" icon="camera-icon" />
          <IconBtn label="Tag friends" icon="computer-icon" />
          <IconBtn label="Add location" icon="small-pin-icon" />

          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
            >
              Preview
            </button>
            <button
              type="submit"
              disabled={!text.trim()}
              className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              Post Status
            </button>
          </div>
        </div>
      </form>
    </div>
  );
}

function IconBtn({ label, icon }: { label: string; icon: string }) {
  return (
    <button
      type="button"
      title={label}
      aria-label={label}
      className="grid h-9 w-9 place-items-center rounded-lg transition hover:bg-[var(--tpl-surface-2)]"
      style={{ color: "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={18} />
    </button>
  );
}
