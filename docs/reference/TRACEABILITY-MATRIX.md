# Traceability Matrix — Requirements ↔ Tests

**Status:** current · **Last verified:** 2026-09-19 — the SPEC-12 section was added and graded against the `_test.go` files on disk (`find backend -name '*_test.go' | wc -l`, 45 at this check — the 2026-09-11 header said 31, a stale count even then); the legend, CC-1, CC-9 and the one-line summary were re-read against the tree the same day and corrected where the frontend's three vitest files or the journal HTTP-contract file made them false; every other `Cov` mark stands as re-graded on 2026-09-11 and carries an **Evidence** cell naming the test that proves it. A ✅ with an empty Evidence cell is a defect in this document, not coverage.

> **⚠️ Read this before trusting the Cov column (added 2026-08-25).**
>
> Every `TC-` id in this matrix is a **document row, not a test**. `grep _test.go
> docs/testing/*.md` returns nothing: no row here names a Go or TypeScript test
> that exists. The Cov column reads 100 % ✅ across all 47 rows with no ⚠️ or ✖,
> and the Result column is entirely empty — that is what an unexecuted plan looks
> like, not what coverage looks like.
>
> The real test inventory is **20 `_test.go` files** under `backend/`. Run
> `cd backend && go test ./...`. What they actually cover, and what no test can
> reach, is in
> [../product/analysis/remaining-work-2026-08-25.md](../product/analysis/remaining-work-2026-08-25.md).
>
> Two structural gaps this matrix does not show: the frontend has **zero** test
> files, and until 2026-08-25 the backend had **zero** `httptest` — no test had
> ever asserted a status code or a response body. That is now two suites
> (`platform/server`, `tenant/middleware`) rather than none.

*(The note above is kept as the dated record of what this file looked like before
the 2026-09-11 re-grade. The `TC-` ids still name rows in the per-spec case
documents, which remain the **plan**; the Evidence column is what **exists**.)*

Maps every spec requirement (`Px.y`) and cross-cutting convention (`CC-n`) to the
planned test cases and to the automated tests that exist. **Coverage gate:** every
`P0` requirement must have ≥1 `_test.go` function in Evidence; a `P0` at ⚠ or ✖ is
a coverage defect (TEST-PLAN §6.2).

Legend — **Cov** is graded on evidence, not intent:

- ✅ a named `_test.go` function, a named `frontend/…/*.test.ts` vitest file (run by
  the `frontend` CI job), or a CI job proves the whole requirement summary — a
  vitest file proves the pure rule it pins, never the component around it;
- ⚠ the owning module has tests, but they prove only part of the summary — the
  Evidence cell names what is proven and the gap;
- ✖ nothing automated proves it. For frontend-only rows this is near-structural:
  the frontend's only tests are three vitest files of pure rules under
  `frontend/src/lib/` (SPEC-12 T2–T5), run by the `frontend` CI job; no component
  or browser test exists.

Evidence paths are relative to `backend/internal/` unless they start with `.github/`, `scripts/` or `frontend/`.

---

