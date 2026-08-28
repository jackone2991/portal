package musicrepo

// Adapter methods for the bulk zip import (0038). Split from adapter.go because
// the import is a job surface, not part of the track CRUD — same *Adapter, so
// cmd/api and cmd/worker still construct exactly one.

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/portal/backend/internal/modules/music"
)

func (a *Adapter) CreateImport(ctx context.Context, ownerID uuid.UUID) (music.ImportJob, error) {
	row, err := a.q.CreateMusicImport(ctx, pgUUID(ownerID))
	if err != nil {
		return music.ImportJob{}, err
	}
	return music.ImportJob{
		ID:          uuidFrom(row.ID),
		OwnerUserID: uuidFrom(row.OwnerUserID),
		Status:      row.Status,
		UploadRef:   deref(row.UploadRef),
		Total:       int(row.Total),
		Succeeded:   int(row.Succeeded),
		Failed:      int(row.Failed),
		Report:      row.Report,
		Error:       deref(row.Error),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (a *Adapter) GetImport(ctx context.Context, id uuid.UUID) (music.ImportJob, error) {
	row, err := a.q.GetMusicImport(ctx, pgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return music.ImportJob{}, music.ErrNotFound
		}
		return music.ImportJob{}, err
	}
	return music.ImportJob{
		ID:          uuidFrom(row.ID),
		OwnerUserID: uuidFrom(row.OwnerUserID),
		Status:      row.Status,
		UploadRef:   deref(row.UploadRef),
		Total:       int(row.Total),
		Succeeded:   int(row.Succeeded),
		Failed:      int(row.Failed),
		Report:      row.Report,
		Error:       deref(row.Error),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (a *Adapter) ListImports(ctx context.Context, ownerID uuid.UUID, limit int) ([]music.ImportJob, error) {
	rows, err := a.q.ListMusicImports(ctx, ListMusicImportsParams{
		OwnerUserID: pgUUID(ownerID),
		Limit:       int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]music.ImportJob, 0, len(rows))
	for _, row := range rows {
		out = append(out, music.ImportJob{
			ID:          uuidFrom(row.ID),
			OwnerUserID: uuidFrom(row.OwnerUserID),
			Status:      row.Status,
			UploadRef:   deref(row.UploadRef),
			Total:       int(row.Total),
			Succeeded:   int(row.Succeeded),
			Failed:      int(row.Failed),
			Report:      row.Report,
			Error:       deref(row.Error),
			CreatedAt:   row.CreatedAt.Time,
			UpdatedAt:   row.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (a *Adapter) SetImportUpload(ctx context.Context, id uuid.UUID, key string) (music.ImportJob, error) {
	row, err := a.q.SetMusicImportUpload(ctx, SetMusicImportUploadParams{
		ID:        pgUUID(id),
		UploadRef: &key,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return music.ImportJob{}, music.ErrNotFound
		}
		return music.ImportJob{}, err
	}
	return music.ImportJob{
		ID:          uuidFrom(row.ID),
		OwnerUserID: uuidFrom(row.OwnerUserID),
		Status:      row.Status,
		UploadRef:   deref(row.UploadRef),
		Total:       int(row.Total),
		Succeeded:   int(row.Succeeded),
		Failed:      int(row.Failed),
		Report:      row.Report,
		Error:       deref(row.Error),
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}, nil
}

func (a *Adapter) StartImport(ctx context.Context, id uuid.UUID, total int) error {
	return a.q.StartMusicImport(ctx, StartMusicImportParams{ID: pgUUID(id), Total: int32(total)})
}

// FinishImport writes the terminal state. `report` arrives as a JSON STRING, not
// []byte, and the query casts it text->jsonb — the pool runs QueryExecModeExec,
// where pgx picks the wire OID from the Go type without describing params, so a
// []byte would go out as bytea and jsonb rejects it (SQLSTATE 22P02).
func (a *Adapter) FinishImport(ctx context.Context, id uuid.UUID, status string, succeeded, failed int, report, errMsg string) error {
	var errPtr *string
	if errMsg != "" {
		errPtr = &errMsg
	}
	return a.q.FinishMusicImport(ctx, FinishMusicImportParams{
		ID:        pgUUID(id),
		Status:    status,
		Succeeded: int32(succeeded),
		Failed:    int32(failed),
		Report:    report,
		Error:     errPtr,
	})
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
