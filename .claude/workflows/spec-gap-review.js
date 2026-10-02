export const meta = {
  name: 'spec-gap-review',
  description: 'Multi-lens BA review of docs/product/specs (SPEC-NN + README) against each other and HEAD code; merge, adversarially verify, write a dated fix worklog to docs/product/analysis/',
  whenToUse: 'Periodic audit of the spec corpus. Optional args: {root: repo root (default "." = the session cwd, which must be the repo root), date: "YYYY-MM-DD" (default: the scout runs `date +%F`), write: false to skip writing the worklog file (the markdown is still returned), runLabel: run id to cite in the worklog Source line}.',
  phases: [
    { title: 'Scout', detail: 'list SPEC files, HEAD sha, today' },
    { title: 'Review', detail: '7 per-area finders + 5 cross-cutting lenses' },
    { title: 'Merge', detail: 'dedup into canonical findings, then a completeness critic' },
    { title: 'Verify', detail: 'adversarial refute + fix audit per area group' },
    { title: 'Write', detail: 'worklog docs/product/analysis/spec-gap-fix-worklog-<date>.md' },
  ],
}

// ---------------------------------------------------------------------------
// Design (2026-10-02 rewrite for the post-renumbering corpus, SPEC-01…18)
//
// Cost/coverage trade-off: one finder per spec (18) plus lenses would be ~25
// finders, most of them reading the same conventions/code twice. Instead the
// specs are grouped by area so each finder stays within ~1.3–2.9k spec lines
// (SPEC-01 alone is ~2.9k, so it gets its own finder):
//   identity 01 · shell+ops 02–03 · media+notify+docs 04–06 · journal/stream
//   07–11 · money 12–13 · comic+music 14–15 · movie+story+social 16–18
// => 7 area finders + 5 cross-cutting lenses = 12 parallel finders.
// A spec number outside these ranges (a future SPEC-19…) lands in an extra
// 'unassigned' finder, so new specs are never silently skipped.
//
// Per run: 1 scout + 12 finders + 1 merge + 1 critic + 2 verifiers per area
// group (8 groups; a group above VERIFY_CHUNK findings is split) + 1 writer
// ≈ 32–36 agents. The barrier after Review is justified (merge needs all
// findings); verification runs per group concurrently.
//
// Paths: every path is relative to the repo root R. With the default R='.'
// agents resolve paths from the session cwd, so run from the repo root, or
// pass args.root. The script has no filesystem access, so the scout agent
// discovers the SPEC list, the HEAD sha and today's date (Date is unavailable
// in workflow scripts).
// ---------------------------------------------------------------------------

const A = args || {}
const R = A.root || '.'
const P = (p) => (R === '.' ? p : R.replace(/\/$/, '') + '/' + p)
const S = P('docs/product/specs')

const SRC = {
  readme: S + '/README.md',
  backlog: P('docs/product/backlog.md'),
  inventory: P('docs/product/feature-inventory.md'),
  context: P('CONTEXT.md'),
  style: P('docs/STYLE.md'),
  events: P('docs/reference/events.md'),
  matrix: P('docs/reference/TRACEABILITY-MATRIX.md'),
  testing: P('docs/testing'),
  security: P('docs/architecture/security.md'),
  frontendDoc: P('docs/architecture/frontend.md'),
  modules: P('backend/MODULES.md'),
  perm: P('backend/internal/modules/account/rbac/permission.go'),
  rbacMig: P('backend/db/migrations/0003_account_rbac.up.sql'),
  migrations: P('backend/db/migrations'),
  openapi: P('shared/openapi.yaml'),
  apiMain: P('backend/cmd/api/main.go'),
  workerMain: P('backend/cmd/worker/main.go'),
  modulesDir: P('backend/internal/modules'),
  feClaude: P('frontend/CLAUDE.md'),
  feTemplates: P('frontend/src/templates'),
  prevWorklogs: P('docs/product/analysis'),
}

const AREAS = [
  { key: 'identity', nums: [1], code: 'account, tenant (RLS) + the admin console frontend' },
  { key: 'shell-ops', nums: [2, 3], code: 'layout, ops, cmd/worker server split, docker-compose/Makefile' },
  { key: 'media-notify-docs', nums: [4, 5, 6], code: 'media, notify; SPEC-06 is historical docs-only (check its claims about the docs tree)' },
  { key: 'journal-stream', nums: [7, 8, 9, 10, 11], code: 'journal (entries, attachments, stream_items, continue rail), people, home/widget frontend' },
  { key: 'money', nums: [12, 13], code: 'bank' },
  { key: 'comic-music', nums: [14, 15], code: 'comic, music, scraper/ (comic sync)' },
  { key: 'movie-story-social', nums: [16, 17, 18], code: 'movie, story, social' },
]
const VERIFY_CHUNK = 15

