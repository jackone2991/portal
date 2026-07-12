export const meta = {
  name: 'spec-gap-review',
  description: 'Multi-lens BA review of docs/product/specs — find gaps/errors, adversarially verify',
  phases: [
    { title: 'Review', detail: '10 per-file + 8 cross-cutting finders' },
    { title: 'Merge', detail: 'dedup into canonical findings' },
    { title: 'Verify', detail: 'adversarial refute + fix audit per file group' },
  ],
}

const R = '/Users/kirito/data/git/ops/repo/portal'
const S = R + '/docs/product/specs'
const B = R + '/docs/product/briefs'

const SRC = {
  readme: S + '/README.md',
  modules: R + '/backend/MODULES.md',
  events: R + '/docs/reference/events.md',
  perm: R + '/backend/internal/modules/account/rbac/permission.go',
  rbacMig: R + '/backend/db/migrations/0003_account_rbac.up.sql',
  inventory: R + '/docs/product/feature-inventory.md',
  feClaude: R + '/frontend/CLAUDE.md',
  frontendDoc: R + '/docs/architecture/frontend.md',
}

const SPECS = [
  { file: 'SPEC-01-media-image-pipeline.md', brief: B + '/01-media-image-pipeline.md' },
  { file: 'SPEC-02-comic-vertical.md', brief: B + '/02-comic-vertical.md' },
  { file: 'SPEC-03-finance-ledger.md', brief: B + '/03-finance-ledger.md' },
  { file: 'SPEC-04-notification-module.md', brief: null },
  { file: 'SPEC-05-journal.md', brief: B + '/05-journal-life-stream.md' },
  { file: 'SPEC-06-life-stream-home.md', brief: B + '/06-life-stream-home.md' },
  { file: 'SPEC-07-continue-rail.md', brief: B + '/07-continue-rail.md' },
  { file: 'SPEC-08-people-registry.md', brief: B + '/08-people-registry.md' },
  { file: 'SPEC-09-platform-ops.md', brief: B + '/09-platform-ops.md' },
  { file: 'README.md', brief: B + '/README.md' },
]

const FIND_SCHEMA = {
  type: 'object',
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          file: { type: 'string', description: 'spec filename under docs/product/specs/ where the fix applies (e.g. SPEC-06-life-stream-home.md), or MULTIPLE for cross-file defects' },
          section: { type: 'string' },
          severity: { type: 'string', enum: ['critical', 'major', 'minor'] },
          category: { type: 'string' },
          summary: { type: 'string' },
          evidence: { type: 'string', description: 'quoted conflicting text with path:line cites' },
          proposed_fix: { type: 'string', description: 'concrete replacement text or precise instruction' },
        },
        required: ['file', 'severity', 'summary', 'evidence', 'proposed_fix'],
      },
    },
  },
  required: ['findings'],
}

const VERDICTS_SCHEMA = {
  type: 'object',
  properties: {
    verdicts: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          index: { type: 'integer' },
          verdict: { type: 'string', enum: ['confirmed', 'downgraded', 'refuted'] },
          reasoning: { type: 'string' },
          revised_fix: { type: 'string' },
        },
        required: ['index', 'verdict', 'reasoning'],
      },
    },
  },
  required: ['verdicts'],
}

const RULES = [
  'You are a senior business analyst at a large IT firm reviewing implementation-ready PRD specs for a Go modular-monolith + Next.js self-hosted platform.',
  'Ground rules:',
  '- READ-ONLY task: never edit or write any file.',
  '- Evidence or it did not happen: every finding must quote or cite (path:line) the conflicting/incorrect text.',
  '- Report substance only. DO NOT report: markdown lint/style, tone, length, heading style, table pipe spacing.',
  '- DO NOT re-litigate items a spec already lists as a resolved decision or a non-blocking open question with an owner — unless the resolution is internally contradictory or factually wrong.',
  "- Severity: critical = following the spec as written causes a wrong implementation, silent data loss, security hole, or an impossible requirement; major = an implementer would stall, guess, or diverge (missing decision, contradiction, wrong dependency/API/schema); minor = incorrect reference/link/number, stale naming, small omission.",
  '- proposed_fix must be concrete and actionable (exact replacement text where feasible), consistent with the binding conventions in ' + SRC.readme + '.',
  '- Today is 2026-07-10. SPEC-05..09 were drafted today from briefs 05..09; SPEC-01..04 predate them. Migrations 0001-0007 are consumed; specs use 000N placeholders deliberately (not a defect).',
  '',
].join('\n')

