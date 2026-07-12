"use client";

import { Avatar } from "../ui/Avatar";
import { renderMarkdown } from "@/lib/markdown";
import type { JournalEntry } from "@/lib/journal";

/**
 * A single journal entry card for the interim home list (SPEC-05 P0.4).
 *
 * Deliberately not the social `Post`/`ReactionBar` kit — SPEC-05 §3 non-goals
 * rule out comments/reactions/sharing for entries (single user, flat rows).
 * Body renders through {@link renderMarkdown}, a renderer that never touches
 * `dangerouslySetInnerHTML` — raw HTML/script in `body_md` always comes out
 * as inert text. Edit/delete are presentational triggers only; the confirm
 * dialog and the optimistic mutations live in `HomeView` (D-32).
 */
export function EntryCard({
  displayName,
  entry,
  onRequestEdit,
  onRequestDelete,
}: {
  displayName: string;
  entry: JournalEntry;
  onRequestEdit: () => void;
  onRequestDelete: () => void;
}) {
  const edited =
    new Date(entry.updated_at).getTime() - new Date(entry.created_at).getTime() > 60_000;

  return (
    <article
      className="rounded-xl p-5 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <header className="flex items-center gap-3">
        <Avatar name={displayName} size={40} />
        <div className="min-w-0 flex-1">
          <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
            {displayName}
          </p>
          <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            <time dateTime={entry.occurred_at}>{formatOccurredAt(entry.occurred_at)}</time>
            {edited && <span> · edited</span>}
          </p>
        </div>
        {entry.mood && <MoodPill mood={entry.mood} />}
      </header>

      <div className="mt-3 text-sm leading-relaxed" style={{ color: "var(--tpl-text)" }}>
        {renderMarkdown(entry.body_md)}
      </div>

      <footer className="mt-4 flex items-center gap-4 border-t pt-3" style={{ borderColor: "var(--tpl-border)" }}>
        <button
          type="button"
          onClick={onRequestEdit}
          className="text-xs font-semibold uppercase tracking-wide transition hover:text-[var(--tpl-accent)]"
          style={{ color: "var(--tpl-muted)" }}
        >
          Edit
        </button>
        <button
          type="button"
          onClick={onRequestDelete}
          className="text-xs font-semibold uppercase tracking-wide transition hover:text-[#ef4444]"
          style={{ color: "var(--tpl-muted)" }}
        >
          Delete
        </button>
      </footer>
    </article>
  );
}

function MoodPill({ mood }: { mood: string }) {
  return (
    <span
      className="shrink-0 rounded-full px-2.5 py-1 text-xs font-medium"
      style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}
    >
      {mood}
    </span>
  );
}

function formatOccurredAt(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
}
