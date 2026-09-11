// The app shell's composition — navigation menu and dashboard widget rails —
// which the API now owns (migration 0036). Types mirror the wired handler JSON
// in `backend/internal/modules/layout/handler.go`.

import { useQuery } from "@tanstack/react-query";
import { api } from "./api-client";

export type WidgetSlot = "left" | "right";

export interface MenuItem {
  id: string;
  key: string;
  label: string;
  /** Sprite name, resolved by the Icon component. */
  icon: string;
  /** In-app path. Null renders a row that navigates nowhere. */
  href: string | null;
  /** Permission needed to see it. Null = everyone signed in. */
  permission: string | null;
  position: number;
  visible: boolean;
  /** Seeded rows: editable and hideable, never deletable. */
  is_system: boolean;
}

export interface LayoutWidget {
  id: string;
  key: string;
  label: string;
  slot: WidgetSlot;
  permission: string | null;
  position: number;
  visible: boolean;
}

export interface LayoutConfig {
  menu: MenuItem[];
  widgets: LayoutWidget[];
}

export const LAYOUT_KEY = ["layout"] as const;
export const ADMIN_LAYOUT_KEY = ["admin", "layout"] as const;

/**
 * The caller's own layout, already filtered server-side.
 *
 * `staleTime` is generous because this drives the frame on every page and the
 * config changes about as often as a deploy used to. The admin screen
 * invalidates this key after a save, so an editor sees their own change at once
 * without everyone else paying for a refetch per navigation.
 */
export function useLayout() {
  return useQuery({
    queryKey: LAYOUT_KEY,
    queryFn: () => api<LayoutConfig>("/api/v1/layout"),
    staleTime: 5 * 60_000,
    retry: false,
  });
}

export function getAdminLayout(): Promise<LayoutConfig> {
  return api<LayoutConfig>("/api/v1/admin/layout");
}

export interface MenuSaveItem {
  key: string;
  label: string;
  icon: string;
  href: string;
  permission: string;
  visible: boolean;
}

/** Array order is authoritative — the server renumbers `position` from it. */
export function saveMenu(items: MenuSaveItem[]): Promise<LayoutConfig> {
  return api<LayoutConfig>("/api/v1/admin/layout/menu", {
    method: "PUT",
    body: JSON.stringify({ items }),
  });
}

export interface WidgetSaveItem {
  key: string;
  label: string;
  slot: WidgetSlot;
  permission: string;
  visible: boolean;
}

export function saveWidgets(widgets: WidgetSaveItem[]): Promise<LayoutConfig> {
  return api<LayoutConfig>("/api/v1/admin/layout/widgets", {
    method: "PUT",
    body: JSON.stringify({ widgets }),
  });
}

/** Move `index` by `delta` within a copy of the list. Out-of-range is a no-op. */
export function reorder<T>(list: T[], index: number, delta: number): T[] {
  const to = index + delta;
  if (to < 0 || to >= list.length) return list;
  const next = [...list];
  const [moved] = next.splice(index, 1);
  if (moved === undefined) return list;
  next.splice(to, 0, moved);
  return next;
}