const perSpecPrompt = (s) => RULES + [
  'TARGET SPEC: ' + S + '/' + s.file,
  s.brief ? 'UPSTREAM BRIEF (coverage source): ' + s.brief : 'No upstream brief — this spec came straight from a gap audit.',
  'Also read for conventions/cross-refs: ' + SRC.readme + ', ' + SRC.events + ', and any file the spec links when needed to validate a specific claim.',
  '',
  'Review the target spec for:',
  '1) Internal contradictions: behavior prose vs acceptance criteria vs §6 data model vs §7 API table vs §9 timeline.',
  '2) Brief coverage: every P0/P1/P2 requirement, user story, open question, and locked recommendation in the upstream brief must be either covered or explicitly dropped/deferred with rationale. List anything silently lost or silently changed.',
  '3) Unimplementable or ambiguous requirements: anything an engineer could not build without guessing (missing field semantics, undefined behavior on edge cases the spec itself raises).',
  '4) Wrong cross-references: § numbers, SPEC numbers, file paths/links, D-N ids, ADR ids, event names.',
  '5) Acceptance-criteria quality: untestable ACs, P0 behavior without any AC, ACs contradicting each other or the data model.',
  'Return all findings via StructuredOutput. An empty findings array is a legitimate answer if the spec is clean.',
].join('\n')

