// What a card shows for a journal Entry: its text, its Attachments and its
// Location — derived in ONE place, so the cards never know where those come
// from.
//
// Today (SPEC-12 T0) everything still comes out of the markdown body via
// `attachments.ts`. T1 switches `assetIds` to the Entry's `asset_ids` column and
// T3 switches `location` to its columns; both change only this file, and every
// card that reads through it follows untouched. (The calendar still prints
// `body_md` raw — a pre-existing gap T1's migration closes by cleaning the body.)

import { decodeAttachments } from "./attachments";
import type { Location } from "./geo";

/** The wire fields a card may receive — a journal Entry or a journal stream item. */
export interface EntryLike {
  body_md?: string | null;
}

export interface EntryPresentation {
  /** The Entry's Attachments — Asset ids in display order (T0: at most one, from the body). */
  assetIds: string[];
  location: Location | null;
  /** The body with any attachment markup removed — what the author typed. */
  text: string;
}

export function presentEntry(entry: EntryLike): EntryPresentation {
  const att = decodeAttachments(entry.body_md ?? "");
  return {
    assetIds: att.photoId ? [att.photoId] : [],
    location: att.location,
    text: att.rest,
  };
}
