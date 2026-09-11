"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { Modal, BtnPrimary, BtnSecondary } from "./Modal";
import { Icon } from "../ui/Icon";
import { listAssets, assetVariantURL, type MediaAsset } from "@/lib/media-assets";
import { uploadImage } from "@/lib/media-upload";

/**
 * "Add Photo" popup for the newsfeed composer — the Olympus
 * `#update-header-photo` two-option dialog (Upload Photo / Choose from My
 * Photos), with the picker as a second pane of the same modal instead of a
 * second stacked dialog.
 *
 * Both options end at the same place: a **real** media-module asset id. Upload
 * runs the standard image pipeline (`uploadImage`: create session → PUT source
 * → complete → poll until the WebP variants are ready), and the library lists
 * the caller's own ready images — no fixture tiles, so what you pick is what
 * the post will show.
 */
type Pane = "choose" | "library";

export function AttachPhotoPopup({
  open,
  onClose,
  onPick,
}: {
  open: boolean;
  onClose: () => void;
  onPick: (assetId: string) => void;
}) {
  const [pane, setPane] = useState<Pane>("choose");
  const [selected, setSelected] = useState<string | null>(null);
  const [pct, setPct] = useState<number | null>(null);
  const [error, setError] = useState<string | null>(null);

  // Fresh dialog every time: a stale pane or half-finished upload from the
  // previous open would be confusing.
  useEffect(() => {
    if (!open) {
      setPane("choose");
      setSelected(null);
      setPct(null);
      setError(null);
    }
  }, [open]);

  const library = useQuery({
    queryKey: ["assets", "image", "ready"],
    queryFn: () => listAssets({ kind: "image", status: "ready" }),
    enabled: open && pane === "library",
  });

  // The list endpoint takes `kind`/`status`, but filter here too: the picker
  // must never offer a video or a still-processing image.
  const photos = (library.data?.assets ?? []).filter(
    (a: MediaAsset) => a.kind === "image" && a.status === "ready",
  );

  async function handleFile(file: File) {
    setError(null);
    setPct(0);
    try {
      const up = await uploadImage(file, setPct);
      onPick(up.assetId);
      onClose();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Tải ảnh lên thất bại.");
    } finally {
      setPct(null);
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={pane === "choose" ? "Add Photo" : "Choose from My Photos"}
      width={pane === "choose" ? 460 : 640}
    >
      {error && (
        <p
          role="alert"
          className="mx-6 mt-4 rounded-lg border px-3 py-2 text-sm"
          style={{
            borderColor: "rgba(239,68,68,.4)",
            background: "rgba(239,68,68,.08)",
            color: "#ef4444",
          }}
        >
          {error}
        </p>
      )}

      {pane === "choose" ? (
        <div className="space-y-3 p-6">
          {pct === null ? (
            <>
              <label className="block cursor-pointer">
                <Option
                  icon="computer-icon"
                  title="Upload Photo"
                  subtitle="Browse your computer."
                />
                <input
                  type="file"
                  accept="image/*"
                  className="hidden"
                  onChange={(e) => {
                    const f = e.target.files?.[0];
                    if (f) void handleFile(f);
                    e.target.value = ""; // allow re-picking the same file
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
            </>
          ) : (
            <div className="py-4">
              <p className="text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
                {pct < 100 ? `Đang tải lên… ${pct}%` : "Đang xử lý ảnh…"}
              </p>
              <div
                className="mt-3 h-2 w-full overflow-hidden rounded-full"
                style={{ background: "var(--tpl-surface-2)" }}
              >
                <div
                  className="h-full transition-[width]"
                  style={{
                    width: `${pct}%`,
                    background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))",
                  }}
                />
              </div>
              <p className="mt-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
                Ảnh được chuyển sang WebP trước khi đính vào bài.
              </p>
            </div>
          )}
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
                  const active = selected === p.id;
                  return (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => setSelected(p.id)}
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
            )}
          </div>

          <div
            className="flex justify-end gap-2 border-t px-6 py-4"
            style={{ borderColor: "var(--tpl-border)" }}
          >
            <BtnSecondary onClick={() => setPane("choose")}>Back</BtnSecondary>
            <BtnPrimary
              disabled={!selected}
              onClick={() => {
                if (selected) {
                  onPick(selected);
                  onClose();
                }
              }}
            >
              Confirm Photo
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
