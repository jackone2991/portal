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
