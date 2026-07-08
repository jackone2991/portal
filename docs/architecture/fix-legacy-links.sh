#!/usr/bin/env bash
# Post-migration link fixes for docs/architecture/{security,frontend}.md
# (moved from doc/en/ without content rewrite). Run from repo root.
set -euo pipefail
for f in docs/architecture/security.md docs/architecture/frontend.md; do
  [ -f "$f" ] || continue
  sed -i \
    -e 's|(architecture/0\([0-9]\)-|(../adr/0\1-|g' \
    -e 's|(archivetech\.md)|(deferred/access-policies.md)|g' \
    -e 's|(archivetech-backend\.md)|(deferred/multi-tenant-backend.md)|g' \
    -e 's|(feature\.md)|(../product/feature-inventory.md)|g' \
    -e 's|(missing-features\.md)|(../product/backlog.md)|g' \
    -e 's|(\.\./\.\./CLAUDE\.md)|(../../CLAUDE.md)|g' \
    "$f"
done
# security.md: retire the legacy "Authoration" title in place
sed -i '1s|# Authoration — |# Security — |' docs/architecture/security.md
echo "legacy links fixed; review with: git diff docs/architecture"
