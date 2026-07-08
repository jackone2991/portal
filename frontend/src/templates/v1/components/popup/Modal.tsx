"use client";

import { useEffect, type ReactNode } from "react";
import { Icon } from "../ui/Icon";

/**
 * Modal shell — port of the Olympus `.modal .window-popup` dialog. Controlled
 * (`open` / `onClose`), consistent with the notification dropdowns. Backdrop
 * click and Escape close it; header carries the title and an X (close-icon).
 */
export function Modal({
  open,
  onClose,
  title,
  children,
  width = 520,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
  width?: number;
}) {
  useEffect(() => {
    if (!open) return;
    const onEsc = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    document.addEventListener("keydown", onEsc);
    return () => document.removeEventListener("keydown", onEsc);
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[60] flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div className="absolute inset-0 bg-black/45" onClick={onClose} aria-hidden />
      <div
        className="relative max-h-[90vh] w-full overflow-hidden rounded-xl shadow-2xl"
        style={{ maxWidth: width, background: "var(--tpl-surface)", color: "var(--tpl-text)" }}
      >
        <div className="flex items-center justify-between border-b px-6 py-4" style={{ borderColor: "var(--tpl-border)" }}>
          <h6 className="text-base font-semibold" style={{ color: "var(--tpl-heading)" }}>
            {title}
          </h6>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close"
            className="text-[var(--tpl-muted)] transition hover:text-[var(--tpl-accent)]"
          >
            <Icon name="close-icon" size={15} />
          </button>
        </div>
        {children}
      </div>
    </div>
  );
}

/** Labelled form field wrapper used across the popups. */
export function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-semibold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </span>
      {children}
    </label>
  );
}

export const inputCls =
  "w-full rounded-md border bg-transparent px-3 py-2 text-sm outline-none transition focus:border-[var(--tpl-accent)]";
export const inputStyle = { borderColor: "var(--tpl-border)", color: "var(--tpl-text)" } as const;

export function BtnSecondary({ children, onClick }: { children: ReactNode; onClick?: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-md border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      {children}
    </button>
  );
}

export function BtnPrimary({
  children,
  type = "button",
  onClick,
  disabled,
}: {
  children: ReactNode;
  type?: "button" | "submit";
  onClick?: () => void;
  disabled?: boolean;
}) {
  return (
    <button
      type={type}
      onClick={onClick}
      disabled={disabled}
      className="rounded-md px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
      style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
    >
      {children}
    </button>
  );
}
