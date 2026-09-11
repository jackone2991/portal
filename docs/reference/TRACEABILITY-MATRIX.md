# Traceability Matrix — Requirements ↔ Test Cases

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



Maps every spec requirement (`Px.y`) and cross-cutting convention (`CC-n`) to the
test cases that cover it. **Coverage gate:** every `P0` requirement must map to
≥1 test case; a `P0` with no case is a coverage defect. Update the **Result**
column each run (`PASS` / `FAIL` / `PARTIAL` / `N/A`).

Legend: **Pri** = highest priority among mapped cases · **Cov** = ✅ covered / ⚠ partial / ✖ gap.

---

## SPEC-01 — Media ([cases](TEST-CASES-SPEC-01-media.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Image ingest, sniff, HEIC/size/dim/animated, variants, orientation | TC-MEDIA-001…019 | P0 | ✅ | |
| P0.2 | Video poster + audio-only skip + non-fatal | TC-MEDIA-030…033 | P0 | ✅ | |
| P0.3 | Delete asset + janitor + event | TC-MEDIA-040…048 | P0 | ✅ | |
| P0.4 | Library page + filters + cursor + LCP | TC-MEDIA-060…069 | P0 | ✅ | |
| P0.5 | Download original (checksum, private, states) | TC-MEDIA-080…086 | P0 | ✅ | |
| P0.6 | Event fan-out prerequisite | TC-MEDIA-090…093 | P0 | ✅ | |
| P1.1/P1.2 | Metadata edit, asset_ready emit | TC-MEDIA-100…102 | P1 | ✅ | |

## SPEC-02 — Comic ([cases](TEST-CASES-SPEC-02-comic.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Entities + CRUD + asset validation | TC-COMIC-001…015 | P0 | ✅ | |
| P0.2 | Publish flow + owner-or-elevated RBAC | TC-COMIC-030…040 | P0 | ✅ | |
| P0.3 | Reader vertical scroll | TC-COMIC-060…066 | P0 | ✅ | |
| P0.4 | Reading progress keyed by page_id | TC-COMIC-080…088 | P0 | ✅ | |
| P0.5 | Library + detail | TC-COMIC-100…104 | P0 | ✅ | |
| P0.6 | Asset-deletion coupling | TC-COMIC-120…123 | P0 | ✅ | |
| P1.7/P1.9 | Zip import, chapter events | TC-COMIC-140…148 | P1 | ✅ | |

## SPEC-03 — Bank ([cases](TEST-CASES-SPEC-03-bank.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Accounts + derived balance + immutability | TC-BANK-001…008 | P0 | ✅ | |
| P0.2 | Transactions + direction/kind + reconciliation | TC-BANK-020…034 | P0 | ✅ | |
| P0.3 | Transfers + fees + leg predicate | TC-BANK-050…059 | P0 | ✅ | |
| P0.4 | Categories hierarchy + delete/reassign | TC-BANK-070…082 | P0 | ✅ | |
| P0.5 | Monthly budgets tree | TC-BANK-100…108 | P0 | ✅ | |
| P0.6 | Dashboard (balances vs flows) | TC-BANK-120…124 | P0 | ✅ | |
| P0.7 | Events (+ bulk carve-out) | TC-BANK-140…144 | P0 | ✅ | |
| P0.8 | RBAC owner isolation + seeds | TC-BANK-160…164 | P0 | ✅ | |
| P0.9 | Import scaffolding | TC-BANK-180…181 | P1 | ✅ | |

## SPEC-04 — Notify ([cases](TEST-CASES-SPEC-04-notify.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Store + read API | TC-NOTIFY-001…008 | P0 | ✅ | |
| P0.2 | Dispatch fan-out + muted/channels/dedup | TC-NOTIFY-020…027 | P0 | ✅ | |
| P0.3 | Email + password reset + abuse controls | TC-NOTIFY-040…053 | P0 | ✅ | |
| P0.4 | First consumer (media:asset_ready) | TC-NOTIFY-070…074 | P0 | ✅ | |
| P0.5 | Bell wiring | TC-NOTIFY-090…094 | P0 | ✅ | |
| P1.1–P1.4 | Web push, SSE, prefs UI, security alert | TC-NOTIFY-110…113 | P1 | ✅ | |

