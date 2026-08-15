"use client";

// Reader chrome (SPEC-02 reader redesign, R1–R3): a fixed top bar (back · title ·
// chapter · chapter-menu · prev/next chapter · settings) and a fixed bottom bar
// (direction-aware page slider + counter). Both slide away when `visible` is false
// (immersive). The slider reads right-to-left in RTL so the thumb tracks the
// reading direction.

import Link from "next/link";
import type { Route } from "next";

export function ReaderChrome({
  visible,
  comicId,
  title,
  chapterLabel,
  dir,
  prevHref,
  nextHref,
  onOpenSettings,
  onOpenChapters,
  onOpenHelp,
  slider,
}: {
  visible: boolean;
  comicId: string;
  title: string;
  chapterLabel: string;
  dir: "ltr" | "rtl";
  prevHref: Route | null;
  nextHref: Route | null;
  onOpenSettings: () => void;
  onOpenChapters: () => void;
  onOpenHelp: () => void;
  slider?: { current: number; total: number; onSeek: (index0: number) => void };
}) {
  return (
    <>
      <header className={`fixed inset-x-0 top-0 z-20 flex items-center gap-1.5 bg-black/80 px-3 py-2 text-white backdrop-blur transition-transform duration-200 motion-reduce:transition-none ${visible ? "" : "-translate-y-full"}`}>
        <Link href={`/library/comic/${comicId}` as Route} aria-label="Về trang truyện" className="shrink-0 rounded-md px-2 py-1 text-lg leading-none text-blue-400 hover:bg-white/10">‹</Link>
        <div className="min-w-0 flex-1 leading-tight">
          <div className="truncate text-sm font-semibold">{title}</div>
          <div className="text-[11px] text-gray-400">{chapterLabel}{dir === "rtl" ? " · manga" : ""}</div>
        </div>
        <ChapBtn href={prevHref} label="Chương trước" glyph="↑" />
        <button type="button" onClick={onOpenChapters} aria-label="Danh sách chương" className="shrink-0 rounded-md px-2 py-1.5 text-sm leading-none text-gray-200 hover:bg-white/10">≡</button>
        <ChapBtn href={nextHref} label="Chương sau" glyph="↓" />
        <button type="button" onClick={onOpenHelp} aria-label="Phím tắt" className="shrink-0 rounded-md px-2 py-1.5 text-sm leading-none text-gray-200 hover:bg-white/10">?</button>
        <button type="button" onClick={onOpenSettings} aria-label="Cài đặt" className="shrink-0 rounded-md px-2 py-1.5 text-lg leading-none text-gray-200 hover:bg-white/10">⚙</button>
      </header>

      {slider && (
        <footer className={`fixed inset-x-0 bottom-0 z-20 flex items-center gap-3 bg-black/80 px-4 py-3 text-white backdrop-blur transition-transform duration-200 motion-reduce:transition-none ${visible ? "" : "translate-y-full"}`}>
          <input
            type="range"
            min={1}
            max={Math.max(1, slider.total)}
            value={Math.min(slider.current, slider.total)}
            onChange={(e) => slider.onSeek(Number(e.target.value) - 1)}
            aria-label="Trang"
            className="min-w-0 flex-1 accent-blue-500"
            style={{ direction: dir === "rtl" ? "rtl" : "ltr" }}
          />
          <span className="shrink-0 font-mono text-xs tabular-nums text-gray-300">
            {String(Math.min(slider.current, slider.total)).padStart(2, "0")} / {String(slider.total).padStart(2, "0")}
          </span>
        </footer>
      )}
    </>
  );
}

function ChapBtn({ href, label, glyph }: { href: Route | null; label: string; glyph: string }) {
  if (!href) return <span aria-hidden className="shrink-0 px-2 py-1.5 text-sm leading-none text-gray-700">{glyph}</span>;
  return (
    <Link href={href} aria-label={label} title={label} className="shrink-0 rounded-md px-2 py-1.5 text-sm leading-none text-gray-200 hover:bg-white/10">{glyph}</Link>
  );
}
