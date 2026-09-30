import { Lightbox } from "portal-frontend";

// Full-screen viewer for an Entry's photos: the picture at `medium`, prev/next
// at the sides (arrow keys and a swipe do the same), the counter below, X or
// the backdrop to close. The images here are inline SVG gradients so the story
// renders without the media API; the real thing shows variant URLs.

const swatch = (hue: number, label: string) =>
  `data:image/svg+xml;utf8,${encodeURIComponent(
    `<svg xmlns="http://www.w3.org/2000/svg" width="1200" height="800"><defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="hsl(${hue} 45% 52%)"/><stop offset="1" stop-color="hsl(${(hue + 35) % 360} 45% 38%)"/></linearGradient></defs><rect width="1200" height="800" fill="url(#g)"/><text x="600" y="420" font-family="sans-serif" font-size="64" fill="rgba(255,255,255,.85)" text-anchor="middle">${label}</text></svg>`,
  )}`;

const images = [
  { src: swatch(20, "1 / 3"), alt: "Ảnh 1 / 3" },
  { src: swatch(200, "2 / 3"), alt: "Ảnh 2 / 3" },
  { src: swatch(120, "3 / 3"), alt: "Ảnh 3 / 3" },
];

const stage = (children: React.ReactNode) => (
  <div
    style={{ position: "relative", transform: "translateZ(0)", minHeight: 560, background: "var(--tpl-bg)" }}
    data-template="v1"
  >
    {children}
  </div>
);

// Opened in the middle of the list, so both arrows are live.
export const Open = () =>
  stage(<Lightbox open images={images} index={1} onIndexChange={() => {}} onClose={() => {}} />);

// At the last photo the "next" arrow is disabled — the ends are ends, no wrap.
export const LastPhoto = () =>
  stage(<Lightbox open images={images} index={2} onIndexChange={() => {}} onClose={() => {}} />);
