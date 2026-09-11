"use client";

// Whether the app shell's two side panels are collapsed.
//
// This is the textbook case from frontend/CLAUDE.md D-32 — "UI state
// (persistent) → Zustand + persist: theme, **sidebar collapsed**, layout
// density". It used to be `useState` inside MasterBase, which meant the collapse
// button worked and then forgot: every reload reopened both panels, so a user
// who wants the rail out of the way had to close it again on every visit. That
// reads as a broken button, not as a missing preference.
//
// Per-device on purpose. Whether the right rail is open depends on the screen
// you are sitting at, not on who you are, so it belongs in localStorage rather
// than on the account.

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

interface ShellLayoutState {
  /** Left menu collapsed to its icon rail. */
  menuCollapsed: boolean;
  /** Right people panel collapsed to its avatar strip. */
  peopleCollapsed: boolean;
  toggleMenu: () => void;
  togglePeople: () => void;
}

export const useShellLayout = create<ShellLayoutState>()(
  persist(
    (set) => ({
      // Both open by default: the shipped look, and the one that shows a new
      // user what is there.
      menuCollapsed: false,
      peopleCollapsed: false,
      toggleMenu: () => set((s) => ({ menuCollapsed: !s.menuCollapsed })),
      togglePeople: () => set((s) => ({ peopleCollapsed: !s.peopleCollapsed })),
    }),
    {
      name: "portal.shell.layout",
      version: 1,
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
