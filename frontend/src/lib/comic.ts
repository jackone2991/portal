// Data layer for the comic vertical (SPEC-02). TanStack owns server state (D-32);
// types mirror the wired handler JSON (snake_case).

import { api, baseURL } from "./api-client";

export type ComicStatus = "draft" | "published";

export interface Comic {
  id: string;
  owner_id: string;
  title: string;
  description: string | null;
  cover_asset_id: string | null;
  status: ComicStatus;
  chapter_count?: number;
  created_at: string;
  updated_at: string;
}

export interface Chapter {
  id: string;
  comic_id: string;
  title: string;
  sort_order: number;
  created_at: string;
}

export interface ComicProgress {
  chapter_id: string;
  page_id: string | null;
  updated_at: string;
}

export interface ComicDetail extends Comic {
  chapters: Chapter[];
  progress?: ComicProgress;
}

export interface ReaderPage {
  page_id: string;
  asset_id: string;
  width: number | null;
  height: number | null;
}

export interface ComicsPage {
  comics: Comic[];
  next_cursor?: string | null;
}

/** Direct variant URL (public-ish, same as media/HLS) for a comic page/cover. */
export function variantURL(assetId: string, variant: "thumb" | "medium" | "poster"): string {
  return `${baseURL}/api/v1/assets/${assetId}/variants/${variant}`;
}

export async function listComics(cursor?: string): Promise<ComicsPage> {
  const q = cursor ? `?cursor=${cursor}` : "";
  const r = await api<ComicsPage>(`/api/v1/comics${q}`);
  return { comics: r.comics ?? [], next_cursor: r.next_cursor };
}
export async function listMyComics(cursor?: string): Promise<ComicsPage> {
  const q = cursor ? `?cursor=${cursor}` : "";
  const r = await api<ComicsPage>(`/api/v1/comics/mine${q}`);
  return { comics: r.comics ?? [], next_cursor: r.next_cursor };
}
export async function createComic(body: { title: string; description?: string | null }): Promise<Comic> {
  return api<Comic>("/api/v1/comics", { method: "POST", body: JSON.stringify(body) });
}
export async function getComic(id: string): Promise<ComicDetail> {
  return api<ComicDetail>(`/api/v1/comics/${id}`);
}
export async function publishComic(id: string): Promise<Comic> {
  return api<Comic>(`/api/v1/comics/${id}/publish`, { method: "POST" });
}
export async function unpublishComic(id: string): Promise<Comic> {
  return api<Comic>(`/api/v1/comics/${id}/unpublish`, { method: "POST" });
}
export async function createChapter(comicId: string, body: { title: string; sort_order: number }): Promise<Chapter> {
  return api<Chapter>(`/api/v1/comics/${comicId}/chapters`, { method: "POST", body: JSON.stringify(body) });
}
export async function getChapterPages(chapterId: string): Promise<ReaderPage[]> {
  const r = await api<{ pages: ReaderPage[] }>(`/api/v1/chapters/${chapterId}/pages`);
  return r.pages ?? [];
}
export function saveComicProgress(comicId: string, chapterId: string, pageId: string | null): void {
  const url = `${baseURL}/api/v1/comics/${comicId}/progress`;
  const body = JSON.stringify({ chapter_id: chapterId, page_id: pageId });
  if (typeof navigator !== "undefined" && navigator.sendBeacon) {
    navigator.sendBeacon(url, new Blob([body], { type: "application/json" }));
  } else {
    fetch(url, { method: "PUT", headers: { "Content-Type": "application/json" }, body, credentials: "include", keepalive: true }).catch(() => {});
  }
}
