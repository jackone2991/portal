"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import type { Route } from "next";
import { useQuery } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { listPeople, listSuggestions, type Person, type PersonCircle } from "@/lib/people";
import { listConnections } from "@/lib/social";
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
 * The dots are the reference's presence dots, repurposed. Portal has no presence
 * system — nobody is "online" — but it does now know whether you are connected
 * to an account, whether they are waiting on your answer, or whether you are
 * waiting on theirs. That is real, it is the thing you would act on, and it maps
 * onto the same four colours. A dot here never claims someone is at their desk.
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

  const rawSuggestions = (suggestQuery.data ?? []).filter((s) => match(s.display_name));

  // One map from account id to how you stand with them. The three query keys are
  // the same ones the header's request menu uses, so TanStack serves both from
  // one fetch rather than doubling the traffic on every page.
  const accepted = useQuery({ queryKey: ["connections", "accepted"], queryFn: () => listConnections("accepted"), staleTime: 60_000 });
  const incoming = useQuery({ queryKey: ["connections", "incoming"], queryFn: () => listConnections("incoming"), staleTime: 60_000 });
  const outgoing = useQuery({ queryKey: ["connections", "outgoing"], queryFn: () => listConnections("outgoing"), staleTime: 60_000 });

  const linkState = useMemo(() => {
    const m = new Map<string, LinkState>();
    for (const c of outgoing.data ?? []) m.set(c.user_id, "asked");
    for (const c of incoming.data ?? []) m.set(c.user_id, "asking");
    for (const c of accepted.data ?? []) m.set(c.user_id, "connected");
    return m;
  }, [accepted.data, incoming.data, outgoing.data]);

  /**
   * The third section is the catch-all — Olympus called it "Uncategorized" —
   * so it holds everyone the first two do not: the people you are connected to
   * but have not filed into a circle, the requests pending either way, and then
   * the accounts you have no link with at all.
   *
   * Before this it held suggestions only, and suggestions are by definition the
   * accounts with NO relationship. Anyone you connected to was subtracted from
   * it and appeared nowhere else, so a connection made the person vanish from
   * the rail and the status dot had nothing to attach to.
   */
  const filed = useMemo(() => {
    const ids = new Set<string>();
    for (const p of [...circles.close_friend, ...circles.family]) {
      if (p.linked_user_id) ids.add(p.linked_user_id);
    }
    return ids;
  }, [circles]);

  const others = useMemo(() => {
    const seen = new Set<string>();
    const out: RailEntry[] = [];
    const push = (userID: string, name: string, state?: LinkState) => {
      if (!userID || seen.has(userID) || filed.has(userID) || !match(name)) return;
      seen.add(userID);
      out.push({ userID, name, state });
    };
    // Order is the order you would act in: settled, then asking you, then
    // waiting on them, then strangers.
    for (const c of accepted.data ?? []) push(c.user_id, c.display_name ?? "Unknown", "connected");
    for (const c of incoming.data ?? []) push(c.user_id, c.display_name ?? "Unknown", "asking");
    for (const c of outgoing.data ?? []) push(c.user_id, c.display_name ?? "Unknown", "asked");
    for (const sug of rawSuggestions) push(sug.user_id, sug.display_name);
    return out;
    // eslint-disable-next-line react-hooks/exhaustive-deps -- `match` is derived from q
  }, [accepted.data, incoming.data, outgoing.data, rawSuggestions, filed, q]);

  const hasAnything =
    circles.close_friend.length > 0 || circles.family.length > 0 || others.length > 0;

  // The collapsed rail is the same three sections flattened into one strip: a
  // narrower view of the panel, not a different list. It used to show only
  // registry people, so an empty registry left a blank column even when there
  // were accounts to meet.
  const railPeople = [...circles.close_friend, ...circles.family];

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
              {railPeople.map((p) => (
                <Link
                  key={p.id}
                  href={`/people/${p.id}` as Route}
                  title={`${p.display_name}${stateLabel(linkState.get(p.linked_user_id ?? ""))}`}
                  aria-label={p.display_name}
                  className="relative"
                >
                  <PersonAvatar person={p} size={40} />
                  <StatusDot state={linkState.get(p.linked_user_id ?? "")} />
                </Link>
              ))}
              {others.map((o) => (
                <Link
                  key={o.userID}
                  href={othersHref(o.state)}
                  title={`${o.name}${stateLabel(o.state)}`}
                  aria-label={o.name}
                  className="relative"
                >
                  <Avatar name={o.name} size={40} />
                  <StatusDot state={o.state} />
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
                    <PersonRow key={p.id} person={p} state={linkState.get(p.linked_user_id ?? "")} />
                  ))}
                </Section>

                <Section
                  title="My Family"
                  manageHref="/people?circle=family"
                  empty="No one in this circle yet."
                >
                  {circles.family.map((p) => (
                    <PersonRow key={p.id} person={p} state={linkState.get(p.linked_user_id ?? "")} />
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
                  {others.map((o) => (
                    <li key={o.userID}>
                      <Link
                        href={othersHref(o.state)}
                        title={`${o.name}${stateLabel(o.state)}`}
                        className="flex items-center gap-3 px-4 py-2 transition hover:bg-[var(--tpl-surface-2)]"
                      >
                        <span className="relative shrink-0">
                          <Avatar name={o.name} size={38} />
                          <StatusDot state={o.state} />
                        </span>
                        <p
                          className="min-w-0 flex-1 truncate text-sm font-semibold"
                          style={{ color: "var(--tpl-heading)" }}
                        >
                          {o.name}
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

function PersonRow({ person, state }: { person: Person; state?: LinkState }) {
  return (
    <li>
      <Link
        href={`/people/${person.id}` as Route}
        className="flex items-center gap-3 px-4 py-2 transition hover:bg-[var(--tpl-surface-2)]"
        title={`${person.display_name}${stateLabel(state)}`}
      >
        <span className="relative shrink-0">
          <PersonAvatar person={person} size={38} />
          <StatusDot state={state} />
        </span>
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

/** One row of the catch-all section: an account, and how you stand with it. */
interface RailEntry {
  userID: string;
  name: string;
  state?: LinkState;
}

/**
 * How you stand with an account. Undefined means no relationship at all, which
 * draws no dot — an absent dot says "nothing between you yet", which is exactly
 * what a suggestion is.
 */
type LinkState = "connected" | "asking" | "asked";

/** A pending row belongs on the requests tab; everything else on suggestions. */
function othersHref(state?: LinkState): Route {
  return (state === "asking" || state === "asked"
    ? "/people?circle=requests"
    : "/people?circle=suggestions") as Route;
}

const STATE: Record<LinkState, { color: string; label: string }> = {
  // Teal reads as "settled" in this palette, and a connection is the settled state.
  connected: { color: "var(--tpl-status-online)", label: "connected" },
  // Accent, because this one is the only state that needs you to do something.
  asking: { color: "var(--tpl-accent)", label: "wants to connect" },
  // Amber for waiting on them.
  asked: { color: "var(--tpl-status-away)", label: "request sent" },
};

function StatusDot({ state }: { state?: LinkState }) {
  if (!state) return null;
  const s = STATE[state];
  return (
    <span
      className="absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full border-2 border-white"
      style={{ background: s.color }}
      aria-hidden
    />
  );
}

/** Tooltip suffix, so the dot's meaning is never left to colour alone. */
function stateLabel(state?: LinkState): string {
  return state ? ` — ${STATE[state].label}` : "";
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
