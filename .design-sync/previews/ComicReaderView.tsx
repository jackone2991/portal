import { ComicReaderView, DSQuerySeed } from "portal-frontend";

// The whole comic reader screen: chrome, pages, and the settings/chapter sheets
// it owns. It takes only ids and fetches everything itself, which is why it was
// blank — both ["comic", id] and ["comic", id, "pages", chapterId] have to be
// seeded before there is anything to read.
//
// Pages come through variantURL() and so need the same DOM-level stand-in as
// PagedReader (see that file for the reasoning). Reader mode comes from the
// Zustand settings store, so this card shows whatever its default is — which is
// the honest first-run view.

const HUES = [212, 268, 22, 152, 336, 46];

function pageSVG(id: string, w = 800, h = 1180) {
  const n = Number(id.replace(/\D/g, "")) || 1;
  const hue = HUES[(n - 1) % HUES.length];
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
    <rect width="100%" height="100%" fill="hsl(${hue} 12% 13%)"/>
    <circle cx="${w / 2}" cy="132" r="86" fill="hsl(${hue} 22% 34%)"/>
    <text x="${w / 2}" y="164" font-family="sans-serif" font-size="86" font-weight="700" fill="#e5e7eb" text-anchor="middle">${n}</text>
    <rect x="40" y="266" width="${w - 80}" height="${h * 0.34}" fill="hsl(${hue} 18% 26%)" rx="8"/>
    <rect x="40" y="${h * 0.66}" width="${(w - 80) * 0.55}" height="${h * 0.2}" fill="hsl(${hue} 18% 26%)" rx="8"/>
    <rect x="${40 + (w - 80) * 0.6}" y="${h * 0.66}" width="${(w - 80) * 0.4}" height="${h * 0.2}" fill="hsl(${hue} 22% 34%)" rx="8"/>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const ASSET = /\/api\/v1\/assets\/([^/]+)\//;

if (typeof HTMLImageElement !== "undefined" && !(HTMLImageElement as never as { __dsReader?: boolean }).__dsReader) {
  const proto = HTMLImageElement.prototype as unknown as { __dsReader?: boolean };
  const swap = (v: string) => {
    const m = ASSET.exec(v);
    return m ? pageSVG(m[1]) : v;
  };
  const origSet = HTMLImageElement.prototype.setAttribute;
  HTMLImageElement.prototype.setAttribute = function (name: string, value: string) {
    return origSet.call(this, name, name === "src" ? swap(value) : value);
  };
  const desc = Object.getOwnPropertyDescriptor(HTMLImageElement.prototype, "src");
  if (desc?.set) {
    Object.defineProperty(HTMLImageElement.prototype, "src", {
      ...desc,
      set(v: string) {
        desc.set!.call(this, swap(v));
      },
    });
  }
  proto.__dsReader = true;
}

const chapters = Array.from({ length: 6 }, (_, i) => {
  const n = 408 - i;
  return { id: `ch-${n}`, comic_id: "c1", title: `Chương ${n}`, sort_order: n, created_at: "2026-08-20T10:00:00Z" };
});

const comic = {
  id: "c1",
  owner_id: "u1",
  title: "Vạn Cổ Thần Đế",
  description: null,
  cover_asset_id: null,
  status: "published",
  reading_direction: "ltr",
  chapter_count: 6,
  created_at: "2026-08-01T10:00:00Z",
  updated_at: "2026-08-20T10:00:00Z",
  chapters,
};

const pages = Array.from({ length: 5 }, (_, i) => ({
  page_id: `p${i + 1}`,
  asset_id: `a${i + 1}`,
  width: 800,
  height: 1180,
}));

const stage = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed
    seed={[
      [["comic", "c1"], seed],
      [["comic", "c1", "pages", "ch-408"], pages],
    ]}
  >
    <div style={{ position: "relative", transform: "translateZ(0)", height: 560, overflow: "hidden", background: "#0b0b0d" }}>
      {children}
    </div>
  </DSQuerySeed>
);

export const Reader = () => stage(comic, <ComicReaderView id="c1" chapterId="ch-408" />);

// A right-to-left title: the reader takes its direction from the comic itself
// unless the reader settings override it.
export const RightToLeft = () =>
  stage({ ...comic, title: "ワンピース", reading_direction: "rtl" }, <ComicReaderView id="c1" chapterId="ch-408" />);
