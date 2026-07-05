"use client";

import { useState, type ReactNode } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";

/**
 * Header notification dropdowns — port of the Olympus header menus:
 * Friend Requests, Messages, Notifications. Each owns its list state, so the
 * badge count and the mark-read / accept / decline actions are live. Open state
 * is controlled by the parent (TopMenu) so only one menu shows at a time.
 */

/* ── shared pieces ─────────────────────────────────────────────── */

function Trigger({
  icon,
  tone,
  badge,
  label,
  open,
  onToggle,
}: {
  icon: string;
  tone: string;
  badge: number;
  label: string;
  open: boolean;
  onToggle: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-label={label}
      aria-expanded={open}
      className={`relative grid h-9 w-9 place-items-center rounded-full transition ${
        open ? "bg-white/15 text-white" : "text-white/75 hover:bg-white/10 hover:text-white"
      }`}
    >
      <Icon name={icon} size={18} />
      {badge > 0 && (
        <span
          className="absolute -right-0.5 -top-0.5 grid h-4 min-w-4 place-items-center rounded-full px-1 text-[10px] font-bold text-white"
          style={{ background: tone }}
        >
          {badge}
        </span>
      )}
    </button>
  );
}

function DropdownCard({
  title,
  actions,
  footer,
  children,
}: {
  title: string;
  actions: ReactNode;
  footer: string;
  children: ReactNode;
}) {
  return (
    <div
      role="menu"
      className="absolute right-0 top-full z-50 mt-3 w-[21rem] overflow-hidden rounded-xl border bg-white shadow-2xl"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
    >
      <span
        className="absolute -top-1.5 right-4 h-3 w-3 rotate-45 border-l border-t bg-white"
        style={{ borderColor: "var(--tpl-border)" }}
      />
      <div className="flex items-center justify-between border-b px-4 py-2.5" style={{ borderColor: "var(--tpl-border)" }}>
        <span className="text-[11px] font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
          {title}
        </span>
        <span className="flex gap-3">{actions}</span>
      </div>
      <ul className="max-h-[22rem] overflow-y-auto">{children}</ul>
      <a
        href="#"
        className="block py-3 text-center text-sm font-semibold text-white transition hover:opacity-95"
        style={{ background: "var(--tpl-blue)" }}
      >
        {footer}
      </a>
    </div>
  );
}

