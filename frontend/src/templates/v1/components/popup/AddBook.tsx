"use client";

import { useState, type FormEvent } from "react";
import { Modal, Field, BtnPrimary, BtnSecondary, inputCls, inputStyle } from "./Modal";
import { Icon } from "../ui/Icon";

/**
 * "Add Book" dialog — port of the Olympus create-page popup
 * (`Fav Page - Settings And Create Popup.html`), repurposed for adding a book
 * (comic / novel) to the library. Triggered from the library views.
 */

export type NewBook = {
  title: string;
  author: string;
  category: string;
  description: string;
};

const CATEGORIES = ["Comic", "Novel", "Manga", "Light Novel", "Short Story"];

export function AddBook({
  open = false,
  onClose = () => {},
  onSave,
}: {
  open?: boolean;
  onClose?: () => void;
  onSave?: (book: NewBook) => void;
}) {
  const [title, setTitle] = useState("");
  const [author, setAuthor] = useState("");
  const [category, setCategory] = useState<string>(CATEGORIES[0] ?? "Comic");
  const [description, setDescription] = useState("");
  const [cover, setCover] = useState<string | null>(null);

  function submit(e: FormEvent) {
    e.preventDefault();
    if (!title.trim()) return;
    onSave?.({ title: title.trim(), author: author.trim(), category, description: description.trim() });
    onClose();
  }

  return (
    <Modal open={open} onClose={onClose} title="Add Book" width={560}>
      <form onSubmit={submit} className="space-y-4 p-6">
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label="Title">
            <input className={inputCls} style={inputStyle} value={title} onChange={(e) => setTitle(e.target.value)} placeholder="The Majestic Canyon" autoFocus />
          </Field>
          <Field label="Author">
            <input className={inputCls} style={inputStyle} value={author} onChange={(e) => setAuthor(e.target.value)} placeholder="Marina Valentine" />
          </Field>
        </div>

        <Field label="Category">
          <select className={inputCls} style={inputStyle} value={category} onChange={(e) => setCategory(e.target.value)}>
            {CATEGORIES.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        </Field>

        <Field label="Description">
          <textarea
            className={`${inputCls} resize-none`}
            style={inputStyle}
            rows={3}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="Write a little description about the book"
          />
        </Field>

        <label
          className="flex cursor-pointer items-center gap-3 rounded-md border border-dashed px-4 py-3 text-sm transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          <Icon name="camera-icon" size={18} />
          <span>{cover ? "Cover selected" : "Upload a cover image"}</span>
          <input
            type="file"
            accept="image/*"
            className="hidden"
            onChange={(e) => setCover(e.target.files?.[0]?.name ?? null)}
          />
        </label>

        <div className="flex justify-end gap-2 pt-1">
          <BtnSecondary onClick={onClose}>Cancel</BtnSecondary>
          <BtnPrimary type="submit">Add Book</BtnPrimary>
        </div>
      </form>
    </Modal>
  );
}
