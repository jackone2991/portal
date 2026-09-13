"use client";

import { useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { AttachPhotoPopup } from "../popup/AttachPhotoPopup";
import { LocationPickerPopup } from "../popup/LocationPickerPopup";
import { assetVariantURL } from "@/lib/media-assets";
import { composeBody } from "@/lib/attachments";
import { locationLabel, type Location } from "@/lib/geo";

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
 *     resolving to a media-module asset id that is already `ready`.
 *   · pin    → {@link LocationPickerPopup} — search or drop a pin on the map.
 * The Attachment travels as the Entry's `asset_ids` (SPEC-12 T1); the Location
 * is still encoded into the body by `lib/attachments.ts` until T3 gives it
 * columns. Tagging friends stays inert — there is no people-tagging surface yet.
 *
 * Controlled/presentational: the caller owns the draft and the mutation (D-32);
 * the attachments and the preview toggle are ephemeral UI state and stay local.
 */
export interface ComposerDraft {
  /** The body to post — the text plus the encoded Location, no photo markup. */
  bodyMd: string;
  /** The Entry's Attachments in the order they were added. */
  assetIds: string[];
}

export interface ComposerProps {
  displayName: string;
  bodyMd: string;
  onBodyMdChange: (value: string) => void;
  /**
   * Resolves to whether the post was accepted. The composer keeps its
   * Attachments and Location until then, so a refused post (the server said no
   * to a photo, say) leaves everything in place to fix and resend — exactly as
   * the text does through `bodyMd`.
   */
  onSubmit: (draft: ComposerDraft) => Promise<boolean> | boolean;
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
  // Attachments are an ordered list even though the UI still caps it at one;
  // SPEC-12 T2 lifts the cap without reshaping this state.
  const [assetIds, setAssetIds] = useState<string[]>([]);
  const [thumbFailed, setThumbFailed] = useState(false);
  const [location, setLocation] = useState<Location | null>(null);
  const [photoOpen, setPhotoOpen] = useState(false);
  const [locationOpen, setLocationOpen] = useState(false);
  const [assetId] = assetIds;

  const composed = composeBody(bodyMd, location);
  // An Entry is text or at least one Attachment — never a Location alone
  // (SPEC-12 story 7), which is exactly what the server refuses; the button
  // says so before the request does. This is the one place the rule lives on
  // the client.
  const canPost = (bodyMd.trim().length > 0 || assetIds.length > 0) && !submitting;

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!canPost) return;
    const accepted = await onSubmit({ bodyMd: composed, assetIds });
    if (!accepted) return; // the parent has shown the error; the draft stays
    setAssetIds([]);
    setLocation(null);
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
              ) : assetId ? null : ( // a photo-only draft previews as its tile below
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

        {(assetId || location) && (
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {assetId && (
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
                    src={assetVariantURL(assetId, "thumb")}
                    alt="Ảnh đính kèm"
                    onError={() => setThumbFailed(true)}
                    className="h-20 w-28 object-cover"
                  />
                )}
                <ChipRemove label="Bỏ ảnh" onClick={() => setAssetIds([])} />
              </span>
            )}

            {location && (
              <span
                className="relative inline-flex items-center gap-2 rounded-full py-1.5 pl-3 pr-8 text-xs font-medium"
                style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-text)" }}
              >
                <span style={{ color: "var(--tpl-accent)" }}>
                  <Icon name="small-pin-icon" size={12} />
                </span>
                <span className="max-w-[16rem] truncate">
                  {locationLabel(location)}
                </span>
                <ChipRemove label="Bỏ địa điểm" onClick={() => setLocation(null)} inline />
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
            label={assetId ? "Đổi ảnh" : "Thêm ảnh"}
            icon="camera-icon"
            active={!!assetId}
            onClick={() => setPhotoOpen(true)}
          />
          <IconBtn label="Tag friends (coming soon)" icon="computer-icon" disabled />
          <IconBtn
            label={location ? "Đổi địa điểm" : "Thêm địa điểm"}
            icon="small-pin-icon"
            active={!!location}
            onClick={() => setLocationOpen(true)}
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
          // Cap of one until T2: a new pick replaces, never appends.
          setAssetIds([id]);
        }}
      />
      <LocationPickerPopup
        open={locationOpen}
        onClose={() => setLocationOpen(false)}
        onPick={setLocation}
        initial={location}
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
