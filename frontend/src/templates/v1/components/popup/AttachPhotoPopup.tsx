"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useInfiniteQuery } from "@tanstack/react-query";
import { Modal, BtnPrimary, BtnSecondary } from "./Modal";
import { Icon } from "../ui/Icon";
import { listAssets, assetVariantURL, type MediaAsset } from "@/lib/media-assets";
import { MAX_ATTACHMENTS, type PhotoPick } from "@/lib/composer-photos";
import { useInfiniteScroll } from "@/lib/use-infinite-scroll";

/**
 * "Add Photo" popup for the newsfeed composer — the Olympus
 * `#update-header-photo` two-option dialog (Upload Photo / Choose from My
 * Photos), with the picker as a second pane of the same modal instead of a
 * second stacked dialog.
 *
 * It only *picks* (SPEC-12 T2): several files from disk, or several of the
 * caller's own ready images from the library — no fixture tiles, so what you
 * pick is what the post will show. The picks go back as {@link PhotoPick}s and
 * the dialog closes; the composer runs each upload and shows its progress on
 * the tile, because a dialog that stayed open for one upload cannot show ten.
 * The cap and the duplicate rule live in `lib/composer-photos.ts`, not here —
 * `remaining` is only the hint on the buttons.
 */
type Pane = "choose" | "library";

