# Domain Docs

**Status:** current · **Last verified:** 2026-10-01 · single-context layout; written by `/setup-matt-pocock-skills`

How the engineering skills should consume this repo's domain documentation when exploring the codebase.

## Before exploring, read these

- **`CONTEXT.md`** at the repo root, or
- **`CONTEXT-MAP.md`** at the repo root if it exists: it points at one `CONTEXT.md` per context. Read each one relevant to the topic.
- **Decision records (`ADR-NN`)**: read the ones that touch the area you're about to work in. This repo has **no `docs/adr/` folder** — since 2026-10-01 each record lives in the spec that owns its subject, under `## Decision records`, behind the anchor `#adr-NN`; the index is [docs/product/specs/README.md § Decision records](../product/specs/README.md#decision-records). Code comments may still cite the old `docs/adr/NN-*.md` path — read it as that anchor.

If any of these files don't exist, **proceed silently**. Don't flag their absence; don't suggest creating them upfront. The `/domain-modeling` skill (reached via `/grill-with-docs` and `/improve-codebase-architecture`) creates them lazily when terms or decisions actually get resolved.

## File structure

This is a single-context repo. What exists today, and what does not:

```
/
├── CONTEXT.md           the product glossary — created 2026-09-12 when the first terms
│                        (Entry, Attachment, Location, Asset) were resolved; grows one
│                        resolved term at a time, never in bulk. docs/README.md § "Genre
│                        rules" is the separate glossary of the *documentation* domain.
├── docs/product/specs/  SPEC-01…18 + README; decision records ADR-01…11 live inside them
│                        (§ Decision records, anchors #adr-NN); corrected in place (ADR-11)
├── docs/product/feature-inventory.md   product decisions D-1…D-41 — cite the IDs
└── backend/ frontend/ scraper/
```

Two decision registers bind, not one: the decision records (inside the specs, cited as `ADR-NN`) and the
feature inventory (`docs/product/feature-inventory.md`, cited as `D-NN`). ADR-11 says how
ADRs are kept true — fact layer rewritten in place, `Last verified` as the only freshness
mark, nothing archived; an ADR is never "superseded by a note".

The multi-context layout (`CONTEXT-MAP.md` + per-context `CONTEXT.md` and per-context ADR folders) is
not in use here and should not be introduced without a decision — backend modules are
bounded contexts already, but their contract lives in `backend/MODULES.md` and each
module's `README.md`, not in per-module glossaries.

## Use the glossary's vocabulary

When your output names a domain concept (in an issue title, a refactor proposal, a hypothesis, a test name), use the term as defined in `CONTEXT.md`. Don't drift to synonyms the glossary explicitly avoids.

If the concept you need isn't in the glossary yet, that's a signal: either you're inventing language the project doesn't use (reconsider) or there's a real gap (note it for `/domain-modeling`).

`CONTEXT.md` covers only the terms that have been through a design session so far. For
everything else, the vocabulary to hold to is the one the code and `CLAUDE.md` already
use (module names, `tenant`/`organization`, `scope`, `approval_status`, the permission
grammar) and the genre names in `docs/README.md` — and a term used inconsistently there
is a candidate for the glossary, not a licence to invent a third name.

## Flag ADR conflicts

If your output contradicts an existing ADR, surface it explicitly rather than silently overriding:

> _Contradicts ADR-07 (tenancy & RLS), but worth reopening because…_

Cite `D-NN` from the feature inventory the same way; both registers bind.
