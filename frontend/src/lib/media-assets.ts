// Data layer for the media library (SPEC-01 P0.4, [F006]). Server state — the
// caller's assets — is owned by TanStack Query (D-32); this module only holds
// the fetch functions and the wire types they return.
//
// NOTE ON TYPES: this intentionally does NOT reuse
// `components["schemas"]["Asset"]` from `src/lib/types.gen.ts`. That schema is
// generated from `shared/openapi.yaml`, which is camelCase (`mimeType`,
// `sizeBytes`, `hlsUrl`, status enum `uploaded|processing|ready|failed`) — but
// the actually-wired handler (`internal/modules/media/handler.go`
// `assetJSON`) emits snake_case (`mime_type`, `size_bytes`, `hls_url`, status
// enum `uploading|processing|ready|failed`). This is the documented
// spec/handler drift (repo CLAUDE.md "Known drift"); `MediaAsset` below
// mirrors the real wire shape, the same way `UploadStudio.tsx`'s local
// `AssetJSON` does.
//
// The `?kind=&status=&cursor=` query params, `DELETE /assets/{id}`,
// `GET /assets/{id}/variants/{variant}` and `GET /assets/{id}/original` are
// SPEC-01 P0.4/P0.5 additions the backend does not implement yet (verified
// against `module.go`/`handler.go` — no route registered, no query parsing).
// This module is written against the target contract in SPEC-01 §7; wiring
// the backend to match is tracked separately.

import { api, baseURL } from "./api-client";

export type AssetKind = "video" | "image" | "audio";

/** Wired today: no `uploaded`, no `deleting` — see module doc comment. */
export type AssetStatus = "uploading" | "processing" | "ready" | "failed";

export type AssetKindFilter = "all" | AssetKind;
export type AssetStatusFilter = "all" | "ready" | "processing" | "failed";

/** SPEC-01 §7 asset variants — grid/pickers use `thumb`, never the original. */
export type AssetVariant = "thumb" | "medium" | "poster";

export interface MediaAsset {
  id: string;
  status: AssetStatus;
  kind: AssetKind;
  mime_type: string;
  size_bytes: number;
  duration_ms: number | null;
  width: number | null;
  height: number | null;
  created_at: string;
  /** Present once `status` is `ready`. */
  hls_url?: string;
  /** Present when `status` is `failed` (wire key is `error`, not `error_message`). */
  error?: string;
  /** SPEC-01 §6 addition (`title` column) — optional until the backend lands it. */
  title?: string;
  /** SPEC-01 §6 addition (`original_filename` column). */
  original_filename?: string;
}

export interface ListAssetsPage {
  assets: MediaAsset[];
  /** Keyset cursor (`created_at DESC, id DESC`); null/absent on the last page. */
  next_cursor?: string | null;
}

export interface ListAssetsParams {
  kind?: AssetKindFilter;
  status?: AssetStatusFilter;
  cursor?: string;
}

/**
 * `GET /api/v1/assets?kind=&status=&cursor=`.
 *
 * The `status` filter's `processing` value is documented (SPEC-01 P0.4 rev 3)
 * to also match still-`uploading` assets server-side, so a freshly created
 * upload is never invisible under every filter — this function just forwards
 * the selected value, the inclusion is the API contract's job.
 */
export async function listAssets(
  params: ListAssetsParams = {},
): Promise<ListAssetsPage> {
  const q = new URLSearchParams();
  if (params.kind && params.kind !== "all") q.set("kind", params.kind);
  if (params.status && params.status !== "all") q.set("status", params.status);
  if (params.cursor) q.set("cursor", params.cursor);
  const qs = q.toString();
  return api<ListAssetsPage>(`/api/v1/assets${qs ? `?${qs}` : ""}`);
}

/** `DELETE /api/v1/assets/{id}` — owner, or `assets:delete:any`. 204; idempotent 404. */
export async function deleteAsset(id: string): Promise<void> {
  await api<void>(`/api/v1/assets/${id}`, { method: "DELETE" });
}

/**
 * Direct URL for `GET /assets/{id}/variants/{variant}` — public-ish /
 * unauthenticated (same as `/hls/*`), so this can go straight in an
 * `<img src>` without routing through `api()`.
 */
export function assetVariantURL(id: string, variant: AssetVariant): string {
  return `${baseURL}/api/v1/assets/${id}/variants/${variant}`;
}

/**
 * Direct URL for `GET /assets/{id}/original` — owner-authenticated attachment
 * stream (cookies ride along via the browser's normal same-site request, same
 * as the HLS proxy / dev source upload). Never public.
 */
export function assetOriginalURL(id: string): string {
  return `${baseURL}/api/v1/assets/${id}/original`;
}

export async function getAsset(id: string): Promise<MediaAsset> {
  return api<MediaAsset>(`/api/v1/assets/${id}`);
}

export interface PlaybackProgress {
  position_ms: number;
  progress_pct?: number;
  completed_at?: string | null;
  updated_at: string;
}

export async function getPlaybackProgress(id: string): Promise<PlaybackProgress> {
  return api<PlaybackProgress>(`/api/v1/assets/${id}/progress`);
}

export async function putPlaybackProgress(id: string, positionMs: number): Promise<void> {
  await api<void>(`/api/v1/assets/${id}/progress`, {
    method: "PUT",
    body: JSON.stringify({ position_ms: positionMs }),
  });
}

export interface ContinueItem {
  module: "media" | "movie" | "music" | "story" | "comic";
  ref_id: string;
  title: string;
  progress_pct: number;
  href: string;
  poster_url?: string | null;
  updated_at: string;
}

export interface ContinueResponse {
  items: ContinueItem[];
}

export async function getContinueItems(limit: number = 10): Promise<ContinueResponse> {
  return api<ContinueResponse>(`/api/v1/continue?limit=${limit}`);
}
