"use client";

import { useMemo, useState } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Right fixed sidebar — port of `components/menu/sidebarRight.blade.php` (Olympus):
 * grouped friend lists with presence status, a live "Search Friends" filter, a
 * settings/collapse footer, and the "Olympus Chat" launcher. Collapses to a thin
 * avatar rail (width driven by --tpl-rightbar-cur so the content reflows).
 */

type Status = "online" | "work" | "away" | "offline" | "invisible";

const STATUS: Record<Status, { label: string; varName: string }> = {
  online: { label: "Online", varName: "--tpl-status-online" },
  work: { label: "At work!", varName: "--tpl-status-work" },
  away: { label: "Away", varName: "--tpl-status-away" },
  offline: { label: "Offline", varName: "--tpl-status-offline" },
  invisible: { label: "Invisible", varName: "--tpl-status-invisible" },
};

type Friend = { name: string; status: Status };
type Group = { title: string; friends: Friend[] };

const GROUPS: Group[] = [
  {
    title: "Close Friends",
    friends: [
      { name: "Carol Summers", status: "online" },
      { name: "Mathilda Brinker", status: "work" },
      { name: "Michael Maximoff", status: "away" },
      { name: "Rachel Howlett", status: "offline" },
      { name: "Nina Kraviz", status: "online" },
    ],
  },
  {
    title: "My Family",
    friends: [{ name: "Sarah Hetfield", status: "online" }],
  },
  {
    title: "Uncategorized",
    friends: [
      { name: "Bruce Peterson", status: "online" },
      { name: "Chris Greyson", status: "away" },
      { name: "Nicholas Grisom", status: "invisible" },
      { name: "James Spiegel", status: "away" },
      { name: "Diana Jones", status: "online" },
    ],
  },
];

export function SidebarRight({
  collapsed,
  onToggle,
}: {
  collapsed: boolean;
  onToggle: () => void;
}) {
  const [q, setQ] = useState("");

  const groups = useMemo(() => {
    const needle = q.trim().toLowerCase();
    if (!needle) return GROUPS;
    return GROUPS.map((g) => ({
      ...g,
      friends: g.friends.filter((f) => f.name.toLowerCase().includes(needle)),
    })).filter((g) => g.friends.length > 0);
  }, [q]);

  const allFriends = useMemo(() => GROUPS.flatMap((g) => g.friends), []);

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
              {allFriends.map((f, i) => (
                <button key={`${f.name}-${i}`} type="button" title={f.name} className="relative" aria-label={f.name}>
                  <Avatar name={f.name} size={40} />
                  <StatusDot status={f.status} ring />
                </button>
              ))}
            </div>
          </div>
          <button
            type="button"
            onClick={onToggle}
            className="grid h-11 place-items-center border-t text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
            style={{ borderColor: "var(--tpl-border)" }}
            aria-label="Expand friends panel"
          >
            <Icon name="popup-left-arrow" size={14} />
          </button>
        </>
      ) : (
        /* ── expanded: grouped lists ── */
        <>
          <div className="flex-1 overflow-y-auto">
            {groups.map((g) => (
              <div key={g.title}>
                <div
                  className="flex items-center justify-between px-4 pb-1 pt-4 text-[11px] font-bold uppercase tracking-wide"
                  style={{ color: "var(--tpl-accent)" }}
                >
                  <span>{g.title}</span>
                  <button type="button" className="text-[var(--tpl-muted)] hover:text-[var(--tpl-heading)]">
                    Settings
                  </button>
                </div>
                <ul>
                  {g.friends.map((f, i) => (
                    <li
                      key={`${f.name}-${i}`}
                      className="group flex items-center gap-3 px-4 py-2 transition hover:bg-[var(--tpl-surface-2)]"
                    >
                      <span className="relative shrink-0">
                        <Avatar name={f.name} size={38} />
                        <StatusDot status={f.status} ring />
                      </span>
                      <div className="min-w-0 flex-1">
                        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                          {f.name}
                        </p>
                        <p className="truncate text-[10px] font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
                          {STATUS[f.status].label}
                        </p>
                      </div>
                      <button
                        type="button"
                        className="text-[var(--tpl-muted)] opacity-0 transition group-hover:opacity-100"
                        aria-label="Friend options"
                      >
                        <Icon name="three-dots-icon" size={16} />
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
            {groups.length === 0 && (
              <p className="px-4 py-6 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
                No friends match “{q}”.
              </p>
            )}
          </div>

          {/* search + actions */}
          <div className="flex items-center gap-2 border-t px-3 py-3" style={{ borderColor: "var(--tpl-border)" }}>
            <input
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search Friends..."
              className="min-w-0 flex-1 rounded-md border bg-transparent px-3 py-1.5 text-sm outline-none focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
            />
            <button type="button" className="text-[var(--tpl-muted)] hover:text-[var(--tpl-accent)]" aria-label="Friend settings">
              <Icon name="settings-v2-icon" size={18} />
            </button>
            <button
              type="button"
              onClick={onToggle}
              className="text-[var(--tpl-muted)] hover:text-[var(--tpl-accent)]"
              aria-label="Collapse friends panel"
            >
              <Icon name="close-icon" size={16} />
            </button>
          </div>

          {/* Olympus chat bar */}
          <button
            type="button"
            className="flex items-center justify-between px-4 py-3.5 text-sm font-bold uppercase tracking-wide text-white transition hover:opacity-95"
            style={{ background: "#7c5ac2" }}
          >
            <span>Olympus Chat</span>
            <Icon name="chat---messages-icon" size={20} />
          </button>
        </>
      )}
    </aside>
  );
}

function StatusDot({ status, ring }: { status: Status; ring?: boolean }) {
  return (
    <span
      className={`absolute bottom-0 right-0 h-2.5 w-2.5 rounded-full ${ring ? "border-2 border-white" : ""}`}
      style={{ background: `var(${STATUS[status].varName})` }}
    />
  );
}
