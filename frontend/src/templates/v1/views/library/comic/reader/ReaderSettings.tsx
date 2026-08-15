"use client";

// Reader settings sheet (SPEC-02 reader redesign, R1–R3). Reads/writes the persisted
// Zustand store (lib/reader-settings). Bottom-sheet modal; every change is applied
// live and remembered per device.

import type { ReactNode } from "react";
import { useReaderSettings, type ReaderDirection, type ReaderFit, type ReaderMode, type ReaderQuality } from "@/lib/reader-settings";

export function ReaderSettings({ open, onClose }: { open: boolean; onClose: () => void }) {
  const { mode, fit, brightness, quality, direction, setMode, setFit, setBrightness, setQuality, setDirection } = useReaderSettings();
  if (!open) return null;

  return (
    <div className="fixed inset-0 z-30 flex items-end justify-center bg-black/60" onClick={onClose} role="dialog" aria-modal="true" aria-label="Cài đặt đọc">
      <div className="w-full max-w-md rounded-t-2xl border-t border-white/10 bg-neutral-900 p-5 text-white" onClick={(e) => e.stopPropagation()}>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-xs font-bold uppercase tracking-wider text-gray-400">Cài đặt đọc</h2>
          <button type="button" onClick={onClose} className="rounded-md bg-blue-600 px-3 py-1 text-sm font-medium hover:bg-blue-500">Xong</button>
        </div>

        <div className="space-y-4">
          <Row label="Chế độ">
            <Seg value={mode} onChange={(v) => setMode(v as ReaderMode)} options={[{ v: "webtoon", label: "Cuộn dọc" }, { v: "single", label: "Trang đơn" }, { v: "double", label: "Trang đôi" }]} />
          </Row>
          <Row label="Hướng đọc">
            <Seg value={direction} onChange={(v) => setDirection(v as ReaderDirection)} options={[{ v: "auto", label: "Tự động" }, { v: "ltr", label: "Trái→Phải" }, { v: "rtl", label: "Phải→Trái" }]} />
          </Row>
          <Row label="Khít trang">
            <Seg value={fit} onChange={(v) => setFit(v as ReaderFit)} options={[{ v: "contain", label: "Vừa màn" }, { v: "width", label: "Vừa ngang" }]} />
          </Row>
          <Row label="Độ sáng">
            <input type="range" min={45} max={100} value={brightness} onChange={(e) => setBrightness(Number(e.target.value))} className="w-40 accent-blue-500" aria-label="Độ sáng" />
          </Row>
          <Row label="Chất lượng ảnh">
            <Seg value={quality} onChange={(v) => setQuality(v as ReaderQuality)} options={[{ v: "data", label: "Tiết kiệm" }, { v: "std", label: "Chuẩn" }, { v: "hi", label: "Cao" }]} />
          </Row>
        </div>
      </div>
    </div>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-3">
      <span className="shrink-0 text-sm font-medium text-gray-200">{label}</span>
      {children}
    </div>
  );
}

function Seg({ value, options, onChange }: { value: string; options: { v: string; label: string }[]; onChange: (v: string) => void }) {
  return (
    <div className="inline-flex flex-wrap justify-end gap-1 rounded-lg bg-white/10 p-1">
      {options.map((o) => (
        <button
          key={o.v}
          type="button"
          onClick={() => onChange(o.v)}
          aria-pressed={value === o.v}
          className={`rounded-md px-3 py-1.5 text-xs font-semibold transition ${value === o.v ? "bg-blue-600 text-white" : "text-gray-300 hover:text-white"}`}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}
