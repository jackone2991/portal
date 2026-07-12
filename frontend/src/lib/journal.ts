// Data layer for the journal composer + interim home list (SPEC-05 P0.4).
// Server state — the owner's journal entries — is owned by TanStack Query
// (D-32); this module only holds the fetch functions and the wire types they
// return.
//
// Mirrors the same convention as `media-assets.ts` / `notifications.ts`: no
// `journal` schema exists yet in `src/lib/types.gen.ts` (the backend module is
// landing in parallel — SPEC-05 P0.1/P0.2 — and `make openapi` hasn't picked
// it up), so these types are hand-rolled against the real wire shape
// documented in SPEC-05 §6 (migration `0011_journal_entries`) and §7 (API
// summary) — snake_case, matching the account/media modules' handlers.
// Reconcile against `types.gen.ts` once the journal module's OpenAPI paths
// land and `make openapi` generates a schema for it.

import { api } from "./api-client";

export interface JournalEntry {
  id: string;
  body_md: string;
  mood: string | null;
  /** SPEC-05 §6 column — `NOT NULL DEFAULT '{}'`; stays empty until P1.5. */
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
  /** 1–20000 chars, else 422 `journal/invalid-body` (§5 P0.2). */
  body_md: string;
  /** Freeform, 1–80 chars after trimming when present (§5 P0.2). */
  mood?: string;
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
