// Post attachments (photo + location) encoded inside the journal entry body.
//
// WHY IN THE BODY. A journal entry has exactly three writable fields —
// `body_md`, `mood`, `occurred_at`. `asset_ids` exists in the table but the
// service rejects any request that sets it (`ErrInvalidAsset`, "P1.5 not
// shipped — fail closed"), and there is no location column at all. So the
// composer's photo and location attachments are written into the markdown body
// as two link forms, and the feed reads them back out:
//
//   photo     ![photo](asset:<uuid>)          → rendered from the media module
//   location  [<name>](geo:<lat>,<lon>)       → rendered as a pin chip
//
// Both are ordinary markdown links, so an entry stays readable and portable
// even where nothing decodes them.
//
// SPEC-12 retires this file: T1 moves photos into `asset_ids`, T3 moves the
// Location into columns, and each deletes its half here. Until then the cards
// read it through `entry-presentation.ts` — the ONE module that knows where an
// Entry's Attachments and Location come from — never directly.

import type { Location } from "./geo";

const PHOTO_RE = /!\[[^\]]*\]\(asset:([0-9a-fA-F-]{36})\)/;
const LOCATION_RE = /\[([^\]]*)\]\(geo:(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)\)/;

export interface Attachments {
  /** Media asset id of the attached photo, if any. */
  photoId: string | null;
  location: Location | null;
  /** The body with both attachment forms removed, whitespace tidied. */
  rest: string;
}

export function encodePhoto(assetId: string): string {
  return `![photo](asset:${assetId})`;
}

export function encodeLocation(location: Location): string {
  // `[` and `]` would break the link form; a Location name never needs them.
  const name = location.name.replace(/[[\]]/g, "").trim() || "Location";
  return `[${name}](geo:${location.lat},${location.lon})`;
}

export function decodeAttachments(body: string): Attachments {
  const photo = PHOTO_RE.exec(body);
  const loc = LOCATION_RE.exec(body);
  let rest = body;
  if (photo?.[0]) rest = rest.replace(photo[0], " ");
  if (loc?.[0]) rest = rest.replace(loc[0], " ");
  return {
    photoId: photo?.[1] ?? null,
    location:
      loc && loc[1] !== undefined && loc[2] && loc[3]
        ? { name: loc[1], lat: Number(loc[2]), lon: Number(loc[3]) }
        : null,
    rest: rest
      .replace(/[ \t]{2,}/g, " ")
      .replace(/\n{3,}/g, "\n\n")
      .trim(),
  };
}

/**
 * Compose the entry body the composer posts: text first, attachments last.
 * Takes the composer's Attachment list (Asset ids) but encodes only the first —
 * the body form has room for one, which is exactly the limit T1 lifts.
 */
export function composeBody(
  text: string,
  assetIds: readonly string[],
  location: Location | null,
): string {
  const parts = [text.trim()];
  const [first] = assetIds;
  if (first) parts.push(encodePhoto(first));
  if (location) parts.push(encodeLocation(location));
  return parts.filter(Boolean).join("\n\n");
}
