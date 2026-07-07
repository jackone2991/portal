# Docs Restructure — Migration Plan (2026-07-07)

**Goal:** replace the flat, mixed-genre, bilingual `doc/en` + `doc/vi` tree with a
single-language, genre-separated `docs/` tree. The information architecture follows
the **Diátaxis** framework (the de-facto standard for technical docs), adapted to a
design-heavy solo project: *product* (why/what), *architecture* (how it's designed),
*guides* (how to work on it), *reference* (lookup material), *adr* (decisions).

Ratified by **ADR-09** (in this bundle). Language policy (English canonical,
Vietnamese mirror retired) is part of that ADR — it supersedes the old bilingual rule.

## Problems with the current tree (context for ADR-09)

1. **Mirror tax**: every edit costs 2× (`doc/en` + `doc/vi`); drift was already
   visible and the owner has switched to English-only.
2. **Mixed genres in one flat folder**: specs (`frontend.md`, `authoration.md`),
   analyses (`missing-features.md`), a decision log (`feature.md`), deferred designs
   (`archivetech*.md`), and diagrams all sit side-by-side — a reader can't tell
   normative from historical from aspirational.
3. **Two "what to build next" sources** (`missing-features.md` order vs the new
   `feature/` folder) with no structural home for either.
4. **Non-standard root** (`doc/` instead of `docs/`, which GitHub tooling favors).

## Target tree

```
docs/
├── README.md                    # index + reading order (NEW)
├── STYLE.md                     # doc conventions (NEW)
├── adr/                         # all decision records
│   ├── README.md                # index + ADR authoring rules (REWRITTEN)
│   ├── 00-architecture-review.md … 07-*.md   (MOVED from doc/en/architecture/)
│   ├── 08-life-os-pivot.md      # NEW (drafted in this bundle)
│   └── 09-docs-architecture.md  # NEW (drafted in this bundle)
├── product/                     # why & what
│   ├── README.md                # (NEW)
│   ├── vision.md                # life-OS positioning one-pager (NEW)
│   ├── feature-inventory.md     # ← doc/en/feature.md  (decision log D-1…D-40)
│   ├── backlog.md               # ← doc/en/missing-features.md
│   ├── checklist.md             # ← doc/en/checklist.md
│   ├── analysis/
│   │   └── facebook-comparison.md   # ← doc/en/facebook-comparison.md (historical yardstick)
│   ├── briefs/                  # ← feature/en/ folder (00–04 brainstorm briefs)
│   └── specs/                   # ← spec/ folder (SPEC-01…03 + README)
├── architecture/                # how it's designed
│   ├── README.md                # (NEW)
│   ├── diagrams.md              # ← doc/en/diagrams.md
│   ├── security.md              # ← doc/en/authoration.md (renamed: clearer)
│   ├── frontend.md              # ← doc/en/frontend.md
│   └── deferred/                # designs explicitly post-v1
│       ├── access-policies.md       # ← doc/en/archivetech.md (renamed)
│       └── multi-tenant-backend.md  # ← doc/en/archivetech-backend.md (renamed)
├── guides/                      # how to work on it
│   └── getting-started.md       # dev setup, run, gotchas (NEW)
├── reference/                   # lookup material
│   ├── README.md                # pointers to canonical sources (NEW)
│   └── events.md                # Asynq event & task registry (NEW)
└── archive/
    └── vi-2026-07/              # ← doc/vi/* frozen as-is, never updated again
```

**Stays where it is (deliberately):** `MILESTONE_CHECKS.md` (repo root — living
status truth), `CLAUDE.md` (root — session conventions), `backend/MODULES.md`
(next to the code it governs), `shared/openapi.yaml` (contract next to source).
`docs/reference/README.md` points at all four.

## Old → new mapping

| Old path | New path | Notes |
|---|---|---|
| `doc/en/README.md` | replaced by `docs/README.md` | rewritten |
| `doc/en/feature.md` | `docs/product/feature-inventory.md` | content unchanged |
| `doc/en/missing-features.md` | `docs/product/backlog.md` | content unchanged; "Suggested next order" now defers to briefs/specs |
| `doc/en/checklist.md` | `docs/product/checklist.md` | |
| `doc/en/facebook-comparison.md` | `docs/product/analysis/facebook-comparison.md` | mark header: historical yardstick, superseded by vision.md |
| `doc/en/diagrams.md` | `docs/architecture/diagrams.md` | |
| `doc/en/authoration.md` | `docs/architecture/security.md` | rename fixes the long-standing typo-ish name |
| `doc/en/frontend.md` | `docs/architecture/frontend.md` | |
| `doc/en/archivetech.md` | `docs/architecture/deferred/access-policies.md` | |
| `doc/en/archivetech-backend.md` | `docs/architecture/deferred/multi-tenant-backend.md` | |
| `doc/en/architecture/*.md` | `docs/adr/*.md` | ADRs get their own top-level home |
| `feature/en/*` (new bundle) | `docs/product/briefs/*` | |
| `spec/*` (new bundle) | `docs/product/specs/*` | |
| `doc/vi/**` | `docs/archive/vi-2026-07/**` | frozen |

## Execution

1. Run `migrate-docs.sh` (in this bundle) from the repo root on a branch.
2. Drop the bundle's NEW files into place (they are laid out in the final structure).
3. **Fix inbound links** — the moves break relative links in moved files and in
   root files (`MILESTONE_CHECKS.md`, `CLAUDE.md`, `backend/MODULES.md`). Find them:
   ```bash
   grep -rn --include='*.md' -E 'doc/(en|vi)/|\.\./\.\./doc|archivetech|authoration|missing-features\.md|facebook-comparison\.md' . \
     | grep -v docs/archive
   ```
   The common rewrites: `doc/en/architecture/` → `docs/adr/`,
   `doc/en/feature.md` → `docs/product/feature-inventory.md`,
   `missing-features.md` → `product/backlog.md`, `authoration.md` →
   `architecture/security.md`.
4. Update `CLAUDE.md` / project instructions: bilingual rule → replaced by ADR-09
   language policy; `doc/*` paths → `docs/*`.
5. Commit only when you're ready (owner commits to `main`); suggested message:
   `docs: restructure to Diátaxis-informed tree (ADR-09), retire vi mirror` +
   `Co-Authored-By: Claude`.

## Out of scope for this migration

- Rewriting the content of moved documents (only headers/links change).
- CI link-checking (nice follow-up: `lychee` or `markdown-link-check` on `docs/`).
- Translating anything. `docs/archive/vi-2026-07/` is read-only history.
