"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import Link from "next/link";
import type { Route } from "next";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { FriendRequestsMenu, MessagesMenu, NotificationsMenu } from "./NotifMenus";
import { baseURL as API_BASE } from "@/lib/api-client";

/**
 * Header bar content — port of `components/headers/menu.blade.php`.
 * Logo · page title · search · Find Friends · notification dropdowns · profile.
 * The right cluster shows one dropdown at a time; clicking outside or Escape
 * closes it. Olympus sprite icons throughout.
 */
export function TopMenu() {
  const [me, setMe] = useState<{ name: string; role: string }>({ name: "Guest", role: "Member" });
  const [menu, setMenu] = useState<string | null>(null);
  const clusterRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let alive = true;
    fetch(`${API_BASE}/api/v1/auth/me`, { credentials: "include" })
      .then((r) => (r.ok ? r.json() : null))
      .then((d: { display_name?: string; roles?: string[] } | null) => {
        if (alive && d?.display_name) setMe({ name: d.display_name, role: d.roles?.[0] ?? "Member" });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => {
    if (!menu) return;
    const onDoc = (e: MouseEvent) => {
      if (clusterRef.current && !clusterRef.current.contains(e.target as Node)) setMenu(null);
    };
    const onEsc = (e: KeyboardEvent) => e.key === "Escape" && setMenu(null);
    document.addEventListener("mousedown", onDoc);
    document.addEventListener("keydown", onEsc);
    return () => {
      document.removeEventListener("mousedown", onDoc);
      document.removeEventListener("keydown", onEsc);
    };
  }, [menu]);

  const toggle = (k: string) => setMenu((m) => (m === k ? null : k));

  return (
    <div className="flex h-full items-stretch">
      {/* Brand block — width tracks the left menu (--tpl-sidebar-cur). */}
      <Link
        href="/"
        className="flex h-full w-[var(--tpl-rail-w)] shrink-0 items-center gap-3 overflow-hidden text-white transition-[width] duration-200 xl:w-[var(--tpl-sidebar-cur)]"
        style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        aria-label="Portal — Newsfeed"
      >
        <span className="grid shrink-0 place-items-center" style={{ width: "var(--tpl-rail-w)" }}>
          <Icon name="thunder-icon" size={26} />
        </span>
        <span className="hidden whitespace-nowrap text-lg font-extrabold uppercase tracking-wide xl:block">
          Portal
        </span>
      </Link>

      <div className="flex flex-1 items-center gap-4 px-5">
        <span className="hidden text-sm font-bold uppercase tracking-wide text-white/90 sm:block">Newsfeed</span>

        {/* Search */}
        <label className="relative hidden min-w-0 flex-1 items-center md:flex" style={{ maxWidth: 380 }}>
          <input
            type="search"
            placeholder="Search here people or pages..."
            className="w-full rounded-full py-2 pl-4 pr-9 text-sm text-white outline-none placeholder:text-white/45"
            style={{ background: "var(--tpl-header-2)" }}
          />
          <span className="pointer-events-none absolute right-3 text-white/60">
            <Icon name="magnifying-glass-icon" size={16} />
          </span>
        </label>

        <Link href="/" className="hidden text-sm text-white/80 hover:text-white lg:block">
          Find Friends
        </Link>

        {/* Right cluster — one dropdown at a time */}
        <div ref={clusterRef} className="ml-auto flex items-center gap-1.5">
          <FriendRequestsMenu open={menu === "requests"} onToggle={() => toggle("requests")} />
          <MessagesMenu open={menu === "messages"} onToggle={() => toggle("messages")} />
          <NotificationsMenu open={menu === "notifications"} onToggle={() => toggle("notifications")} />
          <ProfileMenu
            name={me.name}
            role={me.role}
            open={menu === "profile"}
            onToggle={() => toggle("profile")}
          />
        </div>
      </div>
    </div>
  );
}

/* ── profile dropdown ──────────────────────────────────────────── */

const STATUSES = [
  { key: "online", label: "Online", varName: "--tpl-status-online" },
  { key: "away", label: "Away", varName: "--tpl-status-away" },
  { key: "disconnected", label: "Disconnected", varName: "--tpl-status-offline" },
  { key: "invisible", label: "Invisible", varName: "--tpl-status-invisible" },
] as const;

function ProfileMenu({
  name,
  role,
  open,
  onToggle,
}: {
  name: string;
  role: string;
  open: boolean;
  onToggle: () => void;
}) {
  const [status, setStatus] = useState<string>("online");
  const [subtitle, setSubtitle] = useState(role);
  const [draft, setDraft] = useState(role);

  useEffect(() => {
    setSubtitle(role);
    setDraft(role);
  }, [role]);

  async function logout() {
    try {
      // ensure a valid access token so the authenticated logout endpoint accepts
      // it (and clears cookies incl. the durable session marker), even if the
      // access token had lapsed.
      await fetch(`${API_BASE}/api/v1/auth/refresh`, { method: "POST", credentials: "include" });
      await fetch(`${API_BASE}/api/v1/auth/logout`, { method: "POST", credentials: "include" });
    } catch {
      /* ignore — redirect regardless */
    }
    try {
      localStorage.removeItem("portal_refresh_at");
    } catch {
      /* storage disabled */
    }
    window.location.assign("/login");
  }

  function applyCustom() {
    const v = draft.trim();
    if (v) setSubtitle(v);
  }

  return (
    <div className="relative ml-2">
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={open}
        aria-haspopup="menu"
        className="flex items-center gap-2.5 rounded-lg px-1.5 py-1 hover:bg-white/5"
      >
        <span className="hidden max-w-[10rem] text-right leading-tight sm:block">
          <span className="block truncate text-sm font-semibold text-white">{name}</span>
          <span className="block truncate text-[10px] uppercase tracking-wide text-white/50">{subtitle}</span>
        </span>
        <Avatar name={name} size={36} status={status === "disconnected" ? "offline" : (status as "online" | "away" | "invisible")} />
        <span className={`hidden text-white/50 transition-transform sm:block ${open ? "rotate-180" : ""}`}>
          <Icon name="dropdown-arrow-icon" size={12} />
        </span>
      </button>

      {open && (
        <div
          role="menu"
          className="absolute right-0 top-full z-50 mt-3 w-72 rounded-xl border bg-white shadow-2xl"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
        >
          <span
            className="absolute -top-1.5 right-6 h-3 w-3 rotate-45 border-l border-t bg-white"
            style={{ borderColor: "var(--tpl-border)" }}
          />

          <MenuHeader>Your Account</MenuHeader>
          <MenuItem icon="menu-icon" label="Profile Settings" href="/" />
          <MenuItem icon="star-icon" label="Create Fav Page" href="/" />
          <MenuItem icon="logout-icon" label="Log Out" onClick={logout} />

          <Divider />

          <MenuHeader>Chat Settings</MenuHeader>
          <div className="pb-1">
            {STATUSES.map((s) => {
              const active = status === s.key;
              return (
                <button
                  key={s.key}
                  type="button"
                  onClick={() => setStatus(s.key)}
                  className="flex w-full items-center gap-3 px-4 py-1.5 text-left text-sm transition hover:bg-[var(--tpl-surface-2)]"
                >
                  <span className="h-2.5 w-2.5 rounded-full" style={{ background: `var(${s.varName})` }} />
                  <span className={active ? "font-semibold" : ""} style={{ color: active ? "var(--tpl-accent)" : "var(--tpl-text)" }}>
                    {s.label}
                  </span>
                </button>
              );
            })}
          </div>

          <Divider />

          <MenuHeader>Custom Status</MenuHeader>
          <div className="flex items-center gap-2 px-4 pb-3 pt-1">
            <input
              value={draft}
              onChange={(e) => setDraft(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && applyCustom()}
              placeholder="What's on your mind?"
              className="min-w-0 flex-1 rounded-md border bg-transparent px-3 py-1.5 text-sm outline-none focus:border-[var(--tpl-accent)]"
              style={{ borderColor: "var(--tpl-border)" }}
            />
            <button
              type="button"
              onClick={applyCustom}
              className="grid h-8 w-8 shrink-0 place-items-center rounded-md text-white"
              style={{ background: "#7c5ac2" }}
              aria-label="Save status"
            >
              <Icon name="check-icon" size={14} />
            </button>
          </div>

          <Divider />

          <MenuHeader>About Portal</MenuHeader>
          <div className="pb-2">
            {["Terms and Conditions", "FAQs", "Careers", "Contact"].map((t) => (
              <a
                key={t}
                href="#"
                className="block px-4 py-1.5 text-sm font-medium transition hover:text-[var(--tpl-accent)]"
                style={{ color: "var(--tpl-heading)" }}
              >
                {t}
              </a>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

function MenuHeader({ children }: { children: ReactNode }) {
  return (
    <p className="px-4 pb-1 pt-3 text-[11px] font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
      {children}
    </p>
  );
}

function Divider() {
  return <div className="my-1 h-px" style={{ background: "var(--tpl-border)" }} />;
}

function MenuItem({
  icon,
  label,
  href,
  onClick,
}: {
  icon: string;
  label: string;
  href?: Route;
  onClick?: () => void;
}) {
  const cls =
    "flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]";
  const inner = (
    <>
      <span style={{ color: "var(--tpl-muted)" }}>
        <Icon name={icon} size={18} />
      </span>
      <span style={{ color: "var(--tpl-heading)" }}>{label}</span>
    </>
  );
  if (onClick) {
    return (
      <button type="button" onClick={onClick} className={cls}>
        {inner}
      </button>
    );
  }
  return (
    <Link href={href ?? "/"} className={cls}>
      {inner}
    </Link>
  );
}
