"use client";

import type { FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Journal composer — port of Olympus `.create-post` (Newsfeed.html 2542-2660),
 * rewired for SPEC-05 P0.4. Originally a decorative Status/Multimedia/Blog
 * post box with a local `onPost(text)` callback and no backend; now the real
 * write path for a journal entry: body text (markdown-in-textarea), an
 * optional freeform mood, and an optional `occurred_at` date/time control for
 * backdating (the "last night" user story — SPEC-05 §4).
 *
 * Fully controlled and presentational: the caller (`HomeView`) owns the draft
 * state and the TanStack mutation (D-32) — this component only renders inputs
 * and calls `onSubmit` on submit. That split is what lets the caller clear the
 * draft optimistically on submit and restore it if the mutation errors (SPEC-05
 * P0.4 acceptance criteria).
 *
 * The Multimedia / Blog Post tabs stay as inert UI chrome — photo attachments
 * are P1.5 (needs SPEC-01) and there is no separate "blog" entry type; only
 * the Status tab is wired.
 */
const TABS = [
  { key: "status", label: "Status", icon: "status-icon" },
  { key: "media", label: "Multimedia", icon: "multimedia-icon" },
  { key: "blog", label: "Blog Post", icon: "blog-icon" },
];

export interface ComposerProps {
  displayName: string;
  bodyMd: string;
  mood: string;
  /** `datetime-local` input value (e.g. `2026-07-12T09:30`) — conversion to ISO is the caller's job. */
  occurredAt: string;
  onBodyMdChange: (value: string) => void;
  onMoodChange: (value: string) => void;
  onOccurredAtChange: (value: string) => void;
  onSubmit: () => void;
  submitting?: boolean;
  error?: string | null;
  className?: string;
}

export function Composer({
  displayName,
  bodyMd,
  mood,
  occurredAt,
  onBodyMdChange,
  onMoodChange,
  onOccurredAtChange,
  onSubmit,
  submitting = false,
  error = null,
  className = "",
}: ComposerProps) {
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
          <textarea
            value={bodyMd}
            onChange={(e) => onBodyMdChange(e.target.value)}
            rows={2}
            placeholder="What happened? (markdown supported)"
            className="min-h-[3rem] w-full resize-none border-0 bg-transparent pt-2 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
            style={{ color: "var(--tpl-text)" }}
          />
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-3 border-t pt-3" style={{ borderColor: "var(--tpl-border)" }}>
          <label className="flex min-w-0 flex-1 items-center gap-2 sm:flex-none sm:basis-48">
            <Icon name="happy-face-icon" size={16} style={{ color: "var(--tpl-muted)" }} />
            <input
              type="text"
              value={mood}
              onChange={(e) => onMoodChange(e.target.value)}
              maxLength={80}
              placeholder="Mood (optional)"
              aria-label="Mood"
              className="w-full min-w-0 rounded-md border bg-transparent px-2.5 py-1.5 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
          </label>

          <label className="flex min-w-0 flex-1 items-center gap-2 sm:flex-none sm:basis-56">
            <Icon name="checked-calendar-icon" size={16} style={{ color: "var(--tpl-muted)" }} />
            <input
              type="datetime-local"
              value={occurredAt}
              onChange={(e) => onOccurredAtChange(e.target.value)}
              aria-label="Happened at"
              className="w-full min-w-0 rounded-md border bg-transparent px-2.5 py-1.5 text-sm outline-none transition focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
          </label>
        </div>

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
              {submitting ? "Saving…" : "Save entry"}
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
