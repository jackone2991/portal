import { describe, expect, it } from "vitest";
import { GALLERY_THUMBS, attachmentLayout, presentEntry } from "./entry-presentation";

const ids = (n: number) => Array.from({ length: n }, (_, i) => `a${i}`);

describe("attachmentLayout", () => {
  it("has nothing to show for no Attachments", () => {
    expect(attachmentLayout([])).toEqual({ hero: null, thumbs: [], overflow: 0 });
  });

  it("one photo is a hero alone — the card looks as it did before", () => {
    expect(attachmentLayout(["a0"])).toEqual({ hero: "a0", thumbs: [], overflow: 0 });
  });

  it("two or more: the first is the hero, the rest are thumbs in array order", () => {
    expect(attachmentLayout(ids(3))).toEqual({ hero: "a0", thumbs: ["a1", "a2"], overflow: 0 });
  });

  it("five fill the thumb row exactly, with no badge", () => {
    const l = attachmentLayout(ids(5));
    expect(l.thumbs).toHaveLength(GALLERY_THUMBS);
    expect(l.overflow).toBe(0);
  });

  it("six or more keep four thumbs and count the rest as +N", () => {
    expect(attachmentLayout(ids(6))).toEqual({ hero: "a0", thumbs: ["a1", "a2", "a3", "a4"], overflow: 1 });
    expect(attachmentLayout(ids(10)).overflow).toBe(5);
  });
});

describe("presentEntry", () => {
  it("reads Attachments from asset_ids and tolerates their absence", () => {
    expect(presentEntry({ body_md: "hi", asset_ids: ["x", "y"] }).assetIds).toEqual(["x", "y"]);
    expect(presentEntry({ body_md: "hi" }).assetIds).toEqual([]);
    expect(presentEntry({ body_md: "hi", asset_ids: null }).assetIds).toEqual([]);
  });

  it("reads the Location from the field and tolerates its absence", () => {
    const loc = { name: "Hoàn Kiếm", lat: 21.0286, lon: 105.8506 };
    expect(presentEntry({ body_md: "hi", location: loc }).location).toEqual(loc);
    expect(presentEntry({ body_md: "hi" }).location).toBeNull();
    expect(presentEntry({ body_md: "hi", location: null }).location).toBeNull();
  });

  it("leaves the body alone — a link-shaped body is text, not a Location", () => {
    const shown = presentEntry({ body_md: "  see [Hoàn Kiếm](geo:21.0286,105.8506)  " });
    expect(shown.text).toBe("see [Hoàn Kiếm](geo:21.0286,105.8506)");
    expect(shown.location).toBeNull();
  });
});
