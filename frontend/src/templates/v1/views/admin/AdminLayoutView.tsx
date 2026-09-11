"use client";

import { useEffect, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../components/ui/Icon";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import {
  ADMIN_LAYOUT_KEY,
  LAYOUT_KEY,
  getAdminLayout,
  reorder,
  saveMenu,
  saveWidgets,
  type LayoutConfig,
  type LayoutWidget,
  type MenuItem,
  type WidgetSlot,
} from "@/lib/layout";

/**
 * Quản trị · Menu & Widget — the shell's own composition, edited from inside it.
 *
 * Both panels stage their edits and save as a WHOLE SET, matching the API:
 * reordering is the common change, and a stream of per-row saves would let
 * another admin load a half-applied order. Array order is what the server turns
 * into `position`, so the list you see is literally the list that ships.
 *
 * The two halves are deliberately not symmetric. A menu row is a link, so rows
 * can be added and removed freely. A widget is a React component, so the
 * catalogue is fixed by the bundle and only placement is editable — there is no
 * "add widget" button because the database cannot conjure a component.
 */
export function AdminLayoutView() {
  const qc = useQueryClient();
  const [tab, setTab] = useState<"menu" | "widgets">("menu");
  const [err, setErr] = useState<string | null>(null);

  const layout = useQuery({ queryKey: ADMIN_LAYOUT_KEY, queryFn: getAdminLayout, retry: false });

  const [menu, setMenu] = useState<MenuItem[]>([]);
  const [widgets, setWidgets] = useState<LayoutWidget[]>([]);

  // Re-seed the drafts whenever the server's copy changes — on first load and
  // after every save, since both mutations return the recomputed config.
  useEffect(() => {
    if (layout.data) {
      setMenu(layout.data.menu);
      setWidgets(layout.data.widgets);
    }
  }, [layout.data]);

  function accept(next: LayoutConfig) {
    qc.setQueryData(ADMIN_LAYOUT_KEY, next);
    // The editor is looking at the shell they just changed, so refresh the live
    // copy too rather than leaving their own sidebar stale until it expires.
    qc.invalidateQueries({ queryKey: LAYOUT_KEY });
    setErr(null);
  }

  const persistMenu = useMutation({
    mutationFn: () =>
      saveMenu(
        menu.map((it) => ({
          key: it.key,
          label: it.label,
          icon: it.icon,
          href: it.href ?? "",
          permission: it.permission ?? "",
          visible: it.visible,
        })),
      ),
    onSuccess: accept,
    onError: (e) => setErr(message(e, "Không lưu được menu.")),
  });

  const persistWidgets = useMutation({
    mutationFn: () =>
      saveWidgets(
        widgets.map((w) => ({
          key: w.key,
          label: w.label,
          slot: w.slot,
          permission: w.permission ?? "",
          visible: w.visible,
        })),
      ),
    onSuccess: accept,
    onError: (e) => setErr(message(e, "Không lưu được widget.")),
  });

  // Every edit here is staged until "Lưu", and deleting a row makes it disappear
  // from the table immediately — so without a visible dirty marker an admin can
  // reorder ten entries, navigate away, and never learn the work was dropped.
  const menuDirty = layout.data ? !sameMenu(menu, layout.data.menu) : false;
  const widgetsDirty = layout.data ? !sameWidgets(widgets, layout.data.widgets) : false;

  if (layout.isPending) return <Skeleton />;
  if (layout.isError) return <ErrorState error={layout.error} onRetry={() => layout.refetch()} />;

  return (
    <section>
      <header className="mb-6">
        <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Menu &amp; Widget
        </h1>
        <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
          Thứ tự trong danh sách chính là thứ tự hiển thị. Ẩn một mục thì không ai
          thấy nữa; đặt quyền thì chỉ người có quyền đó mới thấy.
        </p>
      </header>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        <TabBtn active={tab === "menu"} onClick={() => setTab("menu")}>
          Menu ({menu.length}){menuDirty ? " •" : ""}
        </TabBtn>
        <TabBtn active={tab === "widgets"} onClick={() => setTab("widgets")}>
          Widget ({widgets.length}){widgetsDirty ? " •" : ""}
        </TabBtn>
      </div>

      {err && <Banner onDismiss={() => setErr(null)}>{err}</Banner>}

      {tab === "menu" ? (
        <MenuPanel
          items={menu}
          onChange={setMenu}
          onSave={() => persistMenu.mutate()}
          saving={persistMenu.isPending}
          dirty={menuDirty}
          onReset={() => layout.data && setMenu(layout.data.menu)}
        />
      ) : (
        <WidgetPanel
          widgets={widgets}
          onChange={setWidgets}
          onSave={() => persistWidgets.mutate()}
          saving={persistWidgets.isPending}
          dirty={widgetsDirty}
          onReset={() => layout.data && setWidgets(layout.data.widgets)}
        />
      )}
    </section>
  );
}

