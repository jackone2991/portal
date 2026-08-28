"use client";

import { useMemo, useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../components/ui/Icon";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { can, useSession } from "@/lib/session";
import {
  createRole,
  deleteRole,
  getMatrix,
  orderRolesByHierarchy,
  setRolePermissions,
  type MatrixPermission,
  type MatrixRole,
  type PermissionMatrix,
} from "@/lib/admin";

/**
 * Quản trị · Vai trò & quyền — the permission matrix.
 *
 * Permissions down, roles across, ordered parent-before-child so inheritance
 * reads left-to-right. Two marks, deliberately different:
 *
 *   ☑ a DIRECT grant on this role — the editable checkbox
 *   ·  INHERITED from an ancestor — shown, never editable here, because the
 *      place to change it is the ancestor. A matrix that let you "uncheck" an
 *      inherited cell would be lying about what it does.
 *
 * Edits are staged per column and saved as a whole set, matching the API: one
 * role's grants replace atomically, so a half-applied row can never exist.
 */

export function AdminRolesView() {
  const qc = useQueryClient();
  const { data: me } = useSession();
  const mayWrite = can(me?.permissions, "rbac:role:write");

  const [draft, setDraft] = useState<Record<string, string[]>>({});
  const [err, setErr] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [group, setGroup] = useState<string>("all");

  const matrix = useQuery({ queryKey: ["admin", "matrix"], queryFn: getMatrix, retry: false });

  const roles = useMemo(
    () => orderRolesByHierarchy(matrix.data?.roles ?? []),
    [matrix.data?.roles],
  );
  const permissions = matrix.data?.permissions ?? [];

  const groups = useMemo(() => {
    const seen: string[] = [];
    for (const p of permissions) if (!seen.includes(p.group)) seen.push(p.group);
    return seen;
  }, [permissions]);

  const visible = group === "all" ? permissions : permissions.filter((p) => p.group === group);

  function accept(next: PermissionMatrix) {
    qc.setQueryData(["admin", "matrix"], next);
    // The response is the recomputed matrix, so every staged column is now
    // either saved or stale. Dropping the whole draft is the honest reset:
    // keeping other columns would leave them diffed against numbers that moved.
    setDraft({});
  }

  const save = useMutation({
    mutationFn: (v: { role: MatrixRole; codes: string[] }) => setRolePermissions(v.role.id, v.codes),
    onSuccess: accept,
    onError: (e) => setErr(message(e, "Không lưu được ma trận quyền.")),
  });

  const create = useMutation({
    mutationFn: createRole,
    onSuccess: () => {
      setCreating(false);
      qc.invalidateQueries({ queryKey: ["admin", "matrix"] });
    },
    onError: (e) => setErr(message(e, "Không tạo được vai trò.")),
  });

  const remove = useMutation({
    mutationFn: (r: MatrixRole) => deleteRole(r.id),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["admin", "matrix"] }),
    onError: (e) => setErr(message(e, "Không xoá được vai trò.")),
  });

  /** Staged direct grants for a role, falling back to what the server has. */
  function directOf(role: MatrixRole): string[] {
    return draft[role.code] ?? role.direct;
  }

  function toggle(role: MatrixRole, code: string) {
    const current = directOf(role);
    const next = current.includes(code)
      ? current.filter((c) => c !== code)
      : [...current, code].sort();
    setDraft((d) => ({ ...d, [role.code]: next }));
  }

  function isDirty(role: MatrixRole): boolean {
    const d = draft[role.code];
    if (!d) return false;
    return d.length !== role.direct.length || d.some((c) => !role.direct.includes(c));
  }

  if (matrix.isPending) return <Skeleton />;
  if (matrix.isError) return <ErrorState error={matrix.error} onRetry={() => matrix.refetch()} />;

  return (
    <section>
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Vai trò &amp; quyền
          </h1>
          <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
            Vai trò con kế thừa toàn bộ quyền của cha. Ô mờ là quyền kế thừa —
            muốn đổi thì sửa ở vai trò cha.
          </p>
        </div>
        {mayWrite && (
          <button
            type="button"
            onClick={() => setCreating(true)}
            className="rounded-lg border px-4 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
            style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
          >
            Thêm vai trò
          </button>
        )}
      </header>

      {err && <Banner onDismiss={() => setErr(null)}>{err}</Banner>}

      <Hierarchy roles={roles} onDelete={mayWrite ? (r) => remove.mutate(r) : undefined} />

      <div className="mb-3 flex flex-wrap items-center gap-2">
        <GroupBtn active={group === "all"} onClick={() => setGroup("all")}>
          Tất cả
        </GroupBtn>
        {groups.map((g) => (
          <GroupBtn key={g} active={group === g} onClick={() => setGroup(g)}>
            {g}
          </GroupBtn>
        ))}
      </div>

      <div
        className="overflow-x-auto rounded-xl border"
        style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
      >
        <table className="w-full text-sm">
          <thead>
            <tr>
              <th
                className="sticky left-0 z-10 px-3 py-2 text-left text-xs font-semibold uppercase tracking-wide"
                style={{ background: "var(--tpl-surface)", color: "var(--tpl-muted)" }}
              >
                Quyền
              </th>
              {roles.map((r) => (
                <th key={r.code} className="px-2 py-2 text-center align-bottom">
                  <div className="text-xs font-bold" style={{ color: "var(--tpl-heading)" }}>
                    {r.code}
                  </div>
                  <div className="text-[10px] font-normal" style={{ color: "var(--tpl-muted)" }}>
                    {r.parent_code ? `↑ ${r.parent_code}` : "gốc"} · {r.user_count}
                  </div>
                  {mayWrite && isDirty(r) && (
                    <button
                      type="button"
                      onClick={() => save.mutate({ role: r, codes: directOf(r) })}
                      disabled={save.isPending}
                      className="mt-1 rounded px-2 py-0.5 text-[10px] font-bold text-white transition hover:opacity-90 disabled:opacity-50"
                      style={{ background: "var(--tpl-accent)" }}
                    >
                      {save.isPending ? "…" : "Lưu"}
                    </button>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {visible.map((p) => (
              <PermissionRow
                key={p.code}
                perm={p}
                roles={roles}
                directOf={directOf}
                editable={mayWrite}
                onToggle={toggle}
              />
            ))}
          </tbody>
        </table>
      </div>

      {creating && (
        <CreateRoleModal
          roles={roles}
          pending={create.isPending}
          onCancel={() => setCreating(false)}
          onCreate={(body) => create.mutate(body)}
        />
      )}
    </section>
  );
}

/* ── rows ─────────────────────────────────────────────────────────── */

function PermissionRow({
  perm,
  roles,
  directOf,
  editable,
  onToggle,
}: {
  perm: MatrixPermission;
  roles: MatrixRole[];
  directOf: (r: MatrixRole) => string[];
  editable: boolean;
  onToggle: (r: MatrixRole, code: string) => void;
}) {
  return (
    <tr className="border-t" style={{ borderColor: "var(--tpl-border)" }}>
      <th
        className="sticky left-0 z-10 max-w-[280px] px-3 py-1.5 text-left font-normal"
        style={{ background: "var(--tpl-surface)" }}
      >
        <div className="truncate font-mono text-xs" style={{ color: "var(--tpl-heading)" }}>
          {perm.code}
        </div>
        {perm.description && (
          <div className="truncate text-[11px]" style={{ color: "var(--tpl-muted)" }} title={perm.description}>
            {perm.description}
          </div>
        )}
      </th>
      {roles.map((r) => {
        const direct = directOf(r).includes(perm.code);
        // Inherited = the role holds it without a direct grant of its own.
        //
        // `can` rather than `includes`, because `effective` carries grants
        // verbatim: superadmin's whole set is `["*"]`, so a plain membership test
        // would draw every superadmin cell empty and tell the reader that the
        // unrestricted role holds nothing.
        //
        // The server sends `effective` computed from SAVED grants, so a staged
        // edit on the parent is not reflected until it is saved — which is
        // right: the cell describes what is in force, not what is drafted.
        const inherited = !direct && (r.effective.includes(perm.code) || can(r.effective, perm.code));
        return (
          <td key={r.code} className="px-2 py-1.5 text-center">
            {inherited ? (
              <span
                title={
                  r.effective.includes("*")
                    ? "Có qua quyền đại diện (*)"
                    : `Kế thừa từ ${r.parent_code ?? "vai trò cha"}`
                }
                className="inline-block h-2 w-2 rounded-full"
                style={{ background: "var(--tpl-muted)", opacity: 0.45 }}
              />
            ) : (
              <input
                type="checkbox"
                checked={direct}
                disabled={!editable}
                aria-label={`${perm.code} cho ${r.code}`}
                onChange={() => onToggle(r, perm.code)}
              />
            )}
          </td>
        );
      })}
    </tr>
  );
}

function Hierarchy({
  roles,
  onDelete,
}: {
  roles: MatrixRole[];
  onDelete?: (r: MatrixRole) => void;
}) {
  return (
    <div
      className="mb-5 flex flex-wrap items-center gap-2 rounded-xl border px-3 py-2.5"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      {roles.map((r, i) => (
        <span key={r.code} className="flex items-center gap-2">
          {i > 0 && (
            <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
              {r.parent_code === roles[i - 1]?.code ? "→" : "·"}
            </span>
          )}
          <span
            className="flex items-center gap-1 rounded px-2 py-0.5 text-xs font-semibold"
            style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-heading)" }}
            title={r.description || r.name}
          >
            {r.code}
            {/* Only non-system, unused roles can go — the API refuses the rest,
                so the button is not offered where it would only produce a 409. */}
            {onDelete && !r.is_system && r.user_count === 0 && (
              <button
                type="button"
                onClick={() => onDelete(r)}
                aria-label={`Xoá vai trò ${r.code}`}
                className="transition hover:text-[#ef4444]"
                style={{ color: "var(--tpl-muted)" }}
              >
                <Icon name="little-delete" size={12} />
              </button>
            )}
          </span>
        </span>
      ))}
    </div>
  );
}

/* ── create ───────────────────────────────────────────────────────── */

function CreateRoleModal({
  roles,
  pending,
  onCancel,
  onCreate,
}: {
  roles: MatrixRole[];
  pending: boolean;
  onCancel: () => void;
  onCreate: (body: { code: string; name: string; description: string; parent_code: string }) => void;
}) {
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [parent, setParent] = useState("");

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onClick={onCancel}
      role="dialog"
      aria-modal="true"
      aria-label="Thêm vai trò"
    >
      <div
        className="w-full max-w-md rounded-xl p-6 shadow-lg"
        style={{ background: "var(--tpl-surface)" }}
        onClick={(e) => e.stopPropagation()}
      >
        <h2 className="mb-4 text-lg font-semibold" style={{ color: "var(--tpl-heading)" }}>
          Thêm vai trò
        </h2>
        <div className="space-y-3">
          <Field label="Mã (chữ thường, số, - _)">
            <input
              value={code}
              onChange={(e) => setCode(e.target.value.toLowerCase())}
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            />
          </Field>
          <Field label="Tên hiển thị">
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            />
          </Field>
          <Field label="Mô tả">
            <input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            />
          </Field>
          <Field label="Kế thừa từ">
            <select
              value={parent}
              onChange={(e) => setParent(e.target.value)}
              className="w-full rounded-lg border px-3 py-2 text-sm"
              style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-bg)", color: "var(--tpl-text)" }}
            >
              <option value="">— Không kế thừa (vai trò gốc) —</option>
              {roles.map((r) => (
                <option key={r.code} value={r.code}>
                  {r.code}
                </option>
              ))}
            </select>
          </Field>
        </div>
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
            onClick={() => onCreate({ code: code.trim(), name: name.trim(), description, parent_code: parent })}
            disabled={pending || !code.trim()}
            className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-50"
            style={{ background: "var(--tpl-accent)" }}
          >
            {pending ? "Đang tạo…" : "Tạo"}
          </button>
        </div>
      </div>
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

/* ── pieces ───────────────────────────────────────────────────────── */

function GroupBtn({
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
      className="rounded-md px-2.5 py-1 text-xs font-semibold transition"
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
          ? "Tài khoản của bạn không có quyền xem vai trò và ma trận quyền."
          : "Không tải được ma trận quyền."}
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
