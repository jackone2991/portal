// What a card shows for a journal Entry: its text, its Attachments and its
// Location — derived in ONE place, so the cards never know where those come
// from.
//
// Since SPEC-12 T1/T3 all three are columns on the Entry — `body_md` is plain
// markdown, `asset_ids` the Attachments, `location` the Location — and the
// journal Entry and the journal stream item carry them in the same shape. The
// markdown-link workaround that once smuggled the photo and the place through
// the body is gone (migration 0044/0045 cleaned every row); this module is
// where it would have to come back, and it must not.

import type { Location } from "./geo";

/** The wire fields a card may receive — a journal Entry or a journal stream item. */
export interface EntryLike {
  body_md?: string | null;
  asset_ids?: string[] | null;
  location?: Location | null;
}

export interface EntryPresentation {
  /** The Entry's Attachments — Asset ids in display order. */
  assetIds: string[];
  location: Location | null;
  /** What the author typed — plain markdown, nothing encoded in it. */
  text: string;
}

export function presentEntry(entry: EntryLike): EntryPresentation {
  return {
    assetIds: entry.asset_ids ?? [],
    location: entry.location ?? null,
    text: (entry.body_md ?? "").trim(),
  };
}

/** How many `thumb` tiles the card shows under the hero before it says "+N". */
export const GALLERY_THUMBS = 4;

export interface AttachmentLayout {
  /** The first Attachment, shown large at `medium`; null when there are none. */
  hero: string | null;
  /** The next few, as a row of `thumb` variants in array order. */
  thumbs: string[];
  /** Attachments beyond the row — the "+N" badge; 0 hides it. */
  overflow: number;
}

/**
 * Split an Entry's Attachments into what the card draws (SPEC-12 T2). One
 * photo is a hero alone, so an Entry that always had one looks as it did; the
 * lightbox still walks the whole array, so the layout hides nothing for good.
 */
export function attachmentLayout(assetIds: string[]): AttachmentLayout {
  const [hero = null, ...rest] = assetIds;
  return {
    hero,
    thumbs: rest.slice(0, GALLERY_THUMBS),
    overflow: Math.max(0, rest.length - GALLERY_THUMBS),
  };
}
