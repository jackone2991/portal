"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import type { Route } from "next";
import { Icon } from "../ui/Icon";
import { useLayout, type MenuItem } from "@/lib/layout";

/**
 * Left menu — port of `components/menu/sidebarLeft.blade.php` (Olympus).
 * Expanded: icon + label rows, a Collapse toggle, and a profile-completion
 * footer. Collapsed: an icon-only rail. Width is driven by --tpl-sidebar-cur
 * (set on the shell root) so the header brand block moves in lockstep.
 * Icons are the authentic Olympus sprite (see SvgSprite / Icon).
 */

/**
 * The menu comes from `GET /layout`, already filtered to what this caller may
 * see — the server applies the permission on each row, so an admin-only entry
 * never reaches a bundle anyone can read.
 *
 * FALLBACK is the unrestricted part of the seeded menu, rendered while the query
 * is in flight or if it fails. Without it the sidebar would be empty on every
 * cold load and blank forever on an API blip, which is a worse failure than a
 * slightly stale menu. It deliberately contains no permission-gated row: this
 * list ships to everyone, so guessing optimistically would flash admin links at
 * users who do not have them.
 */
const FALLBACK: { icon: string; label: string; href?: string }[] = [
  { icon: "newsfeed-icon", label: "Newsfeed", href: "/" },
  { icon: "multimedia-icon", label: "Upload Video", href: "/upload" },
  { icon: "stats-icon", label: "Ledger", href: "/bank" },
  { icon: "happy-faces-icon", label: "People", href: "/people" },
  { icon: "albums-icon", label: "Commic", href: "/library/comic" },
  { icon: "happy-faces-icon", label: "Friend Groups" },
  { icon: "headphones-icon", label: "Music & Playlists", href: "/library/music" },
  { icon: "weather-icon", label: "Weather App", href: "/weather" },
  { icon: "calendar-icon", label: "Calendar and Events", href: "/calendar" },
  { icon: "badge-icon", label: "Community Badges" },
  { icon: "cupcake-icon", label: "Friends Birthdays" },
  { icon: "stats-icon", label: "Account Stats" },
];

/** The shape Row renders — the API item and the fallback collapse onto it. */
interface Entry {
  key: string;
  icon: string;
  label: string;
  href?: string;
}

function toEntries(items: MenuItem[] | undefined): Entry[] {
  if (!items?.length) {
    return FALLBACK.map((it) => ({ key: it.label, icon: it.icon, label: it.label, href: it.href }));
  }
  return items.map((it) => ({
    key: it.key,
    icon: it.icon,
    label: it.label,
    href: it.href ?? undefined,
  }));
}

export function SidebarLeft({
  collapsed,
  onToggle,
}: {
  collapsed: boolean;
  onToggle: () => void;
}) {
  const pathname = usePathname();
  const { data: layout } = useLayout();
  const items = toEntries(layout?.menu);

  return (
    <aside
      className="fixed left-0 z-30 hidden w-[var(--tpl-sidebar-cur)] flex-col border-r bg-white transition-[width] duration-200 xl:flex"
      style={{
        top: "var(--tpl-header-h)",
        height: "calc(100vh - var(--tpl-header-h))",
        borderColor: "var(--tpl-border)",
      }}
    >
      <div className="flex-1 overflow-y-auto py-3">
        <Row collapsed={collapsed} onClick={onToggle} label="Collapse Menu" icon="menu-icon" />
        <div className="my-2 h-px" style={{ background: "var(--tpl-border)" }} />
        {items.map((it) => (
          <Row
            key={it.key}
            collapsed={collapsed}
            icon={it.icon}
            label={it.label}
            href={it.href as Route | undefined}
            active={isActive(pathname, it.href)}
          />
        ))}
      </div>

      {!collapsed && (
        <div className="border-t px-6 py-5" style={{ borderColor: "var(--tpl-border)" }}>
          <div className="flex items-center justify-between text-sm" style={{ color: "var(--tpl-heading)" }}>
            <span className="font-medium">Profile Completion</span>
            <span>76%</span>
          </div>
          <div className="mt-2 h-1.5 overflow-hidden rounded-full" style={{ background: "var(--tpl-surface-2)" }}>
            <div
              className="h-full rounded-full"
              style={{ width: "76%", background: "linear-gradient(90deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            />
          </div>
          <p className="mt-3 text-xs leading-relaxed" style={{ color: "var(--tpl-muted)" }}>
            Complete{" "}
            <a href="#" className="font-medium hover:underline" style={{ color: "var(--tpl-accent)" }}>
              your profile
            </a>{" "}
            so people can know more about you!
          </p>
        </div>
      )}
    </aside>
  );
}

/**
 * Which row the current URL belongs to. "Newsfeed" is the only exact match —
 * everything else owns its subtree, so `/library/music/<id>` still highlights
 * "Music & Playlists". Rows with no `href` are decorative and never highlight.
 *
 * This used to be a hardcoded `active: true` on Newsfeed, which meant the menu
 * pointed at the home page from every screen in the app.
 */
function isActive(pathname: string, href?: string): boolean {
  if (!href) return false;
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}

function Row({
  icon,
  label,
  href,
  active,
  collapsed,
  onClick,
}: {
  icon: string;
  label: string;
  href?: Route;
  active?: boolean;
  collapsed: boolean;
  onClick?: () => void;
}) {
  const cls = `group flex items-center ${collapsed ? "justify-center px-0" : "gap-4 px-6"} py-2.5 transition hover:bg-[var(--tpl-surface-2)]`;
  const glyph = (
    <span
      className="grid w-6 shrink-0 place-items-center transition group-hover:text-[var(--tpl-accent)]"
      style={{ color: active ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={20} />
    </span>
  );
  const text = !collapsed && (
    <span
      className="whitespace-nowrap text-sm font-medium transition group-hover:text-[var(--tpl-heading)]"
      style={{ color: active ? "var(--tpl-heading)" : "var(--tpl-muted)" }}
    >
      {label}
    </span>
  );

  if (href) {
    return (
      <Link href={href} className={cls} title={label}>
        {glyph}
        {text}
      </Link>
    );
  }
  return (
    <button type="button" onClick={onClick} className={`${cls} w-full text-left`} title={label}>
      {glyph}
      {text}
    </button>
  );
}