export function AttachPhotoPopup({
  open,
  onClose,
  onAdd,
  remaining = MAX_ATTACHMENTS,
}: {
  open: boolean;
  onClose: () => void;
  /** The photos picked, in selection order; the composer takes it from here. */
  onAdd: (picks: PhotoPick[]) => void;
  /** Slots left before the cap — shown, not enforced (the composer enforces). */
  remaining?: number;
}) {
  const [pane, setPane] = useState<Pane>("choose");
  const [selected, setSelected] = useState<string[]>([]);

  // Fresh dialog every time: a stale pane or a half-made selection from the
  // previous open would be confusing.
  useEffect(() => {
    if (!open) {
      setPane("choose");
      setSelected([]);
    }
  }, [open]);

  // Keyset-paginated like every list here (frontend/CLAUDE.md "Cursor lists"):
  // a plain query would show the first 30 photos as if they were all of them.
  const library = useInfiniteQuery({
    queryKey: ["assets", "image", "ready"],
    queryFn: ({ pageParam }: { pageParam: string | undefined }) =>
      listAssets({ kind: "image", status: "ready", cursor: pageParam }),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next_cursor ?? undefined,
    enabled: open && pane === "library",
  });
  const sentinelRef = useInfiniteScroll({
    onLoadMore: () => library.fetchNextPage(),
    hasMore: library.hasNextPage,
    isLoading: library.isFetchingNextPage,
  });

  // The list endpoint takes `kind`/`status`, but filter here too: the picker
  // must never offer a video or a still-processing image.
  const photos = (library.data?.pages.flatMap((p) => p.assets) ?? []).filter(
    (a: MediaAsset) => a.kind === "image" && a.status === "ready",
  );

  function toggle(id: string) {
    setSelected((s) => (s.includes(id) ? s.filter((x) => x !== id) : [...s, id]));
  }

  function confirmLibrary() {
    if (selected.length === 0) return;
    onAdd(selected.map((id) => ({ kind: "asset", id })));
    onClose();
  }

  const slots = `còn ${remaining}/${MAX_ATTACHMENTS} ảnh`;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={pane === "choose" ? "Add Photo" : "Choose from My Photos"}
      width={pane === "choose" ? 460 : 640}
    >
      {pane === "choose" ? (
        <div className="space-y-3 p-6">
          <label className="block cursor-pointer">
            <Option
              icon="computer-icon"
              title="Upload Photo"
              subtitle="Browse your computer — several at once."
            />
            <input
              type="file"
              accept="image/*"
              multiple
              className="hidden"
              onChange={(e) => {
                const files = Array.from(e.target.files ?? []);
                e.target.value = ""; // allow re-picking the same file
                if (files.length === 0) return;
                onAdd(files.map((file) => ({ kind: "file", file })));
                onClose();
              }}
            />
          </label>

          <button
            type="button"
            className="w-full text-left"
            onClick={() => setPane("library")}
          >
            <Option
              icon="photos-icon"
              title="Choose from My Photos"
              subtitle="Choose from your uploaded photos"
            />
          </button>
          <p className="text-center text-xs" style={{ color: "var(--tpl-muted)" }}>
            {slots}
          </p>
        </div>
      ) : (
        <>
          <div className="max-h-[52vh] overflow-y-auto p-6">
            {library.isPending ? (
              <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
                Đang tải thư viện ảnh…
              </p>
            ) : library.isError ? (
              <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
                Không tải được thư viện ảnh.
              </p>
            ) : photos.length === 0 ? (
              <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
                Chưa có ảnh nào. Quay lại và chọn <b>Upload Photo</b> để tải ảnh đầu tiên.
              </p>
            ) : (
              <div className="grid grid-cols-3 gap-3">
                {photos.map((p) => {
                  const order = selected.indexOf(p.id);
                  const active = order >= 0;
                  return (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => toggle(p.id)}
                      aria-pressed={active}
                      className="relative aspect-[3/2] overflow-hidden rounded-lg transition"
                      style={{
                        background: "var(--tpl-surface-2)",
                        boxShadow: active ? "0 0 0 3px var(--tpl-accent)" : "none",
                      }}
                    >
                      {/* eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset */}
                      <img
                        src={assetVariantURL(p.id, "thumb")}
                        alt={p.title || p.original_filename || "photo"}
                        loading="lazy"
                        className="h-full w-full object-cover"
                      />
                      {active && (
                        // The pick order is the display order, so the badge
                        // says which slot this photo takes rather than just ✓.
                        <span
                          className="absolute right-2 top-2 grid h-6 w-6 place-items-center rounded-full text-xs font-semibold text-white"
                          style={{ background: "var(--tpl-accent)" }}
                        >
                          {order + 1}
                        </span>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
            {/* The scroll sentinel: the next page loads as it comes into view. */}
            <div ref={sentinelRef} aria-hidden />
            {library.isFetchingNextPage && (
              <p className="mt-3 text-center text-xs" style={{ color: "var(--tpl-muted)" }}>
                Đang tải thêm…
              </p>
            )}
          </div>

          <div
            className="flex items-center justify-end gap-2 border-t px-6 py-4"
            style={{ borderColor: "var(--tpl-border)" }}
          >
            <span className="mr-auto text-xs" style={{ color: "var(--tpl-muted)" }}>
              {slots}
            </span>
            <BtnSecondary onClick={() => setPane("choose")}>Back</BtnSecondary>
            <BtnPrimary disabled={selected.length === 0} onClick={confirmLibrary}>
              {selected.length > 1 ? `Confirm ${selected.length} Photos` : "Confirm Photo"}
            </BtnPrimary>
          </div>
        </>
      )}
    </Modal>
  );
}

function Option({ icon, title, subtitle }: { icon: string; title: string; subtitle: ReactNode }) {
  return (
    <span
      className="flex items-center gap-4 rounded-lg border px-4 py-4 transition hover:border-[var(--tpl-accent)] hover:bg-[var(--tpl-surface-2)]"
      style={{ borderColor: "var(--tpl-border)" }}
    >
      <span
        className="grid h-11 w-11 shrink-0 place-items-center rounded-full text-white"
        style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
      >
        <Icon name={icon} size={20} />
      </span>
      <span className="min-w-0">
        <span className="block text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </span>
        <span className="block text-xs" style={{ color: "var(--tpl-muted)" }}>
          {subtitle}
        </span>
      </span>
    </span>
  );
}
