# ADR-10 — OpenAPI Contract Direction (spec-first, enforced)

**Status:** **accepted** 2026-07-11 (drafted 2026-07-08, from the 2026-07-08 gap audit)
**Last verified:** 2026-09-11
**Relates to:** [backlog §9](../product/backlog.md) · specs/README "API contract" convention · [ADR-09](09-docs-architecture.md) canonical-source rule · `backend/MODULES.md` (which still does not state the rule — see action items)

## Context

*The state this decision was made against, as the 2026-07-08 audit found it.
Where it stands now is under Consequences and Action items.*

`shared/openapi.yaml` (OpenAPI 3.1, then ~730 lines, 12 paths) declared itself
the source of truth — its `info` block said *"Server stubs (Go) and the
TypeScript client are both generated from it,"* and CLAUDE.md repeated the rule.
On disk that claim was aspirational, not real:

- **No generated code existed.** `backend/internal/handler/api.gen.go` and
  `frontend/src/lib/types.gen.ts` were `.gitignore`d and had never been
  generated or committed — `internal/handler/` did not exist. `make openapi`
  was fully wired (oapi-codegen `chi-server` + `openapi-typescript`, config in
  `backend/oapi-codegen.yaml`) but had never been run in anger.
- **All 12 handlers were hand-written plain-chi** (6 account, 6 media) with zero
  `ServerInterface` — `grep -rn ServerInterface backend/` returned nothing. The
  hand-written `frontend/src/lib/api-client.ts` carried a TODO to switch to the
  generated types.
- **The contract already lied.** Handlers emitted the legacy `{code, message}`
  error body via a local `writeErr`, but the spec mandated RFC 7807 `Problem`.
  The auth-path drift (`/auth/register`, retired `/auth/callback`) had been fixed
  by hand; the error-shape drift persisted.
- **CI did not gate drift — of anything.** The `openapi` job only asserted the
  YAML parsed and had `openapi`/`info`/`paths` keys (a ~10-line Python check): no
  lint, no codegen, no spec-vs-handler comparison. (The audit also credited the
  `backend` job with an sqlc drift gate to mirror. It had none: sqlc output is
  `.gitignore`d and regenerated on every build, so there was nothing to diff.
  The *pattern* — regenerate, then `git diff --exit-code` — was still the right
  one; it just had no precedent in this repo.)

The forcing function: **SPEC-01/02/03 (and SPEC-04) were about to add ~30
endpoints** across media/comic/bank/notify onto a contract nothing
machine-checked. Each spec's Definition of Done said *"fix the drift in the same
or an earlier PR"* — but there was no mechanism to enforce that, so it would
rot. Two facts made timing decisive: the generated side was **greenfield**
(nothing committed to reconcile or delete), and the handler count was at its
**all-time low** (12, two wired modules). This was the cheapest the decision
would ever be; every week of deferral raised the price.

The underlying decision (backlog §9) had never actually been made: *adopt
oapi-codegen/openapi-typescript, or drop the spec as source of truth.* It is
expensive to reverse and touches every module — hence an ADR.

## Decision

**Keep `shared/openapi.yaml` as the single source of truth and make that real and
enforced (spec→code).** Concretely:

1. **Go: adopt oapi-codegen `chi-server` + `models`.** Handlers implement the
   generated `ServerInterface`; **bodies stay hand-written** — cookie/throttle/
   audit logic in the account handlers is unchanged, only signatures and
   request/response types come from generation. Routes register via the generated
   mux instead of ad-hoc `r.Post(...)`.
2. **TS: adopt `openapi-typescript`.** `api-client.ts` consumes `types.gen.ts`;
   the frontend gets end-to-end typed API access.
3. **Commit the generated artifacts** (remove `api.gen.go` + `types.gen.ts` from
   `.gitignore`). CI drift becomes the sqlc pattern: `make openapi` then
   `git diff --exit-code`. Committing means PRs show the contract surface changing —
   that diff *is* the contract review.
4. **Pin the toolchain.** Add oapi-codegen via a `tools.go` / `go.mod` tool
   directive (it is currently unpinned — no `tools.go`, absent from `go.mod`, so
   the drift gate would be non-reproducible without this).
5. **Replace the CI parse-check** with (a) a real lint (`redocly lint` or
   `vacuum`) and (b) the regenerate-and-diff drift gate for **both** Go and TS.
6. **Resolve the RFC 7807 drift as part of the cutover** — a module-wide
   `writeErr` → `Problem` helper, so the error contract stops lying.

Scope: do the cutover **now** on the two wired modules (account, media), before
SPEC-01. Thereafter each spec adds its paths spec-first, and an endpoint missing
from the spec fails CI — the specs' DoD becomes mechanical, not aspirational.

## Options considered

- **A. Spec→code, enforced** *(chosen)*: makes the declared intent real, reuses
  the working sqlc drift-gate pattern, gives the frontend typed access for free,
  and closes the door on ~30 endpoints of future drift at the point of minimum cost.
- **B. Code→spec (swaggo annotations; generate the spec from handlers).** Rejected:
  inverts the source of truth the project has declared three times; imposes a
  per-handler annotation burden forever; swaggo's OpenAPI 3.1 support lags; the
  annotations drift from behavior just as easily as a hand-kept spec.
- **C. Drop the spec as source of truth (code-first, hand-written TS client).**
  Rejected: abandons a principle stated in the spec, CLAUDE.md, and MODULES.md;
  discards a ~730-line asset; lets frontend types drift silently into runtime bugs.
  Saves work now; the frontend repays it with interest. Cheapest today, most
  expensive across the four verticals.
