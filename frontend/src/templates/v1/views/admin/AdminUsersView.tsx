"use client";

import { useState, type ReactNode } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Icon } from "../../components/ui/Icon";
import { ApiError } from "@/lib/api-client";
import { problemDisplayMessage } from "@/lib/problems";
import { useSession, can } from "@/lib/session";
import {
  APPROVAL_LABEL,
  createUser,
  decideUser,
  deleteUser,
  getMatrix,
  listUsers,
  setUserDisabled,
  setUserRoles,
  updateUser,
  type AdminUser,
  type ApprovalStatus,
} from "@/lib/admin";

/**
 * Quản trị · Người dùng — the directory and the approval queue in one screen.
 *
 * They are the same list under different filters, so they share a screen rather
 * than splitting into "queue" and "users": an approver almost always wants to
 * see what a pending account already looks like (roles, when it registered)
 * before deciding, and a second page would hide exactly that.
 *
 * Which buttons appear is driven by the caller's own permissions from
 * `/auth/me`, not by role names — roles are editable from the matrix screen, so
 * "is this person an admin" stopped being a usable proxy for "may they do this".
 * The API re-checks everything regardless; hiding is courtesy, not security.
 */

const PAGE_SIZE = 25;

type Tab = ApprovalStatus | "all";

const TABS: { key: Tab; label: string }[] = [
  { key: "pending", label: "Chờ duyệt" },
  { key: "approved", label: "Đã duyệt" },
  { key: "rejected", label: "Từ chối" },
  { key: "all", label: "Tất cả" },
];

