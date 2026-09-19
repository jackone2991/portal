// i18n catalog for RFC 7807 Problem `type` URIs ([F036] / Sprint 1 §D). The
// API returns `application/problem+json` bodies whose `type` is a stable URI
// (see `shared/openapi.yaml`'s `Problem` schema doc comment: "The stable
// `type` URI doubles as the i18n key"). This module is the other half of that
// contract on the frontend: look up a human message by `type`, with a
// fallback for codes this catalog doesn't know about yet.
//
// Seeded with the SPEC-01 §7 types. New specs append their Problem types here
// as they're built — this is meant to grow, not be SPEC-01-specific.

/** Known Problem `type` URIs. Widen with `| (string & {})` at call sites if needed. */
export type ProblemType =
  | "media/unsupported-format"
  | "media/file-too-large"
  | "media/asset-not-found"
  | "media/asset-not-ready"
  | "journal/entry-not-found"
  | "journal/invalid-body"
  | "journal/invalid-mood"
  | "journal/invalid-asset"
  | "journal/invalid-location"
  | "bank/not-found"
  | "bank/account-not-empty"
  | "bank/account-not-mutable"
  | "bank/is-transfer-leg"
  | "bank/category-in-use"
  | "bank/category-kind-mismatch"
  | "bank/category-immutable"
  | "bank/invalid-category-parent"
  | "bank/same-account-transfer"
  | "bank/currency-mismatch"
  | "bank/direction-kind-mismatch"
  | "bank/invalid-amount"
  | "bank/validation"
  | "bank/invalid-cursor"
  | "comic/not-found"
  | "comic/invalid-cover-asset"
  | "comic/invalid-page-asset"
  | "comic/invalid-progress-target"
  | "comic/not-publishable"
  | "comic/validation"
  | "comic/invalid-cursor"
  | "people/person-not-found"
  | "people/invalid-birthday"
  | "people/validation"
  | "people/invalid-cursor"
  | "stream/invalid-cursor"
  | "platform/rate-limited"
  | "account/account-pending"
  | "account/account-rejected"
  | "account/account-disabled"
  | "account/account-not-approved"
  | "account/self-target"
  | "account/escalation"
  | "account/unknown-role"
  | "account/unknown-permission"
  | "account/role-protected"
  | "account/role-in-use"
  | "account/role-cycle"
  | "account/role-exists"
  | "account/invalid-email"
  | "account/password-policy"
  | "account/email-taken"
  | "account/confirmation-mismatch"
  | "account/last-approver"
  | "layout/validation"
  | "layout/unknown-widget"
  | "music/lookup-disabled"
  | "music/playlist-not-found"
  | "music/playlist-exists"
  | "music/invalid-playlist";