function ActionLink({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="text-[11px] font-bold uppercase tracking-wide transition hover:text-[var(--tpl-accent)]"
      style={{ color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}

function Empty({ children }: { children: ReactNode }) {
  return (
    <li className="px-4 py-8 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </li>
  );
}

function InlineLink({ children, tone = "var(--tpl-accent)" }: { children: ReactNode; tone?: string }) {
  return (
    <a href="#" className="font-semibold hover:underline" style={{ color: tone }}>
      {children}
    </a>
  );
}

const rowBorder = { borderColor: "var(--tpl-border)" };

/* ── Friend Requests ───────────────────────────────────────────── */

type Req = { id: number; name: string; sub: string; info?: boolean };

const FRIEND_REQUESTS: Req[] = [
  { id: 1, name: "Tamara Romanoff", sub: "Mutual Friend: Sarah Hetfield" },
  { id: 2, name: "Tony Stevens", sub: "4 Friends in Common" },
  { id: 3, name: "Green Goo", sub: "8 Friends in Common" },
  { id: 4, name: "Mary Jane Stark", sub: "", info: true },
];

export function FriendRequestsMenu({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const [items, setItems] = useState(FRIEND_REQUESTS);
  const remove = (id: number) => setItems((x) => x.filter((i) => i.id !== id));

  return (
    <div className="relative">
      <Trigger icon="happy-face-icon" tone="var(--tpl-blue-2)" badge={items.length} label="Friend requests" open={open} onToggle={onToggle} />
      {open && (
        <DropdownCard
          title="Friend Requests"
          actions={
            <>
              <ActionLink>Settings</ActionLink>
              <ActionLink>Find Friends</ActionLink>
            </>
          }
          footer="Check all your Events"
        >
          {items.length === 0 && <Empty>No new friend requests</Empty>}
          {items.map((r) =>
            r.info ? (
              <li key={r.id} className="flex items-center gap-3 border-b px-4 py-3" style={rowBorder}>
                <Avatar name={r.name} size={40} />
                <p className="min-w-0 flex-1 text-sm" style={{ color: "var(--tpl-text)" }}>
                  You and <b style={{ color: "var(--tpl-heading)" }}>{r.name}</b> just became friends. Write on{" "}
                  <InlineLink tone="var(--tpl-blue)">her wall</InlineLink>.
                </p>
                <IconBtn onClick={() => remove(r.id)} label="Dismiss" icon="happy-face-icon" />
              </li>
            ) : (
              <li key={r.id} className="flex items-center gap-3 border-b px-4 py-3" style={rowBorder}>
                <Avatar name={r.name} size={40} />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                    {r.name}
                  </p>
                  <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                    {r.sub}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => remove(r.id)}
                  className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-white transition hover:opacity-90"
                  style={{ background: "var(--tpl-blue)" }}
                  aria-label={`Accept ${r.name}`}
                >
                  <Icon name="happy-face-icon" size={16} />
                </button>
                <button
                  type="button"
                  onClick={() => remove(r.id)}
                  className="grid h-8 w-8 shrink-0 place-items-center rounded-md transition hover:bg-black/5"
                  style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}
                  aria-label={`Decline ${r.name}`}
                >
                  <Icon name="little-delete" size={12} />
                </button>
              </li>
            ),
          )}
        </DropdownCard>
      )}
    </div>
  );
}

function IconBtn({ onClick, label, icon }: { onClick: () => void; label: string; icon: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="grid h-8 w-8 shrink-0 place-items-center rounded-md transition hover:bg-black/5"
      style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}
      aria-label={label}
    >
      <Icon name={icon} size={16} />
    </button>
  );
}

/* ── Messages ──────────────────────────────────────────────────── */

type Msg = { id: number; name: string; preview: string; time: string; read?: boolean };

const MESSAGES: Msg[] = [
  { id: 1, name: "Elaine Dreyfuss", preview: "Hi James! I just wanted to let you know we have to reschedule the meeting…", time: "Yesterday at 9:56pm" },
  { id: 2, name: "Sarah Hetfield", preview: "Hey! Would you like to hang out this weekend?", time: "March 16th at 10:23am" },
];

export function MessagesMenu({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const [msgs, setMsgs] = useState(MESSAGES);
  const unread = msgs.filter((m) => !m.read).length;
  const markAll = () => setMsgs((x) => x.map((m) => ({ ...m, read: true })));
  const readOne = (id: number) => setMsgs((x) => x.map((m) => (m.id === id ? { ...m, read: true } : m)));

  return (
    <div className="relative">
      <Trigger icon="chat---messages-icon" tone="#7c5ac2" badge={unread} label="Messages" open={open} onToggle={onToggle} />
      {open && (
        <DropdownCard
          title="Messages"
          actions={
            <>
              <ActionLink onClick={markAll}>Mark as read</ActionLink>
              <ActionLink>Settings</ActionLink>
            </>
          }
          footer="View All Messages"
        >
          {msgs.map((m) => (
            <li
              key={m.id}
              onClick={() => readOne(m.id)}
              className="flex cursor-pointer gap-3 border-b px-4 py-3 transition hover:bg-[var(--tpl-surface-2)]"
              style={{ ...rowBorder, background: m.read ? undefined : "rgba(56,169,255,0.06)" }}
            >
              <span className="relative shrink-0">
                <Avatar name={m.name} size={42} />
                {!m.read && (
                  <span className="absolute right-0 top-0 h-3 w-3 rounded-full border-2 border-white" style={{ background: "var(--tpl-blue)" }} />
                )}
              </span>
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                  {m.name}
                </p>
                <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                  {m.preview}
                </p>
                <p className="mt-0.5 text-[11px]" style={{ color: "var(--tpl-muted)" }}>
                  {m.time}
                </p>
              </div>
            </li>
          ))}
        </DropdownCard>
      )}
    </div>
  );
}

/* ── Notifications ─────────────────────────────────────────────── */

type Notif = { id: number; who: string; action: ReactNode; time: string; icon: string; read?: boolean };

const NOTIFS: Notif[] = [
  { id: 1, who: "Mathilda Brinker", action: <>commented on your <InlineLink>photo</InlineLink>.</>, time: "2 mins ago", icon: "comments-post-icon" },
  { id: 2, who: "Nicholas Grissom", action: <>liked your <InlineLink>status update</InlineLink>.</>, time: "5 mins ago", icon: "like-post-icon" },
  { id: 3, who: "Sarah Hetfield", action: <>started following you.</>, time: "12 mins ago", icon: "happy-face-icon" },
  { id: 4, who: "Jake Parker", action: <>shared your <InlineLink>post</InlineLink>.</>, time: "1 hour ago", icon: "share-post-icon" },
];

export function NotificationsMenu({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const [items, setItems] = useState(NOTIFS);
  const unread = items.filter((n) => !n.read).length;
  const markAll = () => setItems((x) => x.map((n) => ({ ...n, read: true })));

  return (
    <div className="relative">
      <Trigger icon="thunder-icon" tone="var(--tpl-accent)" badge={unread} label="Notifications" open={open} onToggle={onToggle} />
      {open && (
        <DropdownCard
          title="Notifications"
          actions={
            <>
              <ActionLink onClick={markAll}>Mark as read</ActionLink>
              <ActionLink>Settings</ActionLink>
            </>
          }
          footer="Check all your Notifications"
        >
          {items.map((n) => (
            <li
              key={n.id}
              className="flex items-start gap-3 border-b px-4 py-3"
              style={{ ...rowBorder, background: n.read ? undefined : "rgba(255,94,58,0.05)" }}
            >
              <Avatar name={n.who} size={38} />
              <div className="min-w-0 flex-1">
                <p className="text-sm leading-snug" style={{ color: "var(--tpl-text)" }}>
                  <b style={{ color: "var(--tpl-heading)" }}>{n.who}</b> {n.action}
                </p>
                <p className="mt-0.5 text-[11px]" style={{ color: "var(--tpl-muted)" }}>
                  {n.time}
                </p>
              </div>
              <span className="mt-1 shrink-0" style={{ color: "var(--tpl-muted)" }}>
                <Icon name={n.icon} size={16} />
              </span>
            </li>
          ))}
        </DropdownCard>
      )}
    </div>
  );
}
