"use client";

// Reader UI preferences (SPEC-02 reader redesign, R1–R4). Per frontend/CLAUDE.md
// D-32, persistent UI state is owned by Zustand + persist — NOT the server. mode /
// fit / brightness / quality / direction-override live per-device in localStorage.
// The only server-side reader concept is `comics.reading_direction` (R3), a property
// of the work; the user may override it here per device.

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";
import type { ReadingDirection } from "./comic";

export type ReaderMode = "webtoon" | "single" | "double";
export type ReaderFit = "contain" | "width";
export type ReaderQuality = "data" | "std" | "hi";
/** "auto" follows the comic's reading_direction; ltr/rtl force it (R3). */
export type ReaderDirection = "auto" | "ltr" | "rtl";

interface ReaderSettingsState {
  mode: ReaderMode;
  fit: ReaderFit;
  brightness: number; // 45–100 (%). <100 dims the page via an overlay.
  quality: ReaderQuality;
  direction: ReaderDirection;
  setMode: (mode: ReaderMode) => void;
  setFit: (fit: ReaderFit) => void;
  setBrightness: (brightness: number) => void;
  setQuality: (quality: ReaderQuality) => void;
  setDirection: (direction: ReaderDirection) => void;
}

export const useReaderSettings = create<ReaderSettingsState>()(
  persist(
    (set) => ({
      mode: "webtoon", // preserves the shipped P0.3 default
      fit: "contain",
      brightness: 100,
      quality: "std",
      direction: "auto",
      setMode: (mode) => set({ mode }),
      setFit: (fit) => set({ fit }),
      setBrightness: (brightness) => set({ brightness }),
      setQuality: (quality) => set({ quality }),
      setDirection: (direction) => set({ direction }),
    }),
    {
      name: "portal.reader.settings",
      version: 2, // v2 added `direction`
      migrate: (persisted, version) => {
        const s = (persisted ?? {}) as Partial<ReaderSettingsState>;
        if (version < 2 && s.direction === undefined) s.direction = "auto";
        return s as ReaderSettingsState;
      },
      // Guard SSR: the store is created at import time on the server too, where
      // `localStorage` is undefined — hand persist a no-op storage there.
      storage: createJSONStorage(() =>
        typeof window === "undefined"
          ? { getItem: () => null, setItem: () => {}, removeItem: () => {} }
          : window.localStorage,
      ),
    },
  ),
);

/** Image quality → media variant (via lib/comic `variantURL`). */
export function qualityVariant(q: ReaderQuality): "thumb" | "medium" | "poster" {
  return q === "data" ? "thumb" : q === "hi" ? "poster" : "medium";
}

/** Resolve the effective paged direction: the override, else the work's direction
 *  ("vertical" reads left-to-right when forced into a paged mode). */
export function effectiveDirection(override: ReaderDirection, work: ReadingDirection | undefined): "ltr" | "rtl" {
  if (override === "ltr" || override === "rtl") return override;
  return work === "rtl" ? "rtl" : "ltr";
}
