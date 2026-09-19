# Backlog

**Status:** current · **Last verified:** 2026-09-19

The live, triaged list of open work. One line per item; the line says what is
wrong, where the evidence is, and what closes it. Ordering inside a tier is
priority. Anything not listed here is either done, deliberately deferred
(§ Deferred), or has not been found yet.

**How this file is maintained** (ADR-11 rule 6 / [docs/README.md](../README.md)
§ analysis): every audit in [analysis/](analysis/) must either produce lines here
or be closed with a reason. This revision triaged the whole of
[analysis/remaining-work-2026-08-25.md](analysis/remaining-work-2026-08-25.md)
(every numbered item in its §3–§6) plus the open action items from the ADR
re-grade of 2026-09-11. Items the audit raised that were already closed by the
time of triage are listed once under § Closed so the audit can be checked off
line by line. The previous `backlog.md` (a 2026-07 Facebook-parity gap analysis,
archived in place on 2026-08-25) was replaced wholesale on 2026-09-11;
`git log --follow -- docs/product/backlog.md` finds it.

Verify a line before working it — `Last verified` is the date the *whole file*
was checked, and code moves.

---

## P0 — security and correctness

1. **Re-word the `0019`/`0020` migration headers** ("**INERT** until …") in
   the next migration that touches those tables — applied files are not
   edited. Low stakes now that `.env.example` and ADR-07 say the true thing.

## P1 — contract and coverage

6. **No handler implements the generated `ServerInterface`** (ADR-10 action
   item). Every handler is hand-written plain-chi; the `openapi` CI job proves
   codegen matches the spec, not that handlers do. *Closes when:* one module is
   retrofitted and the pattern is documented in `backend/MODULES.md`.
7. **The frontend does not consume `types.gen.ts`** (ADR-10). One importer
   (`lib/comic-sync.ts`); every other `lib/*.ts` hand-declares its types and
   `api-client.ts` still says "once `make openapi` runs". *Closes when:* the
   `lib/*.ts` clients import `components["schemas"][…]`, or the spec's `info`
   block stops promising a generated client.
8. **HTTP contracts are asserted for comic and bank only.** `comic/http_test.go`
   and `bank/http_test.go` (2026-09-11) drive the real router over the fakes
   and pin 404-not-403 on cross-owner access, the RFC 7807 body, and
   delete-twice → 404 ([TRACEABILITY-MATRIX](../reference/TRACEABILITY-MATRIX.md)
   CC-1, CC-3, CC-8). The pattern is ~100 lines per module. *Closes when:*
   movie, music, story, journal, people, notify, layout and social carry the
   same two tests. (Audit Tier A-4 asked for comic + bank; done.)
9. **Workers have no tests** — `media/worker/{transcode,process_image,thumbnail}.go`
   (matrix SPEC-01 P0.1/P0.2), and `comic.RunImport` — the largest function in
   that module, reachable only with an object store, a tenant runner, a real
   zip and wall-clock sleeps. *Closes when:* the image pipeline's cap/variant/
   orientation rules and the poster's audio-skip are unit-tested against
   fixtures, and `RunImport` has a fake-store test.
10. **`pnpm test` is not in CI** — SPEC-12 (2026-09-19) added the first
    frontend tests (three vitest files under `frontend/src/lib/`, 27 tests,
    green locally), so `vitest run` no longer exits non-zero on an empty
    suite (audit §3.1, Tier C-13), but the `frontend` CI job still never runs
    it. *Closes when:* `pnpm test` is in the `frontend` CI job.
11. **oapi-codegen is pinned in CI but not locally** (ADR-10). No `tool`
    directive in `backend/go.mod`; a developer on another version produces a
    diff the gate rejects. *Closes when:* `go.mod` carries the tool directive
    and `make openapi` uses `go tool oapi-codegen`.
12. **No spec lint** (ADR-10 decision item 5a). The `openapi` job parses the
    YAML and diffs codegen; nothing checks the spec for structural mistakes.
    *Closes when:* `redocly lint` or `vacuum` runs in the job.
13. **Bank and comic never assert their own event emits** (matrix SPEC-03 P0.7,
    SPEC-02 P1.9). Consumers are tested; `emitTx` / `chapter_published` are
    not. *Closes when:* each service test asserts the publish.
14. **Rollback errors discarded** in `platform/db/db.go` (3 sites) — audit §5
    bug 8. Lower severity than the commit case (fixed) but hides connection
    death. *Closes when:* they are logged.
14a. **The pgx idle-connection fix lives only in this deployment's `.env`.**
    `host.docker.internal`'s NAT drops idle connections and the symptom is a
    silent 401 on every authenticated route; the cure is
    `pool_max_conn_idle_time=60s&pool_health_check_period=30s&pool_max_conn_lifetime=30m`
    on `DATABASE_URL`. `.env.example` does not carry it and `platform/db.NewPool`
    does not set it, so a fresh clone gets the bug back. *Closes when:* `NewPool`
    sets the three pool options in code (URL params then become optional).
