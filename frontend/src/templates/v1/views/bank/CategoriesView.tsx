"use client";

import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  createCategory,
  deleteCategory,
  listCategories,
  updateCategory,
  type BankCategory,
  type CategoryKind,
} from "@/lib/bank";
import { CategoryChip } from "../../components/bank/CategoryChip";
import { BackLink } from "../../components/bank/BackLink";

/**
 * Danh mục — the ledger's taxonomy.
 *
 * Three rules come from the data and are not this screen's inventions:
 *
 * 1. **`kind` is immutable** (SPEC-03 P0.4). A category that has been an expense
 *    all year cannot become income without rewriting every month already
 *    reported, so it is chosen at creation and never offered again.
 * 2. **Seeds are read-only.** The built-ins (`user_id IS NULL`) are shared by
 *    every account; owner-scoped mutations match zero rows, so the API refuses.
 *    They are shown greyed with a badge rather than hidden — you still need to
 *    see them, because transactions attach to them.
 * 3. **Children inherit the parent's colour** (migration 0042's own comment): a
 *    donut slice and its breakdown rows should read as one family, not sixteen
 *    unrelated hues. So the colour picker appears on top-level categories only,
 *    and a child shows the colour it takes.
 *
 * Deleting is where the ledger pushes back: a category with transactions answers
 * 409 `bank/category-in-use`, because dropping it would silently detach history.
 * The confirm row therefore carries a "move them to…" picker — the API's
 * `?reassign_to=`, offered before you hit the refusal rather than after.
 */

/** The 0042 seed palette — so a category you add sits beside the built-ins. */
const DEFAULT_COLOR = "#f97316";
const PALETTE = [
  "#f97316", "#0ea5e9", "#64748b", "#8b5cf6",
  "#ec4899", "#ef4444", "#a855f7", "#14b8a6",
  "#22c55e", "#16a34a", "#f43f5e", "#10b981",
  "#06b6d4", "#eab308", "#78716c", "#6b7280",
];

const ICON_SUGGESTIONS = [
  "🍜", "☕", "🛒", "🚌", "⛽", "🏠", "💡", "🧾",
  "🛍️", "💊", "🎬", "📚", "🐶", "✈️", "🎁", "💰",
];

export function CategoriesView() {
  const qc = useQueryClient();
  const { data: categories = [], isPending } = useQuery({
    queryKey: ["bank", "categories"],
    queryFn: listCategories,
  });

  const [adding, setAdding] = useState<CategoryKind | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<string | null>(null);
  const [err, setErr] = useState<string | null>(null);

  const invalidate = () => qc.invalidateQueries({ queryKey: ["bank"] });
  const fail = (fallback: string) => (e: unknown) =>
    setErr(e instanceof ApiError ? problemDisplayMessage(e.body) : fallback);

  const create = useMutation({
    mutationFn: (body: Parameters<typeof createCategory>[0]) => createCategory(body),
    onSuccess: () => {
      setAdding(null);
      setErr(null);
      invalidate();
    },
    onError: fail("Không tạo được danh mục."),
  });

  const update = useMutation({
    mutationFn: ({ id, body }: { id: string; body: Parameters<typeof updateCategory>[1] }) =>
      updateCategory(id, body),
    onSuccess: () => {
      setEditing(null);
      setErr(null);
      invalidate();
    },
    onError: fail("Không sửa được danh mục."),
  });

  const remove = useMutation({
    mutationFn: ({ id, to }: { id: string; to?: string }) => deleteCategory(id, to),
    onSuccess: () => {
      setDeleting(null);
      setErr(null);
      invalidate();
    },
    onError: fail("Không xoá được danh mục."),
  });

  // Top-level first, children grouped under their parent. Seeds and own
  // categories live in the same tree — they are one taxonomy to the ledger.
  const tree = useMemo(() => {
    const byKind = (kind: CategoryKind) => {
      const roots = categories.filter((c) => c.kind === kind && !c.parent_id);
      return roots
        .map((root) => ({
          root,
          children: categories.filter((c) => c.parent_id === root.id),
        }))
        .sort((a, b) => a.root.name.localeCompare(b.root.name, "vi"));
    };
    return { expense: byKind("expense"), income: byKind("income") };
  }, [categories]);

  return (
    <section className="pb-8">
      <header className="mb-5">
        <BackLink />
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Danh mục
        </h1>
        <p className="mt-0.5 text-sm" style={{ color: "var(--tpl-muted)" }}>
          Biểu tượng và màu ở đây là thứ bạn thấy trên từng dòng sổ và trong biểu đồ báo cáo.
        </p>
      </header>

      {err && (
        <p
          role="alert"
          className="mb-4 rounded-lg border px-3 py-2 text-sm"
          style={{ borderColor: "rgba(239,68,68,.4)", background: "rgba(239,68,68,.08)", color: "#ef4444" }}
        >
          {err}
        </p>
      )}

      {isPending ? (
        <p className="py-10 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải…
        </p>
      ) : (
        <div className="space-y-6">
          {(["expense", "income"] as CategoryKind[]).map((kind) => (
            <KindSection
              key={kind}
              kind={kind}
              groups={tree[kind]}
              roots={categories.filter((c) => c.kind === kind && !c.parent_id)}
              adding={adding === kind}
              editing={editing}
              deleting={deleting}
              busy={create.isPending || update.isPending || remove.isPending}
              onStartAdd={() => {
                setErr(null);
                setEditing(null);
                setDeleting(null);
                setAdding(kind);
              }}
              onCancelAdd={() => setAdding(null)}
              onCreate={(body) => create.mutate(body)}
              onStartEdit={(id) => {
                setErr(null);
                setAdding(null);
                setDeleting(null);
                setEditing(id);
              }}
              onCancelEdit={() => setEditing(null)}
              onUpdate={(id, body) => update.mutate({ id, body })}
              onStartDelete={(id) => {
                setErr(null);
                setAdding(null);
                setEditing(null);
                setDeleting(id);
              }}
              onCancelDelete={() => setDeleting(null)}
              onDelete={(id, to) => remove.mutate({ id, to })}
            />
          ))}
        </div>
      )}
    </section>
  );
}