export const PROBLEM_MESSAGES: Record<ProblemType, string> = {
  // Playlists (0041). The server sends a `detail` on all three, so — like the
  // account entries below — these are the floor, not what you normally see.
  "music/playlist-not-found": "This playlist doesn't exist or was already removed.",
  "music/playlist-exists": "You already have a playlist with that name.",
  "music/invalid-playlist": "A playlist name needs 1-120 characters.",

  "media/unsupported-format":
    "This file format isn't supported. Try converting it and uploading again.",
  "media/file-too-large": "This file is too large to upload.",
  "media/asset-not-found": "This asset doesn't exist or was already removed.",
  "media/asset-not-ready":
    "This asset is still being uploaded — try again in a moment.",
  "journal/entry-not-found": "This entry doesn't exist or was already removed.",
  "journal/invalid-body":
    "An entry needs some text or at least one photo, and at most 20,000 characters.",
  "journal/invalid-mood": "Mood can't be blank — leave it empty or add a word.",
  "journal/invalid-asset":
    "One of the photos can't be attached — it must be a finished image of yours, listed once, ten at most.",
  "journal/invalid-location":
    "The place needs a name and a point on the map — pick it again.",
  "bank/not-found": "This item doesn't exist or was already removed.",
  "bank/account-not-empty": "This account has transactions — archive it instead of deleting.",
  "bank/account-not-mutable": "You can't change the currency once an account has transactions.",
  "bank/is-transfer-leg": "This is part of a transfer — edit or delete it from the transfer instead.",
  "bank/category-in-use": "This category has transactions. Reassign them to another category first.",
  "bank/category-kind-mismatch": "That category is a different kind (income vs expense).",
  "bank/category-immutable": "A category's kind can't be changed after it's created.",
  "bank/invalid-category-parent": "A parent must be a top-level category of the same kind.",
  "bank/same-account-transfer": "Pick two different accounts for a transfer.",
  "bank/currency-mismatch": "Transfers between different currencies aren't supported yet.",
  "bank/direction-kind-mismatch": "Expenses need an expense category, income needs an income category.",
  "bank/invalid-amount": "Enter an amount greater than zero.",
  "bank/validation": "Please check the form and try again.",
  "bank/invalid-cursor": "Couldn't load the next page — please refresh.",
  "comic/not-found": "This comic doesn't exist or was already removed.",
  "comic/invalid-cover-asset": "The cover must be a ready image you uploaded.",
  "comic/invalid-page-asset": "Each page must be a ready image you uploaded.",
  "comic/invalid-progress-target": "Couldn't save your place on this comic.",
  "comic/not-publishable": "Every chapter needs at least one page before publishing.",
  "comic/validation": "Please check the form and try again.",
  "comic/invalid-cursor": "Couldn't load the next page — please refresh.",
  "people/person-not-found": "This person doesn't exist or was already removed.",
  "people/invalid-birthday": "Enter a real date — day and month together, year optional.",
  "people/validation": "Please check the form and try again.",
  "people/invalid-cursor": "Couldn't load the next page — please refresh.",
  "stream/invalid-cursor": "Couldn't load more of your stream — please refresh.",
  // Emitted by the IP rate limiter on the auth perimeter (login/register/refresh),
  // which used to answer with the legacy {code, message} body — so this 429 fell
  // through to the generic fallback instead of telling the user to wait.
  "platform/rate-limited": "Too many attempts. Wait a moment and try again.",

  // Registration approval (migration 0031). The server sends a `detail` on all
  // of these — a rejection even carries the reviewer's note — so these entries
  // are the floor, used only if `detail` is ever missing.
  "account/account-pending":
    "This account is waiting for an administrator to approve it.",
  "account/account-rejected": "This registration was not approved.",
  "account/account-disabled": "This account is disabled.",
  "account/account-not-approved":
    "This account is no longer approved. Sign in again to see why.",

  // Admin console guardrails.
  "account/self-target": "You can't apply this to your own account.",
  "account/escalation":
    "You can't grant or revoke something you don't hold yourself.",
  "account/unknown-role": "No such role.",
  "account/unknown-permission": "No such permission.",
  "account/role-protected": "System roles can't be edited or deleted.",
  "account/role-in-use":
    "This role is still in use — move its users or child roles off it first.",
  "account/role-cycle": "That parent would create a loop in the role hierarchy.",
  "account/role-exists": "A role with that code already exists.",

  // User create / edit / delete.
  "account/invalid-email": "Enter a valid email address.",
  "account/password-policy": "The password must be at least 8 characters.",
  "account/email-taken": "An account with this email already exists.",
  "account/confirmation-mismatch":
    "Type the account's email address to confirm this deletion.",
  "account/last-approver":
    "This is the last account that can approve registrations — give another account that permission first.",

  // Shell layout. The server names the offending row in `detail`, which
  // problemDisplayMessage prefers — these are only the floor.
  "layout/validation": "Check the menu entries and try again.",
  "layout/unknown-widget":
    "That widget doesn't exist in this build — the widget list comes from the code, not the database.",

  // The server's `detail` names the two env vars, so this is only the floor.
  "music/lookup-disabled":
    "Catalogue lookup is turned off on this deployment.",
};

const FALLBACK_MESSAGE = "Something went wrong. Please try again.";

/** Human message for a Problem `type` URI, or a generic fallback if unknown. */
export function problemMessage(type: string | undefined | null): string {
  if (type && type in PROBLEM_MESSAGES) {
    return PROBLEM_MESSAGES[type as ProblemType];
  }
  return FALLBACK_MESSAGE;
}

/** Narrow, minimal shape of an RFC 7807 body — just enough to read `type`/`detail`. */
export interface ProblemLike {
  type?: string;
  detail?: string;
}

/** Prefer the server's `detail` when present, else the catalog message for `type`. */
export function problemDisplayMessage(body: unknown): string {
  const problem = body as ProblemLike | undefined;
  return problem?.detail || problemMessage(problem?.type);
}