const CROSS = [
  {
    key: 'rbac',
    prompt: RULES + [
      'CROSS-CUTTING LENS: RBAC permission-code validity and coherence.',
      'Read ' + SRC.perm + ' (the REAL grammar: Parse segment rules, scope tokens, wildcard + AllowsCode semantics), ' + SRC.rbacMig + ' (seeded roles/permissions house style), and every §7 permission column + §5 RBAC prose in all ten files under ' + S + '/.',
      'Evaluate:',
      "- EVERY drafted permission code against the real parser: segment count, allowed scope tokens, wildcard behavior. Explicitly test: 'comics:read:published' and 'comics:publish:own' (SPEC-02), 'media:asset:delete:own' / 'media:asset:read:own' / 'media:asset:update:own' (SPEC-01), the 'bank:account:read:own'-style 4-segment family (SPEC-03), 'media:progress:own' (SPEC-07), 'stream:read:own' (SPEC-06), 'journal:create/read:own/update:own/delete:own' (SPEC-05), 'people:*' codes (SPEC-08), 'ops:read', 'queues:read', 'takeout:create:own', 'takeout:read:own' (SPEC-09), and SPEC-04 §7's codes.",
      '- Whether each spec states WHO gets each permission (role seeding) and whether the grant mechanics are specified (which migration seeds the grants) — flag specs that leave seeding unowned.',
      '- Naming coherence across specs (resource-noun style: SPEC-04 uses kebab compound resources like notification-prefs). If drafts diverge, propose ONE coherent reconciliation scheme covering every invalid/ambiguous code, with exact corrected codes in proposed_fix.',
    ].join('\n'),
  },
  {
    key: 'events',
    prompt: RULES + [
      'CROSS-CUTTING LENS: event/payload/consumer consistency and event-driven failure modes.',
      'Read ' + SRC.events + ', all ten files under ' + S + '/, and ' + B + '/06-life-stream-home.md.',
      'Explicitly adjudicate these candidate issues (confirm with evidence or refute), then hunt for more of the same class:',
      "(a) SPEC-06 stream_items UNIQUE(source_module, event_type, ref_id) vs recurring events: people:birthday_upcoming fires at days_until 3 AND 0 for the same person, and again every year. With ref_id = person_id, do the day-of event and every later year silently vanish on the unique key? What ref should be used (e.g. a notice id emitted in the payload)?",
      "(b) Bank transfers emit one bank:transaction_created per leg (2 rows, SPEC-03 P0.3/P0.7). Does SPEC-06 render two stream items for one transfer, contradicting SPEC-03 P0.7's 'group one transfer into a single story item' intent? Should the stream key on transfer_id for is_transfer legs?",
      "(c) SPEC-02 P1.7 zip import creates up to 300 image assets via mediaapi; each reaching ready fires media:asset_ready (SPEC-01 P1.2 'whenever any asset reaches ready'). Does SPEC-04's bell get 300 notifications and SPEC-06's stream 300 items from one chapter import? Is suppression specced anywhere? Propose a concrete cross-spec fix (e.g. an origin/batch marker in the payload + consumer policy).",
      "(d) SPEC-06 P0.3 AC 'survives a refetch (projection row present)' vs the journal projection being an async Asynq consumer — a refetch can race projection lag. Given journal and stream_items are the SAME module, is the bus round-trip for journal rows even necessary, or should the projection row be written transactionally on entry create (event still emitted for future external consumers)? Judge which design the spec should mandate.",
      '(e) Payload contracts: every field a consumer spec reads must exist in the emitter spec payload AND in events.md (check media:asset_ready fields SPEC-06 needs for cards, people:birthday_upcoming fields incl. any age/name needs, ops events).',
      '(f) Registry closure: every event/task a spec emits or consumes exists in events.md with matching name/payload/consumers — and no registry row is orphaned.',
    ].join('\n'),
  },
  {
    key: 'deps',
    prompt: RULES + [
      'CROSS-CUTTING LENS: dependency graph, sequencing, status coherence.',
      'Read ' + S + '/README.md, the header block (Status/Depends on/Downstream) and §9 timeline of all 9 SPEC files, and ' + B + '/README.md.',
      'Check: dependency claims are mutually consistent and acyclic; the suggested implementation order respects every Depends-on; downstream-consumer lists are reciprocal (if X lists Y downstream, Y references X); ordinal/arithmetic claims are right (e.g. SPEC-05 claims journal would be "the third module to be wired end-to-end" — count what is wired today (account, media) plus what the build order wires first (notify per SPEC-04) and verify); the SPEC-09-P0-before-SPEC-03-data pressure is reflected consistently; §9 phase-effort sums match each stated total; statuses agree between briefs README, specs README, and spec headers.',
    ].join('\n'),
  },
  {
    key: 'reality',
    prompt: RULES + [
      'CROSS-CUTTING LENS: repo-reality — verify factual claims the specs make about this codebase. Use Read/Grep/Glob on the repo. Report a finding ONLY where a spec asserts something false, or presents as fact what the code already contradicts (a spec saying "verify before building" is fine unless the truth is knowable now and contradicts it — then report with the truth).',
      'Claims to verify (cite code paths in evidence):',
      '1. media_assets has a duration column usable for SPEC-07 progress_pct (check ' + R + '/backend/db/migrations/0007_media_assets.up.sql and the media module).',
      "2. users carry a timezone column (SPEC-08 P0.3 'the stored user timezone (D-17)') — check migrations 0002/0006 and D-17 in " + SRC.inventory + '.',
      '3. HomeView renders a ~685-line hard-coded newsfeed; Composer/post/comment kits are exported but imported by nothing (' + R + '/frontend/src/templates/v1/views/home/HomeView.tsx; grep Composer imports).',
      '4. NotificationsMenu in NotifMenus.tsx renders a hard-coded NOTIFS fixture (SPEC-04 P0.5).',
      '5. Components named BirthdayCard, FriendCard, PersonalInfoWidget, WidgetCard, ActivityFeed exist under ' + R + '/frontend/src/templates/v1/ (SPEC-06 P0.4 and SPEC-08 P0.5 treat them as dormant kits to "wire"/"reuse"; if any is absent, those specs must say build-new, not wire).',
      "6. docker-compose: pgbouncer service exists (SPEC-09 P0.2's 'not through PgBouncer' note must make sense), mailpit absent today (SPEC-04 adds it).",
      "7. Asynq queue names/weights in " + R + "/backend/cmd/worker/main.go ('transcode' 5, 'thumbnail' 3, 'default' 1) — are SPEC-01's shared low-concurrency heavy-queue plan and SPEC-04's 'default queue (weight 1)' statements consistent with what exists?",
      '8. platform/storage uploader: does it accept io.Reader streaming (answers SPEC-09 §10 open question — if determinable, the spec should state the answer)?',
      '9. Makefile: no existing target name collides with restore-drill; cited targets exist.',
      '10. ' + SRC.inventory + ': D-17, D-20, D-25, D-29 actually say what the specs claim.',
      '11. docs/product/analysis/gap-audit-2026-07.md does NOT exist on disk — SPEC-04 links it as Upstream. Find where its content went (backlog.md? git history?) and propose the correct retarget.',
      '12. A /healthz endpoint exists in ' + R + '/backend/cmd/api (SPEC-09 P0.5 references it as the thing to leave untouched).',
      '13. ' + SRC.frontendDoc + ' has §8 performance budgets (multiple specs cite frontend.md §8).',
      "14. SPEC-08/09 cite 'the shared periodic runner (SPEC-01 P0.3 convention)' — read SPEC-01 P0.3 and confirm it actually establishes a periodic-task convention they can ride.",
      '15. Vidstack is the actual player dependency (package.json / player component) for SPEC-07 beacon/resume claims.',
    ].join('\n'),
  },
  {
    key: 'sql',
    prompt: RULES + [
      'CROSS-CUTTING LENS: data-model/SQL correctness.',
      'Read every §6 Data-model SQL block in all 9 SPEC files under ' + S + '/, plus ' + R + '/backend/db/migrations/0007_media_assets.up.sql and 0003_account_rbac.up.sql for house style.',
      'Check: (a) Postgres-17 validity; (b) prose↔DDL mismatches (constraints the text promises but DDL lacks, or vice versa); (c) missing indexes for access paths the same spec\'s endpoints require (cursor keys, dedup lookups, janitor scans, unread predicates); (d) ON DELETE behaviors vs described semantics (cascades that contradict prose, SET NULL columns declared NOT NULL, etc.); (e) cross-spec collisions: duplicate table names, shared-sequence migration contention handled per convention, identity-anchor users(id) FK applied consistently where specs claim it; (f) uuid[] columns: integrity strategy stated; (g) CHECK constraints that reject legal states the prose allows or admit illegal ones (e.g. birthday month/day/year combos, transfer-leg CHECK in bank, status enums).',
      'Report with exact corrected DDL in proposed_fix.',
    ].join('\n'),
  },
  {
    key: 'api',
    prompt: RULES + [
      'CROSS-CUTTING LENS: API-contract completeness and Problem-type closure.',
      'Read all §7 API-summary tables + §5 requirement bodies in all 9 SPEC files + ' + SRC.readme + ' conventions.',
      'Check: every endpoint mentioned in prose appears in its §7 table and vice versa (methods and paths matching); every Problem type used anywhere in ACs/prose is declared in that spec\'s Problem-types line and follows <module>/<kebab-case>; pagination conventions coherent (cursor vs page — flag unexplained divergence between specs); a permission (or explicit "authenticated"/"public") is present for every row; response shapes are defined wherever ANOTHER spec consumes them (SPEC-06 widgets consume SPEC-03 /bank/dashboard, SPEC-07 /continue, SPEC-08 /people/upcoming-birthdays, SPEC-04 /me/notifications — do the producer specs define the fields the widgets need?); the x-required-permission extension note is consistently applied.',
    ].join('\n'),
  },
  {
    key: 'frontend',
    prompt: RULES + [
      'CROSS-CUTTING LENS: frontend implementability and convention compliance.',
      'Read the frontend requirements: SPEC-02 P0.3/P0.5, SPEC-04 P0.5, SPEC-05 P0.4, SPEC-06 P0.3/P0.4, SPEC-07 P0.2/P0.4, SPEC-08 P0.5, SPEC-03 §8 — plus ' + SRC.feClaude + ', ' + R + '/frontend/src/templates/README.md, and the actual tree under ' + R + '/frontend/src/templates/v1/.',
      'Check: named components/routes exist, or the spec explicitly says to create them (never "wire"/"reuse" something absent); D-32 (TanStack owns server state, never Zustand), D-33 (RSC-first decision tree), D-34 (SessionKeeper is auth-only) are respected by each requirement as written; route claims fit the (app)/(public) route-group + version-switched templates/v{N} registry architecture (a new page needs a template-tree view + registry entry — do specs acknowledge that where they add routes like /people, /bank, /library/media?); client-island vs RSC labels are coherent; the SPEC-07 beacon (sendBeacon/keepalive on pagehide) actually works under the cookie model (portal_access HttpOnly SameSite=Strict Path=/ — do beacons carry auth? any CSRF/token-expiry pitfall worth a spec note?).',
    ].join('\n'),
  },
  {
    key: 'critic',
    prompt: RULES + [
      'CROSS-CUTTING LENS: completeness critic — what does the spec SET as a whole still fail to cover for its stated purpose (implementation-ready, gap-free)?',
      'Read all ten files in ' + S + '/, ' + B + '/README.md, ' + SRC.events + ', and ' + R + '/MILESTONE_CHECKS.md if it exists.',
      'Consider (report only actionable gaps with concrete fixes, not philosophy):',
      '- RBAC permission seeding: multiple specs say "granted to the base user role in the seed" — does any spec own the mechanics (data migration pattern, which migration file)? Should the conventions section?',
      '- i18n: conventions say Problem type URIs are also i18n keys (D-7) — do the new specs (05-09) carry any i18n obligation note, and is that consistent with 01-04?',
      '- Security posture of new surfaces: SPEC-09 asynqmon at /admin/queues (session gating vs its own assets, CSRF), takeout download URL signing/TTL, ops status information disclosure.',
      '- NFRs: stream page size/limits (SPEC-06), scan cost bounds (SPEC-08 at n=1000 people), backup duration/window and dump size growth (SPEC-09), beacon write amplification (SPEC-07).',
      '- Header hygiene: SPEC-01 carries Status/Last-verified/rev history (§11); SPEC-02/03 lack Status headers and revision history — should the set be normalized? ',
      '- Docs-to-update-in-same-PR obligations: SPEC-03 §7 knowingly diverges from D-14/D-7 money-wire rule and asks to reconcile frontend.md §5.3 — is that tracked anywhere actionable? Similar dangling obligations elsewhere?',
      '- Definition-of-done consistency: events.md registration, openapi.yaml landing, MILESTONE_CHECKS.md updates — uniformly stated across specs?',
      '- Anything a brief promised that NO spec picked up at all.',
    ].join('\n'),
  },
]

