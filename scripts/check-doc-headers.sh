#!/usr/bin/env bash
# Status-header check for docs/ (STYLE.md § "Every document starts with a
# status header"; ADR-11). Blocking in CI, alongside check-links.sh.
#
# A document under docs/ must carry `**Last verified:**` within its first
# eight lines — a date, or the literal `never` for a file nobody has checked as
# a whole. The value is not validated beyond presence: the field's job is to be
# visible, and `never` is a legitimate, honest value. 41 of ~60 files lacked
# the header on 2026-09-11; a rule that is not checked is a rule that will be
# broken again.
set -u
tmp=$(mktemp)
trap 'rm -f "$tmp"' EXIT
while IFS= read -r file; do
  if ! head -n 8 "$file" | grep -q 'Last verified'; then
    printf '%s: no **Last verified:** in the first 8 lines\n' "$file"
    echo x >>"$tmp"
  fi
done < <(git ls-files 'docs/*.md')
if [ -s "$tmp" ]; then
  echo "$(wc -l <"$tmp") document(s) without a status header."
  exit 1
fi
echo "every docs/ document carries Last verified."
