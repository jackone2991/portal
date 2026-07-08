"use client";

import { useState } from "react";
import { Modal, BtnPrimary, BtnSecondary } from "./Modal";
import { Icon } from "../ui/Icon";

/**
 * "Choose from My Photos" picker — port of Olympus `#choose-from-my-photo`
 * (`Profile Page.html`). Two tabs (Photos / Albums); Photos is a single-select
 * thumbnail grid; Confirm is disabled until a photo is picked. No image assets
 * in the app, so thumbnails are deterministic gradient tiles.
 */

const PHOTOS = Array.from({ length: 9 }, (_, i) => i);
const ALBUMS = [
  "South America Vacations",
  "Photoshoot Summer 2016",
  "Amazing Street Food",
  "Graffity & Street Art",
  "Amazing Landscapes",
  "The Majestic Canyon",
];

function tile(seed: number) {
  const h = (seed * 47) % 360;
  return `linear-gradient(135deg, hsl(${h} 60% 60%), hsl(${(h + 40) % 360} 60% 45%))`;
}

export function ChoseFromMyPhoto({
  open = false,
  onClose = () => {},
  onConfirm,
}: {
  open?: boolean;
  onClose?: () => void;
  onConfirm?: (photo: number) => void;
}) {
  const [tab, setTab] = useState<"photos" | "albums">("photos");
  const [selected, setSelected] = useState<number | null>(null);

  return (
    <Modal open={open} onClose={onClose} title="Choose from My Photos" width={640}>
      <div className="flex gap-1 border-b px-6" style={{ borderColor: "var(--tpl-border)" }}>
        {(
          [
            ["photos", "photos-icon", "Photos"],
            ["albums", "albums-icon", "Albums"],
          ] as const
        ).map(([k, ic, label]) => (
          <button
            key={k}
            type="button"
            onClick={() => setTab(k)}
            className="flex items-center gap-2 px-4 py-3 text-sm font-semibold transition"
            style={
              tab === k
                ? { color: "var(--tpl-accent)", boxShadow: "inset 0 -2px 0 var(--tpl-accent)" }
                : { color: "var(--tpl-muted)" }
            }
          >
            <Icon name={ic} size={16} />
            {label}
          </button>
        ))}
      </div>

      <div className="max-h-[52vh] overflow-y-auto p-6">
        {tab === "photos" ? (
          <div className="grid grid-cols-3 gap-3">
            {PHOTOS.map((p) => {
              const active = selected === p;
              return (
                <button
                  key={p}
                  type="button"
                  onClick={() => setSelected(p)}
                  className="relative aspect-[3/2] overflow-hidden rounded-lg ring-offset-2 transition"
                  style={{ background: tile(p), boxShadow: active ? "0 0 0 3px var(--tpl-accent)" : "none" }}
                  aria-pressed={active}
                >
                  {active && (
                    <span
                      className="absolute right-2 top-2 grid h-6 w-6 place-items-center rounded-full text-white"
                      style={{ background: "var(--tpl-accent)" }}
                    >
                      <Icon name="check-icon" size={12} />
                    </span>
                  )}
                </button>
              );
            })}
          </div>
        ) : (
          <div className="grid grid-cols-3 gap-4">
            {ALBUMS.map((a, i) => (
              <button
                key={a}
                type="button"
                onClick={() => {
                  setTab("photos");
                  setSelected(i);
                }}
                className="text-left"
              >
                <span className="block aspect-[5/4] rounded-lg" style={{ background: tile(i + 20) }} />
                <span className="mt-2 block truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                  {a}
                </span>
                <span className="block text-xs" style={{ color: "var(--tpl-muted)" }}>
                  Last added: 2 days ago
                </span>
              </button>
            ))}
          </div>
        )}
      </div>

      <div className="flex justify-end gap-2 border-t px-6 py-4" style={{ borderColor: "var(--tpl-border)" }}>
        <BtnSecondary onClick={onClose}>Cancel</BtnSecondary>
        <BtnPrimary
          disabled={selected === null}
          onClick={() => {
            if (selected !== null) {
              onConfirm?.(selected);
              onClose();
            }
          }}
        >
          Confirm Photo
        </BtnPrimary>
      </div>
    </Modal>
  );
}