// ---- Phase 1: finders (barrier justified: dedup needs the full set) ----
const finderThunks = []
for (const s of SPECS) {
  finderThunks.push(() => agent(perSpecPrompt(s), { label: 'spec:' + s.file.replace('.md', ''), phase: 'Review', schema: FIND_SCHEMA, effort: 'high' }))
}
for (const c of CROSS) {
  finderThunks.push(() => agent(c.prompt, { label: 'x:' + c.key, phase: 'Review', schema: FIND_SCHEMA, effort: 'high' }))
}
const raw = (await parallel(finderThunks)).filter(Boolean).flatMap((r) => r.findings || [])
log('Review complete: ' + raw.length + ' raw findings')

// ---- Phase 2: dedup/merge ----
const dedupPrompt = RULES + [
  'You are the dedup/merge editor for ' + raw.length + ' raw findings from 18 independent reviewers (JSON below).',
  'Merge findings that describe the SAME underlying defect (even when filed under different files — set file to where the fix belongs, or MULTIPLE). Keep distinct defects separate — when unsure, keep separate.',
  'For merged items keep: the sharpest evidence, the most complete proposed_fix, the max severity.',
  'Drop only: pure style/lint complaints, and findings that merely restate a documented non-blocking open question without showing it is wrong.',
  'Do NOT invent new findings. Output every surviving finding, sorted critical → major → minor.',
  'RAW FINDINGS JSON:',
  JSON.stringify(raw),
].join('\n')
const merged = await agent(dedupPrompt, { label: 'dedup', phase: 'Merge', schema: FIND_SCHEMA, effort: 'high' })
const canon = (merged && merged.findings) || []
log('Merge complete: ' + canon.length + ' canonical findings')