15. **`/calendar` and `/weather` are not in the auth middleware matcher**
    (`frontend/src/middleware.ts`) — audit §5 bug 9. *Closes when:* the
    matcher lists every `(app)` route, or matches the group.
16. **Tenant-prefixed object keys never happened** (ADR-04 decision item 4).
    Keys are `uploads/<id>/…` and `hls/<id>`; isolation is by RLS on `assets`
    and by presigned URLs, not by prefix. *Closes when:* a decision is recorded —
    either the prefix is dropped from ADR-04 as unnecessary under RLS, or a
    migration of every object is scheduled.
17a. *(closed 2026-09-19 — see § Closed.)*
17. **Composition rule not in `account/README.md`** (ADR-02 item 2) and no
    depguard reservation for `policy`/`usergroup` (item 3). Small; do together.

## P2 — specced, not built (from audit §3.3, still absent 2026-09-11)

18. *(closed 2026-09-19 — SPEC-12 executed; see § Closed. Number kept so
    citations of "backlog #18" still resolve.)*
19. SPEC-06 P1.5 **on-this-day** `GET /stream/memories`; P1.6
    `journal:backfill_stream`.
20. SPEC-06 **stream de-projection is a decision, not a gap** — `0033`/`0034`/
    `0040` (2026-08-28) removed `asset_ready`, comic chapter and catalogue
    publishes from the stream on purpose (library events belong in the bell).
    SPEC-06 P0.1 still describes them as projected: the spec's fact layer is
    stale. *Closes when:* SPEC-06 says what the stream projects today.
21. SPEC-04 P1.1 **Web Push** (table exists, handler is a stub), P1.2 **SSE**,
    P1.3 **notification preferences** route (table exists), P1.4
    **`account.security_alert`** on refresh-reuse. P2 `notify:purge_old` is
    registered and never scheduled — dead code until a `scheduler.Register`.
22. SPEC-03 P1.10 **receipt attachments**, P1.12 **`bank:budget_exceeded`**,
    P1.13 **structured transfer fees** (`fee_amount`). (P1.11 monthly report:
    `GET /bank/report` and `/bank/reports` shipped with 0042 — check the spec's
    acceptance before calling it done.)
23. SPEC-10 phases 2–8 — savings goals, recurring, credit-card cycles,
    investments/net worth, automation rules, splits/tags, shared ledgers — in
    the order the spec gives. Phase 1 (debts) shipped `017ebfe`.
24. SPEC-02 P1.8 **bookmarks** (`comic_bookmarks`) — note the P-number collision
    with the shipped external-source sync (audit §4.6); fix the spec numbering
    when this is picked up.
25. SPEC-07 P2 **comic leg of `/continue`** — `handleContinue` calls only
    `mediaMod.API().Continue`.
26. SPEC-08 P1.6 **interactions log**, P1.7 **avatar reap** (`people` is not
    subscribed to `media:asset_deleted`).
27. SPEC-09 P1.7 **owner takeout** (`ops_exports`, `/me/export`, `ops:takeout`);
    P1.6 queue console.
28. **Movie and story have no frontend.** `NovelDetailView.tsx` is still the
    26-line placeholder; no `/movies` route exists. Music got its UI
    (library, import, playlists, player) in 0038–0041. Finish these to the
    music standard or revert them (audit Tier D-14) — do not leave them.
29. **Story reading progress**, **movie/story FTS** (not now — no corpus at
    n=1), and media's three: **HLS variant ladder** per tier (transcode
    produces one rendition), **S3 multipart upload** for large originals (a
    source is one presigned PUT), **audio transcode profile** (audio is served
    as-is). These were the genuine items in the module READMEs' "Open work"
    sections, removed 2026-09-11 (one owner for status).
30. **`docs/operations/deployment.md` and `r2-setup.md`** do not exist (ADR-03
    item 6, ADR-04 item 7). The VPS sizing rationale and the R2 CORS JSON live
    nowhere but the ADRs.
31. **Dragonfly `--maxmemory` cap** (ADR-03 item 1) and the worker `/tmp` cap
    (ADR-04 item 6) — both absent; the OOM guard is `heavyConcurrency = 1`.
32. **Prove the restore drill against a real nightly dump and record the date**
    (SPEC-09 P0.4; audit Tier D-16). The script exists; whether it has ever
    passed is not in the tree.
33. **`frontend/src/templates/README.md` still describes OIDC auth** and
    `frontend/CLAUDE.md` names React Hook Form as the form-state owner
    (D-32) — RHF is not in `package.json`. D-33 "RSC-first" is inverted in
    practice (65 `"use client"` files). Either fix the docs or the code;
    currently both claim the other.
