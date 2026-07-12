# Spec-gap fix worklog — 2026-07-11

**Source:** `spec-gap-review-B` workflow run `wf_0445d142-18c` (29 agents, adversarially verified). 77 confirmed findings (1 critical · 31 major · 45 minor).

## How to resume (read this first if continuing after a token-out)

- This file is the single source of truth for fix progress. Each finding has a stable ID `Fnnn` and a checkbox.
- `- [ ]` = not started · `- [x]` = fixed · `- [~]` = partial/deferred (see the **Applied** note).
- Every finding carries its own **Fix** text, so you can apply it without re-reading the run output.
- **Work top-to-bottom** — findings are ordered by priority: 🔴 critical → 🟠 systemic (MULTIPLE) → 🟠 per-spec major → 🟡 minor.
- Fix **one spec file at a time**: apply all of that file's findings, then tick their boxes and fill **Applied** in the same edit, so progress survives a crash.
- After ticking, update the **Progress** counter just below.
- Raw evidence/verifier reasoning archive (if a finding is unclear): task output `wc6ljaat2` / journal `wf_0445d142-18c/journal.jsonl`.

## Progress

`[ updated 2026-07-11 ]`  **Fixed: 77 / 77**  ·  critical 1/1 · major 31/31 · minor 45/45

---

## 🔴 Critical


### SPEC-02-comic-vertical.md

#### - [x] F001 · 🔴 CRITICAL · `SPEC-02-comic-vertical.md` · §5 P0.2 Publish flow + RBAC + §7 API summary
*Category:* rbac-enforcement-contradiction  
**Problem:** Owner-scoped comic write endpoints are specified as 'all via RequirePermission' with `comics:write:own`, which neither checks ownership nor admits the seeded `comics:write:any`/`comics:delete:any` moderation grants — so any grant-holder can edit any creator's comic while admin moderation is impossible; there is also no elevated publish/unpublish code.  
**Evidence:** SPEC-02:81 'Permissions (all via `RequirePermission`; wildcard `comics:*` covers every scope)'; :88-93 maps every mutation to `comics:write:own`/`comics:publish:own`; :95-99 seeds `comics:delete:any`+`comics:write:any` → admin-tier for moderation, yet no §7 endpoint (§7:309-315) requires either; AC :110-111 tests an admin holding `comics:*`, a grant nothing seeds. Under backend/internal/modules/account/rbac/permission.go:134-147 a `:any` grant never satisfies a required `:own` (and permission.go:111 `:own` grant satisfies `:own` ONLY), so RequirePermission(comics:write:own) passes any grant-holder regardless of ownership and rejects the admin's `:any` codes — the seeded moderation rows gate nothing. Canonical pattern exists: account/middleware/rbac.go:48-60 RequireOwnerOrPermission.  
**Fix:** Rework P0.2 and §7 to owner-or-elevated: reads via RequirePermission; POST /comics and GET /comics/mine stay `comics:write:own`. Every mutation on an existing comic/chapter/page becomes 'owner, or `comics:write:any`' via `RequireOwnerOrPermission(engine, "comics:write:any", extractComicOwner)`; DELETE /comics/{id} uses `comics:delete:any`; `{status}` changes use a new elevated `comics:publish:any`. Seed: `comics:read`→user; `comics:write:own`+`comics:publish:own`→creator; `comics:write:any`+`comics:publish:any`→editor; `comics:delete:any`→admin (movies precedent). Replace the AC with two: 'creator D (holds comics:write:own) editing creator C's comic → 404/403, no change' and 'editor with comics:write:any editing another creator's comic → 200; admin with comics:delete:any → DELETE passes'.  
**Applied:** ✅ [SPEC-02] owner-or-elevated RBAC + ACs replaced

## 🟠 Major


### MULTIPLE

