"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { listPeople, listSuggestions, type Person, type PersonCircle } from "@/lib/people";
import { assetVariantURL } from "@/lib/media-assets";

/**
 * Right fixed sidebar — port of `components/menu/sidebarRight.blade.php`
 * (Olympus), rebuilt on the people registry (SPEC-08).
 *
 * The reference is a friends list with presence dots and a chat launcher. Portal
 * has neither a social graph nor presence nor chat, so the port used to ship
 * eleven invented people with invented ONLINE/AWAY states. What Portal does have
 * is the people registry, so that is what the rail lists — in three fixed
 * sections:
 *
 *   Close Friends / My Family   the two circles (migration 0035)
 *   Có thể bạn biết             accounts on this instance not yet in the
 *                               registry, so an empty registry still has
 *                               something to offer instead of a blank rail
 *
 * Each heading carries a Settings link to the page that manages that section.
 *
 * Everything the data cannot support is gone rather than faked: no status dots,
 * no per-group "Settings", no row menu, no chat bar. What replaces the chat bar
 * is a link to the page that can actually add someone.
 *
 * Failure-isolated: this renders in the shell of every authenticated page, so a
 * failing or empty query collapses to a quiet empty state, never to a broken
 * layout.
 */
export function SidebarRight({
  collapsed,
  onToggle,
}: {
  collapsed: boolean;
  onToggle: () => void;
}) {
  const [q, setQ] = useState("");

  const query = useQuery({
    queryKey: ["people", "rail"],
    queryFn: () => listPeople(),
    staleTime: 60_000, // the rail is on every page; don't refetch on each nav
  });

  const people = useMemo(() => query.data?.people ?? [], [query.data]);

  // Suggestions are only worth fetching when the rail is open, and they are the
  // section that makes an empty registry useful rather than blank.
  const suggestQuery = useQuery({
    queryKey: ["people", "suggestions"],
    queryFn: listSuggestions,
    enabled: !collapsed,
    staleTime: 300_000,
  });

  const needle = q.trim().toLowerCase();
  const match = (name: string) => !needle || name.toLowerCase().includes(needle);

  const circles = useMemo(() => {
    const pick = (c: PersonCircle) => people.filter((p) => p.circle === c && match(p.display_name));
    return { close_friend: pick("close_friend"), family: pick("family") };
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `match` is derived from q
  }, [people, q]);

  const suggestions = (suggestQuery.data ?? []).filter((s) => match(s.display_name));
  const hasAnything =
    circles.close_friend.length > 0 || circles.family.length > 0 || suggestions.length > 0;

  return (
    <aside
      className="fixed right-0 z-30 hidden w-[var(--tpl-rightbar-cur)] flex-col border-l bg-white transition-[width] duration-200 xl:flex"
      style={{
        top: "var(--tpl-header-h)",
        height: "calc(100vh - var(--tpl-header-h))",
        borderColor: "var(--tpl-border)",
      }}
    >
      {collapsed ? (
        /* ── collapsed: avatar rail ── */
        <>
          <div className="flex-1 overflow-y-auto py-4">
            <div className="flex flex-col items-center gap-2.5">
              {people.map((p) => (
                <Link
                  key={p.id}
                  href={`/people/${p.id}` as Route}
                  title={p.display_name}
                  aria-label={p.display_name}
                >
                  <PersonAvatar person={p} size={40} />
                </Link>
              ))}
            </div>
          </div>
          <button
            type="button"
            onClick={onToggle}
            className="grid h-11 place-items-center border-t text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
            style={{ borderColor: "var(--tpl-border)" }}
            aria-label="Expand people panel"
          >
            <Icon name="popup-left-arrow" size={14} />
          </button>
        </>
      ) : (
        /* ── expanded: grouped lists ── */
        <>
          <div className="flex-1 overflow-y-auto">
            {query.isPending ? (
              <p className="px-4 py-6 text-sm" style={{ color: "var(--tpl-muted)" }}>
                Loading people…
              </p>
            ) : query.isError ? (
              <p className="px-4 py-6 text-sm" style={{ color: "var(--tpl-muted)" }}>
                Couldn&apos;t load your people.
              </p>
            ) : (
              <>
                <Section
                  title="Close Friends"
                  manageHref="/people?circle=close_friend"
                  empty="No one in this circle yet."
                >
                  {circles.close_friend.map((p) => (
                    <PersonRow key={p.id} person={p} />
                  ))}
                </Section>

                <Section
                  title="My Family"
                  manageHref="/people?circle=family"
                  empty="No one in this circle yet."
                >
                  {circles.family.map((p) => (
                    <PersonRow key={p.id} person={p} />
                  ))}
                </Section>

                <Section
                  title="Có thể bạn biết"
                  manageHref="/people?circle=suggestions"
                  empty={
                    suggestQuery.isPending
                      ? "Looking for people…"
                      : "No other accounts on this Portal yet."
                  }
                >
                  {suggestions.map((sug) => (
                    <li key={sug.user_id}>
                      <Link
                        href={"/people?circle=suggestions" as Route}
                        className="flex items-center gap-3 px-4 py-2 transition hover:bg-[var(--tpl-surface-2)]"
                      >
                        <Avatar name={sug.display_name} size={38} />
                        <p
                          className="min-w-0 flex-1 truncate text-sm font-semibold"
                          style={{ color: "var(--tpl-heading)" }}
                        >
                          {sug.display_name}
                        </p>
                      </Link>
                    </li>
                  ))}
                </Section>

                {!hasAnything && needle !== "" && (
                  <p className="px-4 py-6 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
                    No one matches “{q}”.
                  </p>
                )}
              </>
            )}
          </div>

          {/* search + collapse */}
          <div
            className="flex items-center gap-2 border-t px-3 py-3"
            style={{ borderColor: "var(--tpl-border)" }}
          >
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search people..."
              className="min-w-0 flex-1 rounded-md border bg-transparent px-3 py-1.5 text-sm outline-none focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
            <button
              type="button"
              onClick={onToggle}
              className="text-[var(--tpl-muted)] hover:text-[var(--tpl-accent)]"
              aria-label="Collapse people panel"
            >
              <Icon name="close-icon" size={16} />
            </button>
          </div>

          {/* Where the Olympus chat launcher was: the one action this panel can
              actually perform. */}
          <Link
            href={"/people" as Route}
            className="flex items-center justify-between px-4 py-3.5 text-sm font-bold uppercase tracking-wide text-white transition hover:opacity-95"
            style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
          >
            <span>People</span>
            <Icon name="happy-faces-icon" size={20} />
          </Link>
        </>
      )}
    </aside>
  );
}