const FIND_SCHEMA = {
  type: 'object',
  properties: {
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          file: { type: 'string', description: 'filename under docs/product/specs/ where the fix applies (e.g. SPEC-09-life-stream-home.md, README.md), or MULTIPLE for cross-file defects' },
          section: { type: 'string' },
          severity: { type: 'string', enum: ['critical', 'major', 'minor'] },
          kind: { type: 'string', enum: ['spec-defect', 'unrecorded-gap', 'stale-gap-row', 'doc-drift'], description: 'spec-defect: the spec text is wrong/contradictory/ambiguous; unrecorded-gap: code diverges from the spec and the §11 gap table does not record it; stale-gap-row: a §11 row no longer matches HEAD (already fixed, or wrong file/function); doc-drift: a non-spec doc (events.md, matrix, inventory, MODULES.md…) disagrees with the spec' },
          category: { type: 'string' },
          summary: { type: 'string' },
          evidence: { type: 'string', description: 'quoted conflicting text with path:line cites (spec AND code where relevant)' },
          proposed_fix: { type: 'string', description: 'concrete replacement text or precise instruction; for unrecorded-gap, the full new §11 row in that table\'s column format' },
        },
        required: ['file', 'severity', 'kind', 'summary', 'evidence', 'proposed_fix'],
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

const SCOUT_SCHEMA = {
  type: 'object',
  properties: {
    date: { type: 'string', description: 'YYYY-MM-DD' },
    head: { type: 'string', description: 'short HEAD sha' },
    branch: { type: 'string' },
    specs: { type: 'array', items: { type: 'string' }, description: 'basenames matching SPEC-*.md' },
  },
  required: ['date', 'head', 'specs'],
}

let agentCount = 0
const run = (prompt, opts) => { agentCount++; return agent(prompt, opts) }

// ---- Phase 0: scout ---------------------------------------------------------
phase('Scout')
const scout = await run([
  'Run these read-only shell commands from the repo root ' + (R === '.' ? '(your cwd)' : R) + ' and report the results. Do not edit anything.',
  '1. `date +%F` → date' + (A.date ? ' (ignore: the caller fixed the date to ' + A.date + ')' : ''),
  '2. `git rev-parse --short HEAD` → head; `git rev-parse --abbrev-ref HEAD` → branch',
  '3. `ls ' + S + '` → specs = every basename matching SPEC-*.md (exclude README.md)',
].join('\n'), { label: 'scout', phase: 'Scout', schema: SCOUT_SCHEMA, effort: 'low' })
const DATE = A.date || (scout && scout.date) || 'YYYY-MM-DD'
const HEAD = (scout && scout.head) || 'HEAD'
const specFiles = ((scout && scout.specs) || []).filter((f) => /^SPEC-\d+.*\.md$/.test(f)).sort()
if (!specFiles.length) throw new Error('scout found no SPEC-*.md under ' + S + ' — is args.root right?')
const numOf = (f) => parseInt(f.slice(5), 10)
const areas = AREAS.map((a) => ({ ...a, files: specFiles.filter((f) => a.nums.includes(numOf(f))) })).filter((a) => a.files.length)
const assigned = new Set(areas.flatMap((a) => a.files))
const unassigned = specFiles.filter((f) => !assigned.has(f))
if (unassigned.length) {
  areas.push({ key: 'unassigned', nums: unassigned.map(numOf), files: unassigned, code: 'whatever modules these specs own (see each header)' })
  log('Specs outside the known area ranges get their own finder: ' + unassigned.join(', '))
}
log(specFiles.length + ' specs at ' + HEAD + ' (' + DATE + '), ' + areas.length + ' area finders')

const RULES = [
  'You are a senior business analyst reviewing the implementation-ready PRD specs of a Go modular monolith + Next.js self-hosted "life OS" (Portal). Repo root: ' + (R === '.' ? 'your current working directory' : R) + '. Baseline: HEAD ' + HEAD + ', today ' + DATE + '.',
  'Corpus facts (do not report these as defects):',
  '- Specs are SPEC-01…SPEC-NN, numbered by build priority since the 2026-10-01 renumbering; ' + SRC.readme + ' § Renumbering maps old→new. Code comments and the older worklogs in ' + SRC.prevWorklogs + ' still cite OLD numbers and `docs/adr/NN-*.md` paths by design — but a spec or doc under docs/ citing an OLD number where it means the new one IS a defect.',
  '- ADR-01…ADR-17 live inside the owning spec under `## Decision records` behind `<a id="adr-nn">` anchors; the index is ' + SRC.readme + ' § Decision records. There is no docs/adr/ folder and no briefs/. ADR narrative layers (Decision, Options, Trade-offs) are verbatim history; Context/Consequences/Action items are the fact layer and must be true (ADR-11).',
  '- Each built spec ends with "Implementation gaps vs shipped code" (§11): one row per known spec↔code divergence. A divergence recorded there is KNOWN — do not re-report it. ' + SRC.readme + ' § Implementation gaps index counts and routes them; ' + SRC.backlog + ' ranks them.',
  '- Specs use `000N_*` migration placeholders deliberately. "Decisions recorded …" and "Open owner decisions" in the README are owner rulings — do not re-litigate them unless internally contradictory or contradicted by a later ruling.',
  'Ground rules:',
  '- READ-ONLY: never edit or write any file. Use Read/Grep/Glob and read-only git/shell (git log, git show, ls, grep).',
  '- Code is the tie-breaker for facts about shipped behaviour: verify against HEAD, not against prose. Where code and spec disagree, decide which is the target (owner rulings and ADRs say what is intended) and phrase the fix accordingly: either correct the spec, or record the code follow-up as a §11 gap row.',
  '- Evidence or it did not happen: quote and cite path:line for every claim (spec text and the code you checked).',
  '- Substance only. Do NOT report markdown lint/style, tone, length, heading style, table spacing.',
  "- Severity: critical = following the text causes a wrong implementation, silent data loss, a security hole, or an impossible requirement; major = an implementer would stall, guess, or diverge (missing decision, contradiction, wrong dependency/API/schema, an unrecorded divergence with user-visible effect); minor = wrong reference/link/number, stale naming, small omission.",
  '- proposed_fix must be concrete (exact replacement text where feasible) and consistent with ' + SRC.readme + ' § Conventions binding on all specs.',
  '- An empty findings array is a legitimate answer.',
  '',
].join('\n')

const areaPrompt = (a) => RULES + [
  'AREA FINDER: ' + a.key + '. TARGET SPECS: ' + a.files.map((f) => S + '/' + f).join(', '),
  'Owning code to verify against: ' + a.code + ' — under ' + SRC.modulesDir + '/<module>/ (module.go route table + RegisterTasks, query/*.sql, handler/, service/), ' + SRC.migrations + ', ' + SRC.apiMain + ', ' + SRC.workerMain + ', and ' + P('frontend/src') + ' where the spec makes frontend claims.',
  'Read as needed for cross-refs: ' + SRC.readme + ' (conventions, decisions, gaps index), ' + SRC.events + ', ' + SRC.inventory + ' (D-N ids), ' + SRC.context + ' (glossary terms), and any file a spec links when validating a specific claim.',
  '',
  'For each target spec check:',
  '1) Internal contradictions: requirement prose vs ACs vs §6 data model vs §7 API table vs §9 phasing vs its own Decision records.',
  '2) Unimplementable or ambiguous requirements an engineer could not build without guessing.',
  '3) Cross-references: § numbers, SPEC numbers (post-renumbering), links/anchors (#adr-NN must exist in the target file), D-N ids, ADR ids, event names, glossary terms.',
  '4) AC quality: untestable ACs, P0 behaviour without any AC, ACs contradicting each other or the data model.',
  '5) §11 gap table vs HEAD: for EVERY row, open the named file/function and confirm the divergence still exists. Already fixed in code → stale-gap-row (fix: delete the row, regrade the matrix row, adjust the README gaps index count). Wrong file/function → stale-gap-row with the correct location.',
  '6) Unrecorded divergences: for every requirement the spec presents as shipped (Status current, or text describing built behaviour), spot-check the code — routes and methods, permission gates, migrations/columns/constraints, task names and the Asynq server that registers them, Problem types. A divergence absent from §11 → unrecorded-gap, proposed_fix = the full new §11 row (Requirement · Spec requires · Shipped code today · Change needed · Source "spec-gap-review ' + DATE + '").',
  'SPEC-06 is historical and has no §11: check only that its fact-layer claims about the docs tree are true today.',
].join('\n')

const CROSS = [
  {
    key: 'events',
    prompt: RULES + [
      'CROSS-CUTTING LENS: events, payloads, consumers.',
      'Read ' + SRC.events + ', every `Subscribe(`/`RegisterTasks`/`Register*Tasks` call in ' + SRC.apiMain + ' and ' + SRC.workerMain + ', the publish call sites (grep `Publish(` under ' + SRC.modulesDir + '), and the event/consumer text of every spec under ' + S + '/ plus README § Conventions (Events) and ADR-17 (README § Decision records).',
      'Check: (a) registry closure — every event/task a spec emits or consumes exists in events.md with matching name, payload fields and consumers, and no registry row is orphaned; (b) every field a consumer reads exists in the emitter\'s payload (spec AND the Go payload struct); (c) the subscription table is per-binary and Publish of an unregistered name is a silent no-op — every event emitted from cmd/api must be subscribed in cmd/api; (d) each task runs on the Asynq server (heavy / image / light) its spec says; (e) ADR-17: account and layout emit too, notify fans admin changes to effective-`*` holders — is every spec/registry consistent with that ruling, and is any not-yet-built part recorded as a gap; (f) idempotency/dedup keys on projections (stream_items unique key, notify dedup) are coherent across emitter and consumer specs.',
    ].join('\n'),
  },
  {
    key: 'deps',
    prompt: RULES + [
      'CROSS-CUTTING LENS: README coherence, dependency graph, build state, indexes.',
      'Read ' + SRC.readme + ' (Documents table, Renumbering, Build state and what remains, Implementation gaps index, Decisions recorded…, Open owner decisions, Decision records index) and the header block + §9 + §11 of every spec under ' + S + '/; ' + SRC.backlog + '.',
      'Check: Depends-on/Downstream claims mutually consistent, acyclic, and reciprocal; build order respects them; statuses agree between the README Documents table and spec headers (Status vocabulary per ' + SRC.style + '); the gaps-index row counts and severity ranges match each spec\'s actual §11 rows; every backlog item that cites a gap points at a row that exists; the Decision records index lists ADR-01…17 with the correct Location, and every anchor exists exactly once in the named file; README decision rulings are reflected in the specs they govern (a ruling that a spec still contradicts is a finding against that spec); no doc under docs/ still links docs/adr/, briefs/, MILESTONE_CHECKS.md, or an old SPEC filename.',
    ].join('\n'),
  },
  {
    key: 'conventions',
    prompt: RULES + [
      'CROSS-CUTTING LENS: binding conventions — RBAC grammar, tenancy/RLS + the ADR-12 sharing exception, list contract, timezone, Problem types.',
      'Read ' + SRC.perm + ' (the REAL grammar: segment rules, scope tokens, wildcard + AllowsCode), ' + SRC.rbacMig + ' and every later migration that inserts permissions/role_permissions (grep `INSERT INTO permissions`/`role_permissions` in ' + SRC.migrations + '), ' + SRC.security + ', ' + SRC.modules + ', ' + SRC.readme + ' § Conventions, ADR-07/ADR-12 in SPEC-01, ADR-15/16 in the README, and every §5 RBAC/permission prose, §6 DDL and §7 table across ' + S + '/.',
      'Check: (a) every permission code a spec names parses under the real grammar and is seeded (which migration, which role) — or the spec says it is not yet seeded and §11 records it; `:own` codes are never the sole gate where ownership matters (RequirePermission checks no ownership); (b) every user/org-data table a spec defines or cites is tenant-scoped (tenant_id + ENABLE/FORCE RLS + tenant_isolation policy) unless the spec states why it is global, and the migrations at HEAD agree; (c) ADR-12 is the ONLY sanctioned cross-tenant read (published music/movies/stories to household+friends via one SECURITY DEFINER predicate) — flag any spec implying another cross-tenant read, or a vertical spec whose sharing text contradicts ADR-12; (d) every collection endpoint follows ADR-16 `{items[, next_cursor]}` with keyset cursors and a lenient clamped limit (admin users offset is the one exception) or §11 records the divergence; (e) day/month boundaries follow ADR-15; (f) Problem types are `<module>/<kebab-case>`, declared in the spec\'s Problem-types line, and written via internal/platform/server; (g) backend/MODULES.md agrees with the README conventions.',
    ].join('\n'),
  },
  {
    key: 'api',
    prompt: RULES + [
      'CROSS-CUTTING LENS: API contract — spec §7 tables vs ' + SRC.openapi + ' vs the shipped route tables.',
      'Read every §7 API table + §5 endpoint prose under ' + S + '/, ' + SRC.openapi + ' (paths, methods, x-required-permission, response schemas, Problem responses), and each module\'s MountHTTP route table under ' + SRC.modulesDir + '/*/module.go (+ ' + SRC.apiMain + ' for routes mounted there).',
      'Check: every endpoint in a spec §7 exists in openapi.yaml with the same method/path/permission, and vice versa for that module; endpoints a spec presents as shipped are actually routed with the stated method and gate (a mismatch not recorded in §11 is an unrecorded-gap); every endpoint mentioned in prose appears in its §7 table; response shapes are defined wherever ANOTHER spec or a frontend widget consumes them; openapi response schemas match what the handler actually writes for the endpoints you sample (ADR-10: codegen is enforced, handler behaviour is not).',
    ].join('\n'),
  },
  {
    key: 'tests',
    prompt: RULES + [
      'CROSS-CUTTING LENS: test and traceability coverage.',
      'Read ' + SRC.matrix + ', ' + SRC.testing + '/TEST-PLAN.md and TEST-CASES-SPEC-*.md, the requirement IDs (P0.x/P1.x + ACs) of every spec under ' + S + '/, and the real tests (`find ' + P('backend') + ' -name "*_test.go"`, ' + P('frontend/src') + ' *.test.ts(x)).',
      'Check: every P0 requirement of a built spec has a matrix row; matrix rows cite requirement IDs that exist under the CURRENT numbering; a row graded as tested names a test that exists and actually exercises it (open it); TEST-CASES files are named for the current spec numbers and their TC ids are referenced consistently by specs/§11 rows; specs whose test-cases file is missing are stated as such somewhere (README or the spec), not silently absent; manual-run results cited by a spec exist where it says.',
    ].join('\n'),
  },
]

// ---- Phase 1: finders (barrier justified: merge needs the full set) --------
phase('Review')
const finderThunks = []
for (const a of areas) {
  finderThunks.push(() => run(areaPrompt(a), { label: 'area:' + a.key, phase: 'Review', schema: FIND_SCHEMA, effort: 'high' }))
}
for (const c of CROSS) {
  finderThunks.push(() => run(c.prompt, { label: 'x:' + c.key, phase: 'Review', schema: FIND_SCHEMA, effort: 'high' }))
}
const finderResults = await parallel(finderThunks)
const failed = finderResults.map((r, i) => (r ? null : (i < areas.length ? 'area:' + areas[i].key : 'x:' + CROSS[i - areas.length].key))).filter(Boolean)
if (failed.length) log('WARNING: finders returned nothing (skipped or errored): ' + failed.join(', ') + ' — their coverage is missing from this run')
const raw = finderResults.filter(Boolean).flatMap((r) => r.findings || [])
log('Review complete: ' + raw.length + ' raw findings from ' + (finderThunks.length - failed.length) + ' finders')

// ---- Phase 2: merge + completeness critic ----------------------------------
phase('Merge')
const merged = raw.length ? await run(RULES + [
  'You are the dedup/merge editor for ' + raw.length + ' raw findings from ' + finderThunks.length + ' independent reviewers (JSON below).',
  'Merge findings that describe the SAME underlying defect (even when filed under different files — set file to where the fix belongs, or MULTIPLE). Keep distinct defects separate — when unsure, keep separate.',
  'For merged items keep the sharpest evidence, the most complete proposed_fix, the max severity, and the most specific kind.',
  'Drop only pure style/lint complaints and findings that restate a §11 gap row, an owner ruling, or a documented open question without showing it is wrong.',
  'Do NOT invent new findings. Output every surviving finding, sorted critical → major → minor.',
  'RAW FINDINGS JSON:',
  JSON.stringify(raw),
].join('\n'), { label: 'merge', phase: 'Merge', schema: FIND_SCHEMA, effort: 'high' }) : { findings: [] }
const canon = ((merged && merged.findings) || []).slice()
log('Merge complete: ' + canon.length + ' canonical findings')

const critic = await run(RULES + [
  'COMPLETENESS CRITIC. ' + finderThunks.length + ' reviewers covered: ' + areas.map((a) => a.key + ' (' + a.files.join(', ') + ')').join('; ') + '; lenses ' + CROSS.map((c) => c.key).join(', ') + (failed.length ? '. These finders FAILED and covered nothing: ' + failed.join(', ') : '') + '.',
  'Their canonical findings (summaries) are below. Ask what is missing: a spec, section, owner ruling, ADR, §11 table, doc (' + [SRC.events, SRC.matrix, SRC.inventory, SRC.context, SRC.security, SRC.frontendDoc, SRC.modules, SRC.feClaude].join(', ') + ') or code area nobody examined; a class of defect found in one spec but not checked in its siblings; frontend claims (D-32 TanStack owns server state, D-33 RSC-first, D-34 SessionKeeper, the templates/v{N} registry in ' + SRC.feTemplates + '/README.md) nobody checked.',
  'Then CLOSE those holes yourself: investigate and return only NEW, evidenced findings (same schema) that are not duplicates of the list. Empty is fine.',
  'EXISTING FINDINGS:',
  JSON.stringify(canon.map((f, i) => ({ i, file: f.file, section: f.section, summary: f.summary }))),
].join('\n'), { label: 'critic', phase: 'Merge', schema: FIND_SCHEMA, effort: 'high' })
const criticFindings = (critic && critic.findings) || []
canon.push(...criticFindings)
log('Critic added ' + criticFindings.length + ' findings → ' + canon.length + ' to verify')

// ---- Phase 3: adversarial verify, grouped by area ---------------------------
phase('Verify')
const areaOfFile = (file) => {
  const m = /^SPEC-(\d+)/.exec(file || '')
  if (!m) return 'cross'
  const a = areas.find((x) => x.nums.includes(parseInt(m[1], 10)))
  return a ? a.key : 'cross'
}
const groups = {}
for (const f of canon) {
  const k = areaOfFile(f.file)
  if (!groups[k]) groups[k] = []
  groups[k].push(f)
}
const batches = []
for (const k of Object.keys(groups)) {
  const fs = groups[k]
  for (let i = 0; i < fs.length; i += VERIFY_CHUNK) batches.push({ key: k + (fs.length > VERIFY_CHUNK ? '#' + (i / VERIFY_CHUNK + 1) : ''), group: k, items: fs.slice(i, i + VERIFY_CHUNK) })
}
log('Verifying ' + canon.length + ' findings in ' + batches.length + ' batches (≤' + VERIFY_CHUNK + ' each, refuter + fix audit per batch)')

const scopeOf = (g) => (g === 'cross' ? 'README.md / MULTIPLE files under ' + S + '/' : 'the ' + g + ' specs under ' + S + '/')

const refuterPrompt = (g, json) => RULES + [
  'ADVERSARIAL VERIFICATION for findings on ' + scopeOf(g) + '.',
  'Default stance: each finding is WRONG until you re-verify it yourself against the CURRENT spec text and HEAD code. Re-read the cited files; never trust the quoted evidence.',
  "Per indexed finding: 'confirmed' (evidence checks out, severity apt) | 'downgraded' (real but overstated or partly wrong — say what survives) | 'refuted' (not a defect — quote the text or code that disproves it).",
  'Also refute: findings that restate a deliberate decision with its rationale in place, an owner ruling, or a divergence already recorded in that spec\'s §11 table; and any defect the current text no longer exhibits (reasoning begins "FIXED:").',
  'FINDINGS JSON:',
  json,
].join('\n')

const fixAuditPrompt = (g, json) => RULES + [
  'FIX AUDIT for findings on ' + scopeOf(g) + '.',
  'Assume each defect is real. Judge the proposed_fix: would applying it fully resolve the defect without contradicting the binding conventions (' + SRC.readme + '), the ADRs and owner rulings, the other specs, or HEAD code? For unrecorded-gap fixes, check the row follows that spec\'s §11 column format and names real files/functions; for stale-gap-row fixes, check the code really matches now and that the README gaps index and matrix follow-ups are included.',
  "verdict: 'confirmed' (fix right as written) | 'downgraded' (fix incomplete/needs adjustment) | 'refuted' (fix wrong). Whenever not confirmed, ALWAYS give revised_fix with the corrected concrete fix.",
  'FINDINGS JSON:',
  json,
].join('\n')

const verified = await parallel(batches.map((b) => async () => {
  const json = JSON.stringify(b.items.map((f, j) => ({ index: j, ...f })))
  const pair = await parallel([
    () => run(refuterPrompt(b.group, json), { label: 'refute:' + b.key, phase: 'Verify', schema: VERDICTS_SCHEMA, effort: 'high' }),
    () => run(fixAuditPrompt(b.group, json), { label: 'fixaudit:' + b.key, phase: 'Verify', schema: VERDICTS_SCHEMA, effort: 'medium' }),
  ])
  const ref = pair[0], fix = pair[1]
  return b.items.map((f, j) => ({
    ...f,
    refuter: (ref && ref.verdicts && ref.verdicts.find((v) => v.index === j)) || null,
    fix_audit: (fix && fix.verdicts && fix.verdicts.find((v) => v.index === j)) || null,
  }))
}))

const flat = verified.filter(Boolean).flat()
const unverified = flat.filter((f) => !f.refuter)
if (unverified.length) log('WARNING: ' + unverified.length + ' findings got no refuter verdict; they are listed as unverified, not confirmed')
const confirmed = flat.filter((f) => f.refuter && f.refuter.verdict !== 'refuted')
const refuted = flat.filter((f) => f.refuter && f.refuter.verdict === 'refuted')
log('Verify complete: ' + confirmed.length + ' confirmed, ' + refuted.length + ' refuted, ' + unverified.length + ' unverified')

// ---- Phase 4: worklog (format of docs/product/analysis/spec-gap-fix-worklog-2026-09-30.md)
phase('Write')
const SEV = { critical: '🔴 CRITICAL', major: '🟠 MAJOR', minor: '🟡 MINOR' }
const SEV_HEAD = { critical: '## 🔴 Critical', major: '## 🟠 Major', minor: '## 🟡 Minor' }
const fileRank = (f) => (f === 'MULTIPLE' ? '0' : f === 'README.md' ? '1' : '2' + f)
const sevOf = (f) => (f.refuter && f.refuter.verdict === 'downgraded' && f.severity === 'critical' ? 'major' : f.severity) || 'minor'
const ordered = []
for (const sev of ['critical', 'major', 'minor']) {
  const of = confirmed.filter((f) => sevOf(f) === sev).sort((x, y) => fileRank(x.file || 'MULTIPLE').localeCompare(fileRank(y.file || 'MULTIPLE')))
  ordered.push(...of.map((f) => ({ ...f, sev })))
}
const id = (i) => 'F' + String(i + 1).padStart(3, '0')
const count = (sev) => ordered.filter((f) => f.sev === sev).length
const fixText = (f) => {
  const a = f.fix_audit
  if (a && a.verdict !== 'confirmed' && a.revised_fix) return ['**Fix (revised by verify):** ', a.revised_fix]
  if (f.refuter && f.refuter.verdict === 'downgraded' && f.refuter.revised_fix) return ['**Fix (revised by verify):** ', f.refuter.revised_fix]
  return ['**Fix:** ', f.proposed_fix]
}
const lines = []
lines.push('# Spec-gap fix worklog — ' + DATE, '')
lines.push('**Status:** current · **Last verified:** ' + DATE + ' (generated from the run ' + DATE + '; ticks record fix progress)')
lines.push('**Spec numbers:** this audit cites the current (post-2026-10-01) numbering — see [specs/README.md § Renumbering](../specs/README.md#renumbering-2026-10-01).', '')
lines.push('**Source:** `spec-gap-review` workflow run' + (A.runLabel ? ' `' + A.runLabel + '`' : '') + ' (' + (agentCount + 1) + ' agents: scout, ' + finderThunks.length + ' finders, merge, completeness critic, adversarial refute + fix audit in ' + batches.length + ' batches, writer). ' + raw.length + ' raw + ' + criticFindings.length + ' from the critic → ' + canon.length + ' canonical → **' + ordered.length + ' confirmed** (' + count('critical') + ' critical · ' + count('major') + ' major · ' + count('minor') + ' minor), ' + refuted.length + ' refuted (listed at the end, not to be applied)' + (unverified.length ? ', ' + unverified.length + ' unverified (listed at the end)' : '') + '. Baseline: HEAD `' + HEAD + '`' + (scout && scout.branch ? ' on `' + scout.branch + '`' : '') + '.', '')
lines.push('## How to resume (read this first if continuing after a token-out)', '')
lines.push('- This file is the single source of truth for fix progress. Each finding has a stable ID `Fnnn` and a checkbox.')
lines.push('- `- [ ]` = not started · `- [x]` = fixed · `- [~]` = partial/deferred (see the **Applied** note) · `- [c]` = the spec is now correct but the **shipped code** still diverges — record it as a row in that spec\'s "Implementation gaps vs shipped code" section and name it in the **Applied** note.')
lines.push('- *Kind* says what the fix touches: `spec-defect` (spec text) · `unrecorded-gap` (add the given row to the spec\'s gaps section) · `stale-gap-row` (delete/correct a gaps row; regrade TRACEABILITY-MATRIX; adjust the README gaps index) · `doc-drift` (a doc outside specs/).')
lines.push('- Every finding carries its own **Fix** text. Where verification revised the proposal, only the revised text is given — apply that, not the original.')
lines.push('- **Line numbers in Evidence are as of `' + HEAD + '` (' + DATE + ')**; re-locate by quoted text if a file has moved.')
lines.push('- **Code is the tie-breaker**: where a spec and shipped code disagree and the finding says the code is right, correct the spec; otherwise record the code follow-up as a gaps row — never change code inside this worklog.')
lines.push('- **Work top-to-bottom**: 🔴 critical → 🟠 systemic (MULTIPLE / README) → 🟠 per-spec major → 🟡 minor. Fix **one spec file at a time**; tick boxes and fill **Applied** in the same edit.')
lines.push('- After ticking, update the **Progress** counter just below.', '')
lines.push('## Progress', '')
lines.push('`[ updated ' + DATE + ' ]`  **Fixed in the specs: 0 / ' + ordered.length + '**  ·  critical 0/' + count('critical') + ' · major 0/' + count('major') + ' · minor 0/' + count('minor'), '', '---', '')
let lastSev = null, lastFile = null
ordered.forEach((f, i) => {
  if (f.sev !== lastSev) { lines.push(SEV_HEAD[f.sev], ''); lastSev = f.sev; lastFile = null }
  const file = f.file || 'MULTIPLE'
  if (file !== lastFile) { lines.push('### ' + file, ''); lastFile = file }
  const fx = fixText(f)
  lines.push('#### - [ ] ' + id(i) + ' · ' + SEV[f.sev] + ' · `' + file + '` · ' + (f.section || '—'))
  lines.push('*Category:* ' + (f.category || '—') + ' · *Kind:* ' + (f.kind || 'spec-defect') + '  ')
  lines.push('**Problem:** ' + f.summary + '  ')
  lines.push('**Evidence:** ' + f.evidence + '  ')
  if (f.refuter && f.refuter.verdict === 'downgraded') lines.push('**Verify note:** ' + f.refuter.reasoning + '  ')
  lines.push(fx[0] + fx[1] + '  ')
  lines.push('**Applied:** —', '')
})
const followUps = ordered.map((f, i) => ({ f, i })).filter((x) => x.f.kind === 'unrecorded-gap' || x.f.kind === 'stale-gap-row')
lines.push('## Code follow-ups (not done here)', '')
lines.push('Findings whose fix is a gaps-table change, i.e. where HEAD code and the spec disagree. The spec text is the target; the gaps row is the ticket.', '')
if (followUps.length) followUps.forEach((x) => lines.push('- **' + id(x.i) + '** (' + x.f.kind + ', `' + (x.f.file || 'MULTIPLE') + '`) — ' + x.f.summary))
else lines.push('- none')
lines.push('')
if (unverified.length) {
  lines.push('## Unverified (no refuter verdict — re-check before applying)', '')
  unverified.forEach((f) => lines.push('- `' + (f.file || 'MULTIPLE') + '` — ' + f.summary))
  lines.push('')
}
lines.push('## Refuted (do not apply)', '')
if (refuted.length) refuted.forEach((f) => lines.push('- `' + (f.file || 'MULTIPLE') + '` — ' + f.summary + ' *(' + f.refuter.reasoning.replace(/\s+/g, ' ').slice(0, 300) + ')*'))
else lines.push('- none')
const markdown = lines.join('\n') + '\n'
const worklogPath = P('docs/product/analysis/spec-gap-fix-worklog-' + DATE + '.md')

let written = null
if (A.write !== false && ordered.length) {
  written = await run([
    'Write a file. Target path: ' + worklogPath + '. If that file already exists, do NOT overwrite it: use the same name with a "-2" suffix before .md (then "-3", …).',
    'Write the content between the markers below EXACTLY as given (verbatim, no edits, no reformatting, without the marker lines) using the Write tool. Do not touch any other file. Return only the path you wrote.',
    '<<<BEGIN_CONTENT',
    markdown,
    'END_CONTENT>>>',
  ].join('\n'), { label: 'write-worklog', phase: 'Write', effort: 'low' })
  log('Worklog: ' + (written || 'writer returned nothing — use the returned markdown'))
} else {
  log(ordered.length ? 'write=false: worklog not written; markdown returned' : 'No confirmed findings; no worklog written')
}

return {
  date: DATE,
  head: HEAD,
  agents: agentCount,
  worklog: written,
  counts: { raw: raw.length, critic: criticFindings.length, canonical: canon.length, confirmed: ordered.length, critical: count('critical'), major: count('major'), minor: count('minor'), refuted: refuted.length, unverified: unverified.length },
  failed_finders: failed,
  confirmed: ordered.map((f, i) => ({ id: id(i), file: f.file, severity: f.sev, kind: f.kind, summary: f.summary })),
  markdown: markdown,
}
