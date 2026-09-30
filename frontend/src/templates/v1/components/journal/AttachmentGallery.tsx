"use client";

import { useState } from "react";
import { PhotoFrame } from "../ui/PhotoFrame";
import { Lightbox } from "../popup/Lightbox";
import { attachmentLayout } from "@/lib/entry-presentation";
import { assetVariantURL } from "@/lib/media-assets";

/**
 * An Entry's Attachments, the ONE way a card draws them (SPEC-12 T2): the first
 * as a hero at `medium`, the rest as a row of `thumb` tiles — four, then a
 * "+N" badge on the last — and any of them opens the {@link Lightbox} at that
 * photo, walking the whole array in order. The stream card and the entry card
 * are the same card, so this is the only renderer.
 *
 * One photo is the hero alone with no row: exactly the frame it had before
 * there could be more than one. A picture that will not load degrades to
 * {@link PhotoFrame}'s placeholder, never a broken-image glyph.
 */
export function AttachmentGallery({ assetIds }: { assetIds: string[] }) {
  const [open, setOpen] = useState<number | null>(null);
  const { hero, thumbs, overflow } = attachmentLayout(assetIds);
  if (!hero) return null;

  const total = assetIds.length;
  const label = (i: number) => `Mở ảnh ${i + 1} / ${total}`;

  return (
    <div
      className="mt-3 overflow-hidden rounded-lg border"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <button
        type="button"
        onClick={() => setOpen(0)}
        aria-label={label(0)}
        className="block w-full cursor-zoom-in"
      >
        <PhotoFrame
          src={assetVariantURL(hero, "medium")}
          alt="Ảnh đính kèm"
          loading="lazy"
          className="max-h-[32rem] w-full object-contain"
          fallbackClassName="h-64"
          iconSize={40}
        />
      </button>

      {thumbs.length > 0 && (
        <div
          className="grid grid-cols-4 gap-1 border-t p-1"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface-2)" }}
        >
          {thumbs.map((id, i) => {
            const index = i + 1; // position in the Entry's array, hero excluded
            const badge = overflow > 0 && i === thumbs.length - 1;
            return (
              <button
                key={id}
                type="button"
                onClick={() => setOpen(index)}
                aria-label={badge ? `Mở ảnh ${index + 1} / ${total} và ${overflow} ảnh nữa` : label(index)}
                className="relative aspect-[4/3] cursor-zoom-in overflow-hidden rounded"
              >
                <PhotoFrame
                  src={assetVariantURL(id, "thumb")}
                  alt={`Ảnh ${index + 1}`}
                  loading="lazy"
                  className="h-full w-full object-cover"
                  fallbackClassName="h-full"
                />
                {badge && (
                  <span
                    className="absolute inset-0 grid place-items-center text-lg font-semibold text-white"
                    style={{ background: "rgba(0,0,0,.55)" }}
                    aria-hidden
                  >
                    +{overflow}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}

      <Lightbox
        open={open !== null}
        index={open ?? 0}
        images={assetIds.map((id, i) => ({
          src: assetVariantURL(id, "medium"),
          alt: `Ảnh ${i + 1} / ${total}`,
        }))}
        onIndexChange={setOpen}
        onClose={() => setOpen(null)}
      />
    </div>
  );
}
