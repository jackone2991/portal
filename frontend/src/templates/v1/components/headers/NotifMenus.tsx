"use client";

import { useState, type ReactNode } from "react";
import { useRouter } from "next/navigation";
import type { Route } from "next";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import {
  listNotifications,
  markAllRead,
  markRead,
  watermarkCursor,
  type ListNotificationsPage,
  type NotificationItem,
} from "@/lib/notifications";

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
/* SPEC-04 P0.5 — real data via TanStack Query (D-32). `FriendRequestsMenu` /
 * `MessagesMenu` above keep their fixtures on purpose (§3 non-goals); this is
 * the only menu whose backend exists. */

/** Query key for the bell's `{items, unread_count}` cache entry. */
const NOTIFICATIONS_KEY = ["notifications"] as const;

/** Coarse "N mins/hours/days ago" — no locale/date layer exists yet in this template. */
function timeAgo(iso: string): string {
  const minutes = Math.floor((Date.now() - new Date(iso).getTime()) / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes} min${minutes === 1 ? "" : "s"} ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} hour${hours === 1 ? "" : "s"} ago`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? "" : "s"} ago`;
}

export function NotificationsMenu({ open, onToggle }: { open: boolean; onToggle: () => void }) {
  const router = useRouter();
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: NOTIFICATIONS_KEY,
    queryFn: () => listNotifications(),
    staleTime: 0, // personal counter — always refetch-worthy (SPEC-04 P0.5)
    refetchInterval: 60_000, // P0 delivery is polling, not SessionKeeper (D-34)
    refetchOnWindowFocus: true,
  });

  const items = query.data?.items ?? [];
  const unread = query.data?.unread_count ?? 0;

  const markReadMutation = useMutation({
    mutationFn: (id: string) => markRead(id),
    onMutate: async (id) => {
      await queryClient.cancelQueries({ queryKey: NOTIFICATIONS_KEY });
      const previous = queryClient.getQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY);
      queryClient.setQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY, (data) => {
        if (!data) return data;
        const target = data.items.find((n) => n.id === id);
        if (!target || target.read_at) return data; // already read — idempotent no-op
        return {
          ...data,
          items: data.items.map((n) =>
            n.id === id ? { ...n, read_at: new Date().toISOString() } : n,
          ),
          unread_count: Math.max(0, data.unread_count - 1),
        };
      });
      return { previous };
    },
    onError: (_err, _id, context) => {
      if (context?.previous) queryClient.setQueryData(NOTIFICATIONS_KEY, context.previous);
    },
    onSuccess: (result) => {
      // Reconcile from the mutation's own `200 {unread_count}` — no extra GET,
      // and this must win over any in-flight fetch that started before the
      // mutation settled (the anti-flicker rule: never render a pre-settle count).
      queryClient.setQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY, (data) =>
        data ? { ...data, unread_count: result.unread_count } : data,
      );
    },
  });

  const markAllReadMutation = useMutation({
    mutationFn: (before?: string) => markAllRead(before),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: NOTIFICATIONS_KEY });
      const previous = queryClient.getQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY);
      queryClient.setQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY, (data) =>
        data
          ? {
              ...data,
              items: data.items.map((n) =>
                n.read_at ? n : { ...n, read_at: new Date().toISOString() },
              ),
              unread_count: 0,
            }
          : data,
      );
      return { previous };
    },
    onError: (_err, _before, context) => {
      if (context?.previous) queryClient.setQueryData(NOTIFICATIONS_KEY, context.previous);
    },
    onSuccess: (result) => {
      queryClient.setQueryData<ListNotificationsPage>(NOTIFICATIONS_KEY, (data) =>
        data ? { ...data, unread_count: result.unread_count } : data,
      );
    },
  });

  function markAll() {
    // `before` = the newest rendered item's (created_at, id) cursor, so a
    // notification the store writes after this render stays unread.
    const newest = items[0];
    markAllReadMutation.mutate(newest ? watermarkCursor(newest) : undefined);
  }

  function openItem(n: NotificationItem) {
    if (!n.read_at) markReadMutation.mutate(n.id);
    if (n.data.href) router.push(n.data.href as Route);
  }

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
          {query.isPending ? (
            <Empty>Loading…</Empty>
          ) : query.isError ? (
            <Empty>Couldn&apos;t load notifications.</Empty>
          ) : items.length === 0 ? (
            <Empty>No notifications yet</Empty>
          ) : (
            items.map((n) => (
              <li
                key={n.id}
                onClick={() => openItem(n)}
                className="flex cursor-pointer items-start gap-3 border-b px-4 py-3 transition hover:bg-[var(--tpl-surface-2)]"
                style={{ ...rowBorder, background: n.read_at ? undefined : "rgba(255,94,58,0.05)" }}
              >
                <span
                  className="mt-0.5 grid h-9 w-9 shrink-0 place-items-center rounded-full"
                  style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-accent)" }}
                >
                  <Icon name="thunder-icon" size={16} />
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                    {n.title}
                  </p>
                  {n.body && (
                    <p className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                      {n.body}
                    </p>
                  )}
                  <p className="mt-0.5 text-[11px]" style={{ color: "var(--tpl-muted)" }}>
                    {timeAgo(n.created_at)}
                  </p>
                </div>
              </li>
            ))
          )}
        </DropdownCard>
      )}
    </div>
  );
}
