"use client";

import type { ReactNode } from "react";
import { Modal } from "./Modal";
import { Icon } from "../ui/Icon";

/**
 * "Update Header Photo" popup — port of Olympus `#update-header-photo`
 * (`Profile Page.html`). Two options: upload from computer, or choose from the
 * user's existing photos (which opens the ChoseFromMyPhoto picker).
 */
export function UpdateHeaderPhoto({
  open = false,
  onClose = () => {},
  onUpload,
  onChooseFromPhotos,
}: {
  open?: boolean;
  onClose?: () => void;
  onUpload?: (fileName: string) => void;
  onChooseFromPhotos?: () => void;
}) {
  return (
    <Modal open={open} onClose={onClose} title="Update Header Photo" width={460}>
      <div className="space-y-3 p-6">
        <label className="block cursor-pointer">
          <Option icon="computer-icon" title="Upload Photo" subtitle="Browse your computer." />
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => {
              const f = e.target.files?.[0]?.name;
              if (f) onUpload?.(f);
            }}
          />
        </label>

        <button type="button" className="w-full text-left" onClick={() => onChooseFromPhotos?.()}>
          <Option icon="photos-icon" title="Choose from My Photos" subtitle="Choose from your uploaded photos" />
        </button>
      </div>
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
