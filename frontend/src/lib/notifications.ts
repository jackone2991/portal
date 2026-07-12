// Data layer for the notifications bell (SPEC-04 P0.5). Server state — the
// caller's notifications + unread badge — is owned by TanStack Query (D-32);
// this module only holds the fetch functions and the wire types they return.
//
// Mirrors the same convention as `media-assets.ts`: no `notify` module exists
// yet in `src/lib/types.gen.ts` (SPEC-04 §6's `notifications` table hasn't
// shipped a schema block there), so these types are hand-rolled against the
// real wire shape documented in SPEC-04 §6/§7 — snake_case, matching the
// account/media modules' handlers rather than an OpenAPI-generated camelCase
// schema. Update these if/when `notify` lands and `make openapi` adds a
// generated schema to reconcile against.

import { api } from "./api-client";

export type NotificationStatus = "unread" | "all";

/**
 * `data.href` is the click-through target for every notification type (SPEC-04
 * P0.4 "click-through contract") — the bell never maintains a per-type mapping
 * table, it just navigates to `data.href` on click.
 */
export interface NotificationData {
  href?: string;
  [key: string]: unknown;
}

export interface NotificationItem {
  id: string;
  type: string;
  title: string;
  body: string | null;
  data: NotificationData;
  read_at: string | null;
  created_at: string;
}

export interface ListNotificationsPage {
  items: NotificationItem[];
  unread_count: number;
  next_cursor?: string | null;
}

export interface ListNotificationsParams {
  status?: NotificationStatus;
  cursor?: string;
}

export interface MarkReadResult {
  unread_count: number;
}

/** `GET /api/v1/me/notifications?status=&cursor=` — `status` defaults to `all` server-side when omitted. */
export async function listNotifications(
  params: ListNotificationsParams = {},
): Promise<ListNotificationsPage> {
  const q = new URLSearchParams();
  if (params.status) q.set("status", params.status);
  if (params.cursor) q.set("cursor", params.cursor);
  const qs = q.toString();
  return api<ListNotificationsPage>(`/api/v1/me/notifications${qs ? `?${qs}` : ""}`);
}

/** `POST /api/v1/me/notifications/{id}/read` — idempotent, `200 {unread_count}` (never a bare 204). */
export async function markRead(id: string): Promise<MarkReadResult> {
  return api<MarkReadResult>(`/api/v1/me/notifications/${id}/read`, {
    method: "POST",
  });
}

/**
 * `POST /api/v1/me/notifications/read-all?before=` — `before` is the keyset
 * watermark (`created_at`, `id`) of the newest rendered item, so a
 * notification the store creates after the dropdown last rendered stays
 * unread. Omit `before` to mark everything read.
 */
export async function markAllRead(before?: string): Promise<MarkReadResult> {
  const qs = before ? `?before=${encodeURIComponent(before)}` : "";
  return api<MarkReadResult>(`/api/v1/me/notifications/read-all${qs}`, {
    method: "POST",
  });
}

/**
 * Encodes the read-all watermark for an item — same (`created_at`, `id`)
 * keyset pair the list endpoint's own pagination cursor is built from (§6).
 * The backend doesn't exist yet, so the exact codec is a client-side choice
 * documented here; reconcile if `notify`'s real cursor format differs once
 * P0.1 ships.
 */
export function watermarkCursor(item: Pick<NotificationItem, "created_at" | "id">): string {
  return `${item.created_at}_${item.id}`;
}
