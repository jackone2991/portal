# API versioning & deprecation policy

Status: **active** · Decision: [D-31](../../doc/en/feature.md) · Contract: [shared/openapi.yaml](../../shared/openapi.yaml)

Portal uses **URL versioning**. Every HTTP route lives under `/api/v{N}/` — today, `/api/v1/`. The version is visible in every request, log line, and Traefik route, which keeps debugging honest. Header- and date-based versioning were rejected as invisible / heavy for a self-hosted product (see D-31).

## Within a major version: additive only

Anything that a correctly-written client can absorb without changing is free to ship into `v1`. Anything that can break such a client is a **breaking change** and MUST NOT land in `v1`.

| Free (additive) | Breaking (needs a new major) |
| --- | --- |
| New endpoints | Removing or renaming an endpoint |
| New **optional** request fields | Adding a **required** request field |
| New response fields | Removing / renaming a response field |
| New enum values* | Removing an enum value |
| Loosening validation | Tightening validation, changing a field's type or semantics |

\* Clients **MUST** tolerate unknown enum values (ignore or degrade gracefully) — this is what makes new enum values additive. Treat a hard `switch` with no default as a client bug.

## Introducing a new major (`v2`)

Only when a breaking change is genuinely forced. The process:

1. **RFC issue** describing the breaking change, who it affects, and the alternatives considered.
2. **Deprecate** the affected `v1` endpoints with response headers (see below) **at least 6 months** before removal.
3. **Coexist** — `v1` and `v2` are served side-by-side for the entire sunset window.
4. **Migration doc** at `docs/api/migrating-v1-to-v2.md` before `v2` is announced.

Self-hosters pin their frontend to a known API version, so a hosted instance moving to `v2` never breaks a self-hosted `v1` frontend.

## Deprecation headers (RFC 9745 + RFC 8594)

A deprecated endpoint advertises its status on every response:

```http
Deprecation: true
Sunset: Sat, 01 Nov 2026 00:00:00 GMT
Link: <https://portal.example.com/docs/api/migrating-v1-to-v2>; rel="deprecation"
```

- `Deprecation` ([RFC 9745](https://www.rfc-editor.org/rfc/rfc9745)) — the endpoint is deprecated (value `true`, or an HTTP-date when deprecation began).
- `Sunset` ([RFC 8594](https://www.rfc-editor.org/rfc/rfc8594)) — the date on/after which the endpoint may stop responding. MUST be ≥ 6 months out at announcement.
- `Link … rel="deprecation"` — points at the migration guide.

Clients should surface a one-time warning when they observe `Deprecation: true` from the API.

## CI enforcement

The OpenAPI drift gate ([D-9]) is the automated backstop for this policy: it diffs [shared/openapi.yaml](../../shared/openapi.yaml) against `main` and flags **shape-breaking** changes within a major (removed paths, removed/renamed fields, type changes, removed enum values). A PR that intentionally makes such a change to a **new** major is fine; making it to an existing major must be waived explicitly in the PR description with a reason.

> Status note (2026-07-07): the drift job currently validates that the spec is well-formed. The shape-diff gate lands when OpenAPI codegen is wired end-to-end (tracked in `MILESTONE_CHECKS.md`).