- **D. Spec-first-lite: generate only the TS client, keep Go hand-written, add a
  custom route-set diff in CI.** Rejected: the path-presence check is bespoke
  tooling that catches *missing endpoints* but not *schema drift* (request/response
  body shape) — and body-shape is exactly where ledger/finance correctness bugs
  hide. oapi-codegen gives full-shape conformance for less long-run maintenance.

## Trade-offs

- **Rewriting 12 handlers to the generated interface + fixing the error shape is
  real work now (~1 dev-day).** Accepted: one-time, at the all-time-low handler
  count, and it forecloses drift across the four incoming specs.
- **oapi-codegen `chi-server`'s interface is somewhat rigid.** The complex auth
  handlers keep hand-written bodies but must match generated signatures. Mitigated:
  chi-server generates routing + types, not logic — the throttle/cookie/audit code
  is untouched.
- **Committing generated code adds diff noise on contract changes.** Accepted —
  that noise is the point; it surfaces contract changes in review.
- **One more pinned tool in the build.** Accepted: mirrors sqlc; reproducibility
  is the whole reason for the gate.
- **The spec still covers only ~2 of ~7 modules.** Accepted: the specs backfill
  their own paths as they land; enforcement from now prevents the gap widening.

## Consequences

What followed, checked against the tree on 2026-09-11:

- **The drift gate is real, and it is the repo's only one.** `api.gen.go`
  (10,367 lines) and `types.gen.ts` (8,305 lines) have been committed since
  `6160f8e` (2026-07-12); `.gitignore` excludes sqlc output only. CI's `openapi`
  job runs `make openapi` then `git diff --exit-code` on both files. Stale
  codegen fails the build.
- **Spec-first held for every module that came after.** The spec is now
  `wc -l shared/openapi.yaml` lines (5,137 at last check) and
  `python3 -c "import yaml;print(len(yaml.safe_load(open('shared/openapi.yaml'))['paths']))"`
  paths (111), tagged for every wired module — account/admin, media, movies,
  music, stories, comic, bank, journal, notifications, layout, ops, people,
  social, tenant, platform. The "~2 of ~7 modules" gap closed.
- **The error contract stopped lying — but by a different route.**
  `internal/platform/server.Problem` became the single RFC 7807 writer on
  2026-08-25; the four surviving `writeErr`/`writeError` shims (account, media,
  notify, tenant) delegate to it, and `schemas/Error` is deprecated with no
  referent. This landed without the `ServerInterface` cutover it was scoped
  under.
- **No handler implements the generated `ServerInterface`.** `api.gen.go` is
  its only referent in the tree (`grep -rln ServerInterface backend`). Every
  handler is still hand-written plain-chi. The gate therefore proves the
  generated code matches the spec, **not** that handler behaviour does — a
  response shape must still be verified against the handler.
- **The frontend is not typed end-to-end.** `types.gen.ts` is committed, but
  `grep -rl 'from "./types.gen"' frontend/src` finds one importer
  (`lib/comic-sync.ts`); `api-client.ts` still opens with "once `make openapi`
  runs …" and the other `lib/*.ts` modules hand-declare their types. The typed
  client the decision promised is available and unused.
- **backlog §9 closed** (the either/or is resolved). The specs' DoD is
  enforceable for *path presence and schema shape*; handler conformance stays
  a review question.
- The spec's own `info`-block claim ("stubs and client are generated from it")
  is true. CLAUDE.md's "don't hand-edit generated files" protects real files.
- **MODULES.md §8 did not gain its step.** `grep -ci openapi backend/MODULES.md`
  is 0; a new module that follows the checklist today still fails the `openapi`
  job. ADR-09's canonical-source rule holds: the contract stays at
  `shared/openapi.yaml`; `docs/reference/` points at it.

## Action items

- [x] Accept this ADR and flip status to accepted (2026-07-11).
- [x] Un-`.gitignore` `api.gen.go` + `types.gen.ts`; commit them (`6160f8e`).
- [ ] Pin oapi-codegen for **local** runs. CI pins `oapi-codegen@v2.7.2`
      (`.github/workflows/ci.yml`) and `openapi-typescript ^7.4.0`
      (`frontend/package.json`), but `backend/go.mod` has no `tool` directive
      and there is no `tools.go`, so `make openapi` on a developer machine uses
      whatever version is on `PATH` — and a version skew produces a diff the
      gate rejects.
- [x] `make openapi` runs; `Problem` error helper exists
      (`internal/platform/server`, 2026-08-25).
- [ ] Refactor handlers onto the generated `ServerInterface` — **none done**,
      account and media included. Retrofit per module as each is touched.
- [x] CI regenerate-and-diff for Go + TS (`openapi` job, `6160f8e`).
- [ ] Replace the parse-check with a real lint (`redocly lint` / `vacuum`) —
      the job still only checks that the YAML parses.
- [x] Every spec since SPEC-01 added its paths spec-first (the gate makes the
      alternative fail CI). The "cutover PR before SPEC-01" as scoped — handlers
      onto `ServerInterface` — never landed; only the gate did.
- [ ] `backend/MODULES.md` §8: add "add paths to `shared/openapi.yaml`;
      `make openapi`; commit the generated files" (ADR-11 backlog).
- [x] CLAUDE.md generated-files note names the real files.
- [ ] Make the frontend consume `types.gen.ts` beyond `comic-sync.ts`, or
      strike the "typed client" claim from the spec's `info` block.
