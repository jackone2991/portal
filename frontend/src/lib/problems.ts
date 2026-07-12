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
  | "stream/invalid-cursor";

export const PROBLEM_MESSAGES: Record<ProblemType, string> = {
  "media/unsupported-format":
    "This file format isn't supported. Try converting it and uploading again.",
  "media/file-too-large": "This file is too large to upload.",
  "media/asset-not-found": "This asset doesn't exist or was already removed.",
  "media/asset-not-ready":
    "This asset is still being uploaded — try again in a moment.",
  "journal/entry-not-found": "This entry doesn't exist or was already removed.",
  "journal/invalid-body": "Entries need 1-20,000 characters of text.",
  "journal/invalid-mood": "Mood can't be blank — leave it empty or add a word.",
  "journal/invalid-asset": "Photo attachments aren't supported yet.",
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
