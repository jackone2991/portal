// The composer's photo list — the state behind its strip of tiles (SPEC-12 T2).
//
// Pure: every rule the composer enforces before the server can — one tile per
// file, at most ten, Save only when every photo is `ready` — lives here as a
// function over the list, so the component is left with rendering and with
// starting uploads. The server enforces the same cap and readiness
// (`journal/invalid-asset`); this is the client saying so first.
//
// Identity is the `key`, not the Asset id: a file has no Asset until its upload
// has created one, and the tile must exist from the moment it is picked.

export const MAX_ATTACHMENTS = 10;

export type PhotoStatus = "uploading" | "processing" | "ready" | "failed";

export interface ComposerPhoto {
  /** Stable within one composer session: a file fingerprint or `asset:<id>`. */
  key: string;
  /** The media Asset once it exists; null while the file is still uploading. */
  assetId: string | null;
  status: PhotoStatus;
  /** Upload progress 0–100; meaningful while `uploading`. */
  pct: number;
  /** Why the photo failed — shown on the tile so the writer can drop it. */
  error: string | null;
  /** What the tile shows: an object URL for a file, the thumb variant for a library pick. */
  preview: string;
}

/** What the picker hands back: a file to upload, or a ready Asset from the library. */
export type PhotoPick = { kind: "file"; file: File } | { kind: "asset"; id: string };

export interface AddResult {
  /**
   * The picks that became tiles, in selection order — the caller appends them
   * and starts their uploads. (Not the whole list: the caller merges into the
   * live state, so an upload's progress patch queued meanwhile is kept.)
   */
  added: { photo: ComposerPhoto; pick: PhotoPick }[];
  /** Picks skipped because the same file or Asset was already in the list. */
  duplicates: number;
  /** Picks refused because the list was full. */
  refused: number;
}

/**
 * A file's identity for "added twice". Name + size + mtime is what the browser
 * exposes without reading the bytes; a re-saved file changes its mtime and
 * counts as new, which is the right side to err on.
 */
export function fileKey(file: Pick<File, "name" | "size" | "lastModified">): string {
  return `file:${file.name}:${file.size}:${file.lastModified}`;
}

/**
 * Append picks to the list: duplicates are ignored (a double-click is not two
 * photos), then the cap refuses the rest. `preview` is only asked for picks
 * that become tiles, so an object URL is never created for one that is
 * dropped.
 */
export function addPhotos(
  photos: ComposerPhoto[],
  picks: PhotoPick[],
  preview: (pick: PhotoPick) => string,
): AddResult {
  const next = [...photos];
  const added: AddResult["added"] = [];
  let duplicates = 0;
  let refused = 0;

  for (const pick of picks) {
    const key = pick.kind === "file" ? fileKey(pick.file) : `asset:${pick.id}`;
    const dup = next.some((p) => p.key === key || (pick.kind === "asset" && p.assetId === pick.id));
    if (dup) {
      duplicates += 1;
      continue;
    }
    if (next.length >= MAX_ATTACHMENTS) {
      refused += 1;
      continue;
    }
    const photo: ComposerPhoto =
      pick.kind === "file"
        ? { key, assetId: null, status: "uploading", pct: 0, error: null, preview: preview(pick) }
        : { key, assetId: pick.id, status: "ready", pct: 100, error: null, preview: preview(pick) };
    next.push(photo);
    added.push({ photo, pick });
  }

  return { added, duplicates, refused };
}

export function removePhoto(photos: ComposerPhoto[], key: string): ComposerPhoto[] {
  return photos.filter((p) => p.key !== key);
}

/** Update one tile; a key that is no longer in the list (removed mid-upload) is a no-op. */
export function patchPhoto(
  photos: ComposerPhoto[],
  key: string,
  patch: Partial<Omit<ComposerPhoto, "key">>,
): ComposerPhoto[] {
  return photos.map((p) => (p.key === key ? { ...p, ...patch } : p));
}

/**
 * An Entry is text or at least one Attachment — never neither (SPEC-12 story
 * 7, what the server refuses) — and never one whose photos are not all
 * `ready` (story 11: a saved Entry always renders). A failed photo therefore
 * blocks Save until it is removed.
 */
export function canPost(text: string, photos: ComposerPhoto[]): boolean {
  if (!photos.every((p) => p.status === "ready")) return false;
  return text.trim().length > 0 || photos.length > 0;
}

/** The `asset_ids` to send: tile order is selection order is display order. */
export function readyAssetIds(photos: ComposerPhoto[]): string[] {
  return photos.flatMap((p) => (p.status === "ready" && p.assetId ? [p.assetId] : []));
}

/** Some of one pick did not fit. */
export function capMessage(refused: number): string {
  return `Tối đa ${MAX_ATTACHMENTS} ảnh mỗi bài — ${refused} ảnh bị bỏ qua.`;
}

/** The strip is full and the writer asked for more — said on tap, not in a tooltip. */
export function fullMessage(): string {
  return `Đã đủ ${MAX_ATTACHMENTS} ảnh mỗi bài — bỏ bớt một ảnh để thêm ảnh khác.`;
}
