"use client";

import type { ReactNode } from "react";
import Link from "next/link";
import type { Route } from "next";
import type { StreamItem } from "@/lib/stream";
import { firstLink, splitLinks, stripLink } from "@/lib/links";
import { presentEntry } from "@/lib/entry-presentation";
import { locationLabel, osmURL } from "@/lib/geo";
import { Icon } from "../ui/Icon";
import { Post } from "../post/Post";
import { PostOptionsMenu, type PostMenuItem } from "../post/PostOptionsMenu";
import { formatDate, useTimeConfig } from "@/lib/time";

/**
 * One life-stream card (SPEC-06 P0.2).
 *
 * A journal item renders as a full Olympus post card. If its body contains a
 * URL the card takes the reference's "shared a link" shape: the header says so,
 * the URL is lifted out of the paragraph into the link/video preview card, and
 * the rest of the text is rendered with any remaining URLs anchored
 * (`lib/links.ts`). That is the whole "post type" mechanism — a journal entry is
 * markdown, and the feed reads the shape back out of it.
 *
 * System items (media/bank/comic/people) stay compact: they are events, not
 * posts, so they get the same card surface but no author, no reaction bar and no
 * FAB column — the feed shouldn't imply a person wrote them.
 *
 * Reaction counts are 0 and the FABs are inert: there is no social layer yet, so
 * the bar is the design's chrome, not fake engagement. Edit/Delete in the
 * options menu ARE real — they hit the journal entry behind `ref_id`. Editing
 * happens in place: the caller hands in the composer as `editor` and it is
 * rendered where the card was (SPEC-12 T5) — this card knows nothing about
 * how an Entry is edited, only that it is not being shown meanwhile.
 */
export interface StreamItemCardProps {
  item: StreamItem;
  displayName?: string;
  /** When set, rendered in place of the card — the edit surface. */
  editor?: ReactNode;
  onStartEdit?: (item: StreamItem) => void;
  onDelete?: (item: StreamItem) => void;
}

export function StreamItemCard({ item, displayName, editor, onStartEdit, onDelete }: StreamItemCardProps) {
  const { data: tc } = useTimeConfig();
  const when = formatDate(item.occurred_at, tc?.timezone ?? "UTC");

  if (editor) return <>{editor}</>;

  if (item.source_module === "journal") {
    return (
      <JournalPost item={item} when={when} displayName={displayName} onStartEdit={onStartEdit} onDelete={onDelete} />
    );
  }

  return <SystemCard item={item} when={when} />;
}

/* ── Journal post ─────────────────────────────────────────────────── */

