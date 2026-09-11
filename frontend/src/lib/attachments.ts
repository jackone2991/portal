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
// even where nothing decodes them. When `asset_ids` lands (P1.5) the photo form
// is the piece to migrate — the decoder here is the only reader.

import type { Place } from "./geo";

const PHOTO_RE = /!\[[^\]]*\]\(asset:([0-9a-fA-F-]{36})\)/;
const PLACE_RE = /\[([^\]]*)\]\(geo:(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)\)/;

export interface Attachments {
  /** Media asset id of the attached photo, if any. */
  photoId: string | null;
  place: Place | null;
  /** The body with both attachment forms removed, whitespace tidied. */
  rest: string;
}

export function encodePhoto(assetId: string): string {
  return `![photo](asset:${assetId})`;
}

export function encodePlace(place: Place): string {
  // `[` and `]` would break the link form; a place name never needs them.
  const name = place.name.replace(/[[\]]/g, "").trim() || "Location";
  return `[${name}](geo:${place.lat},${place.lon})`;
}

export function decodeAttachments(body: string): Attachments {
  const photo = PHOTO_RE.exec(body);
  const place = PLACE_RE.exec(body);
  let rest = body;
  if (photo?.[0]) rest = rest.replace(photo[0], " ");
  if (place?.[0]) rest = rest.replace(place[0], " ");
  return {
    photoId: photo?.[1] ?? null,
    place:
      place && place[1] !== undefined && place[2] && place[3]
        ? { name: place[1], lat: Number(place[2]), lon: Number(place[3]) }
        : null,
    rest: rest
      .replace(/[ \t]{2,}/g, " ")
      .replace(/\n{3,}/g, "\n\n")
      .trim(),
  };
}

/** Compose the entry body the composer posts: text first, attachments last. */
export function composeBody(text: string, photoId: string | null, place: Place | null): string {
  const parts = [text.trim()];
  if (photoId) parts.push(encodePhoto(photoId));
  if (place) parts.push(encodePlace(place));
  return parts.filter(Boolean).join("\n\n");
}
