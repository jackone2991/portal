#!/usr/bin/env bash
# Relative-link check for Markdown (ADR-11). Blocking in CI.
#
# Deliberately small: relative links only (no network), no anchor validation.
# A link is broken if the path it names, resolved from the file it is in, does
# not exist. Two trees are skipped:
#   template-main/          reference material, not ours
#   docs/product/analysis/  dated audits are immutable (ADR-11 rule 6) — a link
#                           that rotted inside one is part of the record, and
#                           the checker must not force an edit to the body
#
#   scripts/check-links.sh            # prints each broken link, exit 1 if any
#
# Why this exists: an audit on 2026-09-11 found 95 broken relative links across
# the docs. A rule that is not checked is a rule that will be broken again.
set -u

# The inner loop runs in a subshell (pipe), so a counter variable would not
# survive it. A temp file does.
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT

while IFS= read -r file; do
  dir=$(dirname "$file")
  # [text](target) — take everything up to the first ')' or ' ' (title), drop
  # anchors and query strings, keep only relative targets. `%20` is a space in a
  # link and a literal in a path, so it is decoded before the existence test.
  grep -oE '\]\([^)[:space:]]+' "$file" | sed -E 's/^\]\(//; s/[#?].*$//; s/%20/ /g' | while IFS= read -r target; do
    [ -z "$target" ] && continue
    case "$target" in
      http://*|https://*|mailto:*|/*) continue ;;   # absolute or external: not ours
    esac
    if [ ! -e "$dir/$target" ]; then
      printf '%s: broken link -> %s\n' "$file" "$target"
      echo x >>"$tmp"
    fi
  done
done < <(git ls-files '*.md' | grep -v -e '^template-main/' -e '^docs/product/analysis/')

if [ -s "$tmp" ]; then
  echo "$(wc -l <"$tmp") broken relative link(s)."
  exit 1
fi
echo "all relative links resolve."
