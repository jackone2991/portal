package ops

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	opsapi "github.com/portal/backend/internal/modules/ops/api"
	"github.com/portal/backend/internal/platform/audit"
)

const backupContentType = "application/octet-stream"

// Manifest is the backups/pg/LATEST.json body (P0.2 step 4). The restore drill
// (P0.4) reads it to pick the dump — never latest-by-key-listing, which a failed
// partial upload could poison.
type Manifest struct {
	StorageKey string    `json:"storage_key"`
	SHA256     string    `json:"sha256"`
	SizeBytes  int64     `json:"size_bytes"`
	FinishedAt time.Time `json:"finished_at"`
}

// BackupDatabase is the ops:backup_database task body (P0.2), run nightly on the
// shared scheduler. It opens a run row, streams pg_dump -Fc through a sha256
// hasher into storage, closes the row, overwrites LATEST.json, prunes old dumps,
// and emits its event + audit. On any failure it closes the row 'failed' and
// returns nil — a failed run is terminal, never a wedged 'running', so the next
// night simply runs again (P0.2 AC "no wedged state").
func (m *Module) BackupDatabase(ctx context.Context) error {
	if m.deps.Store == nil {
		return errors.New("ops: backup requires a storage backend")
	}

	runID, err := m.deps.Repo.InsertBackupRun(ctx)
	if err != nil {
		return fmt.Errorf("ops: open backup run: %w", err)
	}
	log.Info().Str("run", runID.String()).Msg("ops: backup started")

	size, key, sha, err := m.dumpToStorage(ctx)
	if err != nil {
		msg := truncate(err.Error(), 1000)
		log.Error().Err(err).Str("run", runID.String()).Msg("ops: backup failed")
		if ferr := m.deps.Repo.FinishBackupRunFailed(ctx, runID, msg); ferr != nil {
			log.Error().Err(ferr).Str("run", runID.String()).Msg("ops: record failed run")
		}
		m.emitFailed(ctx, runID, msg)
		m.auditBackup(ctx, ActionBackupFailed, runID, map[string]any{"error": msg})
		return nil
	}

	finishedAt := time.Now().UTC()
	if err := m.deps.Repo.FinishBackupRunOK(ctx, runID, size, key); err != nil {
		// The dump is safely stored; we merely failed to close the row. Surface
		// the error so the scheduler logs it — the object itself is intact.
		return fmt.Errorf("ops: record ok run: %w", err)
	}
	log.Info().Str("run", runID.String()).Str("key", key).Int64("size", size).Msg("ops: backup ok")

	m.writeManifest(ctx, Manifest{StorageKey: key, SHA256: sha, SizeBytes: size, FinishedAt: finishedAt})
	m.pruneRetention(ctx, key)
	m.emitCompleted(ctx, runID, size)
	m.auditBackup(ctx, ActionBackupCompleted, runID, map[string]any{"size_bytes": size, "storage_key": key})
	return nil
}