/* ── one kind (chi / thu) ─────────────────────────────────────────── */

interface SectionProps {
  kind: CategoryKind;
  groups: { root: BankCategory; children: BankCategory[] }[];
  roots: BankCategory[];
  adding: boolean;
  editing: string | null;
  deleting: string | null;
  busy: boolean;
  onStartAdd: () => void;
  onCancelAdd: () => void;
  onCreate: (body: Parameters<typeof createCategory>[0]) => void;
  onStartEdit: (id: string) => void;
  onCancelEdit: () => void;
  onUpdate: (id: string, body: Parameters<typeof updateCategory>[1]) => void;
  onStartDelete: (id: string) => void;
  onCancelDelete: () => void;
  onDelete: (id: string, to?: string) => void;
}

function KindSection(p: SectionProps) {
  const label = p.kind === "expense" ? "Chi" : "Thu";
  const all = p.groups.flatMap((g) => [g.root, ...g.children]);

  return (
    <div className="rounded-xl border" style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}>
      <div className="flex items-center justify-between border-b px-4 py-3" style={{ borderColor: "var(--tpl-border)" }}>
        <h2 className="text-sm font-bold uppercase tracking-wide" style={{ color: "var(--tpl-muted)" }}>
          {label} · {all.length}
        </h2>
        <button
          type="button"
          onClick={p.onStartAdd}
          className="rounded-lg px-3 py-1.5 text-xs font-bold text-white transition hover:opacity-90"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          + Thêm
        </button>
      </div>

      {p.adding && (
        <div className="border-b px-4 py-4" style={{ borderColor: "var(--tpl-border)" }}>
          <CategoryForm
            kind={p.kind}
            roots={p.roots}
            busy={p.busy}
            submitLabel="Tạo danh mục"
            onCancel={p.onCancelAdd}
            onSubmit={(v) =>
              p.onCreate({
                name: v.name,
                kind: p.kind,
                parent_id: v.parentID,
                icon: v.icon,
                color: v.color,
              })
            }
          />
        </div>
      )}

      {p.groups.length === 0 && !p.adding ? (
        <p className="px-4 py-8 text-center text-sm" style={{ color: "var(--tpl-muted)" }}>
          Chưa có danh mục {label.toLowerCase()} nào.
        </p>
      ) : (
        <ul className="divide-y" style={{ borderColor: "var(--tpl-border)" }}>
          {p.groups.map(({ root, children }) => (
            <li key={root.id}>
              <CategoryRow
                category={root}
                parentColor={null}
                roots={p.roots}
                all={p.groups.flatMap((g) => [g.root, ...g.children])}
                editing={p.editing === root.id}
                deleting={p.deleting === root.id}
                busy={p.busy}
                onStartEdit={() => p.onStartEdit(root.id)}
                onCancelEdit={p.onCancelEdit}
                onUpdate={(body) => p.onUpdate(root.id, body)}
                onStartDelete={() => p.onStartDelete(root.id)}
                onCancelDelete={p.onCancelDelete}
                onDelete={(to) => p.onDelete(root.id, to)}
              />
              {children.length > 0 && (
                <ul className="border-t" style={{ borderColor: "var(--tpl-border)" }}>
                  {children.map((child) => (
                    <li key={child.id} className="border-b last:border-b-0" style={{ borderColor: "var(--tpl-border)" }}>
                      <CategoryRow
                        category={child}
                        parentColor={root.color}
                        roots={p.roots}
                        all={p.groups.flatMap((g) => [g.root, ...g.children])}
                        nested
                        editing={p.editing === child.id}
                        deleting={p.deleting === child.id}
                        busy={p.busy}
                        onStartEdit={() => p.onStartEdit(child.id)}
                        onCancelEdit={p.onCancelEdit}
                        onUpdate={(body) => p.onUpdate(child.id, body)}
                        onStartDelete={() => p.onStartDelete(child.id)}
                        onCancelDelete={p.onCancelDelete}
                        onDelete={(to) => p.onDelete(child.id, to)}
                      />
                    </li>
                  ))}
                </ul>
              )}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/* ── one row ──────────────────────────────────────────────────────── */

function CategoryRow({
  category,
  parentColor,
  roots,
  all,
  nested = false,
  editing,
  deleting,
  busy,
  onStartEdit,
  onCancelEdit,
  onUpdate,
  onStartDelete,
  onCancelDelete,
  onDelete,
}: {
  category: BankCategory;
  /** Set for a child — the colour it takes from its parent (0042's convention). */
  parentColor: string | null;
  roots: BankCategory[];
  all: BankCategory[];
  nested?: boolean;
  editing: boolean;
  deleting: boolean;
  busy: boolean;
  onStartEdit: () => void;
  onCancelEdit: () => void;
  onUpdate: (body: Parameters<typeof updateCategory>[1]) => void;
  onStartDelete: () => void;
  onCancelDelete: () => void;
  onDelete: (to?: string) => void;
}) {
  const [reassignTo, setReassignTo] = useState("");

  if (editing) {
    return (
      <div className={`px-4 py-4 ${nested ? "pl-12" : ""}`}>
        <CategoryForm
          kind={category.kind}
          roots={roots.filter((r) => r.id !== category.id)}
          initial={{
            name: category.name,
            parentID: category.parent_id,
            icon: category.icon,
            color: category.color,
          }}
          inheritedColor={parentColor}
          busy={busy}
          submitLabel="Lưu"
          onCancel={onCancelEdit}
          onSubmit={(v) => onUpdate({ name: v.name, parent_id: v.parentID, icon: v.icon, color: v.color })}
        />
      </div>
    );
  }

  return (
    <div className={`flex items-center gap-3 px-4 py-2.5 ${nested ? "pl-12" : ""}`}>
      <CategoryChip
        icon={category.icon}
        color={category.color ?? parentColor}
        name={category.name}
        size={nested ? 30 : 36}
      />

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {category.name}
        </p>
        {category.seed && (
          <span className="text-[11px]" style={{ color: "var(--tpl-muted)" }}>
            Mặc định — không sửa được
          </span>
        )}
      </div>

      {deleting ? (
        <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
          {/* The reassign picker is offered BEFORE the refusal: a category with
              transactions answers 409, and finding that out after clicking is a
              worse way to learn it. Empty = delete without moving anything. */}
          <select
            value={reassignTo}
            onChange={(e) => setReassignTo(e.target.value)}
            className="rounded-md border bg-transparent px-2 py-1 text-xs"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          >
            <option value="">Không chuyển giao dịch</option>
            {all
              .filter((c) => c.id !== category.id && c.kind === category.kind)
              .map((c) => (
                <option key={c.id} value={c.id}>
                  Chuyển sang: {c.name}
                </option>
              ))}
          </select>
          <button
            type="button"
            disabled={busy}
            onClick={() => onDelete(reassignTo || undefined)}
            className="rounded-md px-2 py-1 text-xs font-semibold text-white disabled:opacity-50"
            style={{ background: "#ef4444" }}
          >
            Xoá
          </button>
          <button type="button" onClick={onCancelDelete} className="text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Huỷ
          </button>
        </div>
      ) : (
        !category.seed && (
          <div className="flex shrink-0 items-center gap-3">
            <button type="button" onClick={onStartEdit} className="text-xs font-semibold transition hover:opacity-80" style={{ color: "var(--tpl-accent)" }}>
              Sửa
            </button>
            <button type="button" onClick={onStartDelete} className="text-xs font-semibold transition hover:text-[#ef4444]" style={{ color: "var(--tpl-muted)" }}>
              Xoá
            </button>
          </div>
        )
      )}
    </div>
  );
}

/* ── the form, shared by create and edit ──────────────────────────── */

interface FormValue {
  name: string;
  parentID: string | null;
  icon: string | null;
  color: string | null;
}

function CategoryForm({
  kind,
  roots,
  initial,
  inheritedColor,
  busy,
  submitLabel,
  onCancel,
  onSubmit,
}: {
  kind: CategoryKind;
  roots: BankCategory[];
  initial?: FormValue;
  /** A child's inherited colour, for the swatch beside the explanation. */
  inheritedColor?: string | null;
  busy: boolean;
  submitLabel: string;
  onCancel: () => void;
  onSubmit: (v: FormValue) => void;
}) {
  const [name, setName] = useState(initial?.name ?? "");
  const [parentID, setParentID] = useState(initial?.parentID ?? "");
  const [icon, setIcon] = useState(initial?.icon ?? "");
  const [color, setColor] = useState<string>(initial?.color ?? DEFAULT_COLOR);

  // A child takes its parent's colour (0042), so the picker only makes sense on
  // a top-level category — showing it on a child would promise something the
  // reports do not honour.
  const isChild = parentID !== "";

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!name.trim()) return;
        onSubmit({
          name: name.trim(),
          parentID: parentID || null,
          icon: icon.trim() || null,
          color: isChild ? null : color,
        });
      }}
      className="space-y-3"
    >
      <div className="flex flex-wrap items-end gap-3">
        <label className="min-w-[180px] flex-1">
          <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Tên
          </span>
          <input
            value={name}
            onChange={(e) => setName(e.target.value)}
            autoFocus
            maxLength={80}
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm outline-none"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          />
        </label>

        <label className="min-w-[160px]">
          <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Thuộc nhóm
          </span>
          <select
            value={parentID}
            onChange={(e) => setParentID(e.target.value)}
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-sm"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          >
            <option value="">— Nhóm gốc —</option>
            {roots.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </select>
        </label>

        <label className="w-24">
          <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Biểu tượng
          </span>
          <input
            value={icon}
            onChange={(e) => setIcon(e.target.value)}
            maxLength={16}
            placeholder="🍜"
            className="w-full rounded-lg border bg-transparent px-3 py-2 text-center text-lg outline-none"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-text)" }}
          />
        </label>
      </div>

      <div className="flex flex-wrap gap-1.5">
        {ICON_SUGGESTIONS.map((e) => (
          <button
            key={e}
            type="button"
            onClick={() => setIcon(e)}
            className="grid h-8 w-8 place-items-center rounded-md border text-base transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: icon === e ? "var(--tpl-accent)" : "var(--tpl-border)" }}
          >
            {e}
          </button>
        ))}
      </div>

      {isChild ? (
        <p className="flex items-center gap-2 text-xs" style={{ color: "var(--tpl-muted)" }}>
          <span
            aria-hidden
            className="inline-block h-3 w-3 rounded-full"
            style={{ background: inheritedColor || "#64748b" }}
          />
          Màu lấy theo nhóm cha — để một lát biểu đồ và các mục con của nó đọc ra cùng một họ.
        </p>
      ) : (
        <div>
          <span className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
            Màu
          </span>
          <div className="flex flex-wrap items-center gap-1.5">
            {PALETTE.map((c) => (
              <button
                key={c}
                type="button"
                onClick={() => setColor(c)}
                aria-label={`Màu ${c}`}
                className="h-7 w-7 rounded-full transition"
                style={{ background: c, outline: color === c ? "2px solid var(--tpl-heading)" : "none", outlineOffset: 2 }}
              />
            ))}
            {/* A native colour input for anything the palette doesn't cover. It
                emits "#rrggbb", which is exactly what 0042's CHECK accepts. */}
            <input
              type="color"
              value={color}
              onChange={(e) => setColor(e.target.value)}
              aria-label="Màu tuỳ chọn"
              className="h-7 w-10 cursor-pointer rounded border bg-transparent"
              style={{ borderColor: "var(--tpl-border)" }}
            />
          </div>
        </div>
      )}

      <div className="flex items-center gap-2 pt-1">
        <button
          type="submit"
          disabled={!name.trim() || busy}
          className="rounded-lg px-4 py-2 text-sm font-bold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "linear-gradient(135deg, var(--tpl-accent), var(--tpl-accent-2))" }}
        >
          {submitLabel}
        </button>
        <button type="button" onClick={onCancel} className="text-sm font-semibold" style={{ color: "var(--tpl-muted)" }}>
          Huỷ
        </button>
        {!initial && (
          <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            Loại ({kind === "expense" ? "chi" : "thu"}) không đổi được sau khi tạo.
          </span>
        )}
      </div>
    </form>
  );
}
