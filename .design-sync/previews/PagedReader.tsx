import { PagedReader } from "portal-frontend";

// Single/double-page comic reader. Pages are fetched through `variantURL()`,
// which points at the media API — unreachable from a preview, so every page
// would hit the component's own "⚠ Trang không tải được" branch and the card
// would teach that the reader is broken.
//
// So the preview intercepts image loads at the DOM level and serves a drawn
// stand-in page instead. This is a preview-local hack in the same family as
// GoToTop's forced visibility: it changes nothing about the component, it only
// gives it something to render. Drop it the day previews can reach real assets.
//
// The stand-in is derived from the ASSET ID in the URL, not from a counter:
// each page must look different and always the same, or a reversed RTL spread
// is indistinguishable from an LTR one and the two cards grade as identical.

const HUES = [212, 268, 22, 152, 336, 46];

function pageSVG(id: string, w = 800, h = 1180) {
  const n = Number(id.replace(/\D/g, "")) || 1;
  const hue = HUES[(n - 1) % HUES.length];
  const ink = `hsl(${hue} 18% 26%)`;
  const ink2 = `hsl(${hue} 22% 34%)`;
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">
    <rect width="100%" height="100%" fill="hsl(${hue} 12% 13%)"/>
    <circle cx="${w / 2}" cy="132" r="86" fill="${ink2}"/>
    <text x="${w / 2}" y="164" font-family="sans-serif" font-size="86" font-weight="700" fill="#e5e7eb" text-anchor="middle">${n}</text>
    <rect x="40" y="266" width="${w - 80}" height="${h * 0.34}" fill="${ink}" rx="8"/>
    <rect x="40" y="${h * 0.66}" width="${(w - 80) * 0.55}" height="${h * 0.2}" fill="${ink}" rx="8"/>
    <rect x="${40 + (w - 80) * 0.6}" y="${h * 0.66}" width="${(w - 80) * 0.4}" height="${h * 0.2}" fill="${ink2}" rx="8"/>
  </svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

const ASSET = /\/api\/v1\/assets\/([^/]+)\//;

if (typeof HTMLImageElement !== "undefined" && !(HTMLImageElement as never as { __dsPatched?: boolean }).__dsPatched) {
  const proto = HTMLImageElement.prototype as unknown as { __dsPatched?: boolean };
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
  proto.__dsPatched = true;
}

const pages = Array.from({ length: 6 }, (_, i) => ({
  page_id: `p${i + 1}`,
  asset_id: `a${i + 1}`,
  width: 800,
  height: 1180,
}));

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", height: 520, background: "#0b0b0d", overflow: "hidden" }}>{children}</div>
);

const handlers = { onPrev: () => {}, onNext: () => {}, onToggleChrome: () => {} };

export const SinglePage = () =>
  stage(<PagedReader pages={pages} index={1} mode="single" dir="ltr" variant="medium" fit="contain" {...handlers} />);

// Double-page spread — the mode a scanned manga volume is read in. Pages 3 then 4.
export const DoublePage = () =>
  stage(<PagedReader pages={pages} index={2} mode="double" dir="ltr" variant="medium" fit="contain" {...handlers} />);

// The same spread read right-to-left: the numbers run 4 then 3. That reversal is
// the whole point of the mode, and it is why the stand-in pages are numbered.
export const RightToLeft = () =>
  stage(<PagedReader pages={pages} index={2} mode="double" dir="rtl" variant="medium" fit="contain" {...handlers} />);

// fit="width" fills the viewport width instead of fitting the whole page —
// the setting you want on a phone.
export const FitWidth = () =>
  stage(<PagedReader pages={pages} index={1} mode="single" dir="ltr" variant="medium" fit="width" {...handlers} />);
