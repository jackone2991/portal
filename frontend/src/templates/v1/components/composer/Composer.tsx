"use client";

import { useEffect, useRef, useState, type FormEvent } from "react";
import { Avatar } from "../ui/Avatar";
import { Icon } from "../ui/Icon";
import { PhotoFrame } from "../ui/PhotoFrame";
import { AttachPhotoPopup } from "../popup/AttachPhotoPopup";
import { LocationPickerPopup } from "../popup/LocationPickerPopup";
import { assetVariantURL } from "@/lib/media-assets";
import { uploadImage } from "@/lib/media-upload";
import {
  MAX_ATTACHMENTS,
  addPhotos,
  canPost,
  capMessage,
  fullMessage,
  patchPhoto,
  readyAssetIds,
  removePhoto,
  storedPhotos,
  type ComposerPhoto,
  type PhotoPick,
} from "@/lib/composer-photos";
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
 * The two live add-options buttons:
 *   · camera → {@link AttachPhotoPopup} — pick files to upload, or existing
 *     images from the library. Each pick becomes a tile here (SPEC-12 T2); a
 *     file's upload runs from this component so the tile can show its
 *     progress, then "processing" while the worker cuts the WebP variants,
 *     then ready — or the error, with a remove control. Up to ten, a file
 *     picked twice is ignored, and Post waits until every tile is ready; the
 *     rules are `lib/composer-photos.ts`, this file only draws them.
 *   · pin    → {@link LocationPickerPopup} — search or drop a pin on the map.
 * The Attachments travel as the Entry's `asset_ids` (SPEC-12 T1) and the
 * Location as its `location` (T3); the body is the text and nothing else.
 * Tagging friends stays inert — there is no people-tagging surface yet.
 *
 * The same composer is the edit surface (SPEC-12 T5): rendered in place of the
 * card with `initial` pre-filling the mood, photos and Location (the text
 * comes through `bodyMd` as always) and `onCancel` set. Editing and creating
 * differ only in chrome — the tab says so, the primary button says Save, and
 * a Cancel button appears — the rules are the same rules.
 *
 * Controlled/presentational: the caller owns the draft and the mutation (D-32);
 * the attachments, the mood, the Location and the preview toggle are ephemeral
 * UI state and stay local.
 */
export interface ComposerDraft {
  /** The body to post — plain markdown, exactly what was typed. */
  bodyMd: string;
  /** Freeform, trimmed; null for none. */
  mood: string | null;
  /** The Entry's Attachments in the order they were added — every one `ready`. */
  assetIds: string[];
  /** The Entry's Location, or null for none. */
  location: Location | null;
}

/**
 * What an existing Entry brings into the composer when it is edited — the
 * draft minus the text, which arrives through `bodyMd`. The stored
 * Attachments are `ready` by definition.
 */
export type ComposerInitial = Omit<ComposerDraft, "bodyMd">;

/**
 * Create mode has neither; edit mode has both — an "Edit Post" with no way
 * out is not a state the type allows.
 */
type ComposerMode =
  | { initial?: undefined; onCancel?: undefined }
  | {
      /**
       * Pre-fills mood, photos and Location. Read once, on mount — the caller
       * mounts a fresh composer per Entry.
       */
      initial: ComposerInitial;
      /** The Cancel button. */
      onCancel: () => void;
    };

export type ComposerProps = ComposerMode & {
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
};

const MAX_MOOD = 80;

