import { BulkImportModal } from "portal-frontend";
import { useEffect, useRef } from "react";

// Bulk music import (0038). Two modes behind one dialog, and they differ in more
// than convenience: many files upload from the browser with per-file progress and
// names taken from the filename, while one zip goes to the worker, which reads
// embedded tags with ffprobe and so recovers artist and album too.
//
// Fixed overlay — contained to the card like the other popups.

const stage = (children: React.ReactNode) => (
  <div style={{ position: "relative", transform: "translateZ(0)", minHeight: 560, background: "var(--tpl-bg)" }} data-template="v1">
    {children}
  </div>
);

// The mode lives in internal state, so the zip cell clicks its tab on mount —
// same technique as the PostOptionsMenu and AttachPhotoPopup previews.
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

export const Files = () => stage(<BulkImportModal onClose={() => {}} onImported={() => {}} />);

// Zip mode: one upload, and the worker reads embedded tags with ffprobe, so
// artist and album survive. That is the whole reason both modes exist.
export const Zip = () =>
  stage(
    <ClickInto label="Tải tệp .zip">
      <BulkImportModal onClose={() => {}} onImported={() => {}} />
    </ClickInto>,
  );