34. **`feature-inventory.md` has no `D-30`** (sequence jumps D-29 → D-31) and
    still writes `app.tenant_id` in three §18 bullets under a superseding note.
35. **Module `README.md` "Open work" sections** — audit §4.5 listed seven that
    name migrations that do not exist and work that is done. Not re-verified
    line by line in this triage; treat each as suspect until its
    `Last verified` is bumped.
36a. **`architecture/{diagrams,overview,security}.md`, `adr/diagrams/system-landscape.md`,
    `guides/getting-started.md`, `operations/postgres-tuning.md`** still draw or
    describe `postgres` + `pgbouncer` as compose services (gone 2026-08-21; host
    PG 18). All now carry `Last verified: never` so the reader is warned; fixing
    the diagrams is one pass with `docker-compose.yml` open.
36. **`docs/testing/TEST-PLAN.md`** still describes a container-backed L2
    integration layer that does not exist, says `make up` starts Postgres and
    PgBouncer, and says CI runs `vitest` (audit §4.6). Correct it when line 3
    or line 10 lands, since both change what is true.

## Deferred — not a gap (ADR-01 as re-affirmed by ADR-08; audit §7)

Social layer beyond `social` connections (posts, feed ranking, messaging,
groups) · advanced social (D-35) · creator economy (D-40) · marketplace · ML
safety (D-38) · LiveKit/mediamtx (D-36/D-39) · the observability stack (D-8;
ADR-07's "same sprint as tenancy" coupling is dropped, not ignored) · real bank
integration and with it MFA/TOTP (D-27/D-28 gate on credentials the ledger does
not hold) · full-text search (no corpus at n=1) · native apps, i18n, push
providers (D-5/D-6) · ADR-07 steps 5–7 (`switch-tenant`, `/admin/organizations`,
per-tenant `user_roles`, `/t/{org}` prefix, `cmd/sysjobs`) at one user with one
personal org.

## Closed since the 2026-08-25 audit (so it can be checked off)

- P1 #17a **SPEC-12 residue** — closed 2026-09-19. The manual run against the
  stack: every step passed, one focus defect found and fixed
  ([TEST-RUN-2026-09-19-spec-12.md](../testing/TEST-RUN-2026-09-19-spec-12.md)).
  The `0044`/`0045` backfill loops:
  `modules/journal/backfill_test.go: TestBackfillMigrationsMoveLinksOutOfBodies`
  builds a throwaway database from the migration files, seeds old-style
  bodies and runs up → down → up plus the RAISE case, on the same
  `RLS_TEST_ADMIN_URL` CI already provides. Its first run found that
  `0044 down` could not complete on a row the up had emptied (a link-only
  body whose Asset was gone) — the restored CHECK is now `NOT VALID`.
- P2 #18 **journal photo attachments** (SPEC-05 P1.5) — closed 2026-09-19 by
  [SPEC-12](specs/SPEC-12-journal-attachments.md), executed as tickets
  #9–#15 on `feat/journal-attachments` (`d8b2910`, `b6d8a89`, `1e1f12a`,
  `ebe97ca`, `4215701`, `aba8785`): up to ten Attachments in `asset_ids`
  validated as a whole, the Location in three columns, both backfilled out of
  every body by self-asserting migrations `0044`/`0045` (applied to the live
  database), the asset-deleted consumer stripping the id in the owner's
  scope, one composer for create and in-place edit, and
  `frontend/src/lib/attachments.ts` deleted. Evidence: the SPEC-12 section of
  the [traceability matrix](../reference/TRACEABILITY-MATRIX.md). Tracker #8
  and #9–#14 closed 2026-09-19; #15 closes when CI confirms the four docs
  checks on the close-out commit. Residue has owners: the frontend vitest
  files run locally only (P1 line 10); the manual run was done 2026-09-19
  ([TEST-RUN-2026-09-19-spec-12.md](../testing/TEST-RUN-2026-09-19-spec-12.md))
  and the backfill loops are under test (17a, closed the same day).
- P0 **RLS suite not run in CI** — closed 2026-09-11: the `backend` job
  starts a `postgres:18` service, applies every migration to it with
  `golang-migrate` (so the chain is also proven from zero on each push), and
  sets `RLS_TEST_ADMIN_URL` / `RLS_TEST_APP_URL`; `rls_test.go: setup` fails
  instead of skipping when `CI` is set and the URLs are not, so the gate
  cannot lapse silently. Verified locally first (fresh database, every
  migration from zero, 19/19 green as `portal_app`), then by the first CI
  execution: PR #7 (`f0901bd`), Actions run `34633577648`, `backend` green
  with the guard in place — which it could not be had the suite skipped.
