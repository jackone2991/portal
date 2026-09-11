#!/usr/bin/env bash
# Retired-name check (ADR-11). Blocking in CI, alongside check-links.sh.
#
# The link checker cannot catch the class of defect that produced most of the
# 79 false statements found on 2026-09-11: plain-prose or inline-code mentions
# of things that no longer exist. A repo full of "see MILESTONE_CHECKS.md" passes
# a link checker cleanly. So: a deny-list of retired names, and a living line may
# mention one only if the same line says it is gone — the citation forms
# STYLE.md allows (`name` (deleted in `abc1234`), "then `name`", "renamed",
# "retired", a §-citation into a renamed document, …). Anything else reads as a
# claim that the thing still exists.
#
# Skipped: template-main/ (not ours); docs/product/analysis/ (immutable audits);
# the ADR that retired the names; and, inside every ADR, the Decision / Options /
# Trade-offs layers (kept verbatim by ADR-11 rule 2 — history may name what it
# knew). An ADR's Context, Consequences and Action items — its fact layer — are
# checked like any other document.
set -u

NAMES='MILESTONE_CHECKS\.md|docs/archive/|vi-2026-07|doc/en/|doc/vi/|archivetech(-backend)?\.md|authoration\.md|MIGRATION\.md|delivery-plan\.md|missing-features\.md|gap-audit-2026-07\.md|docker-compose\.prod\.yml|pgbouncer:6432'
ALLOW='deleted|then `|was `|formerly|renamed|never existed|no such file|does not exist|there is no|is gone|retired|→|§|now `|the old '

tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
git ls-files '*.md' \
  | grep -v -e '^template-main/' -e '^docs/product/analysis/' -e '^docs/adr/11-' \
  | while IFS= read -r file; do
      case "$file" in
        docs/adr/*) skip_narrative=1 ;;
        *)          skip_narrative=0 ;;
      esac
      awk -v skip="$skip_narrative" '
        skip && /^## Decision/     { off=1 }
        skip && /^## Consequences/ { off=0 }
        !off { print NR": "$0 }
      ' "$file" | grep -E "$NAMES" | grep -viE "$ALLOW" | while IFS= read -r hit; do
        printf '%s:%s\n' "$file" "$hit"
        echo x >>"$tmp"
      done
    done

if [ -s "$tmp" ]; then
  echo "$(wc -l <"$tmp") mention(s) of a retired name without a retirement cue on the same line."
  exit 1
fi
echo "no retired name is cited as if it still existed."
