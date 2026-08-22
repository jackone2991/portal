// Thin fetch wrapper around the Portal API.
// Once `make openapi` runs, types from src/lib/types.gen.ts can be used to
// strongly-type request/response bodies for each endpoint.

// Exported so callers that need a direct URL (e.g. an <img>/<a href> straight
// to a media variant, rather than a JSON round-trip through `api()`) can build
// on the same base without redeclaring the env fallback.
//
// Resolved from the *current* origin in the browser so one build serves every
// access path: opening the app on https://portal.localhost (PC) keeps using the
// baked api.<domain> TLS host, while reaching it over the LAN by raw IP/host
// (e.g. http://192.168.1.53:3000 from a phone) talks to the API on :8080 of that
// same host+scheme — no per-URL rebuild, no hardcoded hostname the phone can't
// resolve. SSR (no window) falls back to the build-time env.
function resolveBaseURL(): string {
  if (typeof window !== "undefined") {
    const { protocol, hostname } = window.location;
    if (hostname !== "portal.localhost") {
      return `${protocol}//${hostname}:8080`;
    }
  }
  return process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
}

export const baseURL = resolveBaseURL();

export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: unknown,
    message?: string,
  ) {
    super(message ?? `API error ${status}`);
  }
}

export async function api<T>(
  path: string,
  init: RequestInit = {},
): Promise<T> {
  const res = await fetch(`${baseURL}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...init.headers,
    },
    credentials: "include",
  });

  const text = await res.text();
  const body = text ? safeJSON(text) : undefined;

  if (!res.ok) {
    throw new ApiError(res.status, body);
  }
  return body as T;
}

function safeJSON(text: string): unknown {
  try {
    return JSON.parse(text);
  } catch {
    return text;
  }
}
