"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { AttachPhotoPopup } from "../popup/AttachPhotoPopup";
import { PlacePickerPopup } from "../popup/PlacePickerPopup";
import { assetVariantURL } from "@/lib/media-assets";
import { composeBody } from "@/lib/attachments";
import { coordName, type Place } from "@/lib/geo";

/**
 * Newsfeed composer — port of the Olympus `.news-feed-form` create-post box
 * (social/social/Newsfeed.html 2542-2660): a Status tab over an avatar +
 * "Share what you are thinking here…" field, then one add-options row with the
 * photo / tag / location icons on the left and Preview + Post Status on the
 * right.
 *
 * One tab only. The reference's Multimedia and Blog Post tabs were dropped —
 * they wrote the same journal entry with a different wrapper, which is a
 * distinction the feed never made. Pasting a URL into the status text still
 * renders the link/video card (`lib/links.ts`), so nothing was lost with them.
 *
 * The two live add-options buttons return attachments, not files:
 *   · camera → {@link AttachPhotoPopup} — upload or pick an existing image,
 *     resolving to a media-module asset id.
 *   · pin    → {@link PlacePickerPopup} — search or drop a pin on the map.
 * Both are encoded into the entry body by `lib/attachments.ts`, because the
 * journal API accepts no other field for them (see that module's note). Tagging
 * friends stays inert — there is no people-tagging surface yet.
 *
 * Controlled/presentational: the caller owns the draft and the mutation (D-32);
 * the attachments and the preview toggle are ephemeral UI state and stay local.
 */
export interface ComposerProps {
  displayName: string;
  bodyMd: string;
  onBodyMdChange: (value: string) => void;
  /** Receives the composed markdown — text plus any attachments. */
  onSubmit: (bodyMd: string) => void;
  submitting?: boolean;
  error?: string | null;
  className?: string;
}

