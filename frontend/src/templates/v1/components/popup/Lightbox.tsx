"use client";

import { useEffect, useRef } from "react";
import { Icon } from "../ui/Icon";

/**
 * Full-screen image viewer with prev/next (SPEC-12 T2). Controlled like
 * {@link Modal}: the caller owns `index`, so the same list can be opened at any
 * photo. Moves with the arrow keys, the two buttons and a horizontal swipe;
 * Escape, the X or a tap on the backdrop closes it.
 *
 * The order is the caller's array order and the ends are ends — no wrap — so
 * the viewer reads as "the Entry's photos, first to last", the same order the
 * writer composed them in.
 */
export interface LightboxImage {
  src: string;
  alt: string;
}

export interface LightboxProps {
  open: boolean;
  images: LightboxImage[];
  index: number;
  onIndexChange: (index: number) => void;
  onClose: () => void;
}

/** A shorter drag is a tap, not a swipe. */
const SWIPE_PX = 40;

export function Lightbox({ open, images, index, onIndexChange, onClose }: LightboxProps) {
  const touchX = useRef<number | null>(null);
  const count = images.length;
  const image = images[index];

  function go(delta: number) {
    const next = index + delta;
    if (next < 0 || next >= count) return;
    onIndexChange(next);
  }

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
      else if (e.key === "ArrowLeft" && index > 0) onIndexChange(index - 1);
      else if (e.key === "ArrowRight" && index < count - 1) onIndexChange(index + 1);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, index, count, onClose, onIndexChange]);

  if (!open || !image) return null;

  return (
    <div
      className="fixed inset-0 z-[70] flex items-center justify-center select-none"
      style={{ background: "rgba(0,0,0,.92)" }}
      role="dialog"
      aria-modal="true"
      aria-label={`Ảnh ${index + 1} / ${count}`}
      onClick={onClose}
      onTouchStart={(e) => {
        touchX.current = e.touches[0]?.clientX ?? null;
      }}
      onTouchEnd={(e) => {
        const start = touchX.current;
        touchX.current = null;
        const end = e.changedTouches[0]?.clientX;
        if (start === null || end === undefined) return;
        const dx = end - start;
        if (Math.abs(dx) < SWIPE_PX) return;
        go(dx < 0 ? 1 : -1);
      }}
    >
      <button
        type="button"
        onClick={(e) => {
          e.stopPropagation(); // or the backdrop closes it a second time
          onClose();
        }}
        aria-label="Đóng"
        className="absolute right-4 top-4 grid h-10 w-10 place-items-center rounded-full text-white transition hover:bg-white/15"
      >
        <Icon name="close-icon" size={16} />
      </button>

      {count > 1 && (
        <NavBtn side="left" disabled={index === 0} onClick={() => go(-1)} label="Ảnh trước" />
      )}

      {/* eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset */}
      <img
        key={image.src}
        src={image.src}
        alt={image.alt}
        draggable={false}
        onClick={(e) => e.stopPropagation()}
        className="max-h-[92vh] max-w-[94vw] object-contain"
      />

      {count > 1 && (
        <NavBtn side="right" disabled={index === count - 1} onClick={() => go(1)} label="Ảnh sau" />
      )}

      {count > 1 && (
        <span
          className="absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full px-3 py-1 text-xs font-semibold text-white"
          style={{ background: "rgba(255,255,255,.15)" }}
          aria-hidden
        >
          {index + 1} / {count}
        </span>
      )}
    </div>
  );
}

function NavBtn({
  side,
  disabled,
  onClick,
  label,
}: {
  side: "left" | "right";
  disabled: boolean;
  onClick: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      disabled={disabled}
      aria-label={label}
      onClick={(e) => {
        e.stopPropagation();
        onClick();
      }}
      className={`absolute top-1/2 grid h-11 w-11 -translate-y-1/2 place-items-center rounded-full text-white transition hover:bg-white/15 disabled:opacity-25 disabled:hover:bg-transparent ${
        side === "left" ? "left-3" : "right-3"
      }`}
    >
      {/* The sprite's only chevron points down; ±90° turns it into < and >. */}
      <Icon name="dropdown-arrow-icon" size={14} className={side === "left" ? "rotate-90" : "-rotate-90"} />
    </button>
  );
}