export function Composer({
  displayName,
  bodyMd,
  onBodyMdChange,
  onSubmit,
  initial,
  onCancel,
  submitting = false,
  error = null,
  className = "",
}: ComposerProps) {
  const editing = initial !== undefined;
  const [preview, setPreview] = useState(false);
  // An edited Entry's stored Attachments start as ready tiles showing their
  // thumb variant — the same tile a library pick makes.
  const [photos, setPhotos] = useState<ComposerPhoto[]>(() =>
    storedPhotos(initial?.assetIds ?? [], previewOf),
  );
  // The cap message; cleared by the next add or remove.
  const [notice, setNotice] = useState<string | null>(null);
  const [mood, setMood] = useState(initial?.mood ?? "");
  const [moodOpen, setMoodOpen] = useState(!!initial?.mood);
  const moodInput = useRef<HTMLInputElement>(null);
  const [location, setLocation] = useState<Location | null>(initial?.location ?? null);
  const [photoOpen, setPhotoOpen] = useState(false);
  const [locationOpen, setLocationOpen] = useState(false);

  // Which tiles exist — the membership, kept beside the state because the
  // picker's callback can run before an upload's queued progress patch has
  // rendered, and "is this a duplicate / is the list full" must not read a
  // stale list. Every membership change (`handleAdd`, `handleRemove`, the reset
  // in `submit`) writes it; status patches never do. Adds and removes then go
  // to the state as functional updates, so a patch queued in between is
  // merged, never overwritten.
  const members = useRef<ComposerPhoto[]>(photos);

  // An object URL is browser memory until revoked: tiles release theirs when
  // removed, posted, or — whatever is left in the strip — on unmount.
  useEffect(() => () => members.current.forEach(release), []);

  // Text or at least one Attachment — a Location alone is not an Entry — and
  // every photo ready (SPEC-12 stories 7 and 11): what the server would
  // refuse, said by the button first.
  const draftValid = canPost(bodyMd, photos);
  const postable = draftValid && !submitting;
  const waiting = photos.some((p) => p.status === "uploading" || p.status === "processing");
  const remaining = MAX_ATTACHMENTS - photos.length;

  function handleAdd(picks: PhotoPick[]) {
    const result = addPhotos(members.current, picks, previewOf);
    const fresh = result.added.map((a) => a.photo);
    members.current = [...members.current, ...fresh];
    setPhotos((ps) => [...ps, ...fresh]);
    setNotice(result.refused > 0 ? capMessage(result.refused) : null);
    for (const { photo, pick } of result.added) {
      if (pick.kind === "file") void upload(photo.key, pick.file);
    }
  }

  // Opens the chip if needed and puts the caret in it either way — "change
  // the mood" with the chip already open should not be a dead tap.
  function openMood() {
    setMoodOpen(true);
    requestAnimationFrame(() => moodInput.current?.focus());
  }

  function openPhotoPicker() {
    // A full strip says so where the tap landed (SPEC-12 story 8) — a disabled
    // button's tooltip is invisible on touch.
    if (remaining <= 0) {
      setNotice(fullMessage());
      return;
    }
    setPhotoOpen(true);
  }

  async function upload(key: string, file: File) {
    // A tile removed mid-upload makes every patch a no-op; the Asset still
    // lands in the library, which is the same as an upload from /library/media.
    const patch = (p: Partial<Omit<ComposerPhoto, "key">>) =>
      setPhotos((ps) => patchPhoto(ps, key, p));
    try {
      const up = await uploadImage(
        file,
        (pct) => patch({ pct }),
        () => patch({ status: "processing", pct: 100 }),
      );
      patch({ status: "ready", assetId: up.assetId });
    } catch (e) {
      patch({ status: "failed", error: e instanceof Error ? e.message : "Tải ảnh lên thất bại." });
    }
  }

  function handleRemove(photo: ComposerPhoto) {
    release(photo);
    members.current = removePhoto(members.current, photo.key);
    setPhotos((ps) => removePhoto(ps, photo.key));
    setNotice(null);
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!postable) return;
    const accepted = await onSubmit({
      bodyMd,
      mood: mood.trim() || null,
      assetIds: readyAssetIds(photos),
      location,
    });
    if (!accepted) return; // the parent has shown the error; the draft stays
    // An accepted edit is the end of this composer: the caller swaps the card
    // back in and the unmount cleanup releases the strip.
    if (editing) return;
    // Adds are frozen while submitting, so the strip is exactly what was posted.
    members.current.forEach(release);
    members.current = [];
    setPhotos([]);
    setNotice(null);
    setMood("");
    setMoodOpen(false);
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
          <span>{editing ? "Edit Post" : "Status"}</span>
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
              {bodyMd.trim() ? (
                bodyMd
              ) : photos.length > 0 ? null : ( // a photo-only draft previews as its tiles below
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

        {(photos.length > 0 || moodOpen || location) && (
          <div className="mt-2 flex flex-wrap items-center gap-2">
            {photos.map((p) => (
              <PhotoTile key={p.key} photo={p} onRemove={() => handleRemove(p)} />
            ))}

            {moodOpen && (
              <span
                className="relative inline-flex items-center gap-2 rounded-full py-1 pl-3 pr-8 text-xs font-medium"
                style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-text)" }}
              >
                <span style={{ color: "var(--tpl-accent)" }}>
                  <Icon name="happy-face-icon" size={12} />
                </span>
                <input
                  ref={moodInput}
                  value={mood}
                  onChange={(e) => setMood(e.target.value)}
                  maxLength={MAX_MOOD}
                  placeholder="Tâm trạng…"
                  aria-label="Tâm trạng"
                  className="w-32 bg-transparent text-xs outline-none placeholder:text-[var(--tpl-muted)]"
                  style={{ color: "var(--tpl-text)" }}
                />
                <ChipRemove
                  label="Bỏ tâm trạng"
                  onClick={() => {
                    setMood("");
                    setMoodOpen(false);
                  }}
                  inline
                />
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

        {notice && (
          <p role="status" className="mt-2 text-xs font-medium" style={{ color: "#ef4444" }}>
            {notice}
          </p>
        )}

        {/* add-options-message: icons left, Preview + primary right */}
        <div
          className="mt-3 flex items-center gap-1 border-t pt-3"
          style={{ borderColor: "var(--tpl-border)" }}
        >
          <IconBtn
            label={remaining > 0 ? `Thêm ảnh (còn ${remaining})` : `Đã đủ ${MAX_ATTACHMENTS} ảnh`}
            icon="camera-icon"
            active={photos.length > 0}
            disabled={submitting}
            onClick={openPhotoPicker}
          />
          <IconBtn label="Tag friends (coming soon)" icon="computer-icon" disabled />
          <IconBtn
            label={mood ? "Đổi tâm trạng" : "Thêm tâm trạng"}
            icon="happy-face-icon"
            active={mood.trim().length > 0}
            disabled={submitting}
            onClick={openMood}
          />
          <IconBtn
            label={location ? "Đổi địa điểm" : "Thêm địa điểm"}
            icon="small-pin-icon"
            active={!!location}
            disabled={submitting}
            onClick={() => setLocationOpen(true)}
          />

          <div className="ml-auto flex items-center gap-2">
            {onCancel && (
              <SecondaryBtn onClick={onCancel} disabled={submitting}>
                Cancel
              </SecondaryBtn>
            )}
            <SecondaryBtn onClick={() => setPreview((p) => !p)}>
              {preview ? "Edit" : "Preview"}
            </SecondaryBtn>
            <button
              type="submit"
              disabled={!postable}
              title={
                waiting
                  ? "Đợi ảnh tải và xử lý xong"
                  : photos.some((p) => p.status === "failed")
                    ? "Bỏ ảnh lỗi trước khi đăng"
                    : undefined
              }
              className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
              style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
            >
              {primaryLabel(editing, submitting, waiting)}
            </button>
          </div>
        </div>
      </form>

      <AttachPhotoPopup
        open={photoOpen}
        onClose={() => setPhotoOpen(false)}
        onAdd={handleAdd}
        remaining={remaining}
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

/** What a tile shows: the file itself while it uploads, the thumb variant for an Asset. */
function previewOf(pick: PhotoPick): string {
  return pick.kind === "file" ? URL.createObjectURL(pick.file) : assetVariantURL(pick.id, "thumb");
}

/** The primary button says what it will do, or why it is waiting. */
function primaryLabel(editing: boolean, submitting: boolean, waiting: boolean): string {
  if (submitting) return editing ? "Saving…" : "Posting…";
  if (waiting) return "Đang tải ảnh…";
  return editing ? "Save" : "Post Status";
}

/** Only a file pick holds browser memory; a library thumb is just a URL. */
function release(photo: ComposerPhoto) {
  if (photo.preview.startsWith("blob:")) URL.revokeObjectURL(photo.preview);
}

function SecondaryBtn({
  children,
  onClick,
  disabled,
}: {
  children: React.ReactNode;
  onClick: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-50"
      style={{ borderColor: "var(--tpl-border)", background: "transparent", color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}

/**
 * One photo in the strip: its picture, its state over it — a progress bar
 * while uploading, a veil while the worker processes, the error when it
 * failed — and the remove control, always. The state is what tells the writer
 * why Post is not available yet (SPEC-12 story 10).
 */
function PhotoTile({ photo, onRemove }: { photo: ComposerPhoto; onRemove: () => void }) {
  const failed = photo.status === "failed";
  // One place says what each status looks like: the label the tile carries,
  // and the veil over the picture (none while uploading — that is the bar).
  const { label, veil } = TILE_STATE[photo.status](photo);

  return (
    <span
      role="group"
      aria-label={label}
      title={label}
      className="relative inline-block h-20 w-28 overflow-hidden rounded-lg border"
      style={{ borderColor: failed ? "#ef4444" : "var(--tpl-border)" }}
    >
      <PhotoFrame
        src={photo.preview}
        alt="Ảnh đính kèm"
        className="h-full w-full object-cover"
        fallbackClassName="h-full"
        iconSize={22}
      />

      {photo.status === "uploading" && (
        <span
          className="absolute inset-x-0 bottom-0 h-1.5"
          style={{ background: "rgba(0,0,0,.35)" }}
          aria-hidden
        >
          <span
            className="block h-full transition-[width]"
            style={{
              width: `${photo.pct}%`,
              background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))",
            }}
          />
        </span>
      )}
      {veil && (
        <span
          className="absolute inset-0 grid place-items-center px-1 text-center text-[11px] font-semibold leading-tight text-white"
          style={{ background: veil.background }}
          aria-hidden
        >
          {veil.text}
        </span>
      )}

      <ChipRemove label="Bỏ ảnh" onClick={onRemove} />
    </span>
  );
}

const TILE_STATE: Record<
  ComposerPhoto["status"],
  (p: ComposerPhoto) => { label: string; veil: { text: string; background: string } | null }
> = {
  uploading: (p) => ({ label: `Đang tải lên… ${p.pct}%`, veil: null }),
  processing: () => ({
    label: "Đang xử lý ảnh…",
    veil: { text: "Đang xử lý…", background: "rgba(0,0,0,.45)" },
  }),
  ready: () => ({ label: "Sẵn sàng", veil: null }),
  failed: (p) => {
    const error = p.error ?? "Tải ảnh lên thất bại.";
    return { label: error, veil: { text: error, background: "rgba(239,68,68,.78)" } };
  },
};

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