function JournalPost({
  item,
  when,
  displayName,
  onStartEdit,
  onDelete,
}: Pick<StreamItemCardProps, "item" | "displayName" | "onStartEdit" | "onDelete"> & { when: string }) {
  // One place decides what the Entry shows (`entry-presentation.ts`); the
  // shared-link detection below then looks only at what the author typed.
  const shown = presentEntry(item);
  const hasPhotos = shown.assetIds.length > 0;
  // Attached photos own the media slot; a URL in that post stays anchored in
  // the paragraph instead of becoming a second card.
  const link = hasPhotos ? null : firstLink(shown.text);
  const stripped = link ? stripLink(shown.text, link.url) : shown.text;
  const { heading, rest } = splitHeading(stripped);

  // A temp id from the optimistic insert isn't a real entry yet — no menu until
  // the server hands back the row, or Edit/Delete would 404.
  const persisted = !item.id.startsWith("temp-");
  const menuItems: PostMenuItem[] = [];
  if (persisted && onStartEdit) {
    menuItems.push({ label: "Edit Post", onSelect: () => onStartEdit(item) });
  }
  if (persisted && onDelete) {
    menuItems.push({ label: "Delete Post", danger: true, onSelect: () => onDelete(item) });
  }

  return (
    <Post
      author={displayName ?? "You"}
      time={item.mood ? `${when} · ${item.mood}` : when}
      action={link ? <SharedA kind={link.kind} /> : undefined}
      text={
        heading || rest.trim() ? (
          // Guard here, not inside RichText: an element that renders null is
          // still truthy, so Post would draw the empty body spacer on a
          // link-only post.
          <>
            {heading && (
              <p className="mb-2 text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {heading}
              </p>
            )}
            <RichText text={rest} />
          </>
        ) : null
      }
      location={
        shown.location
          ? {
              name: locationLabel(shown.location),
              href: osmURL(shown.location.lat, shown.location.lon),
            }
          : undefined
      }
      media={
        hasPhotos
          ? { type: "photos", assetIds: shown.assetIds }
          : link
          ? {
              type: link.kind,
              // Host as the title and the bare URL as the excerpt: without a
              // crawler those are the only two true things we have, and
              // repeating the host in the `.link-site` line would just pad the
              // card, so `source` stays empty.
              title: link.host,
              desc: link.url.replace(/^https?:\/\//, ""),
              href: link.url,
              seed: link.host,
            }
          : undefined
      }
      menu={menuItems.length > 0 ? <PostOptionsMenu items={menuItems} /> : undefined}
      likes={0}
      likedBy={[]}
      comments={0}
      shares={0}
    />
  );
}

/** Olympus header line: "shared a link" with the noun as an accent anchor. */
function SharedA({ kind }: { kind: "video" | "link" }) {
  return (
    <>
      shared a{" "}
      <span style={{ color: "var(--tpl-accent)" }}>{kind === "video" ? "video" : "link"}</span>
    </>
  );
}

/**
 * Lift a leading ATX heading out of the body. `body_md` is markdown but the
 * feed renders it as text — there is no markdown renderer in the bundle — so
 * the composer's Blog Post tab would otherwise show a literal "# ". This
 * handles exactly what that tab writes: one heading on the first line. Deeper
 * markdown still renders verbatim, which is the honest fallback.
 */
function splitHeading(body: string): { heading: string | null; rest: string } {
  const [first = "", ...others] = body.split("\n");
  const m = /^#{1,3}\s+(.+)$/.exec(first.trim());
  if (!m?.[1]) return { heading: null, rest: body };
  return { heading: m[1].trim(), rest: others.join("\n").trim() };
}

/** Post body: newlines preserved, any surviving URL anchored in the accent. */
function RichText({ text }: { text: string }) {
  if (!text.trim()) return null;
  return (
    <p className="whitespace-pre-wrap">
      {splitLinks(text).map((part, i) =>
        part.href ? (
          <a
            key={i}
            href={part.href}
            target="_blank"
            rel="noreferrer noopener"
            className="hover:underline"
            style={{ color: "var(--tpl-accent)" }}
          >
            {part.text}
          </a>
        ) : (
          <span key={i}>{part.text}</span>
        ),
      )}
    </p>
  );
}

/* ── System event ─────────────────────────────────────────────────── */

function SystemCard({ item, when }: { item: StreamItem; when: string }) {
  const body = (
    <div
      className="flex items-center gap-3 rounded-xl p-4 shadow-sm"
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      <span
        className="grid h-10 w-10 shrink-0 place-items-center rounded-full"
        style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}
      >
        <Icon name={iconFor(item.source_module)} size={18} />
      </span>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" style={{ color: "var(--tpl-heading)" }}>
          {item.title || item.event_type}
        </p>
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          {when}
        </p>
      </div>
      {item.href && (
        <span style={{ color: "var(--tpl-muted)" }}>
          <Icon name="dropdown-arrow-icon" size={12} className="-rotate-90" />
        </span>
      )}
    </div>
  );

  return item.href ? (
    <Link href={item.href as Route} className="block transition hover:opacity-90">
      {body}
    </Link>
  ) : (
    body
  );
}

function iconFor(module: string): string {
  switch (module) {
    case "media":
      return "multimedia-icon";
    case "bank":
      return "stats-icon";
    case "comic":
      return "star-icon";
    case "people":
      return "cupcake-icon";
    default:
      return "newsfeed-icon";
  }
}