export function AdminUsersView() {
  const qc = useQueryClient();
  const { data: me } = useSession();

  const [tab, setTab] = useState<Tab>("pending");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [page, setPage] = useState(0);
  const [err, setErr] = useState<string | null>(null);
  const [rejecting, setRejecting] = useState<AdminUser | null>(null);
  const [editingRoles, setEditingRoles] = useState<AdminUser | null>(null);
  const [editing, setEditing] = useState<AdminUser | null>(null);
  const [deleting, setDeleting] = useState<AdminUser | null>(null);
  const [creating, setCreating] = useState(false);

  const mayApprove = can(me?.permissions, "users:approve");
  // `users:write:any` covers create, edit and disable — the same permission the
  // API gates all three on.
  const mayWrite = can(me?.permissions, "users:write:any");
  const mayDelete = can(me?.permissions, "users:delete:any");
  const mayAssign = can(me?.permissions, "rbac:role:assign");

  const users = useQuery({
    queryKey: ["admin", "users", tab, query, page],
    queryFn: () =>
      listUsers({
        status: tab === "all" ? "" : tab,
        q: query,
        limit: PAGE_SIZE,
        offset: page * PAGE_SIZE,
      }),
  });

  // Only fetched when the role editor opens — the directory itself never needs
  // the matrix, and it is the heavier of the two calls.
  const matrix = useQuery({
    queryKey: ["admin", "matrix"],
    queryFn: getMatrix,
    enabled: editingRoles !== null,
  });

  function invalidate() {
    qc.invalidateQueries({ queryKey: ["admin", "users"] });
  }

  const decide = useMutation({
    mutationFn: (v: { user: AdminUser; action: "approve" | "reject" | "revoke-approval"; note?: string }) =>
      decideUser(v.user.id, v.action, v.note),
    onSuccess: () => {
      setRejecting(null);
      invalidate();
    },
    onError: (e) => setErr(message(e, "Không thực hiện được thao tác.")),
  });

  const toggleDisabled = useMutation({
    mutationFn: (u: AdminUser) => setUserDisabled(u.id, !u.disabled),
    onSuccess: invalidate,
    onError: (e) => setErr(message(e, "Không đổi được trạng thái tài khoản.")),
  });

  const create = useMutation({
    mutationFn: createUser,
    onSuccess: () => {
      setCreating(false);
      invalidate();
    },
    onError: (e) => setErr(message(e, "Không tạo được tài khoản.")),
  });

  const edit = useMutation({
    mutationFn: (v: { id: string; body: { email: string; display_name: string; password?: string } }) =>
      updateUser(v.id, v.body),
    onSuccess: () => {
      setEditing(null);
      invalidate();
    },
    onError: (e) => setErr(message(e, "Không lưu được thay đổi.")),
  });

  const remove = useMutation({
    mutationFn: (v: { id: string; confirmEmail: string }) => deleteUser(v.id, v.confirmEmail),
    onSuccess: () => {
      setDeleting(null);
      invalidate();
    },
    onError: (e) => setErr(message(e, "Không xoá được tài khoản.")),
  });

  const saveRoles = useMutation({
    mutationFn: (v: { id: string; roles: string[] }) => setUserRoles(v.id, v.roles),
    onSuccess: () => {
      setEditingRoles(null);
      invalidate();
    },
    onError: (e) => setErr(message(e, "Không lưu được vai trò.")),
  });

  const counts = users.data?.counts;
  const rows = users.data?.users ?? [];
  const total = users.data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE));

  return (
    <section>
      <header className="mb-6 flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-semibold" style={{ color: "var(--tpl-heading)" }}>
            Người dùng
          </h1>
          <p className="mt-1 text-xs" style={{ color: "var(--tpl-muted)" }}>
            Tài khoản mới đăng ký phải được phê duyệt mới đăng nhập được.
          </p>
        </div>
        <div className="flex items-center gap-2">
        {mayWrite && (
          <button
            type="button"
            onClick={() => {
              setErr(null);
              setCreating(true);
            }}
            className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90"
            style={{ background: "var(--tpl-accent)" }}
          >
            Thêm người dùng
          </button>
        )}
        <input
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              setPage(0);
              setQuery(search.trim());
            }
          }}
          placeholder="Tìm theo email hoặc tên…"
          className="w-64 rounded-lg border px-3 py-2 text-sm"
          style={{
            borderColor: "var(--tpl-border)",
            background: "var(--tpl-bg)",
            color: "var(--tpl-text)",
          }}
        />
        </div>
      </header>

      <div className="mb-4 flex flex-wrap items-center gap-2">
        {TABS.map((t) => (
          <TabBtn
            key={t.key}
            active={tab === t.key}
            onClick={() => {
              setTab(t.key);
              setPage(0);
            }}
          >
            {t.label}
            {t.key !== "all" && counts ? <Badge n={counts[t.key]} active={tab === t.key} /> : null}
          </TabBtn>
        ))}
      </div>

      {err && <Banner onDismiss={() => setErr(null)}>{err}</Banner>}

      {users.isPending ? (
        <Skeleton />
      ) : users.isError ? (
        <ErrorState error={users.error} onRetry={() => users.refetch()} />
      ) : rows.length === 0 ? (
        <Empty tab={tab} />
      ) : (
        <div
          className="overflow-x-auto rounded-xl border"
          style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
        >
          <table className="w-full min-w-[820px] text-sm">
            <thead>
              <tr style={{ color: "var(--tpl-muted)" }}>
                <Th>Tài khoản</Th>
                <Th>Vai trò</Th>
                <Th>Trạng thái</Th>
                <Th>Đăng ký</Th>
                <Th align="right">Thao tác</Th>
              </tr>
            </thead>
            <tbody>
              {rows.map((u) => (
                <tr key={u.id} className="border-t" style={{ borderColor: "var(--tpl-border)" }}>
                  <td className="px-3 py-2.5">
                    <div className="flex items-center gap-2.5">
                      <span
                        className="grid h-8 w-8 shrink-0 place-items-center rounded-full text-xs font-bold text-white"
                        style={{ background: "var(--tpl-accent)" }}
                      >
                        {initial(u.display_name || u.email)}
                      </span>
                      <div className="min-w-0">
                        <div className="truncate font-semibold" style={{ color: "var(--tpl-heading)" }}>
                          {u.display_name}
                          {u.id === me?.id && (
                            <span className="ml-1.5 text-[10px] font-normal" style={{ color: "var(--tpl-muted)" }}>
                              (bạn)
                            </span>
                          )}
                        </div>
                        <div className="truncate text-xs" style={{ color: "var(--tpl-muted)" }}>
                          {u.email}
                        </div>
                      </div>
                    </div>
                  </td>

                  <td className="px-3 py-2.5">
                    <div className="flex flex-wrap gap-1">
                      {u.roles.length === 0 ? (
                        <span className="text-xs" style={{ color: "var(--tpl-muted)" }}>
                          —
                        </span>
                      ) : (
                        u.roles.map((c) => <Chip key={c}>{c}</Chip>)
                      )}
                    </div>
                  </td>

                  <td className="px-3 py-2.5">
                    <StatusBadge status={u.approval_status} disabled={u.disabled} />
                    {u.approval_note && (
                      <div className="mt-1 max-w-[220px] truncate text-xs" style={{ color: "var(--tpl-muted)" }} title={u.approval_note}>
                        {u.approval_note}
                      </div>
                    )}
                  </td>

                  <td className="px-3 py-2.5 text-xs" style={{ color: "var(--tpl-muted)" }}>
                    {formatDate(u.created_at)}
                  </td>

                  <td className="px-3 py-2.5">
                    <div className="flex flex-wrap items-center justify-end gap-2">
                      {/* Self-targeting is refused by the API, so it is not offered. */}
                      {u.id !== me?.id && mayApprove && u.approval_status !== "approved" && (
                        <Action
                          onClick={() => decide.mutate({ user: u, action: "approve" })}
                          disabled={decide.isPending}
                          tone="accent"
                        >
                          Duyệt
                        </Action>
                      )}
                      {u.id !== me?.id && mayApprove && u.approval_status !== "rejected" && (
                        <Action onClick={() => setRejecting(u)} disabled={decide.isPending}>
                          Từ chối
                        </Action>
                      )}
                      {u.id !== me?.id && mayApprove && u.approval_status === "approved" && (
                        <Action
                          onClick={() => decide.mutate({ user: u, action: "revoke-approval" })}
                          disabled={decide.isPending}
                        >
                          Gỡ duyệt
                        </Action>
                      )}
                      {u.id !== me?.id && mayAssign && (
                        <Action onClick={() => setEditingRoles(u)}>Vai trò</Action>
                      )}
                      {/* Editing yourself is allowed — renaming locks nobody out. */}
                      {mayWrite && <Action onClick={() => setEditing(u)}>Sửa</Action>}
                      {u.id !== me?.id && mayWrite && (
                        <Action onClick={() => toggleDisabled.mutate(u)} disabled={toggleDisabled.isPending}>
                          {u.disabled ? "Mở khoá" : "Khoá"}
                        </Action>
                      )}
                      {u.id !== me?.id && mayDelete && (
                        <Action onClick={() => setDeleting(u)} tone="danger">
                          Xoá
                        </Action>
                      )}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {pages > 1 && (
        <div className="mt-4 flex items-center justify-between text-sm" style={{ color: "var(--tpl-muted)" }}>
          <span>
            {total} tài khoản · trang {page + 1}/{pages}
          </span>
          <div className="flex gap-2">
            <Action onClick={() => setPage((p) => Math.max(0, p - 1))} disabled={page === 0}>
              Trước
            </Action>
            <Action onClick={() => setPage((p) => p + 1)} disabled={page + 1 >= pages}>
              Sau
            </Action>
          </div>
        </div>
      )}

      {rejecting && (
        <RejectModal
          user={rejecting}
          pending={decide.isPending}
          onCancel={() => setRejecting(null)}
          onConfirm={(note) => decide.mutate({ user: rejecting, action: "reject", note })}
        />
      )}

      {creating && (
        <CreateUserModal
          canApprove={mayApprove}
          pending={create.isPending}
          onCancel={() => setCreating(false)}
          onCreate={(body) => create.mutate(body)}
        />
      )}

      {editing && (
        <EditUserModal
          user={editing}
          pending={edit.isPending}
          onCancel={() => setEditing(null)}
          onSave={(body) => edit.mutate({ id: editing.id, body })}
        />
      )}

      {deleting && (
        <DeleteUserModal
          user={deleting}
          pending={remove.isPending}
          onCancel={() => setDeleting(null)}
          onDisableInstead={() => {
            toggleDisabled.mutate(deleting);
            setDeleting(null);
          }}
          onDelete={(confirmEmail) => remove.mutate({ id: deleting.id, confirmEmail })}
        />
      )}

      {editingRoles && (
        <RolesModal
          user={editingRoles}
          allRoles={(matrix.data?.roles ?? []).map((r) => r.code)}
          loading={matrix.isPending}
          pending={saveRoles.isPending}
          onCancel={() => setEditingRoles(null)}
          onSave={(roles) => saveRoles.mutate({ id: editingRoles.id, roles })}
        />
      )}
    </section>
  );
}

/* ── modals ───────────────────────────────────────────────────────── */

function RejectModal({
  user,
  pending,
  onCancel,
  onConfirm,
}: {
  user: AdminUser;
  pending: boolean;
  onCancel: () => void;
  onConfirm: (note: string) => void;
}) {
  const [note, setNote] = useState("");
  return (
    <Modal title={`Từ chối ${user.email}`} onClose={onCancel}>
      <p className="mb-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
        Lý do sẽ hiện cho người dùng khi họ thử đăng nhập. Tài khoản được giữ lại
        (không xoá) để email này không thể đăng ký lại nhằm lách quyết định.
      </p>
      <textarea
        value={note}
        onChange={(e) => setNote(e.target.value.slice(0, 500))}
        rows={3}
        placeholder="Lý do (không bắt buộc)"
        className="w-full rounded-lg border px-3 py-2 text-sm"
        style={{
          borderColor: "var(--tpl-border)",
          background: "var(--tpl-bg)",
          color: "var(--tpl-text)",
        }}
      />
      <ModalActions
        onCancel={onCancel}
        confirmLabel={pending ? "Đang lưu…" : "Từ chối"}
        confirmDisabled={pending}
        onConfirm={() => onConfirm(note.trim())}
      />
    </Modal>
  );
}

function RolesModal({
  user,
  allRoles,
  loading,
  pending,
  onCancel,
  onSave,
}: {
  user: AdminUser;
  allRoles: string[];
  loading: boolean;
  pending: boolean;
  onCancel: () => void;
  onSave: (roles: string[]) => void;
}) {
  const [selected, setSelected] = useState<string[]>(user.roles);
  return (
    <Modal title={`Vai trò · ${user.display_name}`} onClose={onCancel}>
      <p className="mb-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
        Vai trò con kế thừa toàn bộ quyền của vai trò cha. Bạn chỉ gán được vai
        trò mà chính bạn đã có đủ quyền.
      </p>
      {loading ? (
        <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
          Đang tải danh sách vai trò…
        </p>
      ) : (
        <div className="space-y-1.5">
          {allRoles.map((code) => (
            <label key={code} className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                checked={selected.includes(code)}
                onChange={(e) =>
                  setSelected((s) => (e.target.checked ? [...s, code] : s.filter((c) => c !== code)))
                }
              />
              <span style={{ color: "var(--tpl-text)" }}>{code}</span>
            </label>
          ))}
        </div>
      )}
      <ModalActions
        onCancel={onCancel}
        confirmLabel={pending ? "Đang lưu…" : "Lưu"}
        confirmDisabled={pending || loading}
        onConfirm={() => onSave(selected)}
      />
    </Modal>
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

/* ── pieces ───────────────────────────────────────────────────────── */

function Th({ children, align }: { children: ReactNode; align?: "right" }) {
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

function Chip({ children }: { children: ReactNode }) {
  return (
    <span
      className="rounded px-1.5 py-0.5 text-[10px] font-semibold"
      style={{ background: "var(--tpl-surface-2)", color: "var(--tpl-muted)" }}
    >
      {children}
    </span>
  );
}

function StatusBadge({ status, disabled }: { status: ApprovalStatus; disabled: boolean }) {
  const tone =
    status === "approved" ? "#16a34a" : status === "rejected" ? "#ef4444" : "var(--tpl-accent)";
  return (
    <div className="flex flex-wrap items-center gap-1">
      <span
        className="rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase text-white"
        style={{ background: tone }}
      >
        {APPROVAL_LABEL[status]}
      </span>
      {/* Disabled is a separate axis: an approved account can still be switched off. */}
      {disabled && (
        <span
          className="rounded px-1.5 py-0.5 text-[10px] font-semibold uppercase"
          style={{ background: "rgba(0,0,0,.08)", color: "var(--tpl-muted)" }}
        >
          Đã khoá
        </span>
      )}
    </div>
  );
}

function Badge({ n, active }: { n: number; active: boolean }) {
  if (!n) return null;
  return (
    <span
      className="ml-1.5 rounded-full px-1.5 py-0.5 text-[10px] font-bold"
      style={{
        background: active ? "rgba(255,255,255,.25)" : "var(--tpl-accent)",
        color: "#fff",
      }}
    >
      {n}
    </span>
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
      className="flex items-center rounded-lg px-3 py-1.5 text-sm font-semibold transition"
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

function Action({
  children,
  onClick,
  disabled,
  tone,
}: {
  children: ReactNode;
  onClick: () => void;
  disabled?: boolean;
  tone?: "accent" | "danger";
}) {
  const color =
    tone === "accent" ? "var(--tpl-accent)" : tone === "danger" ? "#ef4444" : "var(--tpl-muted)";
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="rounded-md border px-2.5 py-1 text-xs font-semibold transition hover:bg-[var(--tpl-surface-2)] disabled:opacity-40"
      style={{
        borderColor: tone === undefined ? "var(--tpl-border)" : color,
        color,
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
      className="divide-y rounded-xl border"
      style={{ borderColor: "var(--tpl-border)", background: "var(--tpl-surface)" }}
    >
      {Array.from({ length: 6 }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 px-3 py-3">
          <div className="h-8 w-8 shrink-0 animate-pulse rounded-full" style={{ background: "var(--tpl-surface-2)" }} />
          <div className="flex-1 space-y-1.5">
            <div className="h-3 w-1/4 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
            <div className="h-2.5 w-1/3 animate-pulse rounded" style={{ background: "var(--tpl-surface-2)" }} />
          </div>
        </div>
      ))}
    </div>
  );
}

function Empty({ tab }: { tab: Tab }) {
  return (
    <div
      className="rounded-xl border border-dashed py-16 text-center text-sm"
      style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-muted)" }}
    >
      {tab === "pending"
        ? "Không có tài khoản nào đang chờ duyệt."
        : "Không có tài khoản nào khớp bộ lọc."}
    </div>
  );
}

/**
 * A 403 here means the account lost `users:read:any` — the screen is reachable
 * only because the nav was rendered from a permission set that has since
 * changed. Retrying cannot fix it, so it is stated rather than offered.
 */
function ErrorState({ error, onRetry }: { error: unknown; onRetry: () => void }) {
  const forbidden = error instanceof ApiError && error.status === 403;
  return (
    <div className="rounded-xl border py-12 text-center" style={{ borderColor: "var(--tpl-border)" }}>
      <p className="text-sm" style={{ color: "var(--tpl-muted)" }}>
        {forbidden
          ? "Tài khoản của bạn không có quyền quản trị người dùng."
          : "Không tải được danh sách người dùng."}
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

/* ── small helpers ────────────────────────────────────────────────── */

function message(e: unknown, fallback: string): string {
  return e instanceof ApiError ? problemDisplayMessage(e.body) : fallback;
}

function initial(s: string): string {
  return (s.trim()[0] ?? "?").toUpperCase();
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "—" : d.toLocaleDateString();
}

function CreateUserModal({
  canApprove,
  pending,
  onCancel,
  onCreate,
}: {
  canApprove: boolean;
  pending: boolean;
  onCancel: () => void;
  onCreate: (body: { email: string; password: string; display_name: string }) => void;
}) {
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const tooShort = password.length > 0 && password.length < 8;

  return (
    <Modal title="Thêm người dùng" onClose={onCancel}>
      <p className="mb-3 text-xs" style={{ color: "var(--tpl-muted)" }}>
        {canApprove
          ? "Tài khoản được duyệt ngay vì bạn có quyền phê duyệt. Vai trò mặc định là “user” — đổi sau bằng nút “Vai trò”."
          : "Bạn không có quyền phê duyệt nên tài khoản này sẽ ở trạng thái chờ duyệt, giống như người tự đăng ký."}
      </p>
      <div className="space-y-3">
        <Field label="Email">
          <TextInput value={email} onChange={setEmail} type="email" autoFocus />
        </Field>
        <Field label="Tên hiển thị">
          <TextInput value={displayName} onChange={setDisplayName} placeholder="Bỏ trống = lấy phần trước @" />
        </Field>
        <Field label="Mật khẩu (ít nhất 8 ký tự)">
          <TextInput value={password} onChange={setPassword} type="password" />
        </Field>
        {tooShort && (
          <p className="text-xs" style={{ color: "#ef4444" }}>
            Mật khẩu phải từ 8 ký tự.
          </p>
        )}
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          Bắt buộc có mật khẩu — tài khoản không có mật khẩu thì vừa không đăng
          nhập được, vừa không tự đặt lại được.
        </p>
      </div>
      <ModalActions
        onCancel={onCancel}
        confirmLabel={pending ? "Đang tạo…" : "Tạo"}
        confirmDisabled={pending || !email.trim() || password.length < 8}
        onConfirm={() =>
          onCreate({ email: email.trim(), password, display_name: displayName.trim() })
        }
      />
    </Modal>
  );
}

function EditUserModal({
  user,
  pending,
  onCancel,
  onSave,
}: {
  user: AdminUser;
  pending: boolean;
  onCancel: () => void;
  onSave: (body: { email: string; display_name: string; password?: string }) => void;
}) {
  const [email, setEmail] = useState(user.email);
  const [displayName, setDisplayName] = useState(user.display_name);
  const [password, setPassword] = useState("");
  const tooShort = password.length > 0 && password.length < 8;

  return (
    <Modal title={`Sửa · ${user.display_name}`} onClose={onCancel}>
      <div className="space-y-3">
        <Field label="Email">
          <TextInput value={email} onChange={setEmail} type="email" />
        </Field>
        <Field label="Tên hiển thị">
          <TextInput value={displayName} onChange={setDisplayName} />
        </Field>
        <Field label="Mật khẩu mới (bỏ trống = giữ nguyên)">
          <TextInput value={password} onChange={setPassword} type="password" />
        </Field>
        {tooShort && (
          <p className="text-xs" style={{ color: "#ef4444" }}>
            Mật khẩu phải từ 8 ký tự.
          </p>
        )}
        {password.length >= 8 && (
          <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
            Đổi mật khẩu sẽ đăng xuất tài khoản này khỏi mọi thiết bị.
          </p>
        )}
        <p className="text-xs" style={{ color: "var(--tpl-muted)" }}>
          Email chính là tên đăng nhập — sửa email là đổi luôn cách họ đăng nhập.
        </p>
      </div>
      <ModalActions
        onCancel={onCancel}
        confirmLabel={pending ? "Đang lưu…" : "Lưu"}
        confirmDisabled={pending || !email.trim() || tooShort}
        onConfirm={() =>
          onSave({
            email: email.trim(),
            display_name: displayName.trim(),
            password: password || undefined,
          })
        }
      />
    </Modal>
  );
}

/**
 * Deleting is not "remove the row". Every foreign key to `users` is
 * ON DELETE CASCADE, so the account's content goes with it and nothing in the
 * database will stop it. The modal therefore says what is destroyed, offers the
 * reversible action first, and makes the confirmation an act of typing rather
 * than a second click — the server checks it too.
 */
function DeleteUserModal({
  user,
  pending,
  onCancel,
  onDisableInstead,
  onDelete,
}: {
  user: AdminUser;
  pending: boolean;
  onCancel: () => void;
  onDisableInstead: () => void;
  onDelete: (confirmEmail: string) => void;
}) {
  const [typed, setTyped] = useState("");
  const matches = typed.trim().toLowerCase() === user.email.toLowerCase();

  return (
    <Modal title={`Xoá vĩnh viễn · ${user.email}`} onClose={onCancel}>
      <div
        className="mb-3 rounded-lg border px-3 py-2.5 text-sm"
        style={{
          borderColor: "rgba(239,68,68,.4)",
          background: "rgba(239,68,68,.08)",
          color: "#ef4444",
        }}
      >
        <p className="font-semibold">Thao tác này xoá luôn toàn bộ nội dung của họ.</p>
        <p className="mt-1 text-xs">
          Ảnh/video đã tải lên, truyện, phim, nhạc, tiểu thuyết, sổ thu chi, nhật
          ký, danh bạ và tổ chức — tất cả bị xoá theo và <b>không khôi phục được</b>.
        </p>
      </div>

      {!user.disabled && (
        <button
          type="button"
          onClick={onDisableInstead}
          className="mb-3 w-full rounded-lg border px-3 py-2 text-sm font-semibold transition hover:bg-[var(--tpl-surface-2)]"
          style={{ borderColor: "var(--tpl-border)", color: "var(--tpl-heading)" }}
        >
          Khoá tài khoản thay vì xoá (đảo lại được)
        </button>
      )}

      <Field label={`Gõ "${user.email}" để xác nhận`}>
        <TextInput value={typed} onChange={setTyped} autoFocus />
      </Field>

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
          onClick={() => onDelete(typed.trim())}
          disabled={pending || !matches}
          className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition hover:opacity-90 disabled:opacity-40"
          style={{ background: "#ef4444" }}
        >
          {pending ? "Đang xoá…" : "Xoá vĩnh viễn"}
        </button>
      </div>
    </Modal>
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
  type = "text",
  placeholder,
  autoFocus,
}: {
  value: string;
  onChange: (v: string) => void;
  type?: string;
  placeholder?: string;
  autoFocus?: boolean;
}) {
  return (
    <input
      // eslint-disable-next-line jsx-a11y/no-autofocus -- modal opens on an explicit user action
      autoFocus={autoFocus}
      type={type}
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
