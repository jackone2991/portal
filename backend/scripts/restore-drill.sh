#!/usr/bin/env bash
# Restore drill — SPEC-09 P0.4. Proves a nightly backup is actually restorable.
# Runbook: docs/guides/backup-restore.md.
#
# Reads the backup MANIFEST (backups/pg/LATEST.json — never latest-by-listing,
# which a partial upload could poison), downloads + sha256-verifies the dump,
# pg_restores it into a THROWAWAY scratch database, and runs sanity checks. It
# consults NO application table (a fresh dev stack has no ops_backup_runs).
#
# Runs **docker-first**: object storage is reached with a one-off `minio/mc`
# container on the compose network, and pg_restore/psql run inside the running
# Postgres container — so no host-installed postgresql-client/aws is required
# (the "fresh dev stack" case, SPEC-09 P0.4). On a bare VPS where the stack
# isn't in Docker, run pg_restore/psql there against RESTORE_PGHOST directly.
#
# Env (auto-read from .env; override by exporting):
#   PG_CONTAINER (default portal-postgres-1)  RESTORE_NET (portal_internal)
#   S3_BUCKET  S3_ACCESS_KEY  S3_SECRET_KEY  (from .env)
#   SCRATCH_DB (portal_restore_check)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${ENV_FILE:-$ROOT/.env}"
val() { grep -E "^${1}=" "$ENV_FILE" 2>/dev/null | head -1 | cut -d= -f2- | sed -E 's/[[:space:]]+#.*$//; s/^["'\'']//; s/["'\'']$//'; }

PG_CONTAINER="${PG_CONTAINER:-portal-postgres-1}"
NET="${RESTORE_NET:-portal_internal}"
BUCKET="${S3_BUCKET:-$(val S3_BUCKET)}"; BUCKET="${BUCKET:-portal-media}"
S3_KEY="${S3_ACCESS_KEY:-$(val S3_ACCESS_KEY)}"; S3_KEY="${S3_KEY:-portal}"
S3_SECRET="${S3_SECRET_KEY:-$(val S3_SECRET_KEY)}"; S3_SECRET="${S3_SECRET:-change-me}"
PGUSER="${RESTORE_PGUSER:-$(val POSTGRES_USER)}"; PGUSER="${PGUSER:-portal}"
SCRATCH_DB="${SCRATCH_DB:-portal_restore_check}"

command -v docker >/dev/null 2>&1 || { echo "restore-drill: docker is required" >&2; exit 2; }
docker exec "$PG_CONTAINER" true >/dev/null 2>&1 || { echo "restore-drill: Postgres container '$PG_CONTAINER' not running — start the stack (make up)" >&2; exit 2; }

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"; docker exec "$PG_CONTAINER" rm -f /tmp/drill.dump 2>/dev/null || true' EXIT

mc() { docker run --rm --network "$NET" -v "$WORK:/w" --entrypoint sh minio/mc -c "mc alias set d http://minio:9000 '$S3_KEY' '$S3_SECRET' >/dev/null 2>&1 && $1"; }
psql_admin() { docker exec "$PG_CONTAINER" psql -U "$PGUSER" -d postgres -v ON_ERROR_STOP=1 -qc "$1"; }
scratchq() { docker exec "$PG_CONTAINER" psql -U "$PGUSER" -d "$SCRATCH_DB" -tAc "$1"; }

echo "==> 1. read manifest backups/pg/LATEST.json"
mc "mc cp d/$BUCKET/backups/pg/LATEST.json /w/LATEST.json >/dev/null"
KEY="$(python3 -c "import json;print(json.load(open('$WORK/LATEST.json'))['storage_key'])")"
WANT_SHA="$(python3 -c "import json;print(json.load(open('$WORK/LATEST.json'))['sha256'])")"
[ -n "$KEY" ] || { echo "manifest has no storage_key" >&2; exit 1; }
echo "    dump=$KEY  sha256=${WANT_SHA:0:16}…"

echo "==> 2. download dump + verify sha256"
mc "mc cp d/$BUCKET/$KEY /w/dump >/dev/null"
GOT_SHA="$( { command -v sha256sum >/dev/null 2>&1 && sha256sum "$WORK/dump" || shasum -a 256 "$WORK/dump"; } | awk '{print $1}')"
[ "$GOT_SHA" = "$WANT_SHA" ] || { echo "!! SHA256 MISMATCH: got $GOT_SHA want $WANT_SHA" >&2; exit 1; }
echo "    sha256 OK ($(wc -c <"$WORK/dump" | tr -d ' ') bytes)"

echo "==> 3. pg_restore into throwaway DB '$SCRATCH_DB'"
psql_admin "DROP DATABASE IF EXISTS $SCRATCH_DB;" >/dev/null
psql_admin "CREATE DATABASE $SCRATCH_DB;" >/dev/null
docker cp "$WORK/dump" "$PG_CONTAINER:/tmp/drill.dump"
docker exec "$PG_CONTAINER" pg_restore --no-owner --no-privileges -U "$PGUSER" -d "$SCRATCH_DB" /tmp/drill.dump
echo "    pg_restore exit 0"

echo "==> 4. sanity checks"
REPO_LATEST="$(ls "$ROOT/backend/db/migrations"/*.up.sql | sed -E 's#.*/0*([0-9]+)_.*#\1#' | sort -n | tail -1)"
VER="$(scratchq 'SELECT version FROM schema_migrations LIMIT 1;')"
USERS="$(scratchq 'SELECT count(*) FROM users;')"
ASSETS="$(scratchq 'SELECT count(*) FROM assets;')"   # media's real table name (not media_assets)
SYSROLES="$(scratchq 'SELECT count(*) FROM roles WHERE is_system;')"
echo "    migration=$VER (repo latest=$REPO_LATEST) · users=$USERS · assets=$ASSETS · system roles=$SYSROLES"
[ -n "$VER" ] && [ "$VER" -le "$REPO_LATEST" ] || { echo "!! restored migration $VER > repo latest $REPO_LATEST" >&2; exit 1; }
[ "${SYSROLES:-0}" -ge 7 ] || { echo "!! expected >= 7 system roles, got $SYSROLES" >&2; exit 1; }
[ -n "$USERS" ] && [ -n "$ASSETS" ] || { echo "!! users/assets not queryable" >&2; exit 1; }

echo "==> 5. drop scratch DB"
psql_admin "DROP DATABASE IF EXISTS $SCRATCH_DB;" >/dev/null
echo ""
echo "RESTORE DRILL PASSED — dump $KEY restored + sanity-checked cleanly."
