"use client";

// Paged renderer (SPEC-02 reader redesign, R2–R4). Single or double-page, LTR/RTL,
// with a unified gesture layer: tap zones (prev · chrome · next, direction-aware),
// horizontal swipe, double-tap / pinch / wheel / keyboard zoom + drag-to-pan.
// Image-load failures render a React fallback (never DOM surgery — the page key
// changes on flip, so a replaced node would crash React).

import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { variantURL, type ReaderPage } from "@/lib/comic";
import { useReducedMotion } from "./useReducedMotion";

type Zoom = { scale: number; x: number; y: number };
const MIN = 1, MAX = 4;
const zoomReducer = (z: Zoom, a: Partial<Zoom> | "reset"): Zoom =>
  a === "reset" ? { scale: 1, x: 0, y: 0 } : { ...z, ...a };

export function PagedReader({
  pages,
  index,
  mode,
  dir,
  variant,
  fit,
  onPrev,
  onNext,
  onToggleChrome,
}: {
  pages: ReaderPage[];
  index: number;
  mode: "single" | "double";
  dir: "ltr" | "rtl";
  variant: "thumb" | "medium" | "poster";
  fit: "contain" | "width";
  onPrev: () => void; // story-order backward
  onNext: () => void; // story-order forward
  onToggleChrome: () => void;
}) {
  // Which pages this screen shows.
  let shown: ReaderPage[];
  if (mode === "double") {
    const base = index - (index % 2);
    shown = [pages[base], pages[base + 1]].filter(Boolean) as ReaderPage[];
    if (dir === "rtl") shown = [...shown].reverse();
  } else {
    shown = pages[index] ? [pages[index]] : [];
  }
  const shownKey = shown.map((p) => p.page_id).join("|");

  const [zoom, dispatch] = useReducer(zoomReducer, { scale: 1, x: 0, y: 0 });
  const [failed, setFailed] = useState<Set<string>>(() => new Set());
  const containerRef = useRef<HTMLDivElement>(null);
  const reduceMotion = useReducedMotion();

  useEffect(() => dispatch("reset"), [shownKey, mode, fit]); // reset zoom on page/mode change

  const zoomAround = useCallback((factor: number, cx: number, cy: number) => {
    dispatch(
      (() => {
        const el = containerRef.current;
        if (!el) return {};
        const r = el.getBoundingClientRect();
        const scale = Math.min(MAX, Math.max(MIN, zoom.scale * factor));
        if (scale === zoom.scale) return {};
        // keep the point under the cursor stationary
        const px = cx - r.left - r.width / 2;
        const py = cy - r.top - r.height / 2;
        const k = scale / zoom.scale;
        const nx = scale === 1 ? 0 : px - (px - zoom.x) * k;
        const ny = scale === 1 ? 0 : py - (py - zoom.y) * k;
        return { scale, x: nx, y: ny };
      })(),
    );
  }, [zoom]);

  // Tap zones (only when not zoomed): visual-left / centre / visual-right.
  const tapNav = useCallback((clientX: number) => {
    const el = containerRef.current;
    if (!el) return;
    const r = el.getBoundingClientRect();
    const third = (clientX - r.left) / r.width;
    if (third < 0.34) (dir === "rtl" ? onNext : onPrev)();
    else if (third > 0.66) (dir === "rtl" ? onPrev : onNext)();
    else onToggleChrome();
  }, [dir, onPrev, onNext, onToggleChrome]);

  // ── mouse: click nav, dblclick zoom, wheel zoom, drag pan ──
  const drag = useRef<{ x: number; y: number; moved: boolean } | null>(null);
  const onMouseDown = (e: React.MouseEvent) => { if (zoom.scale > 1) drag.current = { x: e.clientX - zoom.x, y: e.clientY - zoom.y, moved: false }; };
  const onMouseMove = (e: React.MouseEvent) => { if (drag.current && zoom.scale > 1) { drag.current.moved = true; dispatch({ x: e.clientX - drag.current.x, y: e.clientY - drag.current.y }); } };
  const onMouseUp = () => { drag.current = null; };
  const onClick = (e: React.MouseEvent) => { if (zoom.scale === 1) tapNav(e.clientX); };
  const onDoubleClick = (e: React.MouseEvent) => { e.preventDefault(); if (zoom.scale > 1) dispatch("reset"); else zoomAround(2.5, e.clientX, e.clientY); };
  const onWheel = (e: React.WheelEvent) => { zoomAround(e.deltaY < 0 ? 1.15 : 1 / 1.15, e.clientX, e.clientY); };

  // ── touch: swipe (scale 1), pan (zoomed), pinch (2 fingers), double-tap ──
  const touch = useRef<{ x: number; y: number; t: number; d0: number; s0: number; px: number; py: number; lastTap: number }>({ x: 0, y: 0, t: 0, d0: 0, s0: 1, px: 0, py: 0, lastTap: 0 });
  const onTouchStart = (e: React.TouchEvent) => {
    const t = touch.current;
    const a = e.touches[0], b = e.touches[1];
    if (a && b) {
      t.d0 = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
      t.s0 = zoom.scale;
      t.px = (a.clientX + b.clientX) / 2;
      t.py = (a.clientY + b.clientY) / 2;
    } else if (a) {
      t.x = a.clientX; t.y = a.clientY; t.t = Date.now();
    }
  };
  const onTouchMove = (e: React.TouchEvent) => {
    const t = touch.current;
    const a = e.touches[0], b = e.touches[1];
    if (a && b && t.d0 > 0) {
      const d = Math.hypot(a.clientX - b.clientX, a.clientY - b.clientY);
      const scale = Math.min(MAX, Math.max(MIN, t.s0 * (d / t.d0)));
      dispatch({ scale, x: scale === 1 ? 0 : zoom.x, y: scale === 1 ? 0 : zoom.y });
    } else if (a && zoom.scale > 1) {
      dispatch({ x: zoom.x + (a.clientX - t.x), y: zoom.y + (a.clientY - t.y) });
      t.x = a.clientX; t.y = a.clientY;
    }
  };
  const onTouchEnd = (e: React.TouchEvent) => {
    const t = touch.current;
    if (e.touches.length > 0) return;
    const end = e.changedTouches[0];
    const ex = end?.clientX ?? t.x, ey = end?.clientY ?? t.y;
    const dx = ex - t.x, dy = ey - t.y;
    const dt = Date.now() - t.t;
    if (zoom.scale === 1 && Math.abs(dx) < 8 && Math.abs(dy) < 8 && dt < 250) {
      const now = Date.now();
      if (now - t.lastTap < 300) { t.lastTap = 0; zoomAround(2.5, ex, ey); }
      else { t.lastTap = now; tapNav(ex); }
    } else if (zoom.scale === 1 && Math.abs(dx) > 45 && Math.abs(dx) > Math.abs(dy)) {
      (dx < 0 ? (dir === "rtl" ? onPrev : onNext) : (dir === "rtl" ? onNext : onPrev))();
    }
    t.d0 = 0;
  };

  // keyboard zoom (+/-/0) while a paged reader is mounted
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "+" || e.key === "=") { e.preventDefault(); const el = containerRef.current; if (el) { const r = el.getBoundingClientRect(); zoomAround(1.25, r.left + r.width / 2, r.top + r.height / 2); } }
      else if (e.key === "-" || e.key === "_") { e.preventDefault(); const el = containerRef.current; if (el) { const r = el.getBoundingClientRect(); zoomAround(1 / 1.25, r.left + r.width / 2, r.top + r.height / 2); } }
      else if (e.key === "0") { e.preventDefault(); dispatch("reset"); }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [zoomAround]);

  const imgClass = fit === "width" && mode === "single" ? "w-full" : "max-h-full object-contain";

  return (
    <div
      ref={containerRef}
      className="relative flex h-[100dvh] w-full touch-none select-none items-center justify-center overflow-hidden bg-black"
      style={{ cursor: zoom.scale > 1 ? "grab" : "default" }}
      onClick={onClick}
      onDoubleClick={onDoubleClick}
      onWheel={onWheel}
      onMouseDown={onMouseDown}
      onMouseMove={onMouseMove}
      onMouseUp={onMouseUp}
      onMouseLeave={onMouseUp}
      onTouchStart={onTouchStart}
      onTouchMove={onTouchMove}
      onTouchEnd={onTouchEnd}
    >
      <div
        className="flex h-full max-h-full items-center justify-center gap-0.5"
        style={{ transform: `translate(${zoom.x}px, ${zoom.y}px) scale(${zoom.scale})`, transition: reduceMotion || drag.current || touch.current.d0 ? "none" : "transform .15s ease" }}
      >
        {shown.length === 0 ? (
          <div className="text-gray-500">Chương này chưa có trang.</div>
        ) : (
          shown.map((p) =>
            failed.has(p.page_id) ? (
              <div key={p.page_id} className="px-6 text-center text-gray-500">⚠ Trang không tải được</div>
            ) : (
              <img
                key={p.page_id}
                src={variantURL(p.asset_id, variant)}
                width={p.width ?? undefined}
                height={p.height ?? undefined}
                alt=""
                draggable={false}
                decoding="async"
                className={`${imgClass} ${mode === "double" ? "max-w-[50%]" : "max-w-full"}`}
                onError={() => setFailed((s) => { const n = new Set(s); n.add(p.page_id); return n; })}
              />
            ),
          )
        )}
      </div>
    </div>
  );
}