// ---- Phase 3: adversarial verify, grouped by file ----
const byFile = {}
for (const f of canon) {
  const k = f.file || 'MULTIPLE'
  if (!byFile[k]) byFile[k] = []
  byFile[k].push(f)
}

const refuterPrompt = (file, json) => RULES + [
  'ADVERSARIAL VERIFICATION for findings on: ' + (file === 'MULTIPLE' ? 'multiple files under ' + S + '/' : S + '/' + file),
  'Default stance: each finding is WRONG until you re-verify it against the actual files. Re-read the cited files yourself — never trust the quoted evidence.',
  "For each indexed finding return verdict: 'confirmed' (evidence checks out, severity apt) | 'downgraded' (real but overstated or partially wrong — explain what part survives) | 'refuted' (not a real defect — quote the source text that disproves it).",
  'Also refute findings that merely restate a documented, deliberate decision that has its rationale in place.',
  "IMPORTANT — the spec files were REVISED on 2026-07-10 AFTER these findings were captured; many defects have since been fixed in place (look for '(2026-07-10)' / 'rev 3' annotations). If the current file no longer exhibits the defect, return 'refuted' with reasoning beginning 'FIXED:' — that counts as resolved, not as a false positive. Only 'confirmed'/'downgraded' verdicts represent defects still open in the CURRENT text.",
  'FINDINGS JSON:',
  json,
].join('\n')

