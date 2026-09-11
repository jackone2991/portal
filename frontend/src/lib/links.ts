// URL extraction for the newsfeed link-preview card (SPEC-06 home).
//
// A journal entry is just markdown, so a "shared a link" post is not a distinct
// record type — it is an entry whose body contains a URL. The newsfeed derives
// the Olympus link/video card from that body client-side: no crawler, no
// og:image fetch, no extra table. What we can honestly show is the host and the
// text the author wrote; the thumbnail is a deterministic placeholder.

const URL_RE = /https?:\/\/[^\s<>"'`)\]]+/gi;

/** Hosts whose links get the play-button treatment (Olympus `.post-video`). */
const VIDEO_HOSTS = [
  "youtube.com",
  "youtu.be",
  "vimeo.com",
  "dailymotion.com",
  "twitch.tv",
  "tiktok.com",
  "soundcloud.com",
  "spotify.com",
];

export interface LinkPreview {
  url: string;
  /** Bare host, `www.` stripped — shown as the Olympus `.link-site` line. */
  host: string;
  /** `video` renders the play overlay; `link` renders a plain thumbnail. */
  kind: "video" | "link";
}

/** Trailing sentence punctuation is almost never part of a pasted URL. */
function trimTrailing(url: string): string {
  return url.replace(/[.,;:!?]+$/, "");
}

/**
 * First URL in `body`, or null. Anything `URL` can't parse is skipped, so a
 * malformed match never reaches the card.
 */
export function firstLink(body: string): LinkPreview | null {
  const matches = body.match(URL_RE);
  if (!matches?.length) return null;
  for (const raw of matches) {
    const url = trimTrailing(raw);
    let host: string;
    try {
      host = new URL(url).hostname.replace(/^www\./, "");
    } catch {
      continue;
    }
    const kind = VIDEO_HOSTS.some((h) => host === h || host.endsWith(`.${h}`))
      ? "video"
      : "link";
    return { url, host, kind };
  }
  return null;
}

/**
 * `body` with `url` removed and the resulting whitespace tidied — the raw URL
 * moves into the preview card, the way it does on every social feed, so the
 * paragraph doesn't repeat it. An empty result means "the post was only a link".
 */
export function stripLink(body: string, url: string): string {
  return body
    .split(url)
    .join(" ")
    .replace(/[ \t]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .trim();
}

/** Split text into plain runs and URL runs so a renderer can anchor the URLs. */
export function splitLinks(text: string): { text: string; href?: string }[] {
  const out: { text: string; href?: string }[] = [];
  let last = 0;
  for (const m of text.matchAll(URL_RE)) {
    const start = m.index ?? 0;
    const url = trimTrailing(m[0]);
    if (start > last) out.push({ text: text.slice(last, start) });
    out.push({ text: url, href: url });
    last = start + url.length;
  }
  if (last < text.length) out.push({ text: text.slice(last) });
  return out;
}
