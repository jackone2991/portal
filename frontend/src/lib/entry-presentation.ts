// What a card shows for a journal Entry: its text, its Attachments and its
// Location — derived in ONE place, so the cards never know where those come
// from.
//
// Attachments come from the Entry's `asset_ids` column (SPEC-12 T1); the
// Location still comes out of the markdown body via `attachments.ts` until T3
// gives it columns — that switch changes only this file, and every card that
// reads through it follows untouched.

import { decodeBody } from "./attachments";
import type { Location } from "./geo";

/** The wire fields a card may receive — a journal Entry or a journal stream item. */
export interface EntryLike {
  body_md?: string | null;
  asset_ids?: string[] | null;
}

export interface EntryPresentation {
  /** The Entry's Attachments — Asset ids in display order. */
  assetIds: string[];
  location: Location | null;
  /** The body with any location markup removed — what the author typed. */
  text: string;
}

export function presentEntry(entry: EntryLike): EntryPresentation {
  const body = decodeBody(entry.body_md ?? "");
  return {
    assetIds: entry.asset_ids ?? [],
    location: body.location,
    text: body.text,
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
