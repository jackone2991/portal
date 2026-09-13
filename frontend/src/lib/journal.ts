// Data layer for the journal composer + interim home list (SPEC-05 P0.4).
// Server state — the owner's journal entries — is owned by TanStack Query
// (D-32); this module only holds the fetch functions and the wire types they
// return.
//
// Same convention as `media-assets.ts` / `notifications.ts`: the wire types are
// hand-rolled, snake_case, and kept in step with `JournalEntry` /
// `JournalEntryWrite` in `src/lib/types.gen.ts` (generated from
// `shared/openapi.yaml`). They predate the generated schema; switching the
// consumers onto `components["schemas"]` is open debt, not a reason to let the
// two drift — change the contract first, regenerate, then mirror it here.

import { api } from "./api-client";

export interface JournalEntry {
  id: string;
  /** Plain markdown — may be "" for a photo-only Entry (SPEC-12). */
  body_md: string;
  mood: string | null;
  /** The Entry's Attachments — image Asset ids in display order (SPEC-12 T1). */
  asset_ids: string[];
  /** User-editable "when this happened" — the timeline's sort key (§5 P0.2). */
  occurred_at: string;
  /** Audit timestamp only — never used for ordering (§5 P0.2). */
  created_at: string;
  updated_at: string;
}

export interface ListEntriesPage {
  items: JournalEntry[];
  /** Keyset cursor (`occurred_at DESC, id DESC`); null/absent on the last page. */
  next_cursor?: string | null;
}

export interface ListEntriesParams {
  cursor?: string;
}

export interface CreateEntryInput {
  /**
   * At most 20000 chars. May be omitted or empty only when `asset_ids` has at
   * least one element — an Entry is text or an Attachment, never neither —
   * else 422 `journal/invalid-body` (SPEC-12).
   */
  body_md?: string;
  /** Freeform, 1–80 chars after trimming when present (§5 P0.2). */
  mood?: string;
  /**
   * The Entry's Attachments in display order: at most ten distinct ready image
   * Assets the caller owns, else 422 `journal/invalid-asset` naming the id and
   * nothing is stored. On PATCH it replaces the whole list (SPEC-12 T1).
   */
  asset_ids?: string[];
  /** Omit to default to now server-side; backdating/future-dating unlimited (§5 P0.2). */
  occurred_at?: string;
}

export type PatchEntryInput = Partial<CreateEntryInput>;

/** `GET /api/v1/journal/entries?cursor=` — ordered `occurred_at DESC, id DESC` (§7). */
export async function listEntries(
  params: ListEntriesParams = {},
): Promise<ListEntriesPage> {
  const q = new URLSearchParams();
  if (params.cursor) q.set("cursor", params.cursor);
  const qs = q.toString();
  return api<ListEntriesPage>(`/api/v1/journal/entries${qs ? `?${qs}` : ""}`);
}

/** `POST /api/v1/journal/entries` — `journal:write:own` (§7). */
export async function createEntry(input: CreateEntryInput): Promise<JournalEntry> {
  return api<JournalEntry>("/api/v1/journal/entries", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/** `PATCH /api/v1/journal/entries/{id}` — any subset of create fields (§7). */
export async function patchEntry(
  id: string,
  input: PatchEntryInput,
): Promise<JournalEntry> {
  return api<JournalEntry>(`/api/v1/journal/entries/${id}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

/** `DELETE /api/v1/journal/entries/{id}` — 204; idempotent 404 (§7). */
export async function deleteEntry(id: string): Promise<void> {
  await api<void>(`/api/v1/journal/entries/${id}`, { method: "DELETE" });
}