## SPEC-01 — Media ([cases](../testing/TEST-CASES-SPEC-01-media.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Image ingest, sniff, HEIC/size/dim/animated, variants, orientation | TC-MEDIA-001…019 | P0 | ⚠ | `modules/media/service_test.go: TestCompleteUploadImageAccepted, TestCompleteUploadHEICRejected, TestCompleteUploadUnknownFormatRejected, TestCompleteUploadTooLarge, TestSniffImageType, TestIsHEIC` — ingest/sniff/HEIC/size proven. The worker's rules since 2026-09-19: `modules/media/worker/process_image_test.go: TestCheckImageDims` (the area cap that admits a 704×18000 strip and refuses 8001×8000, the per-side ceiling, animated and zero-size refusals), `TestScaledDims` (never upscaled, width-first, libwebp height clamp, never a zero side), `TestPipelineAgainstFixtures` (real ffmpeg/ffprobe on PNG/GIF fixtures drawn in-test — the probe's dimensions and frame count, thumb and medium variants matching `scaledDims`, a 200×18000 strip encoded under the height limit, an animated GIF refused; CI installs ffmpeg for it and the test fails there if it is missing). Still without a fixture: EXIF orientation (`-map_metadata -1` strips it; auto-rotation is ffmpeg's default and is not asserted) and the `run` orchestration around download/upload/tenant scope. |
| P0.2 | Video poster + audio-only skip + non-fatal | TC-MEDIA-030…033 | P0 | ⚠ | `modules/media/worker/thumbnail_test.go: TestPosterPlan` — the audio-only container is skipped with the error the caller treats as a warning, and the seek rule (10 % of the duration, capped at 10 s, first frame when unknown or zero) is pinned; audio-skip at the service level: `modules/media/service_test.go: TestCompleteUploadAudioReadyWithoutTranscode`. The frame extraction itself (`extractPoster`) has no fixture — a video fixture needs ffmpeg to synthesise one — and "non-fatal" (`Handle` turning the skip into a warning) is asserted nowhere. |
| P0.3 | Delete asset + janitor + event | TC-MEDIA-040…048 | P0 | ✅ | `modules/media/service_test.go: TestDeleteAsset` (asserts `media:asset_deleted` is published), `TestPurgeOrphans`; consumers: `modules/comic/comic_test.go: TestAssetDeletedConsumer`, `modules/{movie,music}/*_test.go: TestAssetDeletedClearsBothReferences`, `modules/story/story_test.go: TestAssetDeletedClearsTheCover`. |
| P0.4 | Library page + filters + cursor + LCP | TC-MEDIA-060…069 | P0 | ⚠ | `modules/media/service_test.go: TestListPaginates, TestCursorRoundTrip, TestExpandStatuses` — API list/filter/cursor proven. Page render and LCP: frontend, no tests. |
| P0.5 | Download original (checksum, private, states) | TC-MEDIA-080…086 | P0 | ✅ | `modules/media/service_test.go: TestDownloadOriginal, TestServeVariant, TestHLSObjectSafety`; range semantics `modules/media/objectreader_test.go: TestServeContentAnswersRangeRequests, TestObjectReaderSeekEndReportsSizeWithoutReading`. |
| P0.6 | Event fan-out prerequisite | TC-MEDIA-090…093 | P0 | ✅ | `platform/events/events_test.go: TestPublishFansOutToEachConsumer, TestPublishNoSubscribersIsNoop, TestPublishPropagatesEnqueueError`. |
| P1.1/P1.2 | Metadata edit, asset_ready emit | TC-MEDIA-100…102 | P1 | ⚠ | emit: `modules/media/service_test.go: TestCompleteUploadAudioReadyWithoutTranscode` (asserts exactly one `media:asset_ready`); the video path's emit happens in the transcode worker, untested. Metadata edit: no test. |

