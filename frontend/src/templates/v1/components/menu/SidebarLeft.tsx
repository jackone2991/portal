"use client";

import Link from "next/link";
import type { Route } from "next";
import { Icon } from "../ui/Icon";

/**
 * Left menu — port of `components/menu/sidebarLeft.blade.php` (Olympus).
 * Expanded: icon + label rows, a Collapse toggle, and a profile-completion
 * footer. Collapsed: an icon-only rail. Width is driven by --tpl-sidebar-cur
 * (set on the shell root) so the header brand block moves in lockstep.
 * Icons are the authentic Olympus sprite (see SvgSprite / Icon).
 */

const ITEMS: { icon: string; label: string; href?: Route; active?: boolean }[] = [
  { icon: "newsfeed-icon", label: "Newsfeed", href: "/", active: true },
  { icon: "multimedia-icon", label: "Upload Video", href: "/upload" },
  { icon: "stats-icon", label: "Ledger", href: "/bank" as Route },
  { icon: "star-icon", label: "Fav Pages Feed", href: "/library/comic" },
  { icon: "happy-faces-icon", label: "Friend Groups" },
  { icon: "headphones-icon", label: "Music & Playlists", href: "/library/novel/1" as Route },
  { icon: "weather-icon", label: "Weather App" },
  { icon: "calendar-icon", label: "Calendar and Events" },
  { icon: "badge-icon", label: "Community Badges" },
  { icon: "cupcake-icon", label: "Friends Birthdays" },
  { icon: "stats-icon", label: "Account Stats" },
  { icon: "manage-widgets-icon", label: "Manage Widgets" },
];

export function SidebarLeft({
  collapsed,
  onToggle,
}: {
  collapsed: boolean;
  onToggle: () => void;
}) {
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
        {ITEMS.map((it) => (
          <Row
            key={it.label}
            collapsed={collapsed}
            icon={it.icon}
            label={it.label}
            href={it.href}
            active={it.active}
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