- P0 **CI red on `main` since 2026-07-23** — closed 2026-09-11 (`a30b887`,
  `0df5a11`): pnpm is the one package manager, `frontend/pnpm-lock.yaml` is
  tracked and `package-lock.json` deleted, the `setup-node` cache path
  matches, `main` green at `4c0049c`; `main` is now branch-protected on all
  five jobs. Residue, not a gap: the Dockerfile still falls back to
  `|| pnpm install` when the frozen install fails.
- P0 **RLS decorative on every fresh install** — closed 2026-09-11:
  `.env.example` defaults `DATABASE_URL` to `portal_app` with the password
  `0019` seeds; `MIGRATE_DATABASE_URL` (owner) added and `make migrate` uses
  it; `BACKUP_DATABASE_URL` stays on `portal`. Evidence the switch is safe:
  this deployment ran as `portal_app` from 2026-08-25 through today's SPEC-10
  work, and all 19 RLS tests pass against the live cluster as `portal_app`.
- P0 **committed dev credential** — closed 2026-09-11. It was the password of
  local *application* accounts (`@portal.localhost`), not a Postgres role; two
  of them still used it, one holding Super Admin. Both rotated to random
  passwords with a one-off tool (Argon2id via the account module's own
  hasher), `token_version` bumped and every refresh token revoked, the tool
  deleted. History not purged — the exposed value now opens nothing.

- §5 bug 5 **`comic.SaveProgress` skips the visibility gate** — closed
  2026-09-11: `SaveProgress` now calls `GetComic` (published-or-owner) before
  any membership answer; `comic_test.go: TestSaveProgressRespectsDraftVisibility`.
- §5 bug 7 **`media/api.SignedURL` returns `("", nil)`** — closed 2026-09-11:
  a real `signFn` seam (`Service.SignedOriginalURL`: tenant-scoped lookup,
  READY only, presigned GET on the source key) and an error when unwired;
  `media/api/api_test.go: TestSignedURLWithoutASignerIsAnError`,
  `media/service_test.go: TestSignedOriginalURL`.
- §5 bug 1 **silent commit failure** — `require_tenant.go` now commits before
  the response is released; `require_tenant_test.go: TestMutatingRequestCommitFailureBecomes500`.
- §5 bug 2 **`imageSrv.Shutdown()`** — present in `cmd/worker/main.go`.
- §5 bug 3 / §3.1 **no brute-force protection on `/auth/login`** — the audit
  looked at `platform/middleware/ratelimit.go` (no importers) and missed the
  handler's own guard: `loginThrottled` / `recordLoginFailure` in
  `account/handler/auth.go` (5 failures per 15 min per IP and per account,
  Redis-backed, 429), wired since `05b6cf7` 2026-07-05. **Still untested.**
- §5 bug 4 / §4.1 **media worker at RLS cutover** — `media.Deps` carries the
  tenant scope; `rls_test.go: TestRLSVariantInsertResolvesTenantFromTheScope`.
- §5 bug 6 **`HasPermission` returns false** — implemented when the layout
  module needed it (0036).
- §3.1 **RFC 7807 not the contract** — `platform/server.Problem` is the single
  writer since `ac4f71d` 2026-08-25; the four shims delegate.
- §3.1 **~30 endpoints outside OpenAPI** — the spec is 111 paths with tags for
  every module including movies/music/stories/tenant/platform.
- §3.2 **cutover chain** steps 1 (runtime), 4, 8 — done here 2026-08-25;
  ADR-07 re-graded.
- §3.3 SPEC-01 P1.1 `PATCH /assets/{id}` — mounted (visibility only, 0032); metadata edit itself is still open, folded into the matrix's P1.1 ⚠.
- §3.4 **music** is no longer half-built (0038–0041); movie/story still are
  (line 28).
- §4.2 **`events.md` wrong** — re-derived 2026-08-25.
- §4.3 **`.env.example` points at dead hosts** — hosts fixed
  (`host.docker.internal`); the *role* default is P0 line 1.
- §4.4 **refresh TTL 30 d vs 24 h** — CLAUDE.md and ADR-06 say 24h.
- §4.6 **MILESTONE_CHECKS cited as live** (all sites), **`/auth/callback`
  drift claim**, **ADR status lines disagree**, **`backlog.md` inverted**,
  **`facebook-comparison.md` unlabelled**, **TRACEABILITY-MATRIX names zero
  tests**, **a session note committed with a credential** (file deleted;
  rotation is P0 line 2) — all closed by the ADR-11 work of 2026-09-11.
- §6 Tier A-3 **`internal/platform/server`** — exists.
- §6 Tier C-10 **stale module READMEs** — *not* closed; line 35.
- §6 Tier C-12 **retire the old backlog / label facebook-comparison** — this file.