// dumpToStorage runs `pg_dump -Fc` connecting DIRECTLY to Postgres (not
// PgBouncer). pg_dump's stdout is spooled to a temp file while a sha256 hasher
// tees the same bytes, then the (now seekable) file is uploaded: the S3 client
// needs a seekable body to compute the payload hash, so a straight stdout→Put
// pipe fails with "request stream is not seekable".
func (m *Module) dumpToStorage(ctx context.Context) (size int64, key, sha string, err error) {
	if m.deps.BackupDatabaseURL == "" {
		return 0, "", "", errors.New("BACKUP_DATABASE_URL is not configured (must point at Postgres directly, not PgBouncer)")
	}
	key = dumpKeyForDate(time.Now().UTC())

	tmp, err := os.CreateTemp("", "pgdump-*.dump")
	if err != nil {
		return 0, "", "", fmt.Errorf("tempfile: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	// -Fc = custom (compressed) format for pg_restore; --no-owner/--no-privileges
	// keep the dump portable into a scratch DB during the restore drill.
	//nolint:gosec // fixed args; the DSN is server-controlled config, not user input.
	cmd := exec.CommandContext(ctx, "pg_dump", "-Fc", "--no-owner", "--no-privileges", "--dbname", m.deps.BackupDatabaseURL)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	hasher := sha256.New()
	cmd.Stdout = io.MultiWriter(tmp, hasher) // dump → temp file + digest in one pass

	if err := cmd.Run(); err != nil {
		return 0, "", "", fmt.Errorf("pg_dump: %w: %s", err, tail(stderr.Bytes(), 400))
	}
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return 0, "", "", fmt.Errorf("seek dump: %w", err)
	}

	if err := m.deps.Store.Put(ctx, key, tmp, backupContentType); err != nil {
		_ = m.deps.Store.Delete(ctx, key) // best-effort: no partial object left behind
		return 0, "", "", fmt.Errorf("upload dump: %w", err)
	}

	size, err = m.deps.Store.Size(ctx, key)
	if err != nil {
		return 0, "", "", fmt.Errorf("stat dump: %w", err)
	}
	return size, key, hex.EncodeToString(hasher.Sum(nil)), nil
}

// writeManifest overwrites LATEST.json. Best-effort: the dump is already durable,
// so a manifest write failure is logged, not fatal (the next run overwrites it).
func (m *Module) writeManifest(ctx context.Context, man Manifest) {
	body, err := json.Marshal(man)
	if err != nil {
		log.Error().Err(err).Msg("ops: marshal LATEST.json")
		return
	}
	if err := m.deps.Store.Put(ctx, LatestManifestKey, bytes.NewReader(body), "application/json"); err != nil {
		log.Error().Err(err).Msg("ops: write LATEST.json")
	}
}

// pruneRetention drops dumps outside the keep window (P0.2 step 5). The candidate
// set is the successful-run ledger (the DB is authoritative for what exists);
// the just-written dump and the manifest are never pruned.
func (m *Module) pruneRetention(ctx context.Context, currentKey string) {
	refs, err := m.deps.Repo.ListSuccessfulRuns(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ops: retention list")
		return
	}

	seen := make(map[string]bool, len(refs))
	dumps := make([]DumpMeta, 0, len(refs))
	for _, r := range refs {
		if r.StorageKey == "" || seen[r.StorageKey] {
			continue // same-day reruns overwrite one object → one DumpMeta
		}
		seen[r.StorageKey] = true
		date, ok := parseDumpDate(r.StorageKey)
		if !ok {
			date = r.FinishedAt // non-standard key → fall back to completion time
		}
		dumps = append(dumps, DumpMeta{Key: r.StorageKey, Date: date})
	}

	_, pruned := keep(dumps, time.Now().UTC())
	for _, d := range pruned {
		if d.Key == currentKey || d.Key == LatestManifestKey {
			continue // never delete the live dump or the manifest
		}
		if err := m.deps.Store.Delete(ctx, d.Key); err != nil {
			log.Warn().Err(err).Str("key", d.Key).Msg("ops: retention prune")
		}
	}
}

// ── events + audit (best-effort; a nil dep is a no-op) ───────────────

func (m *Module) emitCompleted(ctx context.Context, runID uuid.UUID, size int64) {
	if m.deps.Events == nil {
		return
	}
	if err := m.deps.Events.Publish(ctx, opsapi.EventBackupCompleted, opsapi.BackupCompletedEvent{RunID: runID, SizeBytes: size}); err != nil {
		log.Warn().Err(err).Msg("ops: publish backup_completed")
	}
}

func (m *Module) emitFailed(ctx context.Context, runID uuid.UUID, msg string) {
	if m.deps.Events == nil {
		return
	}
	if err := m.deps.Events.Publish(ctx, opsapi.EventBackupFailed, opsapi.BackupFailedEvent{RunID: runID, Error: msg}); err != nil {
		log.Warn().Err(err).Msg("ops: publish backup_failed")
	}
}

func (m *Module) auditBackup(ctx context.Context, action string, runID uuid.UUID, meta map[string]any) {
	if m.deps.Audit == nil {
		return
	}
	m.deps.Audit.Write(ctx, audit.Event{
		Action:     action,
		ActorKind:  "system",
		TargetKind: "ops_backup_run",
		TargetID:   runID.String(),
		Metadata:   meta,
	})
}

// ── small string helpers (mirror media/worker) ──────────────────────

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

func tail(b []byte, n int) string {
	if len(b) > n {
		b = b[len(b)-n:]
	}
	return string(bytes.TrimSpace(b))
}