## SPEC-05 — Journal ([cases](TEST-CASES-SPEC-05-journal.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Module scaffold | TC-JRNL-001…002 | P0 | ✅ | |
| P0.2 | Entries CRUD + validation + ordering | TC-JRNL-010…023 | P0 | ✅ | |
| P0.3 | Event emit (emit-only) | TC-JRNL-030…033 | P0 | ✅ | |
| P0.4 | Composer + home + sanitization | TC-JRNL-050…059 | P0 | ✅ | |
| P1.5/P1.6 | Attachments, mood picker | TC-JRNL-070…074 | P1 | ✅ | |

## SPEC-06 — Stream ([cases](TEST-CASES-SPEC-06-stream.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Projection consumers + idempotency + backfill | TC-STREAM-001…017 | P0 | ✅ | |
| P0.2 | Stream read API + title/href synthesis | TC-STREAM-030…037 | P0 | ✅ | |
| P0.3 | Home replacement | TC-STREAM-050…052 | P0 | ✅ | |
| P0.4 | Widget rail + failure isolation | TC-STREAM-070…072 | P0 | ✅ | |
| P1.5/P1.6 | Memories, backfill task | TC-STREAM-090…092 | P1 | ✅ | |

## SPEC-07 — Continue ([cases](TEST-CASES-SPEC-07-continue.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Progress table + NULL-duration rule | TC-CONT-001…003 | P0 | ✅ | |
| P0.2 | Beacon (resume, clamp, owner, fire-forget) | TC-CONT-020…029 | P0 | ✅ | |
| P0.3 | Aggregator + inclusion predicate | TC-CONT-040…047 | P0 | ✅ | |
| P0.4 | Resume UX | TC-CONT-060…064 | P0 | ✅ | |
| P1.5 | Completion event (latch) | TC-CONT-080…083 | P1 | ✅ | |

## SPEC-08 — People ([cases](TEST-CASES-SPEC-08-people.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.2 | CRUD + birthday validation + notice reset | TC-PPL-001…016 | P0 | ✅ | |
| P0.3 | Upcoming endpoint (TZ, Feb-29, lunar, clamp) | TC-PPL-030…036 | P0 | ✅ | |
| P0.4 | Scan + event (dedup, catch-up, outbox) | TC-PPL-050…057 | P0 | ✅ | |
| P0.5 | Frontend (BirthdayCard, empty, gate) | TC-PPL-070…073 | P0 | ✅ | |
| P1.6/P1.7 | Interactions, avatar | TC-PPL-090…092 | P1 | ✅ | |

## SPEC-09 — Ops ([cases](TEST-CASES-SPEC-09-ops.md))

| Req | Summary | Test cases | Pri | Cov | Result |
|-----|---------|-----------|-----|-----|--------|
| P0.1 | Scaffold + seeding + system-scoped tables | TC-OPS-120…123 | P0 | ✅ | |
| P0.2 | Nightly backup + retention + manifest | TC-OPS-001…007 | P0 | ✅ | |
| P0.3 | Media durability posture (doc) | TC-OPS-060 | P1 | ✅ | |
| P0.4 | Restore drill | TC-OPS-020…024 | P0 | ✅ | |
| P0.5 | Freshness sentinel + precedence | TC-OPS-040…046 | P0 | ✅ | |
| P1.6/P1.7 | Queue console, takeout | TC-OPS-080…105 | P1 | ✅ | |

---

## Cross-cutting conventions (assert across all modules)