#### - [x] F002 · 🟠 MAJOR · `MULTIPLE` · SPEC-01 P0.3 / SPEC-04 P2 / SPEC-08 P0.4 / SPEC-09 P0.2
*Category:* missing-infrastructure-owner  
**Problem:** Three+ P0/P2 features ride 'the shared periodic runner (SPEC-01 P0.3 convention)', but SPEC-01 P0.3 only describes its own hourly janitor and never stands up a reusable Asynq scheduler — no asynq.Scheduler/PeriodicTaskManager exists in the repo, so the runner is unspecified and unowned; SPEC-09 also markets itself dependency-free while requiring it.  
**Evidence:** SPEC-08:130, SPEC-09:72, SPEC-04:159 all cite 'SPEC-01 P0.3's convention — no OS cron'. SPEC-01:184-186 only says 'an Asynq periodic task, hourly'; §9 (357) bundles it into 'DELETE + janitor (1 day)'. Repo: grep 'Scheduler|PeriodicTaskManager|asynq.NewScheduler' over backend/ returns nothing; cmd/worker/main.go:71-75 runs only asynq.Server. account's PurgeExpiredRefreshTokens (query/auth.sql:96) is committed-but-unscheduled. SPEC-09:4/22-24 'Depends on: nothing hard / dependency-free' vs :72 needs the runner; SPEC-04:159 confirms SPEC-01 P0.3 'introduces' it.  
**Fix:** In SPEC-01 P0.3 add an explicit deliverable that stands up the shared Asynq Scheduler in cmd/worker as the single registration point for all periodic tasks (media:purge_orphans first, plus future ops:backup_database, people:scan_birthdays, notify:purge_old, account's PurgeExpiredRefreshTokens), and add it as a distinct §9 timeline line. In SPEC-09 P0.2 add: 'If this lands before SPEC-01 P0.3, ops introduces the shared periodic scheduler itself — what is borrowed is the convention (Asynq periodic, never OS cron), not code.'  
**Applied:** ✅ [SPEC-09] scheduler fallback + header softened; [SPEC-01] shared Asynq Scheduler + §9 line

#### - [x] F003 · 🟠 MAJOR · `MULTIPLE` · SPEC-02 P0.2 vs SPEC-05 §7 / SPEC-06 P0.2 / SPEC-08 P0.2; README AuthZ seeding
*Category:* rbac-seeding-inconsistency  
**Problem:** The spec set never fixes what role the single owner holds, yet splits primary write grants inconsistently: SPEC-05/06/08 seed to base `user`, SPEC-02 seeds comic authoring to `creator`, and the shared `assets:write:own` needed by media-attachment features in 05/08/03 is `creator`-tier in 0003 — so a `user`-provisioned owner can journal but cannot upload the photos/avatars/receipts those specs promise.  
**Evidence:** 0003_account_rbac.up.sql seeds ('user','assets:read:own') but ('creator','assets:write:own')/('creator','assets:delete:own'). SPEC-05:214, SPEC-08:68, SPEC-06:127 all grant to base user. SPEC-02:95-99 grants comics:write:own+publish:own → creator. SPEC-05:155 (P1.5) and SPEC-08:193 (P1.7) require uploaded assets via mediaapi (need assets:write:own). No spec states the owner's role.  
**Fix:** Add a README AuthZ line fixing the owner-account role (e.g. 'the single v1 owner is provisioned creator or higher; grants to user are the floor'), and either (a) reconcile SPEC-05/06/08's 'base user role' grants against SPEC-02's creator-tier and the creator-tier assets:write:own they depend on, or (b) grant assets:write:own to user if the life-OS write features must work for a plain user.  
**⚠ Fix-audit says the proposed fix needs revision → use this instead:** Add one README AuthZ bullet: 'The single v1 owner account is provisioned at the `creator` role (or higher). Because the role hierarchy has creator inherit user, this makes the module permissions each spec seeds to `user` (journal/people/bank/stream :own) available AND provides the media catalog's creator-tier `assets:write:own`/`assets:delete:own` that the photo (SPEC-05 P1.5), avatar (SPEC-08 P1.7) and receipt (SPEC-03 P1.10) upload steps depend on. Grants to `user` are the floor, reached by inheritance.' Do NOT re-grant `assets:write:own` to the base `user` role — that would erase 0003's intended creator-tier upload boundary.  
**Applied:** ✅ [SPEC-08] creator-tier assets:write:own note; [SPEC-05] added creator-tier assets:write:own note; [README] added owner-role line

#### - [x] F004 · 🟠 MAJOR · `MULTIPLE` · SPEC-03 P0.7 / SPEC-06 P0.1(b)+P0.2 / events.md
*Category:* event-payload-contract  
**Problem:** The single collapsed transfer stream item stores whichever leg's event is processed first (ON CONFLICT DO NOTHING), so its rendered direction/account is nondeterministic — and the payload carries no counterparty account, making SPEC-03's declared card copy ('moved 5M TCB→Momo') unrenderable from the stored payload.  
**Evidence:** SPEC-03:278-280 'transfer_id ... single story item ("moved 5M TCB→Momo")'; P0.7 emits per leg with payload {..., account_id, direction, ..., transfer_id} (275-276, events.md:24) — one leg's account_id/direction only. SPEC-06:86 collapses onto ref_id=transfer_id, :92 'ON CONFLICT ... DO NOTHING' (first-arriving leg wins), :143 renders from stored payload; bank:transaction_updated (:87) last-writer-wins.  
**Fix:** In SPEC-03 P0.7 add counterparty_account_id (nullable; on a transfer leg = the other leg's account_id) to the created/updated/deleted payload and mirror in events.md. In SPEC-06 P0.2 add a transfer render rule: 'for is_transfer payloads, normalize on direction — source = debit side (account_id when direction=\'debit\', else counterparty_account_id) — so either leg's payload renders the identical 'moved <amount> <source>→<dest>' card.'  
**Applied:** ✅ [SPEC-03] counterparty_account_id payload+events.md; [SPEC-06] transfer render normalization rule

#### - [x] F005 · 🟠 MAJOR · `MULTIPLE` · SPEC-03 §8 / SPEC-08 P0.5 (new authenticated routes)
*Category:* auth-gating  
**Problem:** New authenticated routes /bank/* and /people are not added to the middleware auth-gate matcher, so the D-34 login gate never runs on them — an unauthenticated visitor is not redirected to /login.  
**Evidence:** frontend/src/middleware.ts:39 matcher: ['/','/login','/register','/upload','/library/:path*']. SPEC-03 §8 adds /bank, /bank/transactions, /bank/accounts, /bank/budgets (:496-505); SPEC-08 P0.5 adds /people + person detail (:179) — neither matched. Neither spec mentions updating the matcher.  
**Fix:** Add to SPEC-03 §8 and SPEC-08 P0.5 an explicit step to extend config.matcher in src/middleware.ts with '/bank/:path*' and '/people/:path*' respectively, so the session gate covers the new routes.  
**Applied:** ✅ [SPEC-03] /bank/:path* matcher; [SPEC-08] /people/:path* matcher step

#### - [x] F006 · 🟠 MAJOR · `MULTIPLE` · SPEC-08 P0.5 / SPEC-03 §8 / SPEC-01 route / SPEC-02 P0.3-P0.5
*Category:* convention-compliance  
**Problem:** Specs add pages/routes without acknowledging the version-switched template manifest + registry architecture all presentation must go through — an implementer following the README has no instruction on where the view lives or how the route resolves, and could bypass the registry (breaking the v2 switch).  
**Evidence:** frontend/src/templates/README.md: 'app/ is routing only ... call activeTemplate()'; every page view is declared in TemplateManifest.views (templates/types.ts:32-43). Yet SPEC-08 P0.5 (:179), SPEC-03 §8 (:498), SPEC-01 (/library/media, :206), SPEC-02 P0.3/P0.5 add pages with no mention of extending TemplateManifest.views or the registry tree.  
**Fix:** In each affected frontend section state that new page views are added to TemplateManifest.views in templates/types.ts, implemented under templates/v1/views/..., and that app/(app)/<route>/page.tsx resolves them via activeTemplate().views.<x> — no version-specific import in app/.  
**⚠ Verify-downgraded (real but overstated):** The factual core is correct: TemplateManifest.views (types.ts) currently lists only home/login/register/libraryComic/libraryNovelDetail, and SPEC-08 P0.5, SPEC-03 §8, SPEC-01 P0.4, SPEC-02, SPEC-05 add page routes without mentioning the manifest/registry. But 'major (implementer would stall/diverge)' is overstated: the versioned-template contract is already a binding, documented convention in frontend/src/templates/README.md AND project CLAUDE.md ('app/ is routing only … read templates/README.md before adding a page'), and the specs explicitly invoke the frontend conventions (D-33/D-32, frontend.md §8). An implementer following those wouldn't stall or bypass the registry; not restating a global convention in each spec is a minor omission, not a blocker. Severity should be minor.  
**Applied:** ✅ [SPEC-02] template-registry note; [SPEC-03] template registry note; [SPEC-08] template-registry note; [SPEC-01] template-registry note

#### - [x] F007 · 🟠 MAJOR · `MULTIPLE` · events.md 'Delivery mechanics' / SPEC-01 P1.2 / SPEC-03 P0.7 / SPEC-04 P0.4 / SPEC-05 P0.3 / SPEC-06 P0.1(b) / SPEC-07 P1.5 / SPEC-08 P0.4
*Category:* missing-shared-infrastructure  
**Problem:** The `platform/events` fan-out helper that seven specs treat as pre-existing does not exist and no spec has a work item to build it — yet the multi-consumer stream/notify architecture depends on it, and events.md warns skipping it panics cmd/worker.  
**Evidence:** events.md:71-73 describes Publish/subscription table as extant; :66-68 states naive subscribe-by-task-type 'panics cmd/worker at startup or starves one consumer' once an event gains a second consumer. Emitters reference it as done (e.g. SPEC-01:256-263). Filesystem: backend/internal/platform/ has audit, config, middleware, storage — no events/. The first emitter (SPEC-01 P1.2) is a P1 'nice to have' and only names the helper.  
**Fix:** Add an explicit P0 work item that owns constructing platform/events (Publish(ctx,name,payload) + the event-name→consumer-task subscription table registered in cmd/worker), gate the multi-consumer events on it, and assign it to SPEC-01 (first producer, earliest in build order) or add a conventions-README bullet declaring it a prerequisite of the first spec to land.  
**Applied:** ✅ [README] platform/events prerequisite bullet, SPEC-01 owns; [SPEC-01] P0.6 platform/events work item


### README.md

#### - [x] F008 · 🟠 MAJOR · `README.md` · Suggested implementation order (specs README) / Build order (briefs README)
*Category:* sequencing  
**Problem:** The suggested order places SPEC-09 (backups) after SPEC-03 (finance ledger), contradicting the constraint — asserted in the same note and in SPEC-09's own header — that SPEC-09 P0 must land BEFORE SPEC-03 accrues irreplaceable ledger data.  
**Evidence:** specs/README.md:81-83 numbers SPEC-03 step 4, SPEC-09 step 5 while step 5's parenthetical says 'SPEC-09 P0 should land before SPEC-03 data accrues'; briefs/README.md:34 mirrors it, briefs/README.md:30 row 09 'Land before SPEC-03 data accrues'; SPEC-09:1 §1 'This spec's P0 should land before that data exists'.  
**Fix:** Reorder both READMEs so SPEC-09 P0 precedes SPEC-03. In specs/README.md: '4. SPEC-02 and SPEC-09 P0 (backups — must land before any SPEC-03 ledger data) 5. SPEC-03 (parallelizable) / SPEC-07 (burst-filler); SPEC-09 P1 here or later'. Update briefs/README.md:34 accordingly.  
**⚠ Verify-downgraded (real but overstated):** The surface tension is real: the numbered 'Suggested implementation order' lists SPEC-03 at step 4 (README:81) and SPEC-09 at step 5 (README:82-83), yet both line 24 ('land P0 before SPEC-03 data accrues') and the step-5 parenthetical ('SPEC-09 P0 should land before SPEC-03 data accrues') assert SPEC-09 P0 must precede SPEC-03's data. However, the finding overstates this as a major, divergence-causing contradiction. (a) The inline caveat is literally in the same numbered step, so an implementer is explicitly warned — they cannot follow the order 'as written' without also reading the constraint that resolves it. (b) 'SPEC-03 build' (step 4, code merged) is not the same event as 'SPEC-03 data accrues' (runtime, once users enter finance transactions in a deployed instance); the note is precisely calibrated to distinguish them, so it is not internally contradictory. What survives: the numbered sequence is mildly confusing and would read cleaner if SPEC-09 P0 were hoisted next to/ahead of SPEC-03, but this is a clarity/tidiness issue, not a major sequencing defect that causes a wrong implementation. Severity minor, not major.  
**Applied:** ✅ [README] reordered steps 4-5: SPEC-09 P0 before SPEC-03


### SPEC-01-media-image-pipeline.md

#### - [x] F009 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · P1.2 / §6 data model
*Category:* event-payload-persistence  
**Problem:** The `origin` field of `media:asset_ready` — the load-bearing zip-import flood guard for SPEC-04's bell and SPEC-06's stream — has no persistence mechanism: no column stores it and the worker task payload ({asset_id}) cannot carry it, so the emitter cannot know the value.  
**Evidence:** SPEC-01:260-262 requires origin='upload'/'import', event fires worker-side at end of media:process_image whose payload is only {asset_id} (:93, events.md:39). §6 migration (308-315) adds only title and original_filename — no origin column. SPEC-02:212-214 says assets are 'marked origin=\'import\'' without saying where. Consumers key on it: SPEC-04:124, SPEC-06:83, events.md:20.  
**Fix:** In SPEC-01 §6 extend the ALTER TABLE with `ADD COLUMN origin text NOT NULL DEFAULT 'upload' CHECK (origin IN ('upload','import'))`. Add to P1.2: 'origin is read from assets.origin, set at row creation — 'upload' by the upload-session endpoint, 'import' by the mediaapi batch-create SPEC-02 P1.7 uses.' State that mediaapi asset listings expose origin (SPEC-06 P1.6 backfill needs it).  
**Applied:** ✅ [SPEC-01] origin column + P1.2 persistence

#### - [x] F010 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1 — HEIC/unsupported-format rejection at /complete
*Category:* undefined-behavior  
**Problem:** The HEIC/unsupported-format rejection at /complete defines only the 422; the fate of the asset row and the already-uploaded object is unspecified — as written the asset stays `uploading` forever and (per the filter rule) shows under `processing` indefinitely.  
**Evidence:** SPEC-01:88-89 HEIC 'rejected with a convert-on-device hint' and AC :136-138 '422 media/unsupported-format' — no status transition or cleanup, unlike the too-large path (:91-93 'object deleted, asset failed'). P0.4 (210-212) 'the processing filter includes still-uploading sessions'.  
**Fix:** Add to P0.1: 'On unsupported-format detection at /complete: return 422 media/unsupported-format, delete the uploaded object, and set status=failed with an error_message naming the detected format (HEIC gets the convert-to-JPEG hint) — mirroring the file-too-large handling.' Add a matching HEIC AC clause.  
**Applied:** ✅ [SPEC-01] unsupported-format delete+failed, HEIC AC

#### - [x] F011 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.1/P0.3/P0.4/P0.5, §7 API summary
*Category:* missing-api-surface  
**Problem:** The spec requires image variants (thumb/medium/poster) to be rendered, URL-referenced, and 404-checked, but never defines how a variant is served — no endpoint, URL scheme, or auth stance exists in §5 or §7.  
**Evidence:** P0.3 AC (194-195) 'its HLS URL, variant URLs, and original-download URL return 404/403'; P0.5 (233-235) 'the original must never be reachable through the public-ish variant/HLS URL scheme'; P0.4 AC (222) 'thumb variants only'. §7 (327-332) lists only DELETE /assets/{id}, GET /assets/{id}/original, PATCH /assets/{id}, GET /assets. Shipped code serves only HLS: media/module.go:68 `r.Get("/{id}/hls/*", ...)`.  
**Fix:** Add GET /api/v1/assets/{id}/variants/{variant} (variant ∈ thumb|medium|poster) to §5 P0.1 and §7 — streamed with the variant content type and cache headers, the same 'public-ish' auth stance as /hls/* (state explicitly whether unauthenticated or requires assets:read:own), returning 404 media/asset-not-found for missing/deleted assets, so it lands in shared/openapi.yaml.  
**Applied:** ✅ [SPEC-01] variants GET endpoint

#### - [x] F012 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 Delete asset + §7 API summary
*Category:* rbac-enforcement-contradiction  
**Problem:** DELETE /assets/{id} is specced as RequirePermission(`assets:delete:own`) while claiming `assets:delete:any` covers admin — but the matcher never lets an `:any` grant satisfy an `:own` requirement, so admin's 0003 grant is inert and admin/moderation delete is impossible as written.  
**Evidence:** SPEC-01:169-172 'assets:delete:own (assets:delete:any covers admin)' and §7:329; P0.3 behavior :176 '(1) authorize ownership'. Conflicts with rbac/permission.go:134-147 (`:any` matches required '' or 'any' only, never 'own') and the codebase's canonical pattern engine.go:61-62 / middleware/rbac.go:48-60 RequireOwnerOrPermission. AC :197-198 ('without wildcard perms, then 403') implies elevated perms should pass.  
**Fix:** In §7 change the DELETE Permission cell to 'owner, or assets:delete:any (RequireOwnerOrPermission)'. In P0.3 replace the parenthetical: enforce via RequireOwnerOrPermission(engine, "assets:delete:any", extractAssetOwner) — a plain RequirePermission("assets:delete:own") would 403 admin's 0003-seeded assets:delete:any because the matcher never lets :any satisfy :own; assets:delete:own remains the catalog entry documenting the owner capability, and the middleware's owner branch admits owners.  
**Applied:** ✅ [SPEC-01] RequireOwnerOrPermission DELETE

#### - [x] F013 · 🟠 MAJOR · `SPEC-01-media-image-pipeline.md` · §5 P0.5 acceptance criteria vs §5 P0.1
*Category:* internal-contradiction  
**Problem:** P0.5's AC guarantees the original of any `failed` asset is downloadable, but P0.1's file-too-large path deletes the source object while setting the asset `failed` — for those assets the guaranteed download is impossible.  
**Evidence:** P0.5 AC (245-247) 'Given an asset in processing/failed, the original is still downloadable ... media/asset-not-found only for missing/deleted assets' vs P0.1 (91-93) 'on violation: object deleted, asset failed' and P0.1 AC (144-146).  
**Fix:** Amend the P0.5 AC to: 'Given an asset in processing/failed whose source object still exists (worker-side failure: corrupt/animated/oversized-dimensions), the original is still downloadable. Given an asset rejected at /complete with its object purged (file-too-large, unsupported-format), then 404 media/asset-not-found — the archival guarantee applies only to accepted uploads.'  
**Applied:** ✅ [SPEC-01] AC split by source-object existence


### SPEC-02-comic-vertical.md

#### - [x] F014 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §5 P0.5 Library + detail pages
*Category:* acceptance-criteria-gap  
**Problem:** P0.5 is the only P0 requirement with no acceptance criteria — no testable definition of done for the library grid, My-comics tab, pagination, or placeholder replacement.  
**Evidence:** SPEC-02:165-171 (P0.5) ends at the description; every other P0 section (P0.1:66, P0.2:105, P0.3:123, P0.4:147, P0.6:194) has an 'Acceptance criteria' block.  
**Fix:** Add to P0.5: 'Acceptance criteria. — Given a published and a draft comic by another creator, a reader opening /library/comic sees only the published one (cover=thumb variant, title, chapter count, updated date). — The creator's My comics tab lists both with status badges; a reader never sees drafts. — >1 page paginates without duplicates. — The pre-existing placeholder component is gone from the route. — Detail page shows Continue iff progress exists (P0.4).'  
**⚠ Verify-downgraded (real but overstated):** The factual claim is correct: P0.5 (:165-171) ends at its description with no 'Acceptance criteria.' block, while P0.1 (:66), P0.2 (:105), P0.3 (:123), P0.4 (:147), P0.6 (:194) each have one. So the omission is real. But major overstates impact: P0.5's prose concretely enumerates the grid fields (cover thumb, title, chapter count, updated date), the My-comics tab with status badges + Create, pagination, placeholder replacement, and the detail-page contents — an implementer would not stall or materially diverge. The two behaviors most at risk (reader must not see drafts; Continue shown iff progress exists) are already testable via P0.2 AC :108 and P0.4. Downgrade to minor: a completeness gap (missing dedicated AC block), not a build-blocking ambiguity.  
**Applied:** ✅ [SPEC-02] P0.5 ACs added

#### - [x] F015 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §5 P1.7 / §7 pages:import-zip
*Category:* ambiguous-requirement  
**Problem:** The spec never defines how the up-to-500 MB zip reaches the worker: the only ingest path is SPEC-01's presigned PUT capped at 50 MB, and events.md's task payload references an `upload_ref` the spec never defines.  
**Evidence:** SPEC-02:207 'POST .../pages:import-zip → worker task comic:import_zip unpacks' and :220-221 'max 500 MB zip' — no transport; events.md:41 payload `{chapter_id, upload_ref}` (upload_ref undefined in SPEC-02); SPEC-01:89-91 enforces the 50 MB content-length-range on the presigned PUT.  
**Fix:** Specify in P1.7: 'The zip is uploaded via a dedicated presigned PUT (separate import prefix, 500 MB content-length-range, no assets row); the client POSTs pages:import-zip {upload_ref: <storage key>}, enqueuing comic:import_zip {chapter_id, upload_ref} (events.md). The worker streams the object and deletes it after processing.' Add the request body to the §7 Notes.  
**Applied:** ✅ [SPEC-02] zip presigned-PUT path + §7 body

#### - [x] F016 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §5 P1.7 Zip chapter upload
*Category:* internal-contradiction / impossible-requirement  
**Problem:** The import task's ~10-minute poll timeout is arithmetically incompatible with the 300-entry ceiling given SPEC-01's heavy-queue concurrency 1–2 and <10 s/image — a maximum legal zip systematically times out and reports failures for assets that would have reached ready.  
**Evidence:** SPEC-02:218-219 'polls status ... bounded timeout ≈ 10 min' and :221 'max 300 entries'; SPEC-01:122 heavy server 'Concurrency: 1–2' and §2 goal 1 'ready ... in < 10 s' — 300×10 s ÷ 1 ≈ 50 min ≫ 10 min. SPEC-02:222-223 treats poll timeouts as per-file failures.  
**Fix:** Replace 'bounded timeout ≈ 10 min' with 'bounded timeout = max(10 min, entry_count × 15 s ÷ heavy-queue concurrency) — sized so a full 300-entry import at concurrency 1 (SPEC-01 P0.1) cannot time out while its assets are still queued; only assets individually stuck > that window are reported as failures.'  
**Applied:** ✅ [SPEC-02] entry-scaled poll timeout

#### - [x] F017 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §6 Data model — migration 000N_comic_core
*Category:* cross-spec-collision / on-delete-semantics  
**Problem:** SPEC-02 alone omits the sanctioned identity-anchor users(id) FK that every other new-module spec applies, so deleting a user orphans comics/chapters/pages/progress forever — contradicting the README convention and SPEC-03's stated rationale.  
**Evidence:** SPEC-02:239 'owner_user_id uuid NOT NULL, -- account module id, no cross-module FK' and :268 'user_id uuid NOT NULL' carry no FK. specs/README.md:38-40 (identity-anchor FK to users(id), matching 0007_media_assets); peers apply it: SPEC-03:381 (with reason 377-378 'a deleted user orphans finance rows forever'), SPEC-04:168, SPEC-05:178, SPEC-06:215, SPEC-07:152, SPEC-08:211.  
**Fix:** Change comics.owner_user_id to `owner_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE, -- identity-anchor exception (0007_media_assets precedent)` and comic_reading_progress.user_id to `user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE`, updating the trailing comment.  
**Applied:** ✅ [SPEC-02] identity-anchor users(id) FKs

#### - [x] F018 · 🟠 MAJOR · `SPEC-02-comic-vertical.md` · §7 API summary vs §4/§5 P0.1
*Category:* missing-api-surface  
**Problem:** Page deletion is required by a user story and exercised by a P0.1 AC, but §7 defines no endpoint to delete (or update) a page — path, method, and permission are left to the implementer.  
**Evidence:** SPEC-02:47-48 user story 'I reorder or remove pages before publishing'; :74-75 AC 'when 20 is deleted, then reader order is stable'; §7 (303-318) contains only POST /chapters/{id}/pages, PUT pages:order, GET pages — no DELETE.  
**Fix:** Add to §7: `DELETE | /api/v1/pages/{id} | owner + comics:write:own (moderation: comics:delete:any) | removes the page row only; the media asset is untouched (P0.1)` and reference it in P0.1's prose.  
**Applied:** ✅ [SPEC-02] DELETE /pages/{id} in §7


### SPEC-03-finance-ledger.md

#### - [x] F019 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · P0.4 Categories / §7 API
*Category:* unimplementable-ambiguous  
**Problem:** The claimed 2-level enforcement ('parent_id must reference a top-level category') is insufficient because PATCH /bank/categories/{id} exists and parent_id mutability is undefined — re-parenting a category that already has children passes the check yet creates a 3-level tree, silently breaking P0.5's direct-children budget roll-up.  
**Evidence:** SPEC-03:172-174 'parent_id must reference a top-level category (parent_id IS NULL — this is what enforces 2 levels max)' vs :460 'PATCH/DELETE /api/v1/bank/categories/{id}'; only kind is declared immutable (:178). P0.5 (:214 'including its children', :179 'Children always means direct children') then under-counts grandchild spend.  
**Fix:** Add to P0.4 invariants: 'parent_id is mutable via PATCH, but a category that has children cannot be assigned a parent (422 bank/invalid-category-parent) — together with the top-level rule this enforces 2 levels max; re-parenting re-runs the same-kind and own-or-seed checks.' (Or declare parent_id immutable after creation alongside kind.)  
**Applied:** ✅ [SPEC-03] parent_id mutability invariant

#### - [x] F020 · 🟠 MAJOR · `SPEC-03-finance-ledger.md` · §7 API summary / §8 Frontend
*Category:* pagination-divergence  
**Problem:** The transactions list uses offset `?page=` pagination, contradicting §8's own 'infinite list' description and the cursor convention SPEC-01 declares all newer specs use; offset paging duplicates/skips rows under inserts and occurred_at edits.  
**Evidence:** §7 (SPEC-03:455) 'GET/POST /api/v1/bank/transactions?account=&month=&category=&page=' vs §8 (:505) 'filterable infinite list'. Cursor peers: SPEC-01:332, SPEC-04:209, SPEC-05:205, SPEC-06:235, SPEC-08:264; SPEC-01 rev3 calls cursor 'the convention all newer specs use'.  
**Fix:** Change line 455 to '...?account=&month=&category=&cursor=' and specify ordering 'occurred_at DESC, id DESC' with a next_cursor response field, matching the cursor convention and §8's infinite-list requirement.  
**Applied:** ✅ [SPEC-03] page->cursor + note


### SPEC-04-notification-module.md

#### - [x] F021 · 🟠 MAJOR · `SPEC-04-notification-module.md` · P0.2 / §6 data model
*Category:* event-redelivery-idempotency  
**Problem:** The notifications store has no idempotency mechanism under at-least-once delivery: an Asynq retry of `notify:dispatch` or SPEC-08's outbox re-publish inserts duplicate bell rows — directly contradicting SPEC-08's AC that consumers see no duplicate item.  
**Evidence:** SPEC-08:151-152 mandates 'at-least-once delivery, with consumers idempotent by notice_id'; its AC (170-172) 'consumers see no duplicate item'; events.md:29 lists notify (SPEC-04) as a consumer. But SPEC-04's notifications DDL (166-175) has no unique/dedup key and the P0.2 handler (:82) specs no redelivery protection (only asynq.SkipRetry for malformed intents, :84). Contrast SPEC-06:92 structural idempotency.  
**Fix:** In §6 add `dedup_key text` (nullable) plus `CREATE UNIQUE INDEX ON notifications (user_id, type, dedup_key) WHERE dedup_key IS NOT NULL`. In P0.2 extend NotificationIntent with optional dedup_key (event-derived natural id, e.g. notice_id, asset_id), have the in-app insert use ON CONFLICT DO NOTHING, and skip channel fan-out when the row already exists. Add AC: 'Given a redelivered dispatch intent with a dedup_key, exactly one notifications row exists.' Document channel sends remain at-least-once.  
**Applied:** ✅ [SPEC-04] dedup_key+unique idx, ON CONFLICT, AC

#### - [x] F022 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.1 seeding + §7 API summary
*Category:* convention-contradiction / permission-naming  
**Problem:** SPEC-04 permission codes use `update`/`create` action verbs (`notifications:update:own`, `notification-prefs:update:own`, `push-subscriptions:create:own`), violating the binding README canon (action ∈ read|write|delete + sparing domain verbs) that SPEC-04 §7 itself cites, and that SPEC-05/08/09 explicitly reconciled away — leaving the catalog with two competing action vocabularies.  
**Evidence:** SPEC-04:56-57, 70-73, 210-213 use update/create; :220 claims it follows 'the canonical scheme in the specs README'. README.md:53 binds 'action ∈ read | write | delete plus sparing domain verbs (publish)'. SPEC-05:210-213 'the earlier journal:create/journal:update:own split added a verb style the catalog doesn't use (reconciled 2026-07-10)'; SPEC-08:66-67; SPEC-09:174-175 'takeout:write:own — canonical verb'. 0003_account_rbac.up.sql:72-119 contains no update/create action.  
**Fix:** Rename throughout §5 P0.1 table + seeding block and §7: notifications:update:own → notifications:write:own; notification-prefs:read/update:own → notification-prefs:read/write:own; push-subscriptions:create:own → push-subscriptions:write:own (keep push-subscriptions:delete:own). Seed the six corrected codes to `user`, mechanics unchanged. Add a one-line 2026-07 reconciliation note mirroring SPEC-05 §7. Do not leave the README claiming the drafts are reconciled while SPEC-04 violates the set.  
**Applied:** ✅ [SPEC-04] update/create->write renamed + note

#### - [x] F023 · 🟠 MAJOR · `SPEC-04-notification-module.md` · §5 P0.4 dependency note vs §9 timeline vs §10
*Category:* unresolved-decision  
**Problem:** P0.4 leaves its only hard dependency as an undecided either/or ('gate on SPEC-01 P1.2 or fold the one-line emit in'), yet §9 phase 4 schedules the consumer unconditionally and §10 doesn't carry the decision — starting phase 4 with SPEC-01 P0-only landed, an implementer must guess.  
**Evidence:** SPEC-04:126 'either gate this item on SPEC-01 P1.2 or fold the one-line emit into this work item'; §9 phase 4 (235) '`media:asset_ready` consumer + wire into cmd/worker' — no conditional; §10 (240-246) no entry. SPEC-01:256-264 confirms P1.2 is 'nice to have'.  
**Fix:** Resolve in P0.4 and mirror in §9 phase 4: 'Decided: P0.4 is not gated. If SPEC-01 P1.2 hasn't landed when phase 4 starts, this item includes the one-line platform/events.Publish("media:asset_ready",…) in media's ready-transition (coordinated with the media owner); the consumer never ships without a producer.'  
**Applied:** ✅ [SPEC-04] decided ungated, mirrored §9


### SPEC-05-journal.md

#### - [x] F024 · 🟠 MAJOR · `SPEC-05-journal.md` · §5 P0.3 — Event emit (acceptance criteria)
*Category:* internal-contradiction / untestable-AC  
**Problem:** The P0.3 AC 'exactly one event is enqueued' contradicts the delivery mechanics it cites: with zero registered subscribers (the spec's own 'emit-only' v1 state), `platform/events` Publish enqueues one task per subscriber, i.e. zero tasks — the AC is unsatisfiable, or an implementer 'satisfies' it by enqueuing the raw `journal:entry_created` name, which no worker consumes (Asynq handler-not-found retry loop).  
**Evidence:** SPEC-05:122-124 AC 'exactly one event is enqueued, after commit' vs :107-108 'published via `platform/events`' and :118-119 'the event above is therefore emit-only'; docs/reference/events.md:74-75 'Publish enqueues one task per subscriber'; events.md:28 lists `journal:entry_created` consumers as '— emit-only for future external consumers'. Zero subscribers → zero enqueued tasks.  
**Fix:** Replace the AC with: 'Given a created entry, then `events.Publish` is called exactly once with `journal:entry_created` and the registered payload, strictly after commit (a rolled-back create publishes nothing). With zero registered subscribers at v1 this enqueues no consumer tasks — assert on the Publish call (spy publisher), not on queue contents, and never enqueue the raw event name as a task type (no handler exists).'  
**⚠ Verify-downgraded (real but overstated):** The tension is real: SPEC-05:123 AC says 'exactly one event is enqueued, after commit', while events.md:71-73 states Publish 'enqueues one task per subscriber' and events.md:28 lists journal:entry_created as '— emit-only for future external consumers' (zero subscribers). Literally, asserting one *task* is enqueued would fail (zero tasks). So the AC wording is imprecise. But 'major' is overstated: (a) SPEC-05:107 explicitly says the event is 'published via platform/events', so the feared 'enqueue the raw journal:entry_created name → handler-not-found retry loop' misimplementation is directly precluded by the spec, not invited; (b) the AC's intent — one publish call strictly after commit, none on rollback (:124 'a rolled-back create emits nothing') — is clear and testable via a spy publisher, so a competent implementer would not stall. The surviving defect is only the loose verb 'enqueued' implying a queue assertion when zero consumer tasks exist; that is a minor wording correction, not a wrong-implementation-causing contradiction.  
**Applied:** ✅ [SPEC-05] replaced unsatisfiable AC with Publish-call assertion


### SPEC-06-life-stream-home.md

#### - [x] F025 · 🟠 MAJOR · `SPEC-06-life-stream-home.md` · §5 P1.6 (vs P0.1 flood guard and §6)
*Category:* internal-contradiction / consumer-policy  
**Problem:** P1.6 backfill seeds stream_items from ALL existing media assets, bypassing the P0.1 origin='import' zip-import flood guard — and the guard is unimplementable at backfill time because SPEC-01 persists no origin column (origin is computed only at event-emit time); the seeded rows' event_type is also unspecified while claiming idempotency via the same unique constraint.  
**Evidence:** SPEC-06:193-197 'seeds stream_items from existing media assets' upload dates via mediaapi listing ... Idempotent via the same unique constraint' — no import exclusion, contradicting :83 'skip origin=\'import\'' and AC :119. specs/README.md:84-86 sequences SPEC-06 after SPEC-02's zip import (≤300 assets/chapter, events.md:20). SPEC-01:259-261 defines origin only on the event; §6 adds no origin column, so a plain listing has nothing to filter on.  
**Fix:** Extend P1.6: 'Seeded rows use source_module=\'media\', event_type=\'media:asset_ready\', ref_id=asset_id — identical to the live consumer's key, so the unique constraint dedups against real events. Apply the same import exclusion as the live consumer. Since SPEC-01 does not persist origin on assets, the mediaapi listing must expose an import/batch discriminator (coordinate with SPEC-01/SPEC-02); until it does, the backfill is restricted to instances with no comic imports.'  
**Applied:** ✅ [SPEC-06] P1.6 seeded-row key + import exclusion


### SPEC-07-continue-rail.md

#### - [x] F026 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · Header (Depends on) / §5 P0.2, P0.3 / §7
*Category:* wrong-dependency  
**Problem:** The header claims 'Depends on: nothing hard', but three P0 ingredients only exist once SPEC-01 P0 ships: the asset `title` (in the /continue item shape), the `deleting` status (P0.2 404s on it), and the `media/asset-not-found` problem type (§7 marks '(reused)').  
**Evidence:** SPEC-07:4 'Depends on: nothing hard'. Migration 0007_media_assets.up.sql:11-27 has no `title` and its CHECK allows only ('uploading','processing','ready','failed'); `deleting` and `title` are added by SPEC-01 (SPEC-01:311,317). `media/asset-not-found` appears only in SPEC-01:340, not shipped code. Yet SPEC-07 P0.3 (103) requires `title`, P0.2 (76-77) 404s `deleting`, §7 (174) says '(reused)'.  
**Fix:** Either change the header to 'Depends on: SPEC-01 P0 (soft — supplies assets.title, the deleting status, and media/asset-not-found); the beacon/resume leg runs on today's stack', or keep the no-dependency claim and define fallbacks: title falls back to a filename from source_key until SPEC-01 lands, the deleting check is a no-op until the status exists, and §7 changes '(reused)' to '(shared with SPEC-01 — first spec to ship defines it)'.  
**Applied:** ✅ [SPEC-07] kept 'nothing hard'; added fallbacks

#### - [x] F027 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.1 vs P0.4
*Category:* internal-contradiction  
**Problem:** P0.1's NULL-duration rule promises 'resume still works from the saved position', but P0.4 gates resume on 'below 95%' — undefined for NULL-duration assets — so a literal reading of P0.4 starts them at 0, contradicting P0.1.  
**Evidence:** P0.1 (58-64) 'for an asset with duration_ms IS NULL ... resume still works ... but progress_pct is undefined'; P0.4 (122-124) 'starts at the saved position when one exists above the 30 s threshold and below 95% ... otherwise starts at 0'.  
**Fix:** Amend P0.4: 'Vidstack starts at the saved position above the 30 s threshold and below 95% — the percent gate is skipped when progress_pct is undefined (NULL duration_ms, P0.1), in which case any saved position ≥ 30 s resumes, with a 'Start over' affordance; otherwise starts at 0.'  
**Applied:** ✅ [SPEC-07] percent-gate skipped for NULL-duration

#### - [x] F028 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.3 / P0.4
*Category:* frontend-implementability  
**Problem:** Resume UX and the continue-rail item href assume a per-asset video playback route that does not exist and no spec creates.  
**Evidence:** P0.4 (122-124) 'Vidstack starts at the saved position'; item shape carries href (:104) with 'clicking one drops me back in'. The only Vidstack Player is in the upload studio — frontend/src/templates/v1/views/upload/UploadStudio.tsx:154 `<Player src={current.hls_url!} />`, mounted only at /upload. app/(app)/ has page, upload, library/comic, library/novel/[id] only. SPEC-01's /library/media is a 'catalogue shell + client islands' grid (SPEC-01:206) with no detail/player route.  
**Fix:** In SPEC-07 P0.4 name the playback host: require creating a per-asset player route (e.g. app/(app)/library/media/[id]/page.tsx + a template-manifest view mounting Vidstack) OR state the continue href deep-links to an existing route, and specify mediaapi.Continue builds href to that exact path.  
**Applied:** ✅ [SPEC-07] named /library/media/[id] route

#### - [x] F029 · 🟠 MAJOR · `SPEC-07-continue-rail.md` · §5 P0.4 / §7 API summary / §5 P0.3
*Category:* unimplementable-requirement  
**Problem:** There is no read path for the saved position: P0.4/P0.2 require resume-at-position on reopen, but the API exposes only PUT progress and /continue items carrying `progress_pct` (a rounded percentage) with no `position_ms`; a direct asset reopen has no source for the position at all.  
**Evidence:** P0.4 (122-124) 'Vidstack starts at the saved position when one exists'; P0.2 AC (86-88) resume at ~12:34 (±10 s) on reopen. Yet §7 (169-172) lists only PUT /assets/{id}/progress and GET /continue, and the P0.3 item shape (103) is `[{module, ref_id, title, poster_url, progress_pct, href, updated_at}]` — no `position_ms`. feature-inventory D-20 (1224-1232) gave ContinuingItem explicit Position/Duration that this spec dropped; NULL-duration assets are excluded from /continue (P0.1:62) yet 'resume still works' (61-62), so /continue cannot be the read path.  
**Fix:** Add GET /api/v1/assets/{id}/progress (RequireAuth, owner-scoped) returning `{position_ms, progress_pct (null when duration_ms IS NULL), completed_at, updated_at}` with the PUT's 404 semantics. In P0.4, state the player fetches this (or an embedded progress object — pick one) before initializing Vidstack. Optionally add `position_ms` to the P0.3 item shape and note the deliberate divergence from D-20.  
**Applied:** ✅ [SPEC-07] GET progress endpoint; P0.4 fetches first


### SPEC-08-people-registry.md

#### - [x] F030 · 🟠 MAJOR · `SPEC-08-people-registry.md` · §5 P0.2 vs §6 data model
*Category:* internal-contradiction  
**Problem:** P0.2 says `PATCH { birthday: null }` 'clears all four columns', but §6 declares `birth_calendar text NOT NULL DEFAULT 'solar'` — NULLing it violates the schema, so prose and schema diverge.  
**Evidence:** SPEC-08 §5 P0.2 (82-83) 'PATCH { birthday: null } clears all four columns' vs §6 (218) 'birth_calendar text NOT NULL DEFAULT \'solar\' CHECK (birth_calendar IN (\'solar\',\'lunar\'))'.  
**Fix:** In P0.2 replace 'clears all four columns' with: 'NULLs `birth_month`/`birth_day`/`birth_year` and resets `birth_calendar` to its `'solar'` default (the column is NOT NULL per §6)'. Keep §6 unchanged.  
**Applied:** ✅ [SPEC-08] birthday clear matches NOT NULL


### SPEC-09-platform-ops.md

#### - [x] F031 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P0.2 vs §5 P0.4 vs §9
*Category:* internal-contradiction / missing AC  
**Problem:** P0.4 requires P0.2's task to overwrite `backups/pg/LATEST.json {storage_key, sha256, size_bytes, finished_at}` after every run, but P0.2's mechanics, ACs, and §9 timeline all omit the manifest write and sha256 computation — an implementer building P0.2 from its own section ships a backup the restore drill cannot use, and `ops_backup_runs` has no sha256 column.  
**Evidence:** P0.4 step 1 (SPEC-09:115-118) requires LATEST.json overwrite with sha256, but P0.2 mechanics (78-89) list only insert/pg_dump/finish/retention/events, its ACs (90-96) never mention manifest or sha256, and §9 step 2 (262) reads 'Backup task + streaming + retention + events + audit'. The dump is streamed straight to storage (§10 resolved, 276-279) so sha256 must be teed; nothing in P0.2 says so, and §6 (205-214) has no sha256 column.  
**Fix:** In P0.2 insert a step between 3 and 4: 'On success, overwrite `backups/pg/LATEST.json` `{storage_key, sha256, size_bytes, finished_at}` — sha256 computed by teeing the pg_dump stream through a hasher during upload. Retention (step 4) must never delete it or its target.' Add AC: 'Given a successful run, LATEST.json points at the new dump and its sha256 matches the stored object.' Update §9 step 2 to include 'manifest'.  
**Applied:** ✅ [SPEC-09] manifest step 4 + AC + §9

#### - [x] F032 · 🟠 MAJOR · `SPEC-09-platform-ops.md` · §5 P1.7 Owner takeout
*Category:* security-underspecified  
**Problem:** The takeout download URL — pointing at one archive bundling the entire account (bank CSVs, journal, original photos with intact GPS EXIF) — is specified only as 'short-TTL' with no value and no signed-vs-proxied decision, at odds with SPEC-01 P0.5's rule that originals must never be reachable through a public-ish URL.  
**Evidence:** SPEC-09:182-183 'GET /api/v1/me/export/{id} returns status + a short-TTL download URL'; :181 archive includes originals. SPEC-01:230-238 'Owner-authenticated proxy only — the original must never be reachable through the public-ish variant/HLS URL scheme ... retains full EXIF including GPS.'  
**Fix:** Specify the mechanism and TTL: serve the archive through an authenticated owner-scoped proxy (matching SPEC-01 P0.5), or if presigning R2 pin an explicit short TTL (≤5 min) and state the key is single-use/opaque. Add a sentence that the export inherits P0.5's private-archive sensitivity because it embeds original EXIF/GPS photos.  
**Applied:** ✅ [SPEC-09] proxy or <=5min presign + sensitivity

## 🟡 Minor


### MULTIPLE

#### - [x] F033 · 🟡 MINOR · `MULTIPLE` · Document headers — SPEC-02, SPEC-03, SPEC-04
*Category:* header-normalization  
**Problem:** The header/metadata block is not normalized across the set: SPEC-02 and SPEC-03 carry no Status or Drafted line and no revision history, and SPEC-04 lacks a Drafted date and revision history — despite all three carrying numerous dated inline reconciliations, so a reader cannot tell status or change lineage from the header.  
**Evidence:** SPEC-01:3 has 'Status: current, rev 3 ... Last verified' + §11 revision history; SPEC-05..09 open 'Status: ready to build, rev 1 · Drafted: 2026-07-10'. SPEC-02:3 and SPEC-03:3 have neither (yet carry dated corrections, e.g. SPEC-02:82, SPEC-03:220); SPEC-04:2 has Status but no Drafted date or revision history.  
**Fix:** Add the standard header block ('Status: ready to build, rev N · Drafted/Last-verified: …') to SPEC-02 and SPEC-03, a Drafted date to SPEC-04, and a short revision-history section to all three capturing their 2026-07-10 reconciliations.  
**Applied:** ✅ [SPEC-02] header block + revision history; [SPEC-03] header block + rev history; [SPEC-04] Drafted date + rev-history

#### - [x] F034 · 🟡 MINOR · `MULTIPLE` · Header Downstream/Depends-on blocks (SPEC-04, SPEC-01)
*Category:* reciprocity  
**Problem:** SPEC-04 lists `media` as a Downstream consumer, but the dependency runs the other way (notify consumes media's media:asset_ready; SPEC-04 Depends-on cites SPEC-01 P1.2), and SPEC-01's own Downstream-consumers list omits SPEC-04.  
**Evidence:** SPEC-04:5 'Downstream consumers: ... media (media:asset_ready)' while :3 'Depends on: SPEC-01 P1.2' and §2 goal 5 'First event consumer: subscribe to media:asset_ready'. SPEC-01:6 'Downstream consumers: SPEC-02, avatars, SPEC-03 P1' omits SPEC-04.  
**Fix:** In SPEC-04:5 drop 'media (media:asset_ready)' from Downstream consumers (or relabel 'Consumes: media:asset_ready (SPEC-01 P1.2)'). In SPEC-01:6 append '..., SPEC-04 (media:asset_ready → in-app notification)'.  
**Applied:** ✅ [SPEC-04] dropped media from Downstream; [SPEC-01] SPEC-04 appended Downstream

#### - [x] F035 · 🟡 MINOR · `MULTIPLE` · SPEC-01 §7 note; SPEC-03 §5 P0.8; SPEC-04 §7 note
*Category:* incorrect-failure-mode  
**Problem:** Three specs claim a malformed (4-segment) permission code 'would 403 everyone, superadmin included' at runtime; through the canonical enforcement path the actual failure is a server-start panic — RequirePermission parses the required code with rbac.MustParse at middleware build time — so the runtime-403 description is only true for bare Set.AllowsCode checks.  
**Evidence:** SPEC-01:335-337, SPEC-03:300-302, SPEC-04:220 all say 'would 403 everyone, superadmin included'. Contradicted by account/middleware/rbac.go:17-20 'a malformed code panics — surfacing a programmer error before the server starts' (RequirePermission → rbac.MustParse, permission.go:79-85); the 403 belongs to Set.AllowsCode (permission.go:189-197).  
**Fix:** In each note replace the '403 everyone' clause with: 'is rejected by rbac.Parse: wired through RequirePermission it panics at server start (MustParse on the required code), and any dynamic AllowsCode check fails closed — returning false even for a * superadmin grant.'  
**Applied:** ✅ [SPEC-03] panic/fail-closed wording; [SPEC-04] panic-at-start wording; [SPEC-01] panic-at-start wording


### README.md

#### - [x] F036 · 🟡 MINOR · `README.md` · Conventions binding — Errors / definition of done
*Category:* definition-of-done-gap  
**Problem:** The convention declares Problem type URIs to also be i18n keys (D-7) and makes events.md registration a DoD step, but there is no equivalent registry or DoD obligation for the ~50 new Problem types the set mints — so nothing ensures they land as i18n keys.  
**Evidence:** specs/README.md:44-45 'Error type URIs given in each spec are also the i18n keys'; :41-43 makes events registration DoD. Every spec's §7 lists Problem types (SPEC-05:216-217, SPEC-03:466-471, SPEC-09:243) but none states where the i18n keys are registered; there is no Problem-type inventory analogous to events.md.  
**Fix:** Add a DoD bullet to the README Errors convention: 'every new Problem type URI is registered in <the i18n key catalog / docs/reference/problems.md> in the same PR, mirroring the events.md rule' — pointing at the concrete file (frontend i18n catalog, frontend.md §5) so the obligation is actionable.  
**⚠ Verify-downgraded (real but overstated):** The observed asymmetry is real: README:41-43 makes events.md registration an explicit definition-of-done step, while README:44-45 only states that Problem type URIs 'are also the i18n keys' with no corresponding DoD/registration obligation, even though the specs collectively mint many new Problem types (e.g. SPEC-03 §7, SPEC-05 §7, SPEC-09 §7). That accurate core survives. But the finding overstates it by demanding a central 'Problem-type inventory analogous to events.md': the two cases are not parallel. events.md is a cross-module coupling/discovery registry (consumers subscribe by name), whereas the README already defines the i18n key as identical to the type URI ('are also the i18n keys') — i.e. the key is self-derived, so no separate discovery registry is strictly needed. The genuine gap is narrower: no DoD hook ensures the frontend i18n catalog gains a translation string for each new Problem type when a spec's endpoints are built. So it is a small process omission, not a missing-registry defect. Severity minor is right; the 'registry analogous to events.md' framing is the overstated part.  
**⚠ Fix-audit says the proposed fix needs revision → use this instead:** Add a concrete DoD bullet to the README Errors convention pointing at the existing frontend i18n message catalog (the one governed by frontend.md §5), not a nonexistent file: 'Every new Problem type `type` URI is registered as an i18n message key in the frontend i18n catalog (per frontend.md §5) in the same PR that introduces the endpoint — mirroring the events.md registration rule. A type URI with no i18n key is a DoD failure.' If a backend-side inventory is also wanted, state that docs/reference/problems.md must be created as the canonical list and reference it explicitly rather than as an alternative.  
**Applied:** ✅ [README] fix-audit wording, DoD bullet to Errors

#### - [x] F037 · 🟡 MINOR · `README.md` · §Conventions binding — AuthZ bullet
*Category:* wrong-cross-reference  
**Problem:** The claim that the canonical naming 'matches the 0003 seed catalog' is wrong for part of that catalog: 0003 seeds codes whose third segment is not a scope and whose actions fall outside read|write|delete|publish, so an implementer told the catalog matches may copy those shapes.  
**Evidence:** specs/README.md:51-55 'matches the 0003 seed catalog ... action ∈ read|write|delete plus sparing domain verbs; scope ∈ own|any only'. 0003_account_rbac.up.sql:101-119: moderation:flag/hide/ban_user, rbac:role:read/write/assign, system:settings:write — rbac:role:read and system:settings:write put an action in the scope slot; flag/hide/ban_user/assign are actions outside the set.  
**Fix:** Qualify the parenthetical: '(reconciles the drafts; matches rbac/permission.go's examples and the 0003 catalog's domain rows (assets:*, movies:*, ...) — 0003's admin-plane codes (rbac:role:*, system:settings:write, moderation:*) predate this scheme and are grandfathered literal codes; they must not be used as templates for new modules).'  
**Applied:** ✅ [README] qualified parenthetical, grandfathered 0003 admin codes


### SPEC-01-media-image-pipeline.md

#### - [x] F038 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.3 janitor / P0.5
*Category:* missing-lifecycle  
**Problem:** The spec acknowledges abandoned `uploading` sessions but defines no cleanup — the janitor sweeps only status='deleting', so abandoned rows and partial objects accumulate forever, against Goal 3's storage-reclamation intent.  
**Evidence:** SPEC-01:185-187 janitor 'WHERE status=\'deleting\' AND updated_at < now()-15min' — nothing sweeps uploading; :247-249 names the abandoned case; Goal 3 (27-28) 'DB rows and all storage objects are gone afterwards'.  
**Fix:** Extend media:purge_orphans in P0.3: also select 'WHERE status=\'uploading\' AND updated_at < now()-24 hours', marking those failed (error_message 'upload abandoned') or transitioning to deleting for purge — or add an explicit non-goal deferring abandoned-upload cleanup with rationale.  
**Applied:** ✅ [SPEC-01] janitor sweeps abandoned uploads

#### - [x] F039 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §5 P0.5 / §6
*Category:* missing-field-semantics  
**Problem:** `Content-Disposition: attachment; filename="<original filename>"` is undefined for every asset uploaded before the §6 migration — original_filename is null for pre-existing rows and no fallback is specified.  
**Evidence:** SPEC-01:232-233 streams with filename=<original filename>; :319-321 'the shipped table stores neither ... The upload-session endpoint starts recording original_filename' (only new uploads).  
**Fix:** Add to P0.5: 'When original_filename is null (assets predating the migration), fall back to {asset_id}.{ext} where ext is derived from the sniffed content type (or the source_key extension).'  
**Applied:** ✅ [SPEC-01] null original_filename fallback

#### - [x] F040 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §6 Data model (assets ALTER) vs P0.3 janitor
*Category:* missing-index  
**Problem:** P0.3 asserts the hourly janitor performs an 'indexed scan' on status='deleting', but neither 0007 nor SPEC-01's ALTER block adds any index on assets(status)/updated_at, so the scan is a seqscan and the prose claim is false; the P0.4 cursor's id tiebreaker is also unindexed.  
**Evidence:** SPEC-01:184-191 'an indexed scan over a near-empty set' filtering status='deleting'; the only index is 0007_media_assets.up.sql:30 assets_owner_idx(owner_id, created_at DESC); SPEC-01's ALTER (308-315) adds no index; the P0.4 cursor created_at DESC, id DESC (212) is only partly served.  
**Fix:** In the amending migration add: `CREATE INDEX assets_deleting_idx ON assets (updated_at) WHERE status = 'deleting';` and `CREATE INDEX assets_owner_cursor_idx ON assets (owner_id, created_at DESC, id DESC);` (replacing reliance on assets_owner_idx for the library keyset).  
**Applied:** ✅ [SPEC-01] two indexes added

#### - [x] F041 · 🟡 MINOR · `SPEC-01-media-image-pipeline.md` · §7 API summary
*Category:* incomplete-api-contract  
**Problem:** §7 omits the modified POST /api/v1/assets/{id}/complete contract even though P0.1 substantially changes it (magic-byte sniffing, HEAD size re-check, new 422 media/unsupported-format and media/file-too-large) — so the openapi.yaml update has no row for the new responses.  
**Evidence:** §7 (327-332) lists only DELETE/GET-original/PATCH/GET-list; P0.1 (82-93,136-138,143-146) changes /complete. specs/README.md:62-63 'every endpoint added here must land in shared/openapi.yaml'.  
**Fix:** Add a §7 row: 'POST | /api/v1/assets/{id}/complete | assets:write:own | modified: magic-byte sniff + HEAD size re-check; new Problems media/unsupported-format (422), media/file-too-large' (optionally the paired POST /assets create row).  
**⚠ Verify-downgraded (real but overstated):** Partly correct. P0.1 substantially changes POST /assets/{id}/complete (magic-byte sniffing, HEAD size re-check, and two new Problem responses — lines 82-93, 136-138, 143-146) and README line 62-63 requires every endpoint to land in shared/openapi.yaml, yet §7's table (lines 327-332) has no /complete row, so the modified endpoint contract is not captured as a table entry. However, the finding overstates 'the openapi.yaml update has no row for the new responses': §7 line 339 DOES enumerate the new Problem types (media/unsupported-format, media/file-too-large, media/asset-not-found, media/asset-not-ready). What survives is that these Problems are not mapped to the /complete endpoint and /complete has no summary row. Minor stands, but the 'no row for the new responses' framing is inaccurate.  
**Applied:** ✅ [SPEC-01] POST /complete row


### SPEC-02-comic-vertical.md

#### - [x] F042 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · P1.9 / §7 (chapter & comic DELETE)
*Category:* event-lifecycle-asymmetry  
**Problem:** Comic is the only stream producer with no removal event: deleting a chapter or comic leaves a comic:chapter_published stream item whose href 404s forever, while media and bank deletions both remove their stream items.  
**Evidence:** §7 offers DELETE /comics/{id} and PATCH/DELETE /chapters/{id} (310-312), but P1.9 emits only comic:chapter_published (225-226) and events.md:23 lists no comic deletion event. SPEC-06:85 treats dangling media cards as a bug and removes bank items on delete (:88); SPEC-06 §3 forbids editing/deleting system items in-stream (46-47).  
**Fix:** Add to P1.9: emit comic:chapter_deleted {comic_id, chapter_id, owner_user_id} on chapter delete and on comic delete (one per chapter), register it in events.md with consumer 'stream — delete the (source_module=\'comic\', ref_id=chapter_id) item (SPEC-06 P0.1)', and add the handler row to SPEC-06's P0.1 table; or, if dangling history cards are acceptable, say so in SPEC-06 §3 and require the render mapping to degrade a dead href gracefully.  
**Applied:** ✅ [SPEC-02] comic:chapter_deleted emit

#### - [x] F043 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · §5 P1.6 / P1.8 vs §6/§7
*Category:* missing-data-model/api  
**Problem:** P1.6's server-side reader-mode preference and P1.8's bookmarks have no storage (no table/column in §6) and no endpoint (absent from §7) — both unbuildable as written.  
**Evidence:** SPEC-02:205-207 'mode persisted per user — server-side pref' and :224 'P1.8 Bookmarks: per user, per page'; §6 (236-275) defines only comics/comic_chapters/comic_pages/comic_reading_progress; §7 (303-318) has no pref or bookmark endpoint.  
**Fix:** Add the surfaces — §6: comic_bookmarks(user_id, page_id fk ON DELETE CASCADE, created_at, pk(user_id,page_id)) plus a reader_mode column (or extend comic_reading_progress); §7: GET/PUT /api/v1/comics/reader-prefs and PUT/DELETE /api/v1/pages/{id}/bookmark + GET /api/v1/comics/{id}/bookmarks — or append to both P1 items: 'schema + endpoints to be added to §6/§7 when scheduled.'  
**Applied:** ✅ [SPEC-02] deferred-surface notes

#### - [x] F044 · 🟡 MINOR · `SPEC-02-comic-vertical.md` · §7 API summary
*Category:* pagination-divergence  
**Problem:** Comic list endpoints use offset `?page=` pagination while SPEC-01/04/05/06/08 all use cursor — an unexplained divergence from the stated convention.  
**Evidence:** SPEC-02:305 'GET /api/v1/comics?page= | comics:read'; :306 /comics/mine also offset-implied. SPEC-01 rev3 (213-214, 381) calls cursor 'the convention all newer specs use'; SPEC-04/05/06/08 expose ?cursor=.  
**Fix:** Switch /api/v1/comics and /api/v1/comics/mine to ?cursor= (order 'status, updated_at DESC' per the §6 index) to match the convention, OR add a one-line note justifying offset paging for the fixed-size published grid if deliberate.  
**Applied:** ✅ [SPEC-02] cursor paging


### SPEC-03-finance-ledger.md

#### - [x] F045 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · Header (Refs)
*Category:* wrong-cross-reference  
**Problem:** The header says the spec 'implements a subset of §8.1–8.2', but P0.5 monthly budgets implement feature-inventory §8.7 (Budgets), which the cited range excludes.  
**Evidence:** SPEC-03:4 '§8 (implements a subset of §8.1–8.2)' vs feature-inventory.md:147 '8.7 Budgets'; SPEC-03 P0.5 (212) ships monthly per-category budgets.  
**Fix:** Change the header ref to '§8 (implements a subset of §8.1–8.2 plus monthly budgets from §8.7)'.  
**Applied:** ✅ [SPEC-03] ref range +§8.7

#### - [x] F046 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.5 Monthly budgets
*Category:* unimplementable-ambiguous  
**Problem:** Budgets on income-kind categories are representable but meaningless — spent is 'Σ expense debits', so an income-category budget renders a permanent 0% bar; accept-or-reject is undefined.  
**Evidence:** SPEC-03:214 'Spent = Σ expense debits' and :223-224 'Category must resolve within the caller's visible set (own or seed)' — no kind restriction, while every other category-referencing write carries one (P0.2 direction match, P0.4 same kind).  
**Fix:** Add to P0.5 write semantics: 'The category must be expense-kind; a PUT naming an income-kind category is 422 bank/category-kind-mismatch (spent is defined only over expense debits).'  
**Applied:** ✅ [SPEC-03] budget expense-kind 422

#### - [x] F047 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.7 Events
*Category:* wrong-cross-reference  
**Problem:** P0.7 claims the bank events 'already have a registered second consumer (SPEC-06's stream)', but events.md registers the stream as their only (first) consumer — and the same paragraph says 'No consumer required to ship', an internal tension.  
**Evidence:** SPEC-03:285-286 'already have a registered second consumer (SPEC-06's stream)' vs events.md:24-26 (Consumers column lists only 'stream (SPEC-06 P0.1)'); the 'second consumer' phrasing belongs to events.md:68's media:asset_ready example.  
**Fix:** Replace with: 'No consumer required to ship, but these events already have a registered first consumer (SPEC-06's stream) — publish via the platform/events helper so a second consumer later is a wiring change.'  
**Applied:** ✅ [SPEC-03] first-consumer wording

#### - [x] F048 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · P0.8 RBAC AC vs P0.4
*Category:* internal-contradiction  
**Problem:** P0.8's seed-immutability AC carves out superadmin ('any non-superadmin mutation → 404'), contradicting P0.4's absolute 'Seeds are not deletable' and P0.8's own 'category mutations stay strictly owner-scoped'; no :any mutation surface exists in §7, so the carve-out is unimplementable.  
**Evidence:** SPEC-03:319-321 'any non-superadmin mutation matches zero rows → 404' vs :182-184 'Seeds are not deletable ... Load-bearing' and :298-299 'mutations stay strictly owner-scoped'; §7 (451-462) lists only :own permissions.  
**Fix:** In the P0.8 AC delete 'non-superadmin': 'Given a seed category (user_id NULL), reads include it for every user, and any mutation via the shipped endpoints matches zero rows → 404 (what makes seeds immutable, P0.4 — no :any mutation surface exists at v1, so this holds for superadmin too).'  
**Applied:** ✅ [SPEC-03] dropped superadmin carve-out

#### - [x] F049 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §3 Non-goals
*Category:* wrong-cross-reference  
**Problem:** The non-goal 'Debts, loans, investments, savings goals' cites feature-inventory §8.3–8.5, but savings goals is §8.6 — the cited range omits it.  
**Evidence:** SPEC-03:42-43 '(feature-inventory.md §8.3–8.5)' vs feature-inventory.md:124-142 '8.3 Debts ... 8.4 Loans ... 8.5 Investments ... 8.6 Savings goals'.  
**Fix:** Change '(feature-inventory.md §8.3–8.5)' to '(feature-inventory.md §8.3–8.6)'.  
**Applied:** ✅ [SPEC-03] range fix

#### - [x] F050 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §6 Data model — bank_budgets.month CHECK
*Category:* check-constraint-immutability  
**Problem:** The first-of-month CHECK uses date_trunc, which for a date argument resolves to the STABLE timestamptz overload, embedding a timezone-dependent function in a CHECK constraint; an immutable formulation should be used.  
**Evidence:** SPEC-03:439 'month date NOT NULL CHECK (date_trunc(\'month\', month) = month)' — resolution keeps the preferred-type (timestamptz) STABLE overload; pg_dump/restore under a different TimeZone re-evaluates it.  
**Fix:** Replace with an immutable equivalent: 'month date NOT NULL CHECK (EXTRACT(day FROM month) = 1)' (or 'CHECK (month = date_trunc(\'month\', month::timestamp)::date)' to keep the truncation form).  
**Applied:** ✅ [SPEC-03] immutable EXTRACT(day)=1 CHECK

#### - [x] F051 · 🟡 MINOR · `SPEC-03-finance-ledger.md` · §8 Frontend
*Category:* route-architecture  
**Problem:** SPEC-03 §8 proposes a new `(bank)` route group, contradicting the fixed two-group (app)/(public) model and its own 'reuse the existing shell/nav' text; a sibling group would not inherit (app)'s authenticated shell.  
**Evidence:** SPEC-03:496 '## 8. Frontend ((bank) route group ...)' and :498 'New route group'. CLAUDE.md fixes two groups: (app) and (public). The same section says 'Left-menu entry added to the shell nav' and 'RSC-first shells' (:509), which only hold under (app).  
**Fix:** Replace 'New route group (bank)' with placing pages under the existing authenticated group, e.g. app/(app)/bank/{page,transactions,accounts,budgets}/page.tsx, so they inherit the (app) shell/nav and gate; drop the separate-route-group language.  
**⚠ Verify-downgraded (real but overstated):** The underlying concern is real: CLAUDE.md fixes exactly two route groups — (app) (authenticated shell) and (public) — and SPEC-03 §8 (:496, :498) introduces a 'New route group ((bank) ...)', a divergence the spec should resolve, especially since the same section wants 'the existing shell' and 'Left-menu entry added to the shell nav' (:498, :509-510). However the finding's load-bearing reasoning — 'a sibling group would not inherit (app)'s authenticated shell' — is overstated: Next.js route groups can nest (e.g. app/(app)/(bank)/...), and a nested route group DOES inherit the parent (app) layout/shell. So the spec's 'reuse the existing shell' text is not necessarily contradictory; the issue is ambiguity about where (bank) sits and a naming divergence from the documented two-group model, not a guaranteed loss of the shell. What survives: the spec should clarify that these pages live under the authenticated (app) group so they inherit its shell/nav/gate.  
**Applied:** ✅ [SPEC-03] pages under (app)


### SPEC-04-notification-module.md

#### - [x] F052 · 🟡 MINOR · `SPEC-04-notification-module.md` · §1 problem statement
*Category:* dangling-reference  
**Problem:** §1 cites '§2 of the gap audit's priorities' although the header records the standalone gap-audit file was never committed and its content was folded into backlog.md — the reference points at a document that does not exist, and backlog §2 is 'Media', not notification-adjacent.  
**Evidence:** SPEC-04:18 'unblocks §2 of the gap audit's priorities' vs :4 'the standalone gap-audit-2026-07.md file was never committed; link fixed 2026-07-10'; backlog.md §2 is 'Media — slice built, gaps' (backlog.md:37); docs/product/analysis/ contains only facebook-comparison.md.  
**Fix:** Replace with a resolvable citation, e.g. 'unblocks the notification-dependent items now tracked in backlog §5 (the notify:* module — the priority-1 gap that unblocks password reset)'; likewise soften line 4's 'priority #2' to cite backlog §5.  
**⚠ Verify-downgraded (real but overstated):** Partially real but the framing overstates. The claim 'points at a document that does not exist' is refuted by SPEC-04:4, which already discloses 'the standalone gap-audit-2026-07.md file was never committed … now folded into backlog.md' — so :18's 'the gap audit's priorities' is not a dangling file link; it is priority-ranking shorthand consistent with :4's 'priority #2'. The finding's 'backlog §2 is Media' is a strawman: :18 references 'the gap audit's priorities', not backlog §2, and backlog does track the notify module in its own §5 (backlog.md:67 '## 5. Notifications module'). What survives is a minor imprecision/circularity: :18 says notify 'unblocks §2 of the gap audit's priorities' while notify itself is priority #2 (per :4) and is catalogued in backlog §5 — a muddy self-referential citation, not a broken reference. Minor at most; downgraded because the stated defect (dangling ref to a non-existent doc) does not hold.  
**Applied:** ✅ [SPEC-04] cited backlog §5

#### - [x] F053 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.1 / §7 API summary
*Category:* underspecified-api  
**Problem:** The default of the `?status=` filter on GET /me/notifications is unspecified — §5 offers unread|all, §7 shows ?status=&cursor=, and no AC covers the parameter being absent, so the bell's initial fetch is implementer-defined.  
**Evidence:** SPEC-04:55 '?status=unread|all&cursor='; :209 '?status=&cursor='; ACs (60-65) exercise only ?status=unread; P0.5 (:138) fetches useQuery(["notifications"]) without a status.  
**Fix:** State the default in the P0.1 note and §7: 'status defaults to all (the bell shows read+unread; the badge uses unread_count)' and add one AC: 'Given no status param, the response contains both read and unread items ordered created_at DESC, id DESC.'  
**Applied:** ✅ [SPEC-04] default=all + AC

#### - [x] F054 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.4 + §7 task inventory
*Category:* missing-registration  
**Problem:** The consumer task type P0.4 requires ('notify handles its own consumer task type') is never named, and is absent from §7's owned-task list and the events.md Tasks table — despite §7 making events.md registration part of definition-of-done.  
**Evidence:** SPEC-04:124 'notify handles its own consumer task type' — no name; §7:222 lists only notify:dispatch/email/web_push/purge_old. events.md:74 names 'media:asset_ready → notify:on_asset_ready' only in prose; its Tasks table (36-50) has no such row.  
**Fix:** In P0.4 name the task ('notify registers notify:on_asset_ready; the cmd/worker subscription table maps media:asset_ready → notify:on_asset_ready per events.md; the handler builds the intent and runs the P0.2 dispatch'), add notify:on_asset_ready to §7's owned-task list, and add the row to events.md's Tasks table as DoD.  
**Applied:** ✅ [SPEC-04] task named+§7; events.md row added (by orchestrator)

#### - [x] F055 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P0.4 AC vs §8 success metrics
*Category:* ac-contradiction  
**Problem:** P0.4's AC promises the notification 'within a few seconds', but §8 states the P0 (poll-only) visibility target is ≤60 s and that 'a 60 s poll cannot honestly claim 10 s' — a tester applying the AC at P0 fails a correct implementation.  
**Evidence:** SPEC-04:131 '... the owner has a new in-app notification ... within a few seconds' vs :227 'the P0 (poll-only) target is visible by the next poll/focus refetch (≤ 60 s, P0.5)'.  
**Fix:** Reword the AC to separate store from visibility: 'when the event fires, then a notifications row for the owner exists within a few seconds (queue latency), and it is visible in the bell by the next poll/focus refetch (P0.5) — or < 10 s once P1.2 SSE lands (§8).'  
**Applied:** ✅ [SPEC-04] AC split latency vs bell

#### - [x] F056 · 🟡 MINOR · `SPEC-04-notification-module.md` · §5 P1.4 vs P0.2 step 2 vs §6
*Category:* missing-mechanism  
**Problem:** P1.4 declares account.security_alert 'email + in-app, not mutable off', but the only channel-forcing mechanism is the intent channels union, and P1.4 never states the emitting intent carries one; the §6 prefs schema cannot make per-channel booleans immutable, so a user with {email:false, in_app:false} would receive nothing.  
**Evidence:** SPEC-04:152 'account.security_alert (email + in-app, not mutable off)'; P0.2 step 2 (:81) only says non-mutable types 'ignore muted', channels still come from the stored pref unioned with the intent override; §6 (179-187) has plain booleans.  
**Fix:** Append to P1.4: 'The emitting intent carries channels:["email","in_app"] — the P0.2 union is what makes the type undisableable; 'not mutable off' is enforced by the override + the muted-ignore rule, not by the prefs schema.'  
**Applied:** ✅ [SPEC-04] channels union clarification


### SPEC-05-journal.md

#### - [x] F057 · 🟡 MINOR · `SPEC-05-journal.md` · Header — Downstream consumers
*Category:* internal-contradiction  
**Problem:** The header says SPEC-06's stream projection 'consumes its event', but P0.3, SPEC-06, and the events registry all state the projection is maintained transactionally in-module and `journal:entry_created` is emit-only — a stale summary that could lead a reader to wire an event consumer.  
**Evidence:** SPEC-05:6 'SPEC-06 (stream projection reads this table and consumes its event)' vs :110-111 'the stream projection is maintained transactionally in-module, not through this event'; SPEC-06:71; events.md:28.  
**Fix:** Replace line 6 with: 'Downstream consumers: SPEC-06 (stream projection reads this table; its projection rows are maintained transactionally in this module's service — journal:entry_created stays emit-only, see P0.3), SPEC-09 P1.7 (takeout exports it).'  
**Applied:** ✅ [SPEC-05] clarified emit-only, transactional projection

#### - [x] F058 · 🟡 MINOR · `SPEC-05-journal.md` · §2 Goal 1 / §4 user story vs §5 P0.4 and P1.6
*Category:* ambiguity / P0-P1 scope conflict  
**Problem:** Goal 1 and the primary user story require capturing 'text + mood' in < 10 s at P0, but the only mood UI is P1.6 (nice-to-have) and P0.4 says the composer merely exposes 'the P1.6 mood slot' — no P0.4 AC exercises mood, so either 'ship a P0 freeform mood field' or 'no P0 mood UI' passes acceptance.  
**Evidence:** SPEC-05:26-27 'captures ... (text + mood) in < 10 s'; :50-51 primary story includes 'with a mood'; :133-134 'exposes ... the P1.6 mood slot'; :163-164 P1.6 mood picker.  
**Fix:** In P0.4 replace 'and the P1.6 mood slot' with 'and a minimal freeform mood text input (P0 — Goal 1 and the primary user story include mood; P1.6 upgrades it with the preset emoji row)', and add a P0.4 AC: 'Given a mood entered, the created entry stores and renders it.' Alternatively reword Goal 1 and the story to drop mood from the P0 path.  
**Applied:** ✅ [SPEC-05] added P0 mood input + new AC

#### - [x] F059 · 🟡 MINOR · `SPEC-05-journal.md` · §5 P2 — future considerations
*Category:* wrong-cross-reference  
**Problem:** P2 attributes 'On-this-day / streaks' to SPEC-06 P1.5, but SPEC-06 P1.5 covers on-this-day memories only — 'streak' appears nowhere in SPEC-06, so the pointer is wrong for half its claim.  
**Evidence:** SPEC-05:170-171 'On-this-day / streaks (SPEC-06 P1.5)' vs SPEC-06:188-192 'P1.5 On-this-day memories ... journal entries only'; grep streak over SPEC-06/brief returns nothing.  
**Fix:** Change to: 'On-this-day (SPEC-06 P1.5) reads this table by month/day of occurred_at; a streak widget is a possible later read of the same column (not specced in SPEC-06) — either way, don't denormalize dates away.'  
**Applied:** ✅ [SPEC-05] split streak from on-this-day claim


### SPEC-06-life-stream-home.md

#### - [x] F060 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · Header (Depends on)
*Category:* wrong-cross-reference  
**Problem:** The header dependency line lists SPEC-04 under 'system events attach as … land' although SPEC-04 produces no stream event (it is a widget/consumer dependency), and omits three of the five actual event producers (SPEC-02 P1.9, SPEC-07 P1.5, SPEC-08 P0.4).  
**Evidence:** SPEC-06:4 'system events attach as SPEC-01 P1.2 / SPEC-03 P0.7 / SPEC-04 land' — but the P0.1 table (83-90) ingests SPEC-01, SPEC-07 P1.5, SPEC-03 P0.7, SPEC-02 P1.9, SPEC-08 P0.4; SPEC-04 supplies only the Activity-feed widget (:176). events.md:20-29 confirms SPEC-04's notify is a sibling consumer.  
**Fix:** Replace with: 'system events attach as their producers land (SPEC-01 P1.2/P0.3, SPEC-02 P1.9, SPEC-03 P0.7, SPEC-07 P1.5, SPEC-08 P0.4); the widget rail additionally consumes SPEC-04's GET /me/notifications — every widget and consumer degrades to an empty state, none is a blocker.'  
**Applied:** ✅ [SPEC-06] header Depends-on rewritten

#### - [x] F061 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · P0.1 known residual risk vs Goal 2
*Category:* event-durability  
**Problem:** Goal 2 claims events are captured 'durably from day one', but every producer except SPEC-08 emits post-commit with no outbox — a producer crash between commit and Publish silently and permanently drops the stream item, a failure mode the residual-risk paragraph does not cover.  
**Evidence:** SPEC-06:32-33 'captures bus events durably from day one'; residual-risk (98-101) covers only create-retry-after-delete. Producers emit at-most-once: SPEC-03:289-290, SPEC-05:106-107, SPEC-07 P1.5. Only SPEC-08 (145-152) closes the hole with an outbox. SPEC-06 §8's durability metric (248-249) tests only the consumer-down case.  
**Fix:** Extend P0.1's residual-risk paragraph: 'Second accepted residual: producers other than SPEC-08 emit post-commit without an outbox, so a producer crash between commit and Publish drops that item permanently. Accepted at v1; a P2 reconcile sweep can diff stream_items against each producer's rows via their api/ packages.' Alternatively require SPEC-03 to adopt SPEC-08's outbox for its P0.7 emits.  
**⚠ Verify-downgraded (real but overstated):** The technical observation is true — SPEC-08 uses an outbox (SPEC-08:144-152: insert with emitted_at NULL in-txn, publish after commit, re-publish NULL rows next scan = at-least-once), while SPEC-03:285-290 and SPEC-05:106-108 emit post-commit with no outbox (at-most-once), so a producer crash between commit and Publish drops that item. But framing this as a contradiction of Goal 2's 'durably from day one' is overstated. In context (problem statement SPEC-06:19-21) that phrase means the projection must EXIST before producers land so events aren't lost for lack of a consumer, and §8:248-249 scopes the durability metric explicitly to the consumer-down / Asynq-redelivery case. Best-effort cross-module emit is an accepted, documented pattern elsewhere (SPEC-01 P0.3:182 'Best-effort, like all cross-module events; a dropped event is recoverable by a consumer-side reconcile'; events.md:88), and SPEC-06 does not own those producers. So what survives is a real minor note that most producers lack SPEC-08's outbox; the 'Goal-2 contradiction' characterization does not survive.  
**Applied:** ✅ [SPEC-06] second residual added

#### - [x] F062 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §5 P0.1(a) + backfill / §6
*Category:* ambiguity  
**Problem:** The stream_items values for journal-sourced rows (event_type, payload) are never specified, though three places must write/match the same key: the transactional path P0.1(a), the migration backfill INSERT…SELECT, and the P0.2 read join.  
**Evidence:** SPEC-06:67-69 'create inserts the stream row'; :103-107 backfill 'INSERT … SELECT'; §6 (218-221) makes event_type part of UNIQUE (source_module, event_type, ref_id) with only system examples. Nothing states journal rows' event_type/payload.  
**Fix:** Add to P0.1(a) and reference from the backfill paragraph: 'Journal projection rows are written as source_module=\'journal\', event_type=\'journal:entry_created\' (registry name, uniform even without bus delivery), ref_id=entry id, payload=\'{}\'; journal items render by joining journal_entries (P0.2). The 000N_journal_stream_items backfill INSERT…SELECT writes the same values.'  
**Applied:** ✅ [SPEC-06] journal projection values

#### - [x] F063 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §5 P0.1(b) consumer table + residual-risk note
*Category:* undefined-edge-case  
**Problem:** Handler behavior for bank:transaction_updated arriving before the matching row exists (created-task retry reordered after update) is undefined — the update no-ops and the later created retry inserts the stale pre-correction payload, which renders wrong indefinitely, contradicting the row's own intent.  
**Evidence:** SPEC-06:87 'bank:transaction_updated ... update the matching row's payload + occurred_at (a corrected amount must not render wrong forever)'; the residual-risk note (98-101) covers only create-after-delete, not create-after-update.  
**Fix:** Change the bank:transaction_updated handler to an upsert: 'INSERT ... ON CONFLICT (source_module, event_type, ref_id) DO UPDATE SET payload, occurred_at — writing under the created-event key when absent. A reordered created retry then hits DO NOTHING and the corrected payload wins.' Alternatively extend the residual-risk paragraph to explicitly accept update-before-create.  
**Applied:** ✅ [SPEC-06] transaction_updated upsert

#### - [x] F064 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §5 P0.2 (and cross-cutting cursor lists)
*Category:* nfr-underspecified  
**Problem:** The stream read endpoint (and the cursor lists in SPEC-01/02/03/05/08) specifies no page-size default or maximum, while SPEC-06's own budget assumes a 50-item stream and only SPEC-07 bounds its list — leaving page size an implementer guess and the LCP budget unenforced at the API.  
**Evidence:** SPEC-06:127-129 'GET /api/v1/stream?cursor= ... cursor-paginated' with no limit, yet :244 'home LCP < 2.5 s with a 50-item stream'. SPEC-07:106 bounds its list '?limit=, default 10, max 50'. Same unbounded pattern: SPEC-05:205, SPEC-01:332, SPEC-02:305, SPEC-03:455, SPEC-08:264.  
**Fix:** Add an explicit ?limit= with default and hard max to SPEC-06 P0.2 (e.g. default 30, max 50, aligning with the 50-item LCP budget), and note in the specs README pagination convention that all cursor list endpoints declare a default+max page size (mirroring SPEC-07).  
**Applied:** ✅ [SPEC-06] limit default 30 max 50

#### - [x] F065 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §5 P0.2 acceptance criteria
*Category:* ac-coverage  
**Problem:** The P0.2 title/href render-mapping — including its load-bearing 'event type without a mapping renders a generic card, never an error' fallback — has no acceptance criterion; P0.2's ACs cover only ordering and user scoping.  
**Evidence:** SPEC-06:135-144 defines the render mapping and generic-card fallback as P0 behavior; the P0.2 ACs (146-149) test only ordering and user scoping, though P0.1 has a parallel ingest-side AC (120-121).  
**Fix:** Add two P0.2 ACs: 'Given stored system items of each mapped event type, the response carries the synthesized title and href per the mapping (e.g. media:asset_ready → '<title> is ready', /library/media#id).' and 'Given a stored item whose event_type has no mapping, GET /stream returns 200 with a generic card (no href), never a 5xx.'  
**Applied:** ✅ [SPEC-06] two ACs added

#### - [x] F066 · 🟡 MINOR · `SPEC-06-life-stream-home.md` · §5 P0.3
*Category:* cross-spec-contradiction  
**Problem:** P0.3's optimistic-insert wording ('prepends'/'appears at the stream top') contradicts SPEC-05's composer contract, where a backdated post must appear at its occurred_at position — top only when now-dated.  
**Evidence:** SPEC-06:156-157 'optimistically prepends to the stream query' and AC :160-161 'appears at the stream top' vs SPEC-05:132-134 (date control) and SPEC-05:144-145 AC 'appears at its occurred_at position (top, when now-dated)'. SPEC-06 P0.3 reuses that composer.  
**Fix:** Reword P0.3 to 'a successful post is optimistically inserted at its occurred_at position in the stream query' and the AC to 'it appears at its occurred_at position (top, when now-dated) optimistically and survives an immediate refetch — guaranteed because the projection row is written in the create transaction (P0.1(a)).'  
**Applied:** ✅ [SPEC-06] optimistic insert at occurred_at


### SPEC-07-continue-rail.md

#### - [x] F067 · 🟡 MINOR · `SPEC-07-continue-rail.md` · P1.5 completion event
*Category:* event-payload-contract  
**Problem:** `media:playback_completed` payload is {asset_id, user_id} only, but SPEC-06 envisions a 'watched X' stream card rendered payload-only — the title needed for the card is absent, unlike media:asset_ready which carries title.  
**Evidence:** SPEC-07:133 and events.md:22 define {asset_id, user_id}. SPEC-06:85 'a watched X card', :143 renders from stored payload, open question resolves payload-only (272-273); events.md:86-87 'payloads carry IDs + the minimum to render'; media:asset_ready carries title (events.md:20, SPEC-01:256).  
**Fix:** Extend the P1.5 payload to {asset_id, user_id, title} (title falls back to original_filename per SPEC-01 P1.2), and add the mapping row to SPEC-06 P0.2: media:playback_completed → ('Watched <title>', /library/media#id).  
**Applied:** ✅ [SPEC-07] payload +title

#### - [x] F068 · 🟡 MINOR · `SPEC-07-continue-rail.md` · §5 P0.1
*Category:* missing-acceptance-criteria  
**Problem:** The P0.1 NULL-duration rule (excluded from /continue, no completion event, resume still works) — a P0 behavior the spec added deliberately — has no acceptance criterion; P0.2/P0.3/P0.4 ACs all assume a known duration.  
**Evidence:** P0.1 (58-64) defines the NULL-duration behavior; the AC lists at 85-90, 115-119, 126-129 contain no NULL-duration case.  
**Fix:** Add to P0.3 ACs: 'Given a saved position of 10:00 on an asset with duration_ms IS NULL, then the item does not appear in /continue, reopening still offers resume at ~10:00, and no media:playback_completed event is ever emitted for it.'  
**Applied:** ✅ [SPEC-07] NULL-duration AC

#### - [x] F069 · 🟡 MINOR · `SPEC-07-continue-rail.md` · §5 P0.2 Beacon
*Category:* frontend-implementability  
**Problem:** Beacon-under-cookie-model works but two nuances are unstated: sendBeacon's content-type (text/plain, not JSON) and drops when the access token has expired at pagehide.  
**Evidence:** P0.2 (80-83) 'on pagehide (via navigator.sendBeacon/keepalive fetch)'. navigator.sendBeacon(url, JSON.stringify(x)) sends Content-Type: text/plain, so a JSON-requiring handler rejects it; portal_access has ~5-min TTL that SessionKeeper cannot refresh during pagehide, so a beacon with an expired token is 401'd and silently dropped (api-client.ts:6).  
**Fix:** Add a note to P0.2: for the sendBeacon path send a Blob([...], {type:'application/json'}) (or use keepalive-fetch, which can set headers) so the body parses; acknowledge that a pagehide beacon after access-token expiry is dropped — acceptable because the ≤10 s/on-pause saves bound the loss. Confirm no CSRF concern (SameSite=Strict).  
**⚠ Verify-downgraded (real but overstated):** Partially survives. P0.2 (SPEC-07:81) says beacons fire 'via navigator.sendBeacon/keepalive fetch.' The substantive half is correct and worth noting: navigator.sendBeacon(url, JSON.stringify(x)) sends Content-Type: text/plain, so a handler that requires application/json would silently fail to parse the body — a real gotcha the spec should flag (send a Blob typed application/json, or use keepalive-fetch which can set headers). The token-expiry half is weaker: the spec already frames beacons as 'Fire-and-forget: beacon failures never surface to the player' (SPEC-07:82-83) and bounds loss with ~10 s/on-pause saves, so a dropped-on-expiry pagehide beacon is already an accepted, documented outcome rather than an unstated defect. Net: valid minor implementability note, but overstated as two independent unstated nuances — one is already covered by the existing fire-and-forget design.  
**Applied:** ✅ [SPEC-07] Blob content-type, pagehide-expiry, CSRF

#### - [x] F070 · 🟡 MINOR · `SPEC-07-continue-rail.md` · §5 P0.2 acceptance criteria
*Category:* stale-reference  
**Problem:** The AC qualifier '(no wildcard perms)' is a leftover from the dropped media:progress:own permission design — with authorization now 'RequireAuth + owner-scoped by construction', no permission is consulted, so the parenthetical implies a superadmin bypass path the spec no longer defines.  
**Evidence:** P0.2 AC (88) 'Given a beacon for another user's asset (no wildcard perms), then 404' vs P0.2 prose (68-73) 'authenticated (RequireAuth), owner-scoped by construction' and the dropped media:progress:own note.  
**Fix:** Replace the AC with: 'Given a beacon for another user's asset (regardless of the caller's role or permission grants — there is no permission-based bypass), then 404 and no row.'  
**Applied:** ✅ [SPEC-07] AC no permission bypass


### SPEC-08-people-registry.md

#### - [x] F071 · 🟡 MINOR · `SPEC-08-people-registry.md` · P0.4 vs §6 people_birthday_notices
*Category:* prose-ddl-mismatch  
**Problem:** P0.4 calls notice_id 'the notices row's uuid PK', but §6 declares id as a non-PK UNIQUE column while the actual PRIMARY KEY is the composite (person_id, year, threshold).  
**Evidence:** SPEC-08:151-152 'notice_id is the notices row's uuid PK (§6)' vs §6:236 'id uuid ... UNIQUE' and :241 'PRIMARY KEY (person_id, year, threshold)'.  
**Fix:** Reword to: 'notice_id is the notices row's id column — a UNIQUE surrogate; the table's PRIMARY KEY is the composite (person_id, year, threshold) (§6).'  
**Applied:** ✅ [SPEC-08] notice id/UNIQUE composite PK

#### - [x] F072 · 🟡 MINOR · `SPEC-08-people-registry.md` · §5 P0.2 vs SPEC-06 consumers
*Category:* ambiguity  
**Problem:** The birthday-edit rule deletes current/future-year people_birthday_notices rows, but says nothing about already-emitted downstream items keyed on those notice_ids — stale stream/bell cards persist and their notice_id refs dangle.  
**Evidence:** SPEC-08 P0.2 (85-88) 'deletes the person's people_birthday_notices rows for the current and future occurrence years' — no retraction/accepted-staleness statement. SPEC-06:90 keys ref_id = notice_id on the deleted row.  
**Fix:** Add to P0.2: 'Already-emitted stream/notification items from the deleted notices are NOT retracted at v1 — accepted staleness (they age out; the corrected date emits fresh events under new notice_ids). Consumers must not re-fetch by notice_id (the row may be gone); the payload is self-sufficient for rendering.' (Or define a people:birthday_notice_revoked event — pick one.)  
**Applied:** ✅ [SPEC-08] staleness note

#### - [x] F073 · 🟡 MINOR · `SPEC-08-people-registry.md` · §5 P1.7 vs §7 API summary
*Category:* missing-api-field  
**Problem:** P1.7 introduces settable `avatar_asset_id` (with problem type people/invalid-asset), but no §7 endpoint carries the field — the POST body is enumerated without it and the PATCH row mentions only birthday semantics, so how a client sets/clears an avatar is unspecified.  
**Evidence:** §7:263 POST body '{display_name, relationship?, birthday?, contact?, note_md?}' — no avatar_asset_id; :266 PATCH row birthday-only; yet :271-272 declares 'people/invalid-asset (P1.7)' and P1.7 (195-196) requires avatar_asset_id via mediaapi.  
**Fix:** In §7 extend the PATCH Notes: 'P1.7 adds avatar_asset_id?: uuid|null (validated via mediaapi: exists, kind image, status ready, owned — else 422 people/invalid-asset; null clears)', and note in P1.7 the field lands on PATCH (and optionally POST) when P1.7 ships.  
**Applied:** ✅ [SPEC-08] avatar_asset_id PATCH row + P1.7


### SPEC-09-platform-ops.md

#### - [x] F074 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §5 P0.5 / P1.6 / P1.7 vs §6
*Category:* ambiguous-requirement  
**Problem:** 'This phase's migration' is ambiguous for the P1 permission seeds: P1.6 seeds queues:read 'alongside ops:read' (seeded by the P0 migration), while §6 says the P1.7 table 'may ship in a later migration' — an implementer can't tell whether the four permission rows go in 000N_ops_backup_runs or a later P1 migration.  
**Evidence:** SPEC-09:139-140 (ops:read seeded by ops migration), :166-169 (queues:read 'in this phase's migration, alongside ops:read'), :174-176 (takeout codes 'in this phase's migration'), §6:217 'P1.7 (may ship in a later migration)'. specs/README.md:56-60 requires the introducing migration named.  
**Fix:** State explicitly in P0.1: 'Migration 000N_ops_backup_runs seeds all four permission rows up front — ops:read, queues:read → admin; takeout:read:own, takeout:write:own → user (unseeded codes 403; seeding early is harmless). The ops_exports table may still ship in a later P1 migration.' Drop the per-requirement 'this phase's migration' phrasing.  
**Applied:** ✅ [SPEC-09] P0.1 seeding stated

#### - [x] F075 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §5 P0.5 acceptance criteria
*Category:* AC-contradicts-behavior  
**Problem:** The stale AC ('last success 27 h ago → stale') contradicts precedence rule 1 (failed wins) when a run failed after that success, and the two newly-resolved semantics (failed-wins, never-ran ⇒ stale) have no AC.  
**Evidence:** SPEC-09:147-148 'failed ... wins even if an older success is < 26 h old' vs :162 'Given last success 27 h ago, then state=stale'; precedence/zero-runs added because 'were undefined' (143-144) yet unexercised.  
**Fix:** Rewrite the AC block: 'Given last success 27 h ago and no completed run since → stale; a fresh success → ok. Given a success 2 h ago followed by a failed run → failed (precedence). Given zero runs ever → stale with last_success_at: null, hours_since_success: null. Non-admin → 403; unauthenticated → 401.'  
**⚠ Verify-downgraded (real but overstated):** The 'AC contradicts precedence rule 1' framing is refuted by the source. The AC (SPEC-09:162) reads 'Given last success 27 h ago, then state=stale' — it posits no completed failed run after that success, so precedence rule 1 (147, failed-wins) never engages and rule 2 (149, no success within 26h ⇒ stale) correctly yields stale. No contradiction exists; the category 'AC-contradicts-behavior' is wrong. What survives is the coverage half: the two newly-resolved semantics — failed-wins-over-recent-success (147) and never-ran ⇒ stale with null fields (149-150) — have no acceptance criterion exercising them (ACs at 162-163 cover only 27h-stale, fresh-ok, 403, 401). That is a minor missing-AC-coverage gap, not a contradiction.  
**Applied:** ✅ [SPEC-09] rewrote AC precedence/zero/stale

#### - [x] F076 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §5 P1.7 / §7 API summary
*Category:* undefined-behavior  
**Problem:** Problem type `ops/export-not-ready` is declared in §7 but no P1.7 behavior triggers it — GET returns status (any state) and 410 export-expired for expired, leaving when not-ready fires undefined.  
**Evidence:** SPEC-09:243-244 lists ops/export-not-ready, but P1.7 (181-183) defines GET as 'returns status + a short-TTL download URL' and only specifies 410 export-expired. No state maps to not-ready.  
**Fix:** Either delete ops/export-not-ready from §7, or define its trigger in P1.7, e.g. 'GET on a pending/running/failed export returns 200 with status and no download_url; ops/export-not-ready is reserved for a client explicitly requesting the download URL of a non-ready export (409)' — pick one.  
**Applied:** ✅ [SPEC-09] deleted unused problem type

#### - [x] F077 · 🟡 MINOR · `SPEC-09-platform-ops.md` · §8 vs §5 P0.5
*Category:* stale-naming  
**Problem:** §8 refers to the status field as `hours_since`, but the P0.5 response schema names it `hours_since_success`.  
**Evidence:** SPEC-09:253 'hours_since never exceeds 26' vs :142 'Returns {last_success_at, hours_since_success, state, last_run}'.  
**Fix:** In §8 replace 'hours_since' with 'hours_since_success'.  
**Applied:** ✅ [SPEC-09] hours_since_success
