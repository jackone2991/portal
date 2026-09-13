// The Location encoded inside the journal entry body.
//
// WHY IN THE BODY. There is no Location column on a journal Entry yet, so the
// composer writes the Location it picked into the markdown body as one link
// form, and the feed reads it back out:
//
//   location  [<name>](geo:<lat>,<lon>)       → rendered as a pin chip
//
// It is an ordinary markdown link, so an entry stays readable and portable
// even where nothing decodes it.
//
// The Attachment half of this file is gone: SPEC-12 T1 moved Attachments into
// the Entry's `asset_ids` column (migration 0044 cleaned every existing body),
// so there is no `![photo](asset:…)` form to write or read any more — do not
// bring it back by habit. T3 does the same for the Location and deletes this
// file. Until then the cards read it through `entry-presentation.ts` — the ONE
// module that knows where an Entry's Location comes from — never directly.

import type { Location } from "./geo";

const LOCATION_RE = /\[([^\]]*)\]\(geo:(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)\)/;

/** A body split into the Location it encodes and the text the author typed. */
export interface DecodedBody {
  location: Location | null;
  /** The body with the location form removed, whitespace tidied. */
  text: string;
}

export function encodeLocation(location: Location): string {
  // `[` and `]` would break the link form; a Location name never needs them.
  const name = location.name.replace(/[[\]]/g, "").trim() || "Location";
  return `[${name}](geo:${location.lat},${location.lon})`;
}

export function decodeBody(body: string): DecodedBody {
  const loc = LOCATION_RE.exec(body);
  let text = body;
  if (loc?.[0]) text = text.replace(loc[0], " ");
  return {
    location:
      loc && loc[1] !== undefined && loc[2] && loc[3]
        ? { name: loc[1], lat: Number(loc[2]), lon: Number(loc[3]) }
        : null,
    text: text
      .replace(/[ \t]{2,}/g, " ")
      .replace(/\n{3,}/g, "\n\n")
      .trim(),
  };
}

/**
 * Compose the entry body the composer posts: text first, the Location last.
 * Attachments are NOT part of the body — they travel as `asset_ids`.
 */
export function composeBody(text: string, location: Location | null): string {
  const parts = [text.trim()];
  if (location) parts.push(encodeLocation(location));
  return parts.filter(Boolean).join("\n\n");
}
