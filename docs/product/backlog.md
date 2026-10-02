# Backlog

**Status:** current · **Last verified:** 2026-09-19

The **single priority list** for open work: what to do next, in what order.
Ordering inside a tier is priority; nothing else in `docs/` ranks work.

Two kinds of work are ranked here, and they are recorded differently:

- **Spec↔code divergences** are *recorded* in each spec's "Implementation
  gaps vs shipped code" section (indexed in
  [specs/README.md](specs/README.md#implementation-gaps-index-2026-10-01)),
  which owns the detail and the closing condition. This file only *ranks*
  them — by spec and row in § "P0 — order for closing spec gaps", with a
  numbered line of its own only when a row needs to stand out. A gap row not
  named here is still open; it is worked in its tier's order.
- **Everything else** — ops, docs, CI, unspecced work — is one numbered line
  here, saying what is wrong, where the evidence is, and what closes it.

Anything in neither place is done, deliberately deferred (§ Deferred), or has
not been found yet.

**How this file is maintained** (ADR-11 rule 6 / [docs/README.md](../README.md)
§ analysis): every audit in [analysis/](analysis/) must either produce lines here
or be closed with a reason. This revision triaged the whole of the 2026-08-25
audit, `remaining-work-2026-08-25.md` (every numbered item in its §3–§6; the
file was deleted once fully triaged — "audit §N" below cites
`git show ea100d8:docs/product/analysis/remaining-work-2026-08-25.md`) plus the
open action items from the ADR
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

## P0 — order for closing spec gaps

Moved here from the specs README on 2026-10-01 so that one file ranks all
work. Rows are cited as *SPEC-NN row N* (that spec's gaps section); lines
#37–#44 below lift the most severe of them out for visibility.

1. **Security.** SPEC-14 §11 rows 1–3 (internal endpoints off the public edge,
   constant-time secret), SPEC-05 §11 rows 1–2 (no plaintext reset token;
   atomic consume + revoke-all), SPEC-03 §11 row 1 (queue console read/write
   split + CSRF), SPEC-04 §11 rows 4–6 and 1 (`/original` states and size; the
   delete commit), SPEC-12 §12 row 1 (caller filter on balance queries),
   SPEC-01 §11 rows 1–5 (role re-parent escalation, atomic refresh rotation,
   the unwired `IPRateLimiter`, trusted client IP, uniform login timing),
   SPEC-02 §11 row 1 (`href` bypass). Then the account rows the 2026-10-02
   owner decisions made targets, in this order: SPEC-01 row 26 (no
   first-registrant Superadmin; pre-created Superadmin who must change the
   password — A2; it also closes row 10), row 30 (admin email change waits for
   verification — A9), row 25 (uniform registration — A1, together with row 5,
   which it makes load-bearing), row 28 (lockout per (email, IP) — A4, after
   row 4), row 27 (logout ends one Session — A3). Then the **shared-read
   work of [ADR-12](specs/SPEC-01-account-identity-admin.md#adr-12)**
   (Decision 2026-10-02b (B14), revising B13): it opens a deliberate,
   read-only door in ADR-07's tenant fence, so it ranks with the security
   rows and lands as one ordered sequence, each step with its RLS-suite test
   in CI — SPEC-01 row 32 (`000N_tenant_links`, `portal_acl`,
   `app_can_read_shared`, the links API and screen) with SPEC-18 row 15's
   `000N_social_acl_grant`; SPEC-04 row 20 (`000N_media_shared_visibility`,
   the read rule) together with SPEC-01 row 33 (request scopes without the
   tenant-admin flag) and SPEC-04 row 21 (`mediaapi.SetVisibility`); then
   SPEC-15 row 29 (music), SPEC-16 row 19 (movie) and SPEC-17 row 22 (story),
   each with its `shared_read` policy, and SPEC-10 row 15 (readers keep their
   own progress row; after SPEC-10 row 6). SPEC-18 row 15's discovery
   narrowing ships **no earlier than the links screen**: on an instance of
   personal organisations it empties the directory until owners link their
   tenants. Each step depends on the one before it. Then the remaining
   Sec / AuthZ rows: SPEC-14 rows 4–5, SPEC-03 rows 2–4, F009, SPEC-01
   rows 6–7, SPEC-15 row 25, SPEC-16 rows 6–7, SPEC-17 rows 7–8.
2. **Data loss.** SPEC-10 rows 1–3, SPEC-07 row 1 together with the other
   publish-after-commit rows (SPEC-15 row 4, SPEC-16 row 3, SPEC-17 row 4,
   SPEC-18 row 1), SPEC-09 rows 1–2 together with SPEC-11 rows 1–4 (the
   retraction events and their stream consumers), SPEC-04 rows 2–3, SPEC-05
   rows 3–4, SPEC-03 rows 5–6, SPEC-13 row 1, SPEC-12 rows 2–5, SPEC-14 row 6,
   the unscoped asset-deleted consumers (SPEC-14 row 18, SPEC-15 row 1,
   SPEC-16 row 1, SPEC-17 row 1), SPEC-15 row 2, SPEC-16 row 2, SPEC-04
   row 19 then SPEC-01 row 8 (deleting a user purges their media first —
   Decision 2026-10-01b (D1)) then SPEC-01 row 29 (every module's
   `PurgeOwnerData` and the `deleted_users` snapshot — Decision 2026-10-02
   (A6)); SPEC-01 row 31 (audit identity data encrypted and anonymised after
   90 days — A7). Both are data-protection P0s: until they land, a deleted
   User leaves objects and keys behind and the audit log keeps IPs and emails
   in clear forever. Since Decision 2026-10-02b (B1) row 29 also seals the
   `deleted_users` snapshot through row 31's `platform/audit` encryption
   function (exported for that caller), so row 29 lands with or after row 31's
   encryption half — or ships writing `pii` NULL.
3. **Cross-cutting foundations.** Timezone (account change, then its readers),
   Envelopes (per module, with OpenAPI and `problems.ts` in the same PR), Audio
   (SPEC-04 row 16 before SPEC-10 rows 4, 5, 8), then SPEC-15 row 27 (the
   music player resumes and saves through the audio asset's media progress;
   tracks join `/continue` as media items — Decision 2026-10-02b (B2); needs
   SPEC-10 rows 5 and 4, and row 15 for other readers' own resume — B13,
   B14).
4. **Remaining integrity, contract and UX rows**, per spec, in each section's
   order; the `x-required-permission` retrofit with its drift check. Rows
   Decision 2026-10-02b added rank here: SPEC-17 row 21 together with SPEC-14
   row 9 (blank chapters of a published work hidden from non-owners — one
   pattern, B7), SPEC-15 row 28 (a self-publish makes no bell entry;
   `actor_user_id` on `music:track_published` — B4), SPEC-18 row 13 (24-hour
   re-request cooldown after a decline, `social_declines` — B8).
5. **Unbuilt P1**: SPEC-04 row 14, SPEC-05 row 21, SPEC-09 row 16, SPEC-10
   row 14, SPEC-11 row 13, SPEC-03 row 11 (owner takeout, above), SPEC-16
   rows 16–17 and SPEC-17 rows 18–19 (committed: finish, not revert —
   Decision 2026-10-01b (D2)), SPEC-02 row 13 (P1.2–P1.3 only; P1.1
   optimistic concurrency dropped — Decision 2026-10-02b (B12)), SPEC-18
   row 14 (`social:connection_removed`, emit-only, with or after SPEC-18 row 1
   — B9); SPEC-01 P1.4 (self-service
   email change, on row 30's machinery — A9); the admin-change events
   (SPEC-05 row 23 first, then SPEC-01 row 24 and SPEC-02 row 14 — D3); the
   `user` authoring grants (SPEC-15 row 26, SPEC-16 row 18, SPEC-17 row 20,
   with or after F009 — D4).

## P0 — code defects found by the as-built specs (2026-10-01)

The as-built specs SPEC-01, 02 and 15–18 were written retroactively from the shipped code, and each ends with
an implementation-gaps table that owns its rows (cite them by spec and row).
Only the security and data-integrity rows are raised here; the rest stay in
their spec. Severity order.

37. **Role re-parent escalates to `*`** — an `rbac:role:write` holder creates an
    empty role, gets it assigned, then re-parents it under `superadmin`;
    `handler/admin.go` `UpdateRole` checks existence, `is_system` and cycles
    only. [SPEC-01](specs/SPEC-01-account-identity-admin.md) §11 row 1.
    *Closes when:* a parent change is refused unless the actor holds every
    permission the new parent adds (TC-ACC-071/072).
38. **Layout `href` open-redirect bypass** — `/\evil.example` and a
    TAB-split `//` pass `layout/service.go` `SaveMenu`'s leading-`/` check and
    render in every user's sidebar.
    [SPEC-02](specs/SPEC-02-shell-layout.md) §11 row 1. *Closes when:* `\`,
    control characters and whitespace are refused and
    `TestSaveMenuRejectsOffSiteLinks` covers both bypasses.
39. **An admin can disable, reject or revoke a superadmin** — `decide` and
    `SetDisabled` skip `targetAuthorityDenial`, which guards only edit and
    delete; disabling bumps the target's `token_version`. SPEC-01 §11 row 6.
    *Closes when:* the five operations answer 403 `account/escalation` for an
    out-ranking target.
40. **`/auth/register` and `/auth/refresh` are unthrottled** —
    `platform/middleware/ratelimit.go` `IPRateLimiter` is imported by no
    binary and Traefik's `rate-limit` middleware is attached to no router, so
    each anonymous register writes a pending row and enqueues up to 50
    approver notifications. The login throttle's per-IP key trusts any
    `X-Forwarded-For`. SPEC-01 §11 rows 3–4. *Closes when:* the limiter is
    mounted on both routes keyed on a trusted client IP.
41. **Refresh rotation is not atomic** — `auth/refresh.go` `Rotate` checks
    `revoked_at`, issues, then marks the old token replaced unconditionally,
    so two concurrent presentations both succeed (forked chain, no theft
    detection). SPEC-01 §11 row 2. *Closes when:* the old row is claimed with
    `… WHERE revoked_at IS NULL RETURNING` before the successor is issued.
42. **`media:asset_deleted` consumers run without a tenant scope** — comic,
    movie, story and music handle the event on the bare pool, which FORCE RLS
    refuses as `portal_app`; the task retries out and published works keep
    pointing at deleted assets. [SPEC-15](specs/SPEC-15-music-vertical.md) §12
    row 1, [SPEC-16](specs/SPEC-16-movie-vertical.md) §11 row 1,
    [SPEC-17](specs/SPEC-17-story-vertical.md) §11 row 1, and comic's copy in
    [SPEC-14](specs/SPEC-14-comic-vertical.md) §11 row 18. *Closes when:* each
    consumer runs inside `runInUserTenant` for the payload owner, proven by an
    RLS-suite test.
43. **Re-uploading a music import creates every track again** —
    `SetMusicImportUpload` has no status predicate, so a second
    `PUT …/upload` on a `processing` or `done` job enqueues a second
    `music:import_zip`. SPEC-15 §12 row 2. *Closes when:* only a `pending` job
    accepts an upload (409 otherwise, checked before the store write).
44. **A concurrent duplicate connection request is a 500** — the pair-index
    violation aborts the request's tenant transaction, so its 409 becomes a
    commit-failure 500. [SPEC-18](specs/SPEC-18-social-connections.md) §11
    row 2. *Closes when:* `CreateRequest` uses `ON CONFLICT DO NOTHING` and the
    race test (TC-SOC-006) passes.

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
8. *(closed 2026-09-19 — see § Closed.)*
8a. *(closed 2026-09-19 — see § Closed.)*
9. **Workers are thinly tested** — the image pipeline's admission and scaling
   rules, its encode/probe on drawn fixtures, and the whole poster path
   (`Handle` over an in-memory store with lavfi-synthesised video: cut, stored,
   audio-only skipped, non-fatal) are under test since 2026-09-19
   (`media/worker/{process_image,thumbnail,poster_fixture}_test.go`; CI
   installs ffmpeg for them). Still untested: EXIF orientation (no way to draw
   an EXIF fixture with the standard library — needs a checked-in JPEG or a
   tiny writer) and `comic.RunImport` — the largest function in that module,
   reachable only with an object store, a tenant runner, a real zip and
   wall-clock sleeps — it needs a fake `storage.Storage` it can import.
   That fake exists since 2026-09-19: `platform/storage/storagetest.MemStore`
   (the media tests' two unexported copies, folded into one owner with its
   semantics pinned by its own tests), so this half is unblocked. *Closes when:* an
   orientation fixture and `RunImport` with a fake store are under test; `transcode.go`'s HLS output is out of this line's scope (its own item
   when someone needs it).
10. *(closed 2026-09-19 — see § Closed.)*
11. **oapi-codegen is pinned in CI but not locally** (ADR-10). No `tool`
    directive in `backend/go.mod`; a developer on another version produces a
    diff the gate rejects. *Closes when:* `go.mod` carries the tool directive
    and `make openapi` uses `go tool oapi-codegen`.
12. **No spec lint** (ADR-10 decision item 5a). The `openapi` job parses the
    YAML and diffs codegen; nothing checks the spec for structural mistakes.
    *Closes when:* `redocly lint` or `vacuum` runs in the job.
13. **Bank and comic never assert their own event emits** (matrix SPEC-12 P0.7,
    SPEC-14 P1.9). Consumers are tested; `emitTx` / `chapter_published` are
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
15. **`/admin`, `/calendar` and `/weather` are not in the auth middleware
    matcher** (`frontend/src/middleware.ts`) — audit §5 bug 9 (calendar,
    weather); `/admin` is [SPEC-02](specs/SPEC-02-shell-layout.md) §11 row 10
    and [SPEC-01](specs/SPEC-01-account-identity-admin.md) §11 row 21. The API
    still refuses; a signed-out visitor gets an error state instead of
    `/login`. *Closes when:* the matcher lists every `(app)` route, or matches
    the group.
16. **Tenant-prefixed object keys never happened** (ADR-04 decision item 4).
    Keys are `uploads/<id>/…` and `hls/<id>`; isolation is by RLS on `assets`
    and by presigned URLs, not by prefix. *Closes when:* a decision is recorded —
    either the prefix is dropped from ADR-04 as unnecessary under RLS, or a
    migration of every object is scheduled.
17a. *(closed 2026-09-19 — see § Closed.)*
17. **Composition rule not in `account/README.md`** (ADR-02 item 2) and no
    depguard reservation for `policy`/`usergroup` (item 3). Small; do together.
    The layering itself is [SPEC-01](specs/SPEC-01-account-identity-admin.md)
    P2; the account module's other gaps are SPEC-01 §11, not lines here.

## P2 — specced, not built (from audit §3.3, still absent 2026-09-11)

18. *(closed 2026-09-19 — SPEC-08 executed; see § Closed. Number kept so
    citations of "backlog #18" still resolve.)*
19. SPEC-09 P1.5 **on-this-day** `GET /stream/memories`; P1.6
    `journal:backfill_stream`.
20. SPEC-09 **stream de-projection is a decision, not a gap** — `0033`/`0034`/
    `0040` (2026-08-28) removed `asset_ready`, comic chapter and catalogue
    publishes from the stream on purpose (library events belong in the bell).
    SPEC-09 P0.1 still describes them as projected: the spec's fact layer is
    stale. *Closes when:* SPEC-09 says what the stream projects today.
21. SPEC-05 P1.1 **Web Push** (table exists, handler is a stub), P1.2 **SSE**,
    P1.3 **notification preferences** route (table exists), P1.4
    **`account.security_alert`** on refresh-reuse (the account half is
    [SPEC-01](specs/SPEC-01-account-identity-admin.md) P1.2). P2 `notify:purge_old` is
    registered and never scheduled — dead code until a `scheduler.Register`.
22. SPEC-12 P1.10 **receipt attachments**, P1.12 **`bank:budget_exceeded`**,
    P1.13 **structured transfer fees** (`fee_amount`). (P1.11 monthly report:
    `GET /bank/report` and `/bank/reports` shipped with 0042 — check the spec's
    acceptance before calling it done.)
23. SPEC-13 phases 2–8 — savings goals, recurring, credit-card cycles,
    investments/net worth, automation rules, splits/tags, shared ledgers — in
    the order the spec gives. Phase 1 (debts) shipped `017ebfe`.
24. SPEC-14 P1.8 **bookmarks** (`comic_bookmarks`) — note the P-number collision
    with the shipped external-source sync (audit §4.6); fix the spec numbering
    when this is picked up.
25. SPEC-10 P2 **comic leg of `/continue`** — `handleContinue` calls only
    `mediaMod.API().Continue`.
26. SPEC-11 P1.6 **interactions log**, P1.7 **avatar reap** (`people` is not
    subscribed to `media:asset_deleted`).
27. SPEC-03 P1.7 **owner takeout** (`ops_exports`, `/me/export`, `ops:takeout`);
    P1.6 queue console.
28. **Movie and story have no frontend.** Movie: no route, view or
    `lib/movie.ts` ([SPEC-16](specs/SPEC-16-movie-vertical.md) P1.1, §11
    row 16). Story: `NovelDetailView.tsx` is a placeholder and
    `/library/novel` has no route ([SPEC-17](specs/SPEC-17-story-vertical.md)
    P1.1, §11 row 18). Music got its UI (library, import, playlists, player)
    in 0038–0041 ([SPEC-15](specs/SPEC-15-music-vertical.md); its remaining
    gaps are SPEC-15 §12). **Decided: finish** to the music standard, not
    revert (owner decision 2026-10-01b (D2), audit Tier D-14): SPEC-16 P1.1 and
    SPEC-17 P1.1 are committed scope. Kept in this tier — specced, not built;
    the decision only removed the blocker. The `user` authoring grants that
    make both useful to a second account are SPEC-16 P1.3 / SPEC-17 P1.3 (P0
    order, step 5).
29. **Story reading progress** and its `/continue` leg (SPEC-17 P1.2, §11
    row 19), **movie/story FTS** (SPEC-16 P2, SPEC-17 P2 — not now, no corpus
    at n=1), and media's three: **HLS variant ladder** per tier (transcode
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
    (SPEC-03 P0.4; audit Tier D-16). The script exists; whether it has ever
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
36a. **`architecture/{diagrams,overview,security}.md`,
    `guides/getting-started.md`** still draw or
    describe `postgres` + `pgbouncer` as compose services (gone 2026-08-21; host
    PG 18). All now carry `Last verified: never` so the reader is warned; fixing
    the diagrams is one pass with `docker-compose.yml` open. Two more files
    had the same defect and were deleted instead: `adr/diagrams/system-landscape.md`
    (a duplicate of `architecture/diagrams.md` §1) and
    `operations/postgres-tuning.md` (a stub, folded into SPEC-03 P2).
36. **`docs/testing/TEST-PLAN.md`** still describes a container-backed L2
    integration layer that does not exist, says `make up` starts Postgres and
    PgBouncer, and says CI runs `vitest` (audit §4.6). Correct it when line 3
    or line 10 lands, since both change what is true.

## Deferred — not a gap (ADR-01 as re-affirmed by ADR-08; audit §7)

Social layer beyond `social` connections (posts, feed ranking, messaging,
groups, block/mute, follow graph, profiles — [SPEC-18](specs/SPEC-18-social-connections.md)
§3 and §12; each needs its own spec and envelope argument) · advanced social (D-35) · creator economy (D-40) · marketplace · ML
safety (D-38) · LiveKit/mediamtx (D-36/D-39) · the observability stack (D-8;
ADR-07's "same sprint as tenancy" coupling is dropped, not ignored) · real bank
integration and with it MFA/TOTP (D-27/D-28 gate on credentials the ledger does
not hold) · full-text search (no corpus at n=1) · native apps, i18n, push
providers (D-5/D-6) · ADR-07 steps 5–7 (`switch-tenant`, `/admin/organizations`,
per-tenant `user_roles`, `/t/{org}` prefix, `cmd/sysjobs`) at one user with one
personal org.

**Parked with a re-entry condition** — the trigger that puts each item back on
the table, so the decision is not re-litigated every session. Folded in from the
2026-07-07 parking-lot brief (deleted —
`git show ea100d8:docs/product/briefs/04-deferred.md`).

| Item | Why deferred | Re-entry condition |
|---|---|---|
| **Statement import** (bank) | The owner's bank (TCB) exports **PDF**, so a decent import means PDF parsing/OCR — an effort black hole. The schema is import-ready (SPEC-12 P0.9), so nothing is lost by waiting. | CSV/xlsx can be had from a bank in use, **or** the generic CSV + column-mapping path is accepted first and PDF later. The design is pre-agreed (SPEC-12 §3): mapping templates as data, not code; `dedup_hash`; per-batch rollback. |
| **TOTP / MFA / step-up** (D-27/D-28) | It once gated "bank"; the ledger holds no bank credentials, so the gate does not apply. | Real bank credentials or API sync, or any money-*moving* feature. TOTP is then the named unlock task, not a floating P2. |
| **Messenger / people search** (the friend graph left — see below) | The feature-parity trap at n=1; a life OS starts from one user. | Real second users on an instance (e.g. family). Re-enter through "share to household member", not full Facebook parity. |
| **Email verification** | No real second users. | The first real external user. |
| **Playback ACL** | One user on LAN/VPS tolerates public-ish HLS short-term. | Any second user. (The HLS variant ladder is P2 line 29.) |
| **Time domain** (calendar/tasks) | It was the cheapest first life domain; the owner chose money + entertainment first. | "After SPEC-12" — met, so it now waits only on a spec; it is the likely next facet, wiring the calendar widgets that already exist. The birthday slice shipped separately as SPEC-11 (contact data, not calendar/tasks). |
| **HEIC/HEIF image ingest** | ffmpeg HEIC decode hinges on libheif/HEVC build flags — a build-matrix rabbit hole outside the v1 envelope (SPEC-04 §3). | Dogfooding involves an iPhone user: HEIC becomes P0 for photo upload (likely a libheif pre-step in the worker image). |

Left the parking lot since 2026-07: the notifications module and password reset
(SPEC-05), the music vertical (`0038`–`0041`, SPEC-15), movie and story
(P2 line 28; SPEC-16, SPEC-17), the friend graph's first slice as social
connections (`0037`, SPEC-18 — approval-gated registration made the instance
n>1),
debts and loans (SPEC-13 phase 1, built) and investments (SPEC-13 phase 5),
presigned direct upload (shipped; multipart for large originals is P2 line 29).

## Closed since the 2026-08-25 audit (so it can be checked off)

- P1 #8a **HTTP-contract helpers in nine copies** — closed 2026-09-19:
  `platform/server/servertest` (`RequireAuth`, `CurrentUser`, `Do`,
  `Problem`) is the one owner; the nine `modules/<m>/http_test.go`
  import it (journal wraps `RequireAuth` to add its tenant-scope marker).
  `servertest_test.go` pins the helpers themselves against the real
  `server.Problem` writer. MODULES.md §8 step 9 names it for the next module.
- P1 #8 **HTTP contracts asserted for comic and bank only** — closed
  2026-09-19: movie, music, story, journal, people, social and notify carry
  the two tests over the real router with fakes (`modules/<m>/http_test.go`;
  notify's second is mark-read-twice, its only write). Layout is the
  exception by shape, not by omission: it has no per-id resource, so
  "a stranger's id" and "delete twice" do not exist there — its contract is
  the whole-set save with `layout/unknown-widget` / `layout/validation`,
  under `layout/service_test.go`. Writing the tests found movie, music and
  story answering 204 to a repeat DELETE — the module left the 404 to
  cmd/api's owner guard; their `Delete*` are `:execrows` now, like comic's.
  Matrix CC-1 / CC-3 / CC-8 name every file.
- P1 #10 **`pnpm test` not in CI** — closed 2026-09-19: the `frontend` job
  runs `pnpm test` between typecheck and build (`vitest.config.ts` gives the
  suite the app's `@/` alias); the three vitest files SPEC-08 added are the
  first frontend suite CI has ever run, and the matrix checker accepts a
  `frontend/…/x.test.ts` reference as evidence, so the SPEC-08 T2 row is ✅
  on them. The job's display name is unchanged (why: the note on the
  `backend` job in `ci.yml`).
- P1 #17a **SPEC-08 residue** — closed 2026-09-19. The manual run against the
  stack: every step passed, one focus defect found and fixed
  ([SPEC-08 § Manual run](specs/SPEC-08-journal-attachments.md#manual-run-against-the-stack-2026-09-19)).
  The `0044`/`0045` backfill loops:
  `modules/journal/backfill_test.go: TestBackfillMigrationsMoveLinksOutOfBodies`
  builds a throwaway database from the migration files, seeds old-style
  bodies and runs up → down → up plus the RAISE case, on the same
  `RLS_TEST_ADMIN_URL` CI already provides. Its first run found that
  `0044 down` could not complete on a row the up had emptied (a link-only
  body whose Asset was gone) — the restored CHECK is now `NOT VALID`.
- P2 #18 **journal photo attachments** (SPEC-07 P1.5) — closed 2026-09-19 by
  [SPEC-08](specs/SPEC-08-journal-attachments.md), executed as tickets
  #9–#15 on `feat/journal-attachments` (`d8b2910`, `b6d8a89`, `1e1f12a`,
  `ebe97ca`, `4215701`, `aba8785`): up to ten Attachments in `asset_ids`
  validated as a whole, the Location in three columns, both backfilled out of
  every body by self-asserting migrations `0044`/`0045` (applied to the live
  database), the asset-deleted consumer stripping the id in the owner's
  scope, one composer for create and in-place edit, and
  `frontend/src/lib/attachments.ts` deleted. Evidence: the SPEC-08 section of
  the [traceability matrix](../reference/TRACEABILITY-MATRIX.md). Tracker #8
  and #9–#14 closed 2026-09-19; #15 closes when CI confirms the four docs
  checks on the close-out commit. Residue has owners: the frontend vitest
  files run in CI since the same day (line 10, closed); the manual run was done 2026-09-19
  ([SPEC-08 § Manual run](specs/SPEC-08-journal-attachments.md#manual-run-against-the-stack-2026-09-19))
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
  this deployment ran as `portal_app` from 2026-08-25 through today's SPEC-13
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
- §3.3 SPEC-04 P1.1 `PATCH /assets/{id}` — mounted (visibility only, 0032); metadata edit itself is still open, folded into the matrix's P1.1 ⚠.
- §3.4 **music** is no longer half-built (0038–0041); movie/story still are
  (line 28).
- §4.2 **`events.md` wrong** — re-derived 2026-08-25.
- §4.3 **`.env.example` points at dead hosts** — hosts fixed
  (`host.docker.internal`); the *role* default is P0 line 1.
- §4.4 **refresh TTL 30 d vs 24 h** — CLAUDE.md and ADR-06 say 24h.
- §4.6 **MILESTONE_CHECKS cited as live** (all sites), **`/auth/callback`
  drift claim**, **ADR status lines disagree**, **`backlog.md` inverted**,
  **`facebook-comparison.md` unlabelled** (the file is since deleted), **TRACEABILITY-MATRIX names zero
  tests**, **a session note committed with a credential** (file deleted;
  rotation is P0 line 2) — all closed by the ADR-11 work of 2026-09-11.
- §6 Tier A-3 **`internal/platform/server`** — exists.
- §6 Tier C-10 **stale module READMEs** — *not* closed; line 35.
- §6 Tier C-12 **retire the old backlog / label facebook-comparison** — this file
  (`facebook-comparison.md` itself was later deleted — `git show ea100d8:docs/product/analysis/facebook-comparison.md`).
