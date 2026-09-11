"use client";

import { useEffect, useRef, useState } from "react";
import { Icon } from "../ui/Icon";

/**
 * The three-dots dropdown on a post header — port of Olympus `.more-dropdown`
 * (Newsfeed.html 2678-2694). The reference lists Edit / Delete / Turn Off
 * Notifications / Select as Featured; only the two that map to a real endpoint
 * are offered, so the menu never advertises something the API can't do.
 *
 * Closes on outside click and on Escape, and returns focus to the trigger.
 */
export interface PostMenuItem {
  label: string;
  onSelect: () => void;
  /** Renders the item in the danger colour (delete). */
  danger?: boolean;
}

export function PostOptionsMenu({ items }: { items: PostMenuItem[] }) {
  const [open, setOpen] = useState(false);
  const root = useRef<HTMLDivElement>(null);
  const trigger = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDown(e: MouseEvent) {
      if (!root.current?.contains(e.target as Node)) setOpen(false);
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setOpen(false);
        trigger.current?.focus();
      }
    }
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  if (items.length === 0) return null;

  return (
    <div ref={root} className="relative">
      <button
        ref={trigger}
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-label="Post options"
        aria-haspopup="menu"
        aria-expanded={open}
        className="grid h-8 w-8 place-items-center rounded-md transition hover:bg-[var(--tpl-surface-2)]"
        style={{ color: "var(--tpl-muted)" }}
      >
        <Icon name="three-dots-icon" size={6} />
      </button>

      {open && (
        <ul
          role="menu"
          className="absolute right-0 top-9 z-20 min-w-[11rem] overflow-hidden rounded-lg py-1 shadow-lg"
          style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
        >
          {items.map((it) => (
            <li key={it.label} role="none">
              <button
                type="button"
                role="menuitem"
                onClick={() => {
                  setOpen(false);
                  it.onSelect();
                }}
                className="block w-full px-4 py-2 text-left text-sm transition hover:bg-[var(--tpl-surface-2)]"
                style={{ color: it.danger ? "#ef4444" : "var(--tpl-text)" }}
              >
                {it.label}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
