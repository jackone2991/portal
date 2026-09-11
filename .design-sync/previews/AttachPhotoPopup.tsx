import { AttachPhotoPopup, DSQuerySeed } from "portal-frontend";
import { useEffect, useRef } from "react";

// Photo picker for the composer. It has two panes and opens on the chooser, so a
// plain render only ever shows the two big buttons — the gallery, which is the
// interesting half, is one click away and its `pane` state is internal.
//
// So the gallery cell clicks "Choose from My Photos" on mount, the same way the
// PostOptionsMenu preview opens its menu: through the component's own DOM, not
// its internals. The grid reads ["assets","image","ready"], seeded below.
//
// Thumbnails point at the media API and will not resolve here — the tiles render
// as the empty frames they are, which is the honest look of "these images exist
// but this page cannot reach them".

const asset = (id: string, name: string) => ({
  id,
  owner_id: "u1",
  kind: "image",
  status: "ready",
  original_filename: name,
  content_type: "image/jpeg",
  size_bytes: 482_113,
  created_at: "2026-09-05T09:00:00Z",
  updated_at: "2026-09-05T09:00:00Z",
});

const assets = [
  asset("a1", "bien-nha-trang.jpg"),
  asset("a2", "ca-phe-sang.jpg"),
  asset("a3", "sinh-nhat-ha.jpg"),
  asset("a4", "deo-hai-van.jpg"),
  asset("a5", "meo-hang-xom.jpg"),
  asset("a6", "man-hinh-lam-viec.jpg"),
];

function ClickInto({ label, children }: { label: string; children: React.ReactNode }) {
  const host = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const hit = [...(host.current?.querySelectorAll("button") ?? [])].find((b) =>
      (b.textContent ?? "").includes(label),
    );
    hit?.click();
  }, [label]);
  return <div ref={host}>{children}</div>;
}

const stage = (seed: unknown, children: React.ReactNode) => (
  <DSQuerySeed seed={[[["assets", "image", "ready"], seed]]}>
    <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 560, background: "var(--tpl-bg)" }} data-template="v1">
      {children}
    </div>
  </DSQuerySeed>
);

export const Choose = () => stage({ assets }, <AttachPhotoPopup open onClose={() => {}} onPick={() => {}} />);

export const Gallery = () =>
  stage(
    { assets },
    <ClickInto label="Choose from My Photos">
      <AttachPhotoPopup open onClose={() => {}} onPick={() => {}} />
    </ClickInto>,
  );

// Nothing uploaded yet: the gallery has to say so rather than showing an empty
// grid with no explanation.
export const GalleryEmpty = () =>
  stage(
    { assets: [] },
    <ClickInto label="Choose from My Photos">
      <AttachPhotoPopup open onClose={() => {}} onPick={() => {}} />
    </ClickInto>,
  );