/* ── menu ─────────────────────────────────────────────────────────── */

function MenuPanel({
  items,
  onChange,
  onSave,
  saving,
  dirty,
  onReset,
}: {
  items: MenuItem[];
  onChange: (next: MenuItem[]) => void;
  onSave: () => void;
  saving: boolean;
  dirty: boolean;
  onReset: () => void;
}) {
  const [adding, setAdding] = useState(false);

  function patch(index: number, changes: Partial<MenuItem>) {
    onChange(items.map((it, i) => (i === index ? { ...it, ...changes } : it)));
  }

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <button
          type="button"
          onClick={() => setAdding(true)}
          className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Thêm mục
        </button>
        <SaveButton onClick={onSave} saving={saving} dirty={dirty} />
        {dirty && <ResetButton onClick={onReset} />}
      </div>

      <div
        className="overflow-x-auto rounded-xl border"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        <table className="w-full min-w-[900px] text-sm">
          <thead>
            <tr style={{ color: "var(--tpl-muted)" }}>
              <Th>Thứ tự</Th>
              <Th>Nhãn</Th>
              <Th>Icon</Th>
              <Th>Đường dẫn</Th>
              <Th>Quyền để thấy</Th>
              <Th>Hiện</Th>
              <Th align="right" />
            </tr>
          </thead>
          <tbody>
            {items.map((it, i) => (
              <tr key={it.key} className="border-t" style={{ borderColor: "var(--tpl-border)" }}>
                <td className="px-3 py-2">
                  <MoveButtons
                    onUp={() => onChange(reorder(items, i, -1))}
                    onDown={() => onChange(reorder(items, i, 1))}
                    first={i === 0}
                    last={i === items.length - 1}
                    index={i}
                  />
                </td>
                <td className="px-3 py-2">
                  <Cell value={it.label} onChange={(v) => patch(i, { label: v })} />
                  <div className="mt-0.5 font-mono text-[10px]" style={{ color: "var(--tpl-muted)" }}>
                    {it.key}
                  </div>
                </td>
                <td className="px-3 py-2">
                  <Cell value={it.icon} onChange={(v) => patch(i, { icon: v })} mono />
                </td>
                <td className="px-3 py-2">
                  <Cell
                    value={it.href ?? ""}
                    onChange={(v) => patch(i, { href: v || null })}
                    placeholder="(không có link)"
                    mono
                  />
                </td>
                <td className="px-3 py-2">
                  <Cell
                    value={it.permission ?? ""}
                    onChange={(v) => patch(i, { permission: v || null })}
                    placeholder="(ai cũng thấy)"
                    mono
                  />
                </td>
                <td className="px-3 py-2 text-center">
                  <input
                    type="checkbox"
                    checked={it.visible}
                    aria-label={`Hiện ${it.label}`}
                    onChange={(e) => patch(i, { visible: e.target.checked })}
                  />
                </td>
                <td className="px-3 py-2 text-right">
                  {/* Seeded rows can be hidden but not removed — the API refuses
                      it too, so offering the button would only produce an error. */}
                  {it.is_system ? (
                    <span
                      className="text-[10px]"
                      style={{ color: "var(--tpl-muted)" }}
                      title="Đây là lối vào chính màn hình này — xoá đi thì phải gõ URL mới quay lại được, nên nó không xoá được. Vẫn ẩn/đổi tên/đổi thứ tự bình thường."
                    >
                      không xoá được
                    </span>
                  ) : (
                    <button
                      type="button"
                      onClick={() => onChange(items.filter((_, j) => j !== i))}
                      aria-label={`Xoá ${it.label}`}
                      className="transition hover:text-[#ef4444]"
                      style={{ color: "var(--tpl-muted)" }}
                    >
                      <Icon name="little-delete" size={14} />
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <p className="mt-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
        Xoá một mục chỉ bỏ nó khỏi bản nháp — bấm “Lưu” mới ghi lại, và có thể
        bấm “Hoàn tác” trước đó. Đường dẫn phải nằm trong ứng dụng, bắt đầu bằng
        một dấu “/”; bỏ trống nếu chỉ muốn một dòng không bấm được.
      </p>

      {adding && (
        <AddMenuItemModal
          existingKeys={items.map((it) => it.key)}
          onCancel={() => setAdding(false)}
          onAdd={(item) => {
            onChange([...items, item]);
            setAdding(false);
          }}
        />
      )}
    </>
  );
}

function AddMenuItemModal({
  existingKeys,
  onCancel,
  onAdd,
}: {
  existingKeys: string[];
  onCancel: () => void;
  onAdd: (item: MenuItem) => void;
}) {
  const [key, setKey] = useState("");
  const [label, setLabel] = useState("");
  const [icon, setIcon] = useState("newsfeed-icon");
  const [href, setHref] = useState("");
  const [permission, setPermission] = useState("");

  const duplicate = existingKeys.includes(key.trim());
  const valid = /^[a-z0-9_-]{1,40}$/.test(key.trim()) && label.trim() !== "" && !duplicate;

  return (
    <Modal title="Thêm mục menu" onClose={onCancel}>
      <div className="space-y-3">
        <Field label="Mã (chữ thường, số, - _)">
          <TextInput value={key} onChange={(v) => setKey(v.toLowerCase())} autoFocus />
        </Field>
        {duplicate && (
          <p className="text-xs" style={{ color: "#ef4444" }}>
            Mã này đã có trong menu.
          </p>
        )}
        <Field label="Nhãn">
          <TextInput value={label} onChange={setLabel} />
        </Field>
        <Field label="Icon (tên trong sprite)">
          <TextInput value={icon} onChange={setIcon} />
        </Field>
        <Field label="Đường dẫn (bỏ trống = không có link)">
          <TextInput value={href} onChange={setHref} placeholder="/library/music" />
        </Field>
        <Field label="Quyền để thấy (bỏ trống = ai cũng thấy)">
          <TextInput value={permission} onChange={setPermission} placeholder="users:read:any" />
        </Field>
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          Mục mới chỉ nằm trong bản nháp — bấm “Lưu menu” mới ghi lại.
        </p>
      </div>
      <ModalActions
        onCancel={onCancel}
        confirmLabel="Thêm"
        confirmDisabled={!valid}
        onConfirm={() =>
          onAdd({
            // No id until the server assigns one; the key is what identifies the
            // row on save, so a placeholder id is honest here.
            id: `new:${key.trim()}`,
            key: key.trim(),
            label: label.trim(),
            icon: icon.trim(),
            href: href.trim() || null,
            permission: permission.trim() || null,
            position: 0,
            visible: true,
            is_system: false,
          })
        }
      />
    </Modal>
  );
}

/* ── widgets ──────────────────────────────────────────────────────── */

const SLOTS: { key: WidgetSlot; label: string; hint: string }[] = [
  { key: "left", label: "Cột trái", hint: "hiện từ màn hình lớn (lg)" },
  { key: "right", label: "Cột phải", hint: "chỉ hiện từ màn hình rất lớn (2xl)" },
];

function WidgetPanel({
  widgets,
  onChange,
  onSave,
  saving,
  dirty,
  onReset,
}: {
  widgets: LayoutWidget[];
  onChange: (next: LayoutWidget[]) => void;
  onSave: () => void;
  saving: boolean;
  dirty: boolean;
  onReset: () => void;
}) {
  function patch(key: string, changes: Partial<LayoutWidget>) {
    onChange(widgets.map((w) => (w.key === key ? { ...w, ...changes } : w)));
  }

  /**
   * Reordering happens inside a slot, but the array is flat — so splice the
   * slot's members out, move within them, and stitch the whole list back in that
   * order. The server renumbers per slot from this order.
   */
  function move(slot: WidgetSlot, index: number, delta: number) {
    const inSlot = widgets.filter((w) => w.slot === slot);
    const moved = reorder(inSlot, index, delta);
    if (moved === inSlot) return;
    const others = widgets.filter((w) => w.slot !== slot);
    onChange(slot === "left" ? [...moved, ...others] : [...others, ...moved]);
  }

  return (
    <>
      <div className="mb-3 flex flex-wrap items-center gap-2">
        <SaveButton onClick={onSave} saving={saving} dirty={dirty} />
        {dirty && <ResetButton onClick={onReset} />}
      </div>

      <div className="grid gap-4 lg:grid-cols-2">
        {SLOTS.map((slot) => {
          const inSlot = widgets.filter((w) => w.slot === slot.key);
          return (
            <div
              key={slot.key}
              className="rounded-xl border p-3"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
            >
              <h2 className="text-sm font-bold" style={{ color: "var(--tpl-heading)" }}>
                {slot.label}
                <span className="ml-2 text-[10px] font-normal" style={{ color: "var(--tpl-muted)" }}>
                  {slot.hint}
                </span>
              </h2>

              {inSlot.length === 0 ? (
                <p className="py-6 text-center text-xs" style={{ color: "var(--tpl-muted)" }}>
                  Chưa có widget nào ở cột này.
                </p>
              ) : (
                <ul className="mt-2 space-y-2">
                  {inSlot.map((w, i) => (
                    <li
                      key={w.key}
                      className="rounded-lg border p-2.5"
                      style={{ borderColor: "var(--tpl-border)" }}
                    >
                      <div className="flex items-center gap-2">
                        <MoveButtons
                          onUp={() => move(slot.key, i, -1)}
                          onDown={() => move(slot.key, i, 1)}
                          first={i === 0}
                          last={i === inSlot.length - 1}
                          index={i}
                        />
                        <div className="min-w-0 flex-1">
                          <Cell value={w.label} onChange={(v) => patch(w.key, { label: v })} />
                          <div className="mt-0.5 font-mono text-[10px]" style={{ color: "var(--tpl-muted)" }}>
                            {w.key}
                          </div>
                        </div>
                        <label className="flex shrink-0 items-center gap-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
                          <input
                            type="checkbox"
                            checked={w.visible}
                            aria-label={`Hiện ${w.label}`}
                            onChange={(e) => patch(w.key, { visible: e.target.checked })}
                          />
                          Hiện
                        </label>
                      </div>

                      <div className="mt-2 flex flex-wrap items-center gap-2">
                        <select
                          value={w.slot}
                          aria-label={`Cột của ${w.label}`}
                          onChange={(e) => patch(w.key, { slot: e.target.value as WidgetSlot })}
                          className="rounded-md border px-2 py-1 text-xs"
                          style={{
                            borderColor: "var(--tpl-border)",
                            background: "var(--tpl-bg)",
                            color: "var(--tpl-text)",
                          }}
                        >
                          {SLOTS.map((s) => (
                            <option key={s.key} value={s.key}>
                              {s.label}
                            </option>
                          ))}
                        </select>
                        <Cell
                          value={w.permission ?? ""}
                          onChange={(v) => patch(w.key, { permission: v || null })}
                          placeholder="(ai cũng thấy)"
                          mono
                        />
                      </div>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          );
        })}
      </div>

      <p className="mt-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
        Không thêm/xoá được widget ở đây: mỗi widget là một component trong mã
        nguồn, danh sách do bản build quyết định. Ở đây chỉ đổi được vị trí, nhãn,
        quyền và ẩn/hiện.
      </p>
    </>
  );
}

/* ── pieces ───────────────────────────────────────────────────────── */

function SaveButton({
  onClick,
  saving,
  dirty,
}: {
  onClick: () => void;
  saving: boolean;
  dirty: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      // Nothing to send when the draft matches the server; leaving it live would
      // invite a pointless round trip that looks like it did something.
      disabled={saving || !dirty}
      className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
      style={{ background: "var(--tpl-accent)" }}
    >
      {saving ? "Đang lưu…" : dirty ? "Lưu thay đổi" : "Đã lưu"}
    </button>
  );
}

function ResetButton({ onClick }: { onClick: () => void }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-lg border px-3 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      Hoàn tác
    </button>
  );
}

/**
 * Draft-vs-server comparison. Compares the fields the save actually sends —
 * `position` is excluded because the server derives it from array order, so a
 * pure reorder shows up as a change of ORDER, which the key sequence already
 * captures.
 */
function sameMenu(a: MenuItem[], b: MenuItem[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((x, i) => {
    const y = b[i];
    return (
      y !== undefined &&
      x.key === y.key &&
      x.label === y.label &&
      x.icon === y.icon &&
      (x.href ?? "") === (y.href ?? "") &&
      (x.permission ?? "") === (y.permission ?? "") &&
      x.visible === y.visible
    );
  });
}

function sameWidgets(a: LayoutWidget[], b: LayoutWidget[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((x, i) => {
    const y = b[i];
    return (
      y !== undefined &&
      x.key === y.key &&
      x.label === y.label &&
      x.slot === y.slot &&
      (x.permission ?? "") === (y.permission ?? "") &&
      x.visible === y.visible
    );
  });
}

function MoveButtons({
  onUp,
  onDown,
  first,
  last,
  index,
}: {
  onUp: () => void;
  onDown: () => void;
  first: boolean;
  last: boolean;
  index: number;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1">
      <span className="w-5 text-center text-xs tabular-nums" style={{ color: "var(--tpl-muted)" }}>
        {index + 1}
      </span>
      <button
        type="button"
        onClick={onUp}
        disabled={first}
        aria-label="Lên"
        className="rounded border px-1.5 text-xs disabled:opacity-30"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
      >
        ↑
      </button>
      <button
        type="button"
        onClick={onDown}
        disabled={last}
        aria-label="Xuống"
        className="rounded border px-1.5 text-xs disabled:opacity-30"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
      >
        ↓
      </button>
    </div>
  );
}

function Cell({
  value,
  onChange,
  placeholder,
  mono,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  mono?: boolean;
}) {
  return (
    <input
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className={`w-full rounded-md border px-2 py-1 text-xs ${mono ? "font-mono" : ""}`}
      style={{
        borderColor: "var(--tpl-border)",
        background: "var(--tpl-bg)",
        color: "var(--tpl-text)",
      }}
    />
  );
}

function Th({ children, align }: { children?: ReactNode; align?: "right" }) {
  return (
    <th
      className={`px-3 py-2 text-xs font-semibold uppercase tracking-wide ${
        align === "right" ? "text-right" : "text-left"
      }`}
    >
      {children}
    </th>
  );
}

function TabBtn({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="rounded-lg px-3 py-1.5 text-sm font-semibold transition"
      style={{
        background: active ? "var(--tpl-accent)" : "transparent",
        color: active ? "#fff" : "var(--tpl-muted)",
        border: `1px solid ${active ? "var(--tpl-accent)" : "var(--tpl-border)"}`,
      }}
    >
      {children}
    </button>
  );
}

function Modal({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-label={title}
    >
      <div
        className="w-full max-w-md rounded-xl p-6 shadow-lg"
        style={{ background: "var(--tpl-surface)" }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold" style={{ color: "var(--tpl-heading)" }}>
          {title}
        </h2>
        {children}
      </div>
    </div>
  );
}

function ModalActions({
  onCancel,
  onConfirm,
  confirmLabel,
  confirmDisabled,
}: {
  onCancel: () => void;
  onConfirm: () => void;
  confirmLabel: string;
  confirmDisabled?: boolean;
}) {
  return (
    <div className="mt-5 flex justify-end gap-2">
      <button
        type="button"
        onClick={onCancel}
        className="rounded-lg border px-4 py-2 text-sm font-medium transition hover:bg-[var(--tpl-surface-2)]"
        style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
      >
        Huỷ
      </button>
      <button
        type="button"
        onClick={onConfirm}
        disabled={confirmDisabled}
        className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
        style={{ background: "var(--tpl-accent)" }}
      >
        {confirmLabel}
      </button>
    </div>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <label className="mb-1 block text-xs font-semibold" style={{ color: "var(--tpl-muted)" }}>
        {label}
      </label>
      {children}
    </div>
  );
}

function TextInput({
  value,
  onChange,
  placeholder,
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  autoFocus?: boolean;
}) {
  return (
    <input
      // eslint-disable-next-line jsx-a11y/no-autofocus -- modal opens on an explicit user action
      autoFocus={autoFocus}
      value={value}
      placeholder={placeholder}
      onChange={(e) => onChange(e.target.value)}
      className="w-full rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: "var(--tpl-border)",
        background: "var(--tpl-bg)",
        color: "var(--tpl-text)",
      }}
    />
  );
}

function Banner({ children, onDismiss }: { children: ReactNode; onDismiss?: () => void }) {
  return (
    <p
      className="mb-3 flex items-center justify-between gap-3 rounded-lg border px-3 py-2 text-sm"
      style={{
        borderColor: "rgba(239,68,68,.4)",
        background: "rgba(239,68,68,.08)",
        color: "#ef4444",
      }}
    >
      <span>{children}</span>
      {onDismiss && (
        <button type="button" onClick={onDismiss} aria-label="Đóng">
          <Icon name="close-icon" size={10} />
        </button>
      )}
    </p>
  );
}

function Skeleton() {
  return (
    <div
      className="h-72 animate-pulse rounded-xl border"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface-2)" }}
    />
  );
}

function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const forbidden = error instanceof ApiError && error.status === 403;
  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        {forbidden
          ? "Tài khoản của bạn không có quyền sửa menu và widget."
          : "Không tải được cấu hình giao diện."}
      </p>
      {!forbidden && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 rounded-md border px-3 py-1.5 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
        >
          Thử lại
        </button>
      )}
    </div>
  );
}

function message(e: unknown, fallback: string): string {
  return e instanceof ApiError ? problemDisplayMessage(e.body) : fallback;
}
