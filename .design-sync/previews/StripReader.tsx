import { StripReader } from "portal-frontend";
import { useRef } from "react";

// Webtoon reader: one continuous vertical strip, with the next chapter appended
// as you approach the end. Same image problem as PagedReader — `variantURL()`
// points at the media API, so the preview swaps page loads for a drawn stand-in
// at the DOM level (see PagedReader.tsx for the reasoning; the patch is repeated
// here because previews compile as standalone files).
//
// Frames are deliberately short so several stack inside one card: a webtoon is
// defined by panels butted together with no gap, and a card showing one frame
// does not show that.
//
// It also calls getChapterPages() to prefetch the next chapter. That request
// fails in a preview and is swallowed — the strip simply stops at the pages it
// was handed, which is the correct end-of-content behaviour anyway.

function stripSVG(id: string, w = 800, h = 340) {
  const n = Number(id.replace(/\D/g, "")) || 1;
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
    <rect width="100%" height="100%" fill="#16181d"/>
    <rect x="0" y="0" width="${w}" height="${h * 0.5}" fill="#22262e"/>
    <circle cx="${w * 0.3}" cy="${h * 0.24}" r="${h * 0.11}" fill="#2f3541"/>
    <rect x="${w * 0.52}" y="${h * 0.14}" width="${w * 0.36}" height="${h * 0.2}" fill="#2f3541" rx="10"/>
    <rect x="${w * 0.08}" y="${h * 0.58}" width="${w * 0.84}" height="${h * 0.3}" fill="#22262e" rx="10"/>
    <text x="${w / 2}" y="70" font-family="sans-serif" font-size="40" font-weight="700" fill="#8b95a3" text-anchor="middle">Khung ${n}</text>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const ASSET = /\/api\/v1\/assets\/([^/]+)\//;

if (typeof HTMLImageElement !== "undefined" && !(HTMLImageElement as never as { __dsStrip?: boolean }).__dsStrip) {
  const proto = HTMLImageElement.prototype as unknown as { __dsStrip?: boolean };
  const origSet = HTMLImageElement.prototype.setAttribute;
  HTMLImageElement.prototype.setAttribute = function (name: string, value: string) {
    if (name === "src") {
      const m = ASSET.exec(value);
      if (m) value = stripSVG(m[1]);
    }
    return origSet.call(this, name, value);
  };
  const desc = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, "src");
  if (desc?.set) {
    Object.defineProperty(HTMLImageElement.prototype, "src", {
      ...desc,
      set(v: string) {
        const m = ASSET.exec(v);
        desc.set!.call(this, m ? stripSVG(m[1]) : v);
      },
    });
  }
  proto.__dsStrip = true;
}

const chapters = [
  { id: "ch-408", comic_id: "c1", title: "Chương 408", sort_order: 408, created_at: "2026-08-20T10:00:00Z" },
  { id: "ch-409", comic_id: "c1", title: "Chương 409", sort_order: 409, created_at: "2026-08-21T10:00:00Z" },
];

const pages = Array.from({ length: 4 }, (_, i) => ({
  page_id: `p${i + 1}`,
  asset_id: `a${i + 1}`,
  width: 800,
  height: 340,
}));

export const Webtoon = () => {
  const scrollRef = useRef<HTMLDivElement>(null);
  return (
    <div ref={scrollRef} style={{ height: 560, overflowY: "auto", background: "#0b0b0d" }}>
      <StripReader
        comicId="c1"
        chapters={chapters}
        startChapterId="ch-408"
        startPages={pages}
        variant="medium"
        initialPageId={null}
        scrollRef={scrollRef}
        onActive={() => {}}
        onToggleChrome={() => {}}
      />
    </div>
  );
};
