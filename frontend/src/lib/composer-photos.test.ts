import { describe, expect, it } from "vitest";
import {
  MAX_ATTACHMENTS,
  addPhotos,
  canPost,
  capMessage,
  fileKey,
  fullMessage,
  patchPhoto,
  readyAssetIds,
  removePhoto,
  storedPhotos,
  type ComposerPhoto,
  type PhotoPick,
} from "./composer-photos";

const preview = (pick: PhotoPick) => (pick.kind === "file" ? `blob:${pick.file.name}` : `thumb:${pick.id}`);

/** What the composer does with a result: append the new tiles to the live list. */
function add(photos: ComposerPhoto[], picks: PhotoPick[]) {
  const r = addPhotos(photos, picks, preview);
  return { ...r, photos: [...photos, ...r.added.map((a) => a.photo)] };
}

function file(name: string, size = 100, lastModified = 1): File {
  return new File([new Uint8Array(size)], name, { type: "image/jpeg", lastModified });
}

function ready(key: string, assetId = key): ComposerPhoto {
  return { key, assetId, status: "ready", pct: 100, error: null, preview: `thumb:${assetId}` };
}

describe("addPhotos", () => {
  it("adds files in selection order as uploading tiles with a preview", () => {
    const r = add([], [{ kind: "file", file: file("a.jpg") }, { kind: "file", file: file("b.jpg") }]);
    expect(r.photos.map((p) => p.preview)).toEqual(["blob:a.jpg", "blob:b.jpg"]);
    expect(r.photos.every((p) => p.status === "uploading" && p.assetId === null && p.pct === 0)).toBe(true);
    expect(r.added.map((a) => a.pick)).toEqual([
      { kind: "file", file: file("a.jpg") },
      { kind: "file", file: file("b.jpg") },
    ]);
    expect(r.duplicates).toBe(0);
    expect(r.refused).toBe(0);
  });

  it("adds a library pick as already ready", () => {
    const r = add([], [{ kind: "asset", id: "x" }]);
    expect(r.photos).toEqual([
      { key: "asset:x", assetId: "x", status: "ready", pct: 100, error: null, preview: "thumb:x" },
    ]);
  });

  it("ignores a file added twice — same name, size and mtime — and counts it", () => {
    const once = add([], [{ kind: "file", file: file("a.jpg") }]);
    const twice = add(once.photos, [{ kind: "file", file: file("a.jpg") }]);
    expect(twice.photos).toHaveLength(1);
    expect(twice.added).toHaveLength(0);
    expect(twice.duplicates).toBe(1);
  });

  it("treats a different mtime as a different file", () => {
    const once = add([], [{ kind: "file", file: file("a.jpg", 100, 1) }]);
    const again = add(once.photos, [{ kind: "file", file: file("a.jpg", 100, 2) }]);
    expect(again.photos).toHaveLength(2);
  });

  it("ignores a library Asset that is already attached, by id", () => {
    const r = add([ready("asset:x", "x")], [{ kind: "asset", id: "x" }]);
    expect(r.photos).toHaveLength(1);
    expect(r.duplicates).toBe(1);
  });

  it("ignores the same file twice within one pick", () => {
    const r = add([], [{ kind: "file", file: file("a.jpg") }, { kind: "file", file: file("a.jpg") }]);
    expect(r.photos).toHaveLength(1);
    expect(r.duplicates).toBe(1);
  });

  it("stops at the cap and counts what it refused, after skipping duplicates", () => {
    const nine = Array.from({ length: MAX_ATTACHMENTS - 1 }, (_, i) => ready(`asset:${i}`, String(i)));
    const r = add(nine, [
      { kind: "asset", id: "0" }, // duplicate — not a slot
      { kind: "file", file: file("ten.jpg") }, // fills the last slot
      { kind: "file", file: file("eleven.jpg") }, // refused
      { kind: "file", file: file("twelve.jpg") }, // refused
    ]);
    expect(r.photos).toHaveLength(MAX_ATTACHMENTS);
    expect(r.photos.at(-1)?.preview).toBe("blob:ten.jpg");
    expect(r.duplicates).toBe(1);
    expect(r.refused).toBe(2);
  });

  it("does not compute a preview for a pick it does not add", () => {
    const calls: string[] = [];
    const spy = (pick: PhotoPick) => {
      calls.push(pick.kind === "file" ? pick.file.name : pick.id);
      return "p";
    };
    const full = Array.from({ length: MAX_ATTACHMENTS }, (_, i) => ready(`asset:${i}`, String(i)));
    addPhotos(full, [{ kind: "asset", id: "0" }, { kind: "file", file: file("over.jpg") }], spy);
    expect(calls).toEqual([]);
  });
});

describe("storedPhotos", () => {
  it("starts an edited Entry with its Attachments as ready tiles, in stored order", () => {
    const tiles = storedPhotos(["b", "a"], preview);
    expect(tiles.map((p) => [p.key, p.assetId, p.status, p.preview])).toEqual([
      ["asset:b", "b", "ready", "thumb:b"],
      ["asset:a", "a", "ready", "thumb:a"],
    ]);
    expect(readyAssetIds(tiles)).toEqual(["b", "a"]);
  });

  it("makes a stored Attachment picked again from the library a duplicate, not a second tile", () => {
    const r = add(storedPhotos(["a"], preview), [{ kind: "asset", id: "a" }]);
    expect(r.photos).toHaveLength(1);
    expect(r.duplicates).toBe(1);
  });
});

describe("removePhoto / patchPhoto", () => {
  it("removes by key and leaves the others in order", () => {
    const list = [ready("a"), ready("b"), ready("c")];
    expect(removePhoto(list, "b").map((p) => p.key)).toEqual(["a", "c"]);
  });

  it("patches one tile and is a no-op for a key that was removed", () => {
    const list = [ready("a")];
    expect(patchPhoto(list, "a", { status: "failed", error: "x" })[0]).toMatchObject({ status: "failed", error: "x" });
    expect(patchPhoto(list, "gone", { status: "failed" })).toEqual(list);
  });
});

describe("canPost", () => {
  const uploading: ComposerPhoto = { key: "u", assetId: null, status: "uploading", pct: 40, error: null, preview: "" };
  const failed: ComposerPhoto = { key: "f", assetId: null, status: "failed", pct: 0, error: "bad", preview: "" };

  it("needs text or at least one photo", () => {
    expect(canPost("", [])).toBe(false);
    expect(canPost("   ", [])).toBe(false);
    expect(canPost("hi", [])).toBe(true);
    expect(canPost("", [ready("a")])).toBe(true);
  });

  it("waits until every photo is ready, even with text", () => {
    expect(canPost("hi", [ready("a"), uploading])).toBe(false);
    expect(canPost("hi", [failed])).toBe(false);
    expect(canPost("hi", removePhoto([failed], "f"))).toBe(true);
  });
});

describe("readyAssetIds", () => {
  it("returns the Asset ids in tile order", () => {
    expect(readyAssetIds([ready("k1", "a"), ready("k2", "b")])).toEqual(["a", "b"]);
  });
});

describe("fileKey / messages", () => {
  it("fingerprints a file by name, size and mtime", () => {
    expect(fileKey({ name: "a.jpg", size: 3, lastModified: 9 })).toBe("file:a.jpg:3:9");
  });

  it("names the cap and how many were dropped", () => {
    expect(capMessage(2)).toContain(String(MAX_ATTACHMENTS));
    expect(capMessage(2)).toContain("2");
    expect(fullMessage()).toContain(String(MAX_ATTACHMENTS));
  });
});
