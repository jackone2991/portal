// Data layer for connections between accounts (migration 0037).
//
// A connection is mutual and has one row whichever way it was created. Every
// shape below is already rendered from the caller's point of view by the API:
// `user_id`/`display_name` are the OTHER person, and `outgoing` says who asked.
// Nothing here has to work out which end you are.

import { api } from "./api-client";

export type ConnectionStatus = "pending" | "accepted";

/** Which list to read. `accepted` is the friends list. */
export type ConnectionKind = "accepted" | "incoming" | "outgoing";

export interface Connection {
  /** The connection's id — what accept/remove take, not the user's id. */
  id: string;
  user_id: string;
  display_name?: string;
  status: ConnectionStatus;
  /** True when you sent the request: Cancel, rather than Accept. */
  outgoing: boolean;
  created_at: string;
  responded_at?: string | null;
}

export async function listConnections(kind: ConnectionKind = "accepted"): Promise<Connection[]> {
  const r = await api<{ connections?: Connection[] }>(`/api/v1/connections?status=${kind}`);
  return r.connections ?? [];
}

/**
 * Ask someone to connect. If they already asked you, this accepts theirs — the
 * API treats sending a request to someone who asked first as agreement.
 */
export async function requestConnection(userId: string): Promise<Connection> {
  return api<Connection>("/api/v1/connections", {
    method: "POST",
    body: JSON.stringify({ user_id: userId }),
  });
}

export async function acceptConnection(id: string): Promise<Connection> {
  return api<Connection>(`/api/v1/connections/${id}/accept`, { method: "POST" });
}

/** Withdraw, decline, or disconnect — one call for all three. */
export async function removeConnection(id: string): Promise<void> {
  await api<void>(`/api/v1/connections/${id}`, { method: "DELETE" });
}

/** Pending requests waiting on you — the header badge's number. */
export async function connectionSummary(): Promise<number> {
  const r = await api<{ incoming?: number }>("/api/v1/connections/summary");
  return r.incoming ?? 0;
}
