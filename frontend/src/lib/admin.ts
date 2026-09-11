// Data layer for the admin console: the user directory / approval queue and the
// role × permission matrix. Types mirror the wired handler JSON (snake_case) in
// `backend/internal/modules/account/handler/admin.go`.

import { api } from "./api-client";

export type ApprovalStatus = "pending" | "approved" | "rejected";

export interface AdminUser {
  id: string;
  email: string;
  display_name: string;
  avatar_url: string | null;
  approval_status: ApprovalStatus;
  approval_note: string | null;
  approved_at: string | null;
  approved_by: string | null;
  /** Switched off after the fact — orthogonal to `approval_status`. */
  disabled: boolean;
  has_password: boolean;
  created_at: string;
  roles: string[];
}

export interface AdminUserPage {
  users: AdminUser[];
  total: number;
  limit: number;
  offset: number;
  counts: Record<ApprovalStatus, number>;
}

export interface MatrixRole {
  id: string;
  code: string;
  name: string;
  description: string;
  parent_code: string | null;
  is_system: boolean;
  user_count: number;
  /** Granted to this role itself — the editable checkboxes. */
  direct: string[];
  /** `direct` plus every ancestor's grants. Read-only in the UI. */
  effective: string[];
}

export interface MatrixPermission {
  code: string;
  description: string;
  /** Leading resource segment; how the grid groups its rows. */
  group: string;
}

export interface PermissionMatrix {
  roles: MatrixRole[];
  permissions: MatrixPermission[];
}

/* ── users ────────────────────────────────────────────────────────── */

export interface ListUsersParams {
  status?: ApprovalStatus | "";
  q?: string;
  limit?: number;
  offset?: number;
}

export async function listUsers(p: ListUsersParams = {}): Promise<AdminUserPage> {
  const qs = new URLSearchParams();
  if (p.status) qs.set("status", p.status);
  if (p.q) qs.set("q", p.q);
  if (p.limit) qs.set("limit", String(p.limit));
  if (p.offset) qs.set("offset", String(p.offset));
  const suffix = qs.toString() ? `?${qs}` : "";
  const r = await api<AdminUserPage>(`/api/v1/admin/users${suffix}`);
  return { ...r, users: r.users ?? [] };
}

/**
 * Approve / reject / send back to pending.
 *
 * `note` is optional on every decision and is shown back to a rejected user on
 * their next login attempt, so it is a message to a person, not an internal memo.
 */
export function decideUser(
  id: string,
  decision: "approve" | "reject" | "revoke-approval",
  note?: string,
): Promise<AdminUser> {
  return api<AdminUser>(`/api/v1/admin/users/${id}/${decision}`, {
    method: "POST",
    body: JSON.stringify({ note: note ?? "" }),
  });
}

export function setUserDisabled(id: string, disabled: boolean): Promise<AdminUser> {
  return api<AdminUser>(`/api/v1/admin/users/${id}/${disabled ? "disable" : "enable"}`, {
    method: "POST",
  });
}

/** Whole-set replacement — send every role the user should end up with. */
export function setUserRoles(id: string, roles: string[]): Promise<AdminUser> {
  return api<AdminUser>(`/api/v1/admin/users/${id}/roles`, {
    method: "PUT",
    body: JSON.stringify({ roles }),
  });
}

/* ── create / edit / delete ───────────────────────────────────────── */

export interface CreateUserInput {
  email: string;
  password: string;
  display_name?: string;
}

/**
 * Provision an account directly.
 *
 * The approval state is decided by the SERVER from your own permissions — a
 * creator who cannot approve produces a pending account. There is deliberately
 * no way to ask for "approved" here, and roles are assigned afterwards with
 * `setUserRoles`.
 */
export function createUser(body: CreateUserInput): Promise<AdminUser> {
  return api<AdminUser>("/api/v1/admin/users", { method: "POST", body: JSON.stringify(body) });
}

export interface UpdateUserInput {
  email?: string;
  display_name?: string;
  /** Setting this signs the account out everywhere. */
  password?: string;
}

export function updateUser(id: string, body: UpdateUserInput): Promise<AdminUser> {
  return api<AdminUser>(`/api/v1/admin/users/${id}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

/**
 * Permanent, and it takes the account's content with it — every FK to `users`
 * cascades, so assets, comics, movies, tracks, stories, the ledger and journal
 * entries all go. `confirmEmail` must match the account's email; the server
 * checks it too, so this is not merely a UI speed bump.
 */
export async function deleteUser(id: string, confirmEmail: string): Promise<void> {
  await api<void>(`/api/v1/admin/users/${id}`, {
    method: "DELETE",
    body: JSON.stringify({ confirm_email: confirmEmail }),
  });
}

/* ── roles + matrix ───────────────────────────────────────────────── */

export function getMatrix(): Promise<PermissionMatrix> {
  return api<PermissionMatrix>("/api/v1/admin/permission-matrix");
}

/**
 * Replace one role's DIRECT grants. Returns the whole recomputed matrix, because
 * changing one role changes every descendant's effective set.
 */
export function setRolePermissions(
  roleId: string,
  permissions: string[],
): Promise<PermissionMatrix> {
  return api<PermissionMatrix>(`/api/v1/admin/roles/${roleId}/permissions`, {
    method: "PUT",
    body: JSON.stringify({ permissions }),
  });
}

export interface RoleInput {
  code?: string;
  name: string;
  description?: string;
  parent_code?: string;
}

export function createRole(body: RoleInput) {
  return api<MatrixRole>("/api/v1/admin/roles", { method: "POST", body: JSON.stringify(body) });
}

export function updateRole(id: string, body: RoleInput) {
  return api<MatrixRole>(`/api/v1/admin/roles/${id}`, {
    method: "PATCH",
    body: JSON.stringify(body),
  });
}

export async function deleteRole(id: string): Promise<void> {
  await api<void>(`/api/v1/admin/roles/${id}`, { method: "DELETE" });
}

/* ── display helpers ──────────────────────────────────────────────── */

export const APPROVAL_LABEL: Record<ApprovalStatus, string> = {
  pending: "Chờ duyệt",
  approved: "Đã duyệt",
  rejected: "Từ chối",
};

/**
 * Order the hierarchy parent-before-child so the matrix columns read as the
 * inheritance chain (guest → user → creator → …) instead of alphabetically,
 * which is what makes an inherited checkmark legible at a glance.
 *
 * Falls back to the input order for anything it cannot place — a cycle, or a
 * parent that is not in the list — rather than dropping rows.
 */
export function orderRolesByHierarchy(roles: MatrixRole[]): MatrixRole[] {
  const byParent = new Map<string, MatrixRole[]>();
  for (const r of roles) {
    const key = r.parent_code ?? "";
    byParent.set(key, [...(byParent.get(key) ?? []), r]);
  }
  const out: MatrixRole[] = [];
  const seen = new Set<string>();
  const walk = (parent: string) => {
    for (const r of byParent.get(parent) ?? []) {
      if (seen.has(r.code)) continue;
      seen.add(r.code);
      out.push(r);
      walk(r.code);
    }
  };
  walk("");
  for (const r of roles) if (!seen.has(r.code)) out.push(r);
  return out;
}
