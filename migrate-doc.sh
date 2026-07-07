#!/usr/bin/env bash
# Docs restructure per docs/MIGRATION.md + ADR-09. Run from the repo root, on a branch.
# Windows + Git Bash note: plain git mv, no volume mounts involved.
set -euo pipefail

[ -d doc/en ] || { echo "doc/en not found — run from the repo root"; exit 1; }

mkdir -p docs/adr docs/product/analysis docs/product/briefs docs/product/specs \
         docs/architecture/deferred docs/guides docs/reference docs/archive

# --- ADRs -------------------------------------------------------------------
for f in doc/en/architecture/*.md; do
  git mv "$f" "docs/adr/$(basename "$f")"
done

# --- Product ----------------------------------------------------------------
git mv doc/en/feature.md              docs/product/feature-inventory.md
git mv doc/en/missing-features.md     docs/product/backlog.md
git mv doc/en/checklist.md            docs/product/checklist.md
git mv doc/en/facebook-comparison.md  docs/product/analysis/facebook-comparison.md

# Brainstorm briefs + detailed specs (if already dropped at repo root from the
# earlier bundles; adjust paths if you placed them elsewhere)
if [ -d feature/en ]; then git mv feature/en/* docs/product/briefs/ && rm -rf feature; fi
if [ -d spec ];       then git mv spec/*       docs/product/specs/  && rmdir spec;    fi

# --- Architecture -------------------------------------------------------------
git mv doc/en/diagrams.md             docs/architecture/diagrams.md
git mv doc/en/authoration.md          docs/architecture/security.md
git mv doc/en/frontend.md             docs/architecture/frontend.md
git mv doc/en/archivetech.md          docs/architecture/deferred/access-policies.md
git mv doc/en/archivetech-backend.md  docs/architecture/deferred/multi-tenant-backend.md

# --- Archive the Vietnamese mirror (frozen) -----------------------------------
git mv doc/vi docs/archive/vi-2026-07

# --- Retire the old root -------------------------------------------------------
git rm doc/en/README.md   # superseded by docs/README.md from the bundle
rmdir doc/en doc 2>/dev/null || true

echo
echo "Done. Now: (1) copy the bundle's NEW files (README/STYLE/ADR-08/ADR-09/"
echo "section READMEs/guides/reference) into docs/, (2) fix inbound links —"
echo "see MIGRATION.md step 3, (3) update CLAUDE.md paths + language rule."
