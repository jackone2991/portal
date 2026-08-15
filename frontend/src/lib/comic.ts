// Data layer for the comic vertical (SPEC-02). TanStack owns server state (D-32);
// types mirror the wired handler JSON (snake_case).

import { api, baseURL } from "./api-client";

export type ComicStatus = "draft" | "published";
/** Reading direction of the work (SPEC-02 R3). manga = "rtl"; "vertical" = webtoon. */
export type ReadingDirection = "ltr" | "rtl" | "vertical";

export interface Comic {
  id: string;
  owner_id: string;
  title: string;
  description: string | null;
  cover_asset_id: string | null;
  status: ComicStatus;
  reading_direction: ReadingDirection;
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
/** Patch comic metadata (owner or comics:write:any). Only send changed fields. */
export type ComicPatch = Partial<{ title: string; description: string | null; reading_direction: ReadingDirection; cover_asset_id: string | null }>;
export async function updateComic(id: string, patch: ComicPatch): Promise<Comic> {
  return api<Comic>(`/api/v1/comics/${id}`, { method: "PATCH", body: JSON.stringify(patch) });
}
export async function deleteComic(id: string): Promise<void> {
  await api<void>(`/api/v1/comics/${id}`, { method: "DELETE" });
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
export async function updateChapter(chapterId: string, body: { title: string }): Promise<Chapter> {
  return api<Chapter>(`/api/v1/chapters/${chapterId}`, { method: "PATCH", body: JSON.stringify(body) });
}
export async function deleteChapter(chapterId: string): Promise<void> {
  await api<void>(`/api/v1/chapters/${chapterId}`, { method: "DELETE" });
}
export async function reorderChapters(comicId: string, order: string[]): Promise<void> {
  await api<void>(`/api/v1/comics/${comicId}/chapters:order`, { method: "PUT", body: JSON.stringify({ order }) });
}
export async function getChapterPages(chapterId: string): Promise<ReaderPage[]> {
  const r = await api<{ pages: ReaderPage[] }>(`/api/v1/chapters/${chapterId}/pages`);
  return r.pages ?? [];
}
export async function createPages(chapterId: string, pages: { asset_id: string; sort_order: number }[]): Promise<ReaderPage[]> {
  const r = await api<{ pages: ReaderPage[] }>(`/api/v1/chapters/${chapterId}/pages`, { method: "POST", body: JSON.stringify({ pages }) });
  return r.pages ?? [];
}
export async function deletePage(pageId: string): Promise<void> {
  await api<void>(`/api/v1/pages/${pageId}`, { method: "DELETE" });
}
export async function reorderPages(chapterId: string, order: string[]): Promise<void> {
  await api<void>(`/api/v1/chapters/${chapterId}/pages:order`, { method: "PUT", body: JSON.stringify({ order }) });
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