| CC | Convention | Representative cases |
|----|-----------|---------------------|
| CC-1 | RFC-7807 on every non-2xx + i18n key | TC-MEDIA-110/111, TC-COMIC-160/161, TC-BANK-200/201, TC-NOTIFY-130, TC-JRNL-090/091, TC-STREAM-037, TC-CONT-100, TC-PPL-110/111, TC-OPS-122 |
| CC-2 | Permission grammar (2–3 seg, fail-closed, seeding) | TC-COMIC-040, TC-BANK-162, TC-NOTIFY-008, TC-JRNL-092, TC-PPL-112, TC-OPS-044/120 |
| CC-3 | Owner isolation (404 not 403, no list leak) | TC-MEDIA-069/082, TC-COMIC-031/162, TC-BANK-160, TC-NOTIFY-004, TC-JRNL-011, TC-STREAM-031, TC-CONT-021/045, TC-PPL-004, TC-OPS-103 |
| CC-4 | Cursor pagination stable | TC-MEDIA-063, TC-COMIC-102, TC-BANK-033, TC-NOTIFY-005, TC-JRNL-012, TC-STREAM-030, TC-PPL-015 |
| CC-5 | Events after-commit + fan-out edges registered + idempotent | TC-MEDIA-090/091/092, TC-COMIC-123/147, TC-BANK-140/144, TC-NOTIFY-074, TC-JRNL-030…032, TC-STREAM-014, TC-CONT-083, TC-PPL-113, TC-OPS-007 |
| CC-6 | Money integer minor units, no floats | TC-BANK-026/202 |
| CC-7 | Migration-only schema + generated files not hand-edited + drift gates | TC-MEDIA-112/114, TC-COMIC-164, TC-BANK-204/205, all `-*` migration cases |
| CC-8 | Idempotent deletes (404 not 500) | TC-MEDIA-041, TC-COMIC-163, TC-BANK-203, TC-JRNL-023, TC-PPL-016 |
| CC-9 | Frontend state ownership + no fixtures | TC-MEDIA-065, TC-COMIC-103, TC-NOTIFY-090, TC-JRNL-054, TC-STREAM-050, TC-PPL-070 |

## Risk coverage (from TEST-PLAN §10)

| Risk | Anchor cases |
|------|-------------|
| R1 Money incorrectness | TC-BANK-024, -027, -050, -052, -058, -103, -122 |
| R2 Data loss (backup/delete) | TC-OPS-001, -004, -005, -020, -022; TC-MEDIA-040, -045 |
| R3 Cross-user leak | all CC-3 cases (esp. TC-BANK-160, TC-JRNL-011, TC-PPL-004) |
| R4 Stream/event correctness | TC-STREAM-004…016, TC-MEDIA-091, TC-BANK-144 |
| R5 Account takeover | TC-NOTIFY-041…050, -044, -043 |
| R6 Worker OOM / stuck queue | TC-MEDIA-009, -010, -015, -047 |
| R7 Capture not saved | TC-JRNL-059, TC-STREAM-051, TC-PPL-054, TC-COMIC-088 |

## Coverage summary

| Spec | P0 reqs | Cases (P0+P1) | Notes |
|------|---------|---------------|-------|
| SPEC-01 media | 6 | 61 | full |
| SPEC-02 comic | 6 | 65 | full; zip import P1 tagged |
| SPEC-03 bank | 9 | 78 | highest case count (money risk) |
| SPEC-04 notify | 5 | 47 | email/reset security-heavy |
| SPEC-05 journal | 4 | 39 | sanitization + persistence |
| SPEC-06 stream | 4 | 38 | projection/idempotency-heavy |
| SPEC-07 continue | 4 | 33 | resume + predicate |
| SPEC-08 people | 4 | 43 | timezone/Feb-29/outbox |
| SPEC-09 ops | 6 | 32 | backup/restore/takeout |
| **Total** | **48 P0 reqs** | **436 cases** | every P0 requirement mapped |

> Any row whose **Cov** is ⚠/✖ after a review pass is a **coverage gap** and must
> be closed before the affected spec can be signed off (TEST-PLAN §6.2).
