"use client";

// External-source sync (SPEC-02 P1.8). A sync source binds a comic to an external
// URL; triggering a sync creates a comic-level import job (the scraper produces the
// zip) whose progress is polled via lib/comic-import (getImport on last_import_id).

import { api } from "./api-client";

export type SyncStatus = "idle" | "syncing" | "done" | "failed" | "cancelled";

export interface SyncSource {
  id: string;
  comic_id: string;
  source_url: string;
  source_site: string;
  chapters_hint: string;
  last_status: SyncStatus;
  last_import_id?: string | null;
  last_error?: string | null;
  last_synced_at?: string | null;
  total_chapters: number;
  scraped_chapters: number;
  created_at: string;
  updated_at: string;
}

export async function listSyncSources(comicId: string): Promise<SyncSource[]> {
  const r = await api<{ sources: SyncSource[] }>(`/api/v1/comics/${comicId}/sync-sources`);
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