export function Composer({
  displayName,
  bodyMd,
  onBodyMdChange,
  onSubmit,
  submitting = false,
  error = null,
  className = "",
}: ComposerProps) {
  const [preview, setPreview] = useState(false);
  const [photoId, setPhotoId] = useState<string | null>(null);
  const [thumbFailed, setThumbFailed] = useState(false);
  const [place, setPlace] = useState<Place | null>(null);
  const [photoOpen, setPhotoOpen] = useState(false);
  const [placeOpen, setPlaceOpen] = useState(false);

  const composed = composeBody(bodyMd, photoId, place);
  const canPost = composed.trim().length > 0 && !submitting;

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!canPost) return;
    onSubmit(composed);
    setPhotoId(null);
    setPlace(null);
    setPreview(false);
  }

  return (
    <div
      className={`overflow-hidden rounded-xl shadow-sm ${className}`}
      style={{ background: "var(--tpl-surface)", border: "1px solid var(--tpl-border)" }}
    >
      {/* Nav tab — Bootstrap card-tab look: the active tab is a white panel
          that joins the form below it by cancelling its own bottom border. */}
      <div
        role="tablist"
        aria-label="Post type"
        className="flex"
        style={{
          background: "var(--tpl-surface-2)",
          borderBottom: "1px solid var(--tpl-border)",
        }}
      >
        <span
          role="tab"
          aria-selected
          className="-mb-px flex items-center gap-2 px-5 py-3.5 text-sm font-semibold"
          style={{
            color: "var(--tpl-heading)",
            background: "var(--tpl-surface)",
            border: "1px solid var(--tpl-border)",
            borderTopColor: "transparent",
            borderBottomColor: "var(--tpl-surface)",
          }}
        >
          <Icon name="status-icon" size={16} style={{ color: "var(--tpl-accent)" }} />
          <span>Status</span>
        </span>
      </div>

      <form onSubmit={submit} className="p-4">
        {error && (
          <p
            role="alert"
            className="mb-3 rounded-lg border px-3 py-2 text-sm"
            style={{
              borderColor: "rgba(239,68,68,.4)",
              background: "rgba(239,68,68,.08)",
              color: "#ef4444",
            }}
          >
            {error}
          </p>
        )}

        <div className="flex gap-3">
          <Avatar name={displayName} size={40} />
          {preview ? (
            <div
              className="min-h-[7rem] w-full whitespace-pre-wrap pt-2 text-sm"
              style={{ color: "var(--tpl-text)" }}
            >
              {composed.trim() ? (
                composed
              ) : (
                <span style={{ color: "var(--tpl-muted)" }}>Nothing to preview yet.</span>
              )}
            </div>
          ) : (
            <textarea
              value={bodyMd}
              onChange={(e) => onBodyMdChange(e.target.value)}
              rows={4}
              placeholder="Share what you are thinking here..."
              className="min-h-[7rem] w-full resize-none border-0 bg-transparent pt-2 text-sm outline-none placeholder:text-[var(--tpl-muted)]"
              style={{ color: "var(--tpl-text)" }}
            />
          )}
        </div>

        {(photoId || place) && (
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {photoId && (
              <span
                className="relative inline-block overflow-hidden rounded-lg border"
                style={{ borderColor: "var(--tpl-border)" }}
              >
                {thumbFailed ? (
                  // The variant URL can 404 (see PhotoFrame in Post.tsx); the
                  // attachment is still valid, so show a tile, not a broken icon.
                  <span
                    className="grid h-20 w-28 place-items-center"
                    style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}
                  >
                    <Icon name="photos-icon" size={22} />
                  </span>
                ) : (
                  /* eslint-disable-next-line @next/next/no-img-element -- dynamic, API-proxied variant, not a static/optimizable asset */
                  <img
                    src={assetVariantURL(photoId, "thumb")}
                    alt="Ảnh đính kèm"
                    onError={() => setThumbFailed(true)}
                    className="h-20 w-28 object-cover"
                  />
                )}
                <ChipRemove label="Bỏ ảnh" onClick={() => setPhotoId(null)} />
              </span>
            )}

            {place && (
              <span
                className="relative inline-flex items-center gap-2 rounded-full py-1.5 pl-3 pr-8 text-xs font-medium"
                style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-text)" }}
              >
                <span style={{ color: "var(--tpl-accent)" }}>
                  <Icon name="small-pin-icon" size={12} />
                </span>
                <span className="max-w-[16rem] truncate">
                  {place.name || coordName(place.lat, place.lon)}
                </span>
                <ChipRemove label="Bỏ địa điểm" onClick={() => setPlace(null)} inline />
              </span>
            )}
          </div>
        )}

        {/* add-options-message: icons left, Preview + primary right */}
        <div
          className="mt-3 flex items-center gap-1 border-t pt-3"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          <IconBtn
            label={photoId ? "Đổi ảnh" : "Thêm ảnh"}
            icon="camera-icon"
            active={!!photoId}
            onClick={() => setPhotoOpen(true)}
          />
          <IconBtn label="Tag friends (coming soon)" icon="computer-icon" disabled />
          <IconBtn
            label={place ? "Đổi địa điểm" : "Thêm địa điểm"}
            icon="small-pin-icon"
            active={!!place}
            onClick={() => setPlaceOpen(true)}
          />

          <div className="ml-auto flex items-center gap-2">
            <button
              type="button"
              onClick={() => setPreview((p) => !p)}
              className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
              style={{
                borderColor: "var(--tpl-border)",
                background: "transparent",
                color: "var(--tpl-muted)",
              }}
            >
              {preview ? "Edit" : "Preview"}
            </button>
            <button
              type="submit"
              disabled={!canPost}
              className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              {submitting ? "Posting…" : "Post Status"}
            </button>
          </div>
        </div>
      </form>

      <AttachPhotoPopup
        open={photoOpen}
        onClose={() => setPhotoOpen(false)}
        onPick={(id) => {
          setThumbFailed(false);
          setPhotoId(id);
        }}
      />
      <PlacePickerPopup
        open={placeOpen}
        onClose={() => setPlaceOpen(false)}
        onPick={setPlace}
        initial={place}
      />
    </div>
  );
}

function ChipRemove({
  label,
  onClick,
  inline = false,
}: {
  label: string;
  onClick: () => void;
  inline?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={label}
      title={label}
      className={`absolute grid place-items-center rounded-full text-white transition hover:opacity-90 ${
        inline ? "right-1.5 top-1/2 h-5 w-5 -translate-y-1/2" : "right-1 top-1 h-5 w-5"
      }`}
      style={{ background: "rgba(63,66,87,.75)" }}
    >
      <Icon name="close-icon" size={8} />
    </button>
  );
}

function IconBtn({
  label,
  icon,
  disabled,
  active,
  onClick,
}: {
  label: string;
  icon: string;
  disabled?: boolean;
  active?: boolean;
  onClick?: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
      disabled={disabled}
      className="grid h-9 w-9 place-items-center rounded-lg transition hover:bg-[var(--tpl-surface-2)] disabled:cursor-not-allowed disabled:opacity-40"
      style={{ color: active ? "var(--tpl-accent)" : "var(--tpl-muted)" }}
    >
      <Icon name={icon} size={18} />
    </button>
  );
}