/**
 * One rail section: heading, a Settings link to the page that manages exactly
 * this section, and either its rows or a one-line reason there are none. The
 * empty line matters — a section that vanishes when empty makes the rail look
 * broken rather than new.
 */
function Section({
  title,
  manageHref,
  empty,
  children,
}: {
  title: string;
  manageHref: string;
  empty: string;
  children: React.ReactNode[];
}) {
  return (
    <div>
      <div className="flex items-center justify-between px-4 pb-1 pt-4">
        <span
          className="text-[11px] font-bold uppercase tracking-wide"
          style={{ color: "var(--tpl-accent)" }}
        >
          {title}
        </span>
        <Link
          href={manageHref as Route}
          className="text-[11px] font-semibold uppercase tracking-wide transition hover:text-[var(--tpl-accent)]"
          style={{ color: "var(--tpl-muted)" }}
        >
          Settings
        </Link>
      </div>
      {children.length === 0 ? (
        <p className="px-4 py-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
          {empty}
        </p>
      ) : (
        <ul>{children}</ul>
      )}
    </div>
  );
}

function PersonRow({ person }: { person: Person }) {
  return (
    <li>
      <Link
        href={`/people/${person.id}` as Route}
        className="flex items-center gap-3 px-4 py-2 transition hover:bg-[var(--tpl-surface-2)]"
      >
        <PersonAvatar person={person} size={38} />
        <p
          className="min-w-0 flex-1 truncate text-sm font-semibold"
          style={{ color: "var(--tpl-heading)" }}
        >
          {person.display_name}
        </p>
      </Link>
    </li>
  );
}

/**
 * A person's avatar: their uploaded image when they have one, initials
 * otherwise. The variant URL can 404 (a deleted asset, or one that never
 * finished processing), so a failure falls back to initials rather than leaving
 * a broken image in the shell of every page.
 */
function PersonAvatar({ person, size }: { person: Person; size: number }) {
  const [failed, setFailed] = useState(false);

  if (person.avatar_asset_id && !failed) {
    return (
      /* eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset */
      <img
        src={assetVariantURL(person.avatar_asset_id, "thumb")}
        alt={person.display_name}
        onError={() => setFailed(true)}
        className="shrink-0 rounded-full object-cover"
        style={{ width: size, height: size }}
      />
    );
  }
  return <Avatar name={person.display_name} size={size} />;
}
