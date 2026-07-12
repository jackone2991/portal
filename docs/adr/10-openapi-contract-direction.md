# ADR-10 — OpenAPI Contract Direction (spec-first, enforced)

**Status:** **accepted** 2026-07-11 (drafted 2026-07-08, from the 2026-07-08 gap audit)
**Relates to:** [backlog §9](../product/backlog.md) · specs/README "API contract" convention · [ADR-09](09-docs-architecture.md) canonical-source rule · `backend/MODULES.md` ("OpenAPI is contract")

> **Update 2026-07-11 (Sprint 1 foundation gate).** Accepted and execution started: the generated code is committed (`backend/internal/handler/api.gen.go`, `frontend/src/lib/types.gen.ts`) and CI now gates codegen drift (the `openapi` job runs `make openapi` then `git diff --exit-code`, mirroring the sqlc gate). Retrofitting the hand-written handlers onto the generated `ServerInterface` proceeds per module as each is touched (see [delivery-plan.md](../product/delivery-plan.md) Sprint 1 §A); the drift gate makes the direction non-optional from here.

## Context

`shared/openapi.yaml` (OpenAPI 3.1, ~730 lines, 12 paths) declares itself the
source of truth — its `info` block says *"Server stubs (Go) and the TypeScript
client are both generated from it,"* and CLAUDE.md + MODULES.md repeat the rule.
On disk that claim is aspirational, not real:

- **No generated code exists.** `backend/internal/handler/api.gen.go` and
  `frontend/src/lib/types.gen.ts` are `.gitignore`d and have **never been
  generated or committed** — `internal/handler/` does not exist. `make openapi`
  is fully wired (oapi-codegen `chi-server` + `openapi-typescript`, config in
  `backend/oapi-codegen.yaml`) but has apparently never been run in anger.
- **All 12 handlers are hand-written plain-chi** (6 account, 6 media) with zero
  `ServerInterface` — `grep -rn ServerInterface backend/` returns nothing. The
  hand-written `frontend/src/lib/api-client.ts` still carries a TODO to switch to
  the generated types.
- **The contract already lies.** Handlers emit the legacy `{code, message}` error
  body via a local `writeErr`, but the spec mandates RFC 7807 `Problem`. The
  auth-path drift (`/auth/register`, retired `/auth/callback`) was fixed by hand;
  the error-shape drift persists.
- **CI does not gate drift.** The `openapi` job only asserts the YAML parses and
  has `openapi`/`info`/`paths` keys (a ~10-line Python check). It runs no lint, no
  codegen, no spec-vs-handler comparison. By contrast the `backend` job **does**
  gate sqlc drift (`sqlc generate` then `git diff`). OpenAPI is the asymmetric hole.

The forcing function: **SPEC-01/02/03 (and SPEC-04) are about to add ~30 endpoints**
across media/comic/bank/notify onto a contract nothing machine-checks. Each spec's
Definition of Done says *"fix the drift in the same or an earlier PR"* — but there
is no mechanism to enforce that, so it will rot. Two facts make timing decisive:
the generated side is **greenfield** (nothing committed to reconcile or delete),
and the handler count is at its **all-time low** (12, two wired modules). This is
the cheapest this decision will ever be; every week of deferral raises the price.

The underlying decision (backlog §9) was never actually made: *adopt
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

- **backlog §9 closes** (the open P2 either/or is resolved).
- **specs/README's "fix drift in the same or earlier PR" gains teeth** — it becomes
  a CI gate, not a good intention; the specs' DoD is enforceable.
- The spec's own `info`-block claim ("stubs and client are generated from it")
  stops being false.
- CLAUDE.md's "don't hand-edit generated files" now has real files to protect
  (`api.gen.go`, `types.gen.ts` are already named there).
- `frontend/src/lib/api-client.ts`'s standing TODO resolves; the client is typed.
- **The new-module checklist (MODULES.md §8) gains a step:** "add paths to
  `shared/openapi.yaml`; `make openapi`; commit the generated files."
- ADR-09's canonical-source rule holds: the contract stays at `shared/openapi.yaml`
  next to what it governs; `docs/reference/` keeps pointing at it, not copying it.

## Action items

- [ ] Accept this ADR (owner) and flip status to accepted.
- [ ] Pin oapi-codegen (`tools.go` / `go.mod` tool directive); un-`.gitignore`
      `api.gen.go` + `types.gen.ts`.
- [ ] `make openapi`; refactor account + media handlers onto the generated
      `ServerInterface`; add the module-wide `Problem` error helper.
- [ ] Replace the CI `openapi` parse-check with lint + regenerate-and-diff (Go + TS),
      mirroring the `backend` job's sqlc gate.
- [ ] Land the cutover PR **before SPEC-01**; thereafter each spec adds its paths
      spec-first.
- [ ] Update MODULES.md §8 checklist + the CLAUDE.md generated-files note.