## SPEC-02 — Comic ([cases](../testing/TEST-CASES-SPEC-02-comic.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Entities + CRUD + asset validation | TC-COMIC-001…015 | P0 | ⚠ | `modules/comic/comic_test.go: TestPageAssetValidation, TestCoverAssetValidation` — validation proven; CRUD paths have no test. |
| P0.2 | Publish flow + owner-or-elevated RBAC | TC-COMIC-030…040 | P0 | ⚠ | `modules/comic/comic_test.go: TestPublishValidation, TestDraftVisibility` — publish guards and draft visibility proven; the elevated-permission path (`RequireOwnerOrPermission`) is not exercised. |
| P0.3 | Reader vertical scroll | TC-COMIC-060…066 | P0 | ✖ | frontend; no test files. |
| P0.4 | Reading progress keyed by page_id | TC-COMIC-080…088 | P0 | ✅ | `modules/comic/comic_test.go: TestProgressMembership` (chapter/page membership, unknown comic is 404), `TestSaveProgressRespectsDraftVisibility` (a stranger's save on a draft is 404 and writes no row; owner can; published opens it). |
| P0.5 | Library + detail | TC-COMIC-100…104 | P0 | ✖ | frontend; the backend list has no test either. |
| P0.6 | Asset-deletion coupling | TC-COMIC-120…123 | P0 | ✅ | `modules/comic/comic_test.go: TestAssetDeletedConsumer`. |
| P1.7/P1.9 | Zip import, chapter events | TC-COMIC-140…148 | P1 | ⚠ | import ordering `modules/comic/comic_test.go: TestChapterSortOrder`; scraper source guard `modules/comic/sourceguard_test.go` (6 tests: allow-list, private-IP block, echoed-owner check). `comic:chapter_published` is consumed in `modules/notify/service_test.go: TestOnComicPublished` (the stream projection was removed in `0034`), but no comic test asserts it is emitted. |

## SPEC-03 — Bank ([cases](../testing/TEST-CASES-SPEC-03-bank.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Accounts + derived balance + immutability | TC-BANK-001…008 | P0 | ✅ | `modules/bank/bank_test.go: TestDerivedBalanceReconciles, TestAccountDeleteAndCurrencyGuards`. |
| P0.2 | Transactions + direction/kind + reconciliation | TC-BANK-020…034 | P0 | ✅ | `modules/bank/bank_test.go: TestDirectionKindMismatch, TestInvalidAmount, TestDerivedBalanceReconciles`. |
| P0.3 | Transfers + fees + leg predicate | TC-BANK-050…059 | P0 | ✅ | `modules/bank/bank_test.go: TestTransferZeroSum, TestTransferLegNotIndividuallyMutable, TestTransferDeleteRemovesBothLegs, TestSameAccountAndCurrencyGuards`; predicate in aggregates `TestReportRollsUpChildrenAndSplitsByKind`. |
| P0.4 | Categories hierarchy + delete/reassign | TC-BANK-070…082 | P0 | ✅ | `modules/bank/bank_test.go: TestCategoryParentRules, TestCategoryDeleteReassignMatrix, TestSeedCategoryImmutable`. |
| P0.5 | Monthly budgets tree | TC-BANK-100…108 | P0 | ✅ | `modules/bank/bank_test.go: TestBudgetKindGuardAndDelete, TestBudgetSpentIncludesChildren`. |
| P0.6 | Dashboard (balances vs flows) | TC-BANK-120…124 | P0 | ⚠ | `modules/bank/bank_test.go: TestReportRollsUpChildrenAndSplitsByKind` proves the month aggregate; the dashboard view is frontend, untested. |
| P0.7 | Events (+ bulk carve-out) | TC-BANK-140…144 | P0 | ⚠ | consumed in `modules/journal/journal_test.go: TestStreamTransferCollapse, TestStreamReadMapping`; `modules/bank/bank_test.go` never asserts `bank:transaction_created` is published (`emitTx` is unexercised). |
| P0.8 | RBAC owner isolation + seeds | TC-BANK-160…164 | P0 | ✅ | `modules/bank/bank_test.go: TestOwnerScoping, TestSeedCategoryImmutable`; at the database: `platform/db/rls_test.go: TestRLSSharedSeedCategoriesAreVisibleToEveryTenant` (env-gated, see CC-10). |
| P0.9 | Import scaffolding | TC-BANK-180…181 | P1 | ⚠ | no test; schema-only deliverable. |

## SPEC-10 — Ledger expansion ([spec](../product/specs/SPEC-10-ledger-expansion.md); no case document yet)

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| Phase 1 | Debts & loans: accrual arithmetic, outstanding principal | — | P0 | ⚠ | `modules/bank/debts_test.go: TestAccrueInterest, TestOutstandingFrom` — the arithmetic; the principal-as-transfer posting, `nothing-to-accrue` refusal and the `bank:scan_debts_due` sweep are unexercised. |

## SPEC-04 — Notify ([cases](../testing/TEST-CASES-SPEC-04-notify.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Store + read API | TC-NOTIFY-001…008 | P0 | ⚠ | no test for the read/mark-read API; `modules/notify/service_test.go` covers dispatch only. |
| P0.2 | Dispatch fan-out + muted/channels/dedup | TC-NOTIFY-020…027 | P0 | ✅ | `modules/notify/service_test.go: TestDispatchDefaultPrefs, TestDispatchEmailPref, TestDispatchMutedMutable, TestDispatchNonMutableOverride, TestDispatchDedupRedelivery, TestDispatchMalformedSkipsRetry`. |
| P0.3 | Email + password reset + abuse controls | TC-NOTIFY-040…053 | P0 | ⚠ | reset tokens `modules/account/auth/reset_test.go: TestResetManagerIssueVerifyReuse, TestResetManagerExpired, TestNewResetManagerValidation`; send throttle `modules/account/handler/password_reset_test.go: TestResetSendAllowed`. SMTP delivery (`notify/email.go`) has no test. |
| P0.4 | First consumer (media:asset_ready) | TC-NOTIFY-070…074 | P0 | ✅ | `modules/notify/service_test.go: TestOnAssetReady` (+ `TestOnComicPublished`, `TestOnWorkPublished` for the later consumers). |
| P0.5 | Bell wiring | TC-NOTIFY-090…094 | P0 | ✖ | frontend; no test files. |
| P1.1–P1.4 | Web push, SSE, prefs UI, security alert | TC-NOTIFY-110…113 | P1 | ✖ | not built. |

## SPEC-05 — Journal ([cases](../testing/TEST-CASES-SPEC-05-journal.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Module scaffold | TC-JRNL-001…002 | P0 | ✅ | structural: `journal.New` constructed and mounted in `cmd/api/main.go`; `modules/journal/journal_test.go` compiles against the module. |
| P0.2 | Entries CRUD + validation + ordering | TC-JRNL-010…023 | P0 | ✅ | `modules/journal/journal_test.go: TestCreateValidationPublishesNothing, TestGetOwnerScopedNotFound, TestListCursorPaginates`. |
| P0.3 | Event emit (emit-only) | TC-JRNL-030…033 | P0 | ✅ | `modules/journal/journal_test.go: TestCreateEmitsExactlyOnceAfterCommit, TestCreateRollbackPublishesNothing`. |
| P0.4 | Composer + home + sanitization | TC-JRNL-050…059 | P0 | ✖ | frontend composer/home untested; no sanitization test on either side. |
| P1.5/P1.6 | Attachments, mood picker | TC-JRNL-070…074 | P1 | ⚠ | Attachments superseded by SPEC-12 (section below) and proven there — `modules/journal/http_test.go: TestHTTPCreateStoresAttachmentsInOrder`; the mood is edited in the composer and cleared over PATCH (T5, `TestHTTPPatchMoodSetClearKeep`), but the picker itself (TC-JRNL-074, the preset row) is frontend and untested. |

## SPEC-12 — Journal attachments ([spec](../product/specs/SPEC-12-journal-attachments.md); no case document — the spec's Testing Decisions name the seam)

Executed 2026-09-19 as tickets T0–T6 (#9–#15) on `feat/journal-attachments`. The primary seam is the journal HTTP-contract file over the real router with fakes (`modules/journal/http_test.go`); the frontend half has vitest unit tests for its pure rules, run by the `frontend` CI job since 2026-09-19 — a vitest file counts as evidence for the rule it pins, never for the component around it.

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| T1 | `asset_ids` validated as a whole (dup, eleventh, unknown, non-image, not ready, other owner → 422 `journal/invalid-asset` naming the id; nothing stored); order kept; PATCH replaces the whole list; lookup inside the request scope | — | P0 | ✅ | `modules/journal/http_test.go: TestHTTPCreateStoresAttachmentsInOrder, TestHTTPInvalidAttachmentListIsRefusedWhole, TestHTTPMalformedAssetIDNamesIt, TestHTTPPatchReplacesWholeList, TestHTTPAssetLookupRunsInsideRequestScope, TestHTTPTextOnlyCreateSkipsLookup`. |
| T1 | An Entry is text or at least one Attachment — never neither, never a Location alone — judged on the patched result | — | P0 | ✅ | `modules/journal/http_test.go: TestHTTPTextOrAttachment, TestHTTPLocationAloneIsNotAnEntry`. |
| T1/T3 | Entry JSON and stream item JSON carry `asset_ids` and `location` in one shape (`[]` / explicit null, never absent) | — | P0 | ✅ | `modules/journal/http_test.go: TestHTTPEntryAndStreamItemShareAssetIDsShape, TestHTTPLocationStoredAndSharedShape`. |
| T1/T3 | Migrations `0044`/`0045`: the photo and geo links move out of every body into columns, the closing DO block raises on any leftover, `down` re-encodes | — | P0 | ✅ | `modules/journal/backfill_test.go: TestBackfillMigrationsMoveLinksOutOfBodies` — builds a throwaway database to 0043 from the migration files, seeds five old-style bodies (text + photo + place, photo whose Asset is gone, place-only, plain, inline link), runs 0044 → 0045 → down → down → up → up asserting bodies, `asset_ids` and the three columns at each step, the 0045 CHECK refusing six half-shapes, the restored 0044 CHECK still binding new writes, then the three refusals — an off-Earth link and a leftover second link making 0045 RAISE, a leftover second photo link making 0044 RAISE — each with its DDL rolled back. Needs `RLS_TEST_ADMIN_URL` (skips locally when unset, fails under CI). Its first run caught a rollback failure — 0044 down's restored CHECK refused the row the up had emptied — fixed by restoring it `NOT VALID`. |
| T3 | Location rules: name-only, coordinates-only, blank name, out of range, wrong type → 422 `journal/invalid-location`, nothing stored; PATCH object sets, null clears, absent keeps | — | P0 | ✅ | `modules/journal/http_test.go: TestHTTPLocationStoredAndSharedShape, TestHTTPInvalidLocationIsRefused, TestHTTPPatchLocationSetClearKeep`. |
| T4 | `media:asset_deleted` strips the id from every Entry of the owner (order kept, empty Entry survives, other owners untouched), inside the owner's tenant scope, idempotent on redelivery; an owner-less event is dropped | — | P0 | ✅ | `modules/journal/journal_test.go: TestAssetDeletedStripsAttachmentFromEveryEntry, TestAssetDeletedWithoutOwnerIsDropped, TestStreamAssetDeletedRemoves` (+ `TestBankDeletedRunsInsideOwnerScope` for the sibling consumer scoped in the same change). |
| T5 | PATCH `mood`: a string sets (trimmed, 1–80), null clears, absent keeps; blank or wrong type → 422 `journal/invalid-mood` | — | P1 | ✅ | `modules/journal/http_test.go: TestHTTPPatchMoodSetClearKeep`. |
| T2 | Composer rules: a file picked twice is one tile, the eleventh is refused with a message, Save only when every photo is ready and the draft has text or a photo; card layout hero + four thumbs + "+N" | — | P1 | ✅ | the rules are pure functions under vitest, run by CI job `frontend`: `frontend/src/lib/composer-photos.test.ts` (dedup by name+size+mtime, the cap and its counts, `canPost`, tile order, stored tiles), `frontend/src/lib/entry-presentation.test.ts` (hero / four thumbs / "+N", the field reads). The composer component, the upload orchestration and the card around them stay in the T2/T5 row below. |
| T2/T5 | Lightbox; edit in place with the composer (pre-fill, cancel, whole-Entry save); Location chip on both cards | — | P1 | ✖ | frontend components; no component or browser test (one pure rule of the picker is: `frontend/src/lib/geo.test.ts`, the longitude wrap that keeps a dropped pin on the Earth). Exercised by hand on the live stack on 2026-09-19, every step passing — [TEST-RUN-2026-09-19-spec-12.md](../testing/TEST-RUN-2026-09-19-spec-12.md) — which is a record, not evidence for this column. |

## SPEC-06 — Stream ([cases](../testing/TEST-CASES-SPEC-06-stream.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Projection consumers + idempotency + backfill | TC-STREAM-001…017 | P0 | ⚠ | `modules/journal/journal_test.go: TestStreamIdempotency, TestStreamTransferCollapse, TestStreamAssetDeletedRemoves` — consumers and idempotency proven; backfill has no test. |
| P0.2 | Stream read API + title/href synthesis | TC-STREAM-030…037 | P0 | ✅ | `modules/journal/journal_test.go: TestStreamReadMapping`. |
| P0.3 | Home replacement | TC-STREAM-050…052 | P0 | ✖ | frontend; no test files. |
| P0.4 | Widget rail + failure isolation | TC-STREAM-070…072 | P0 | ⚠ | registry-backed placement `modules/layout/service_test.go: TestSaveWidgetsRejectsAnUnknownKey, TestSaveWidgetsNumbersEachSlotIndependently, TestForCallerDropsRowsTheCallerCannotSee`; per-widget failure isolation is frontend, untested. |
| P1.5/P1.6 | Memories, backfill task | TC-STREAM-090…092 | P1 | ✖ | no test. |

## SPEC-07 — Continue ([cases](../testing/TEST-CASES-SPEC-07-continue.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Progress table + NULL-duration rule | TC-CONT-001…003 | P0 | ✅ | `modules/media/service_test.go: TestGetProgressNoRow, TestPutProgressGuards` (nil `DurationMs` path). |
| P0.2 | Beacon (resume, clamp, owner, fire-forget) | TC-CONT-020…029 | P0 | ✅ | `modules/media/service_test.go: TestPutProgressGuards, TestPutProgressCompletionLatch`. |
| P0.3 | Aggregator + inclusion predicate | TC-CONT-040…047 | P0 | ✅ | `modules/media/service_test.go: TestContinueItemsPredicate`. |
| P0.4 | Resume UX | TC-CONT-060…064 | P0 | ✖ | frontend; no test files. |
| P1.5 | Completion event (latch) | TC-CONT-080…083 | P1 | ✅ | `modules/media/service_test.go: TestPutProgressCompletionLatch` (exactly one `media:playback_completed`). |

## SPEC-08 — People ([cases](../testing/TEST-CASES-SPEC-08-people.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.2 | CRUD + birthday validation + notice reset | TC-PPL-001…016 | P0 | ⚠ | `modules/people/people_test.go: TestBirthdayValidation`; CRUD and notice reset have no test. |
| P0.3 | Upcoming endpoint (TZ, Feb-29, lunar, clamp) | TC-PPL-030…036 | P0 | ⚠ | `modules/people/people_test.go: TestNextOccurrence, TestNextOccurrenceTimezone, TestUpcomingBirthdays` — solar dates, timezone and clamp proven; lunar conversion has no test. |
| P0.4 | Scan + event (dedup, catch-up, outbox) | TC-PPL-050…057 | P0 | ✅ | `modules/people/people_test.go: TestScanDedupAndOutbox, TestOutboxRetry`. |
| P0.5 | Frontend (BirthdayCard, empty, gate) | TC-PPL-070…073 | P0 | ✖ | frontend; no test files. |
| P1.6/P1.7 | Interactions, avatar | TC-PPL-090…092 | P1 | ✖ | no test. (Directory suggestions, not in the spec's P1 list, are: `TestSuggestionsSubtractsAlreadyAdded, TestSuggestionsWithoutDirectory`.) |

## SPEC-09 — Ops ([cases](../testing/TEST-CASES-SPEC-09-ops.md))

| Req | Summary | Test cases | Pri | Cov | Evidence |
|-----|---------|-----------|-----|-----|----------|
| P0.1 | Scaffold + seeding + system-scoped tables | TC-OPS-120…123 | P0 | ⚠ | structural only (`ops.New` mounted; migrations apply). No test asserts the system-scope (NULL tenant) rule for ops tables. |
| P0.2 | Nightly backup + retention + manifest | TC-OPS-001…007 | P0 | ⚠ | retention policy `modules/ops/retention_test.go: TestKeepRetention, TestParseDumpDate, TestDumpKeyRoundTrip`. The backup run itself (`pg_dump` → storage, manifest) is proven only by the drill script below. |
| P0.3 | Media durability posture (doc) | TC-OPS-060 | P1 | ✅ | document, not code: [operations/backup-restore.md](../operations/backup-restore.md). |
| P0.4 | Restore drill | TC-OPS-020…024 | P0 | ⚠ | `scripts/restore-drill.sh` via `make restore-drill` — a manual end-to-end check, not a `_test.go`, not run in CI. |
| P0.5 | Freshness sentinel + precedence | TC-OPS-040…046 | P0 | ✅ | `modules/ops/state_test.go: TestComputeState, TestHoursSince`. |
| P1.6/P1.7 | Queue console, takeout | TC-OPS-080…105 | P1 | ✖ | no test. |

---

## Cross-cutting conventions (assert across all modules)

| CC | Convention | Representative cases | Cov | Evidence |
|----|-----------|---------------------|-----|----------|
| CC-1 | RFC-7807 on every non-2xx + i18n key | TC-MEDIA-110/111, TC-COMIC-160/161, TC-BANK-200/201, TC-NOTIFY-130, TC-JRNL-090/091, TC-STREAM-037, TC-CONT-100, TC-PPL-110/111, TC-OPS-122 | ⚠ | writer: `platform/server/server_test.go: TestProblemEmitsAllFourStandardMembers, TestProblemWithCannotOverrideStandardMembers, TestNotFoundCarriesModuleScopedType, TestDecodeRejectsMalformedJSONWithProblem`. Over HTTP, per module: `modules/comic/http_test.go: TestHTTPDraftIsNotFoundToAStranger, TestHTTPDeleteTwiceIs404`; `modules/bank/http_test.go: TestHTTPAnotherUsersAccountIsNotFound, TestHTTPDeleteAccountTwiceIs404`. The assertion those tests share — media type, `type`, `status`, `title` — has one owner, `platform/server/servertest/servertest_test.go: TestProblemAcceptsTheRealWriterAndRefusesTheRest` (it accepts the real writer's output and refuses a wrong type, a wrong status, a text/plain 404, a `status` member that disagrees with the HTTP status, and a missing or empty `title`). Since 2026-09-19 the same pair over HTTP in `modules/movie/http_test.go`, `modules/music/http_test.go`, `modules/story/http_test.go`, `modules/people/http_test.go`, `modules/social/http_test.go` (each: `TestHTTPDraftIsNotFoundToAStranger` / `TestHTTPPersonIsNotFoundToAStranger` / `TestHTTPConnectionIsNotFoundToAThirdParty` + `TestHTTPDeleteTwiceIs404`), `modules/notify/http_test.go: TestHTTPNotificationIsNotFoundToAStranger, TestHTTPMarkReadTwiceIsIdempotent`, `modules/journal/http_test.go: TestHTTPEntryIsNotFoundToAStranger, TestHTTPDeleteTwiceIs404` — layout has no per-id resource for the pair to apply to. Media, ops, account and tenant handlers use the same writer by construction since 2026-08-27 but have no HTTP test; the i18n catalogue (`frontend/src/lib/problems.ts`) is untested. |
| CC-2 | Permission grammar (2–3 seg, fail-closed, seeding) | TC-COMIC-040, TC-BANK-162, TC-NOTIFY-008, TC-JRNL-092, TC-PPL-112, TC-OPS-044/120 | ✅ | `modules/account/rbac/permission_test.go: TestParse, TestMatches, TestSetAllows, TestSetAllowsMalformedDenied`; escalation guards `modules/account/handler/admin_test.go` (32 tests: no-escalation, no-self-edit, last-approver, token_version bump). |
| CC-3 | Owner isolation (404 not 403, no list leak) | TC-MEDIA-069/082, TC-COMIC-031/162, TC-BANK-160, TC-NOTIFY-004, TC-JRNL-011, TC-STREAM-031, TC-CONT-021/045, TC-PPL-004, TC-OPS-103 | ✅ | over HTTP — 404 not 403, byte-identical to "never existed": `modules/comic/http_test.go: TestHTTPDraftIsNotFoundToAStranger`, `modules/bank/http_test.go: TestHTTPAnotherUsersAccountIsNotFound`, and since 2026-09-19 `modules/movie/http_test.go`, `modules/music/http_test.go`, `modules/story/http_test.go: TestHTTPDraftIsNotFoundToAStranger`, `modules/people/http_test.go: TestHTTPPersonIsNotFoundToAStranger`, `modules/social/http_test.go: TestHTTPConnectionIsNotFoundToAThirdParty`, `modules/notify/http_test.go: TestHTTPNotificationIsNotFoundToAStranger`, `modules/journal/http_test.go: TestHTTPEntryIsNotFoundToAStranger`. At the service: `modules/media/service_test.go: TestGetOwnerScoped`; `modules/journal/journal_test.go: TestGetOwnerScopedNotFound`; `modules/comic/comic_test.go: TestDraftVisibility, TestSaveProgressRespectsDraftVisibility`; `modules/bank/bank_test.go: TestOwnerScoping`; `modules/{movie,music,story}/*_test.go: TestDraftIsInvisibleToOthers`; `modules/social/social_test.go: TestRemoveOnlyByAParty`; at the database, CC-10. |
| CC-4 | Cursor pagination stable | TC-MEDIA-063, TC-COMIC-102, TC-BANK-033, TC-NOTIFY-005, TC-JRNL-012, TC-STREAM-030, TC-PPL-015 | ✅ | `platform/server/server_test.go: TestCursorRoundTripsTimestampKey, TestCursorRoundTripsSortKeyContainingSeparator, TestDecodeCursorRejectsGarbage, TestCursorIsURLSafe, TestLimitDefaultsAndClamps`; `modules/media/service_test.go: TestListPaginates`; `modules/journal/journal_test.go: TestListCursorPaginates`. |
| CC-5 | Events after-commit + fan-out edges registered + idempotent | TC-MEDIA-090/091/092, TC-COMIC-123/147, TC-BANK-140/144, TC-NOTIFY-074, TC-JRNL-030…032, TC-STREAM-014, TC-CONT-083, TC-PPL-113, TC-OPS-007 | ⚠ | after-commit `modules/journal/journal_test.go: TestCreateEmitsExactlyOnceAfterCommit, TestCreateRollbackPublishesNothing`; fan-out `platform/events/events_test.go`; idempotent consumers `modules/{movie,music,story}/*_test.go: TestAssetDeletedIsIdempotent`, `modules/journal/journal_test.go: TestStreamIdempotency`; publisher-failure tolerance `TestPublishSurvivesAFailingOrAbsentPublisher`. Gap: bank and comic never assert their own emits. |
| CC-6 | Money integer minor units, no floats | TC-BANK-026/202 | ⚠ | type-level (`int64` throughout `modules/bank`); `modules/bank/bank_test.go: TestInvalidAmount` rejects non-positive amounts; no test guards against a float creeping into an API body. |
| CC-7 | Migration-only schema + generated files not hand-edited + drift gates | TC-MEDIA-112/114, TC-COMIC-164, TC-BANK-204/205, all `-*` migration cases | ⚠ | CI, not a test: `.github/workflows/ci.yml` job `openapi` regenerates `api.gen.go` + `types.gen.ts` and fails on diff (ADR-10). There is **no** sqlc drift gate — sqlc output is not committed. Migrations: job `backend` applies the whole chain to a fresh `postgres:18` on every push (since 2026-09-11), so "applies from zero" is proven; there is still no round-trip (`down`) job. |
| CC-8 | Idempotent deletes (404 not 500) | TC-MEDIA-041, TC-COMIC-163, TC-BANK-203, TC-JRNL-023, TC-PPL-016 | ⚠ | over HTTP — 204 then 404: `modules/comic/http_test.go: TestHTTPDeleteTwiceIs404` (needed `DeleteComic` to become `:execrows` — a repeat used to answer 204), `modules/bank/http_test.go: TestHTTPDeleteAccountTwiceIs404`, and since 2026-09-19 `modules/movie/http_test.go`, `modules/music/http_test.go`, `modules/story/http_test.go`, `modules/people/http_test.go`, `modules/social/http_test.go`, `modules/journal/http_test.go: TestHTTPDeleteTwiceIs404` (movie, music and story needed their `Delete*` to become `:execrows` too — the module answered 204 to a repeat and left the 404 to cmd/api's owner guard) and `modules/notify/http_test.go: TestHTTPMarkReadTwiceIsIdempotent` (its one write). Event-consumer deletes: `modules/{movie,music,story}/*_test.go: TestAssetDeletedIsIdempotent`, `modules/journal/journal_test.go: TestStreamAssetDeletedRemoves`. HTTP deletes without a twice-test: media assets, account's admin user delete, comic chapters/pages and sync sources, music playlists, bank categories/transactions/transfers/debts. |
| CC-9 | Frontend state ownership + no fixtures | TC-MEDIA-065, TC-COMIC-103, TC-NOTIFY-090, TC-JRNL-054, TC-STREAM-050, TC-PPL-070 | ✖ | no frontend component test; the three vitest files under `frontend/src/lib/` cover pure rules only (run in CI since 2026-09-19). |
| CC-10 | Tenant scope per request + RLS at the database (ADR-07) | — | ✅ | request transaction: `modules/tenant/middleware/require_tenant_test.go: TestMutatingRequestCommitFailureBecomes500, TestMutatingRequestHandlerErrorRollsBack, TestMutatingRequestPanicRollsBackAndRepanics, TestUnauthenticatedRequestNeverOpensAScope, TestOversizedMutatingResponseStreamsIntact`. RLS itself: `platform/db/rls_test.go: TestRLSTenantCannotReadAnotherTenantsRows, TestRLSTenantCannotWriteIntoAnotherTenant, TestRLSTenantCannotRelocateARow, TestRLSTenantCannotDeleteAnotherTenantsRow, TestRLSWriteWithoutATenantScopeFails, TestRLSEveryProtectedTableHasAPolicyAndForce`, `platform/db/rls_media_test.go: TestRLSMediaMemberCannotReadAnotherMembersAsset, TestRLSMediaAdminOfAnotherTenantSeesNothing`, `platform/db/rls_social_test.go: TestRLSConnectionVisibleOnlyToItsTwoParties, TestRLSCannotForgeARequestFromAnotherUser`. Env-gated on `RLS_TEST_ADMIN_URL` / `RLS_TEST_APP_URL`; CI job `backend` sets both (since 2026-09-11). |
| CC-11 | Auth primitives | — | ✅ | `modules/account/auth/password_test.go: TestHashAndVerifyPassword, TestVerifyPasswordMalformed`; `modules/account/auth/reset_test.go` (3). No test for `/auth/login`'s lockout counters or for refresh-token reuse detection. |

## Risk coverage (from TEST-PLAN §10)

| Risk | Anchor cases | What actually covers it |
|------|-------------|-------------------------|
| R1 Money incorrectness | TC-BANK-024, -027, -050, -052, -058, -103, -122 | `bank_test.go` (15) + `debts_test.go` (2) — the strongest-covered risk. |
| R2 Data loss (backup/delete) | TC-OPS-001, -004, -005, -020, -022; TC-MEDIA-040, -045 | delete side ✅ (`TestDeleteAsset`, `TestPurgeOrphans`, consumers); backup side only `retention_test.go` + the manual drill. |
| R3 Cross-user leak | all CC-3 cases | CC-3 service tests and the CC-10 RLS suite both run in CI. |
| R4 Stream/event correctness | TC-STREAM-004…016, TC-MEDIA-091, TC-BANK-144 | `journal_test.go` stream suite + `events_test.go`. |
| R5 Account takeover | TC-NOTIFY-041…050, -044, -043 | `admin_test.go` (32) for escalation; `reset_test.go`, `password_test.go`. Login lockout and refresh reuse detection: no test. |
| R6 Worker OOM / stuck queue | TC-MEDIA-009, -010, -015, -047 | none — the guard is structural (`heavyConcurrency = 1`). |
| R7 Capture not saved | TC-JRNL-059, TC-STREAM-051, TC-PPL-054, TC-COMIC-088 | `TestCreateEmitsExactlyOnceAfterCommit` (journal); the rest are frontend, untested. |

## Coverage summary (re-graded 2026-09-11; SPEC-12 rows added 2026-09-19)

| Spec | P0 rows | ✅ | ⚠ | ✖ | Notes |
|------|---------|----|----|----|-------|
| SPEC-01 media | 6 | 3 | 3 | 0 | the image worker's rules and the poster plan are tested since 2026-09-19; `transcode` and the orchestration around the workers are not |
| SPEC-02 comic | 6 | 2 | 2 | 2 | reader/library are frontend |
| SPEC-03 bank | 8 (+1 P1) | 6 | 2 | 0 | best-covered module; emits unasserted |
| SPEC-10 ledger expansion | 1 | 0 | 1 | 0 | no case document yet |
| SPEC-04 notify | 5 | 2 | 2 | 1 | store/read API untested |
| SPEC-05 journal | 4 | 3 | 0 | 1 | P1.5/P1.6 now ⚠ via SPEC-12 (backend proven; picker not) |
| SPEC-12 journal attachments | 6 | 6 | 0 | 0 | added 2026-09-19; P1 rows 2 ✅ / 0 ⚠ / 1 ✖ |
| SPEC-06 stream | 4 | 1 | 2 | 1 | |
| SPEC-07 continue | 4 | 3 | 0 | 1 | |
| SPEC-08 people | 4 | 1 | 2 | 1 | lunar untested |
| SPEC-09 ops | 4 (+1 doc) | 1 | 3 | 0 | backup/restore proven manually only |
| **Total** | **52 P0** | **28** | **17** | **7** | plus 1 doc row (✅) and 13 P1 rows (4 ✅, 4 ⚠, 5 ✖) |

Cross-cutting: CC-2/3/4/11 ✅ · CC-1/5/6/7/8/10 ⚠ (CC-1 and CC-8 are proven over HTTP for comic, bank, journal, movie, music, story, people, social and notify since 2026-09-19; ⚠ for the media/ops/account/tenant handlers and the i18n catalogue) · CC-9 ✖.

What this says, in one line: **the service layer of every backend module is
tested, and comic, bank, journal, movie, music, story, people, social and notify
hold their HTTP contracts (status codes, 7807 bodies, 404-not-403, delete-twice)
under test; the workers, the media/ops/account/tenant HTTP surfaces, and the
frontend beyond three vitest files of pure rules are not.** Closing a ⚠/✖ means
adding a named test and putting it in the Evidence cell — nothing else moves a
mark.

> Any P0 row at ⚠/✖ is a **coverage gap** and must be closed before the affected
> spec can be signed off (TEST-PLAN §6.2). The 2026-08-25 audit's list of what no
> test can reach still applies.