const fixAuditPrompt = (file, json) => RULES + [
  'FIX AUDIT for findings on: ' + (file === 'MULTIPLE' ? 'multiple files' : S + '/' + file),
  'Assume each defect below was real when captured. The spec files were REVISED on 2026-07-10 after capture: first check whether the current file already resolves the defect — if yes, judge the APPLIED fix (the current text) instead of the proposed one and return verdict for it; if the applied fix is sound, return \'refuted\' with reasoning beginning \'FIXED:\'. Otherwise judge the proposed_fix: would applying it fully resolve the defect without contradicting the binding conventions (' + SRC.readme + '), the other specs, or the code reality?',
  "verdict: 'confirmed' (fix right as written) | 'downgraded' (fix incomplete/needs adjustment) | 'refuted' (fix wrong, or already correctly fixed in the current text — prefix 'FIXED:' for the latter). Whenever not confirmed, ALWAYS provide revised_fix with the corrected concrete fix (empty if FIXED).",
  'FINDINGS JSON:',
  json,
].join('\n')

const verified = await parallel(Object.entries(byFile).map(([file, fs]) => async () => {
  const json = JSON.stringify(fs.map((f, j) => ({ index: j, ...f })))
  const pair = await parallel([
    () => agent(refuterPrompt(file, json), { label: 'refute:' + file.replace('.md', '').slice(0, 18), phase: 'Verify', schema: VERDICTS_SCHEMA, effort: 'high' }),
    () => agent(fixAuditPrompt(file, json), { label: 'fixaudit:' + file.replace('.md', '').slice(0, 16), phase: 'Verify', schema: VERDICTS_SCHEMA, effort: 'medium' }),
  ])
  const ref = pair[0], fix = pair[1]
  return fs.map((f, j) => ({
    ...f,
    refuter: (ref && ref.verdicts && ref.verdicts.find((v) => v.index === j)) || null,
    fix_audit: (fix && fix.verdicts && fix.verdicts.find((v) => v.index === j)) || null,
  }))
}))

const flat = verified.filter(Boolean).flat()
const confirmed = flat.filter((f) => f.refuter && f.refuter.verdict !== 'refuted')
const refuted = flat.filter((f) => !f.refuter || f.refuter.verdict === 'refuted')
log('Verify complete: ' + confirmed.length + ' confirmed, ' + refuted.length + ' refuted')

return {
  counts: { raw: raw.length, canonical: canon.length, confirmed: confirmed.length, refuted: refuted.length },
  confirmed: confirmed,
  refuted: refuted.map((f) => ({ file: f.file, severity: f.severity, summary: f.summary, why: (f.refuter && f.refuter.reasoning) || 'no verdict returned' })),
}