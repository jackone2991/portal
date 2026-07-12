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
  | "journal/invalid-asset";

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
