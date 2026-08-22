"use client";

// External-source sync (SPEC-02 P1.8). A sync source binds a comic to an external
// URL; triggering a sync creates a comic-level import job (the scraper produces the
// zip). The UI currently shows only scrape progress (scraped_chapters/total_chapters
// from the source). Per-batch IMPORT progress is not surfaced yet — last_import_id is
// returned for it, but nothing polls getImport on it, despite SPEC-02 rev 13 asking
// for "live scrape+import status".

import { api } from "./api-client";
import type { components } from "./types.gen";

// Shapes come from shared/openapi.yaml via `pnpm openapi` — do not restate them
// here. A hand-written copy is what let the sync contract drift from the server
// in the first place.
export type SyncSource = components["schemas"]["SyncSource"];
export type SyncStatus = SyncSource["last_status"];
type SyncSourceList = components["schemas"]["SyncSourceList"];

export async function listSyncSources(comicId: string): Promise<SyncSource[]> {
  const r = await api<SyncSourceList>(`/api/v1/comics/${comicId}/sync-sources`);
  return r.sources ?? [];
}

export async function createSyncSource(comicId: string, body: { source_url: string; chapters_hint?: string }): Promise<SyncSource> {
  return api<SyncSource>(`/api/v1/comics/${comicId}/sync-sources`, { method: "POST", body: JSON.stringify(body) });
}

export async function triggerSync(sourceId: string): Promise<SyncSource> {
  return api<SyncSource>(`/api/v1/sync-sources/${sourceId}/sync`, { method: "POST" });
}

export async function cancelSync(sourceId: string): Promise<SyncSource> {
  return api<SyncSource>(`/api/v1/sync-sources/${sourceId}/cancel`, { method: "POST" });
}

export async function deleteSyncSource(sourceId: string): Promise<void> {
  await api<void>(`/api/v1/sync-sources/${sourceId}`, { method: "DELETE" });
}
