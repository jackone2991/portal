package musicrepo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/portal/backend/internal/modules/music"
)

// Playlist persistence (0041). Kept in its own file so the feature adds nothing
// to adapter.go, which is under active work elsewhere.

func (a *Adapter) CreatePlaylist(ctx context.Context, ownerID uuid.UUID, name string, description *string) (music.Playlist, error) {
	row, err := a.q.CreatePlaylist(ctx, CreatePlaylistParams{
		OwnerUserID: pgUUID(ownerID), Name: name, Description: description,
	})
	if err != nil {
		// The INSERT is ON CONFLICT DO NOTHING, so a duplicate name comes back as
		// "no rows" rather than 23505 — which is the point: a raised constraint
		// would abort the request's tenant transaction and turn the 409 into a
		// 500 at COMMIT.
		if errors.Is(err, pgx.ErrNoRows) {
			return music.Playlist{}, music.ErrPlaylistExists
		}
		return music.Playlist{}, err
	}
	return toPlaylist(row, 0), nil
}

func (a *Adapter) GetPlaylist(ctx context.Context, ownerID, id uuid.UUID) (music.Playlist, error) {
	row, err := a.q.GetPlaylist(ctx, GetPlaylistParams{ID: pgUUID(id), OwnerUserID: pgUUID(ownerID)})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return music.Playlist{}, music.ErrPlaylistNotFound
		}
		return music.Playlist{}, err
	}
	return toPlaylist(row, 0), nil
}

func (a *Adapter) ListPlaylists(ctx context.Context, ownerID uuid.UUID) ([]music.Playlist, error) {
	rows, err := a.q.ListPlaylists(ctx, pgUUID(ownerID))
	if err != nil {
		return nil, err
	}
	out := make([]music.Playlist, 0, len(rows))
	for _, r := range rows {
		out = append(out, music.Playlist{
			ID:          uuidFrom(r.ID),
			OwnerID:     uuidFrom(r.OwnerUserID),
			Name:        r.Name,
			Description: r.Description,
			TrackCount:  int(r.TrackCount),
			CreatedAt:   r.CreatedAt.Time,
			UpdatedAt:   r.UpdatedAt.Time,
		})
	}
	return out, nil
}

func (a *Adapter) RenamePlaylist(ctx context.Context, ownerID, id uuid.UUID, name *string, setDescription bool, description *string) (music.Playlist, error) {
	// UPDATE has no ON CONFLICT, so the collision is checked before the write
	// rather than caught after it: a raised unique would abort the request's
	// tenant transaction and cost the caller a 500 instead of a 409.
	if name != nil {
		taken, err := a.q.PlaylistNameTaken(ctx, PlaylistNameTakenParams{
			OwnerUserID: pgUUID(ownerID), Column2: *name, ID: pgUUID(id),
		})
		if err != nil {
			return music.Playlist{}, err
		}
		if taken {
			return music.Playlist{}, music.ErrPlaylistExists
		}
	}
	row, err := a.q.RenamePlaylist(ctx, RenamePlaylistParams{
		ID: pgUUID(id), OwnerUserID: pgUUID(ownerID),
		Name: name, SetDescription: setDescription, Description: description,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return music.Playlist{}, music.ErrPlaylistNotFound
		}
		return music.Playlist{}, err
	}
	return toPlaylist(row, 0), nil
}

func (a *Adapter) DeletePlaylist(ctx context.Context, ownerID, id uuid.UUID) error {
	_, err := a.q.DeletePlaylist(ctx, DeletePlaylistParams{ID: pgUUID(id), OwnerUserID: pgUUID(ownerID)})
	if errors.Is(err, pgx.ErrNoRows) {
		return music.ErrPlaylistNotFound
	}
	return err
}

// AddTracks appends the caller's own tracks, skipping duplicates.
//
// The ownership filter is the point: the handler receives an id list from the
// client, and "we looped over what you sent" is not an authorisation check. Any
// id the caller does not own is dropped here, before a row is written.
func (a *Adapter) AddTracks(ctx context.Context, ownerID, playlistID uuid.UUID, trackIDs []uuid.UUID) (int, error) {
	if _, err := a.GetPlaylist(ctx, ownerID, playlistID); err != nil {
		return 0, err
	}
	owned, err := a.OwnedTrackIDs(ctx, ownerID, trackIDs)
	if err != nil {
		return 0, err
	}
	next, err := a.q.NextPlaylistPosition(ctx, pgUUID(playlistID))
	if err != nil {
		return 0, err
	}
	added := 0
	for _, id := range owned {
		n, err := a.q.AddPlaylistTrack(ctx, AddPlaylistTrackParams{
			PlaylistID: pgUUID(playlistID), TrackID: pgUUID(id), Position: next,
		})
		if err != nil {
			return added, err
		}
		if n == 0 {
			continue // already in the playlist: not an error, and not an addition
		}
		next++
		added++
	}
	return added, nil
}

func (a *Adapter) RemoveTrack(ctx context.Context, ownerID, playlistID, trackID uuid.UUID) error {
	if _, err := a.GetPlaylist(ctx, ownerID, playlistID); err != nil {
		return err
	}
	return a.q.RemovePlaylistTrack(ctx, RemovePlaylistTrackParams{
		PlaylistID: pgUUID(playlistID), TrackID: pgUUID(trackID),
	})
}

func (a *Adapter) ListPlaylistTracks(ctx context.Context, ownerID, playlistID uuid.UUID) ([]music.Track, error) {
	if _, err := a.GetPlaylist(ctx, ownerID, playlistID); err != nil {
		return nil, err
	}
	rows, err := a.q.ListPlaylistTracks(ctx, pgUUID(playlistID))
	if err != nil {
		return nil, err
	}
	// The join row carries every music_tracks column plus `position`, so it maps
	// through the same MusicTrack shape the rest of the adapter uses — one
	// conversion, not a second copy of it that can drift.
	out := make([]music.Track, 0, len(rows))
	for _, r := range rows {
		out = append(out, toTrack(MusicTrack{
			ID: r.ID, OwnerUserID: r.OwnerUserID, TenantID: r.TenantID, Title: r.Title,
			Artist: r.Artist, Album: r.Album, Description: r.Description,
			AudioAssetID: r.AudioAssetID, CoverAssetID: r.CoverAssetID, Status: r.Status,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			ReleaseYear: r.ReleaseYear, Genre: r.Genre,
			MbRecordingID: r.MbRecordingID, MbReleaseID: r.MbReleaseID,
			LookupStatus: r.LookupStatus, LookupNote: r.LookupNote, LookupAt: r.LookupAt,
		}))
	}
	return out, nil
}

func (a *Adapter) OwnedTrackIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	pg := make([]pgtype.UUID, 0, len(ids))
	for _, id := range ids {
		pg = append(pg, pgUUID(id))
	}
	rows, err := a.q.OwnedTrackIDs(ctx, OwnedTrackIDsParams{OwnerUserID: pgUUID(ownerID), Ids: pg})
	if err != nil {
		return nil, err
	}
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, uuidFrom(r))
	}
	return out, nil
}

func toPlaylist(r MusicPlaylist, trackCount int) music.Playlist {
	return music.Playlist{
		ID:          uuidFrom(r.ID),
		OwnerID:     uuidFrom(r.OwnerUserID),
		Name:        r.Name,
		Description: r.Description,
		TrackCount:  trackCount,
		CreatedAt:   r.CreatedAt.Time,
		UpdatedAt:   r.UpdatedAt.Time,
	}
}
