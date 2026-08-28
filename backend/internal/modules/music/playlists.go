package music

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Playlists (0041) — one owner's ordered selection of their own tracks.
//
// Kept in its own file, with its own repository interface, so the feature adds
// nothing to types.go and does not collide with work in flight elsewhere in the
// module.

var (
	ErrPlaylistNotFound = errors.New("music: playlist not found")
	// ErrPlaylistExists is the (owner, lower(name)) unique: you already have a
	// playlist by that name, and a second one would be a mistake every time.
	ErrPlaylistExists  = errors.New("music: a playlist with that name already exists")
	ErrPlaylistInvalid = errors.New("music: invalid playlist")
)

const maxPlaylistName = 120

// Playlist is one playlist, with how many tracks are in it.
type Playlist struct {
	ID          uuid.UUID
	OwnerID     uuid.UUID
	Name        string
	Description *string
	TrackCount  int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PlaylistRepository is the playlist persistence surface. The same adapter that
// implements Repository implements this; it is a separate interface only so the
// playlist feature stays in its own files.
type PlaylistRepository interface {
	CreatePlaylist(ctx context.Context, ownerID uuid.UUID, name string, description *string) (Playlist, error)
	GetPlaylist(ctx context.Context, ownerID, id uuid.UUID) (Playlist, error)
	ListPlaylists(ctx context.Context, ownerID uuid.UUID) ([]Playlist, error)
	RenamePlaylist(ctx context.Context, ownerID, id uuid.UUID, name *string, setDescription bool, description *string) (Playlist, error)
	DeletePlaylist(ctx context.Context, ownerID, id uuid.UUID) error
	// AddTracks appends the given tracks, skipping any already present and any
	// the owner does not own. Returns how many were actually added.
	AddTracks(ctx context.Context, ownerID, playlistID uuid.UUID, trackIDs []uuid.UUID) (int, error)
	RemoveTrack(ctx context.Context, ownerID, playlistID, trackID uuid.UUID) error
	ListPlaylistTracks(ctx context.Context, ownerID, playlistID uuid.UUID) ([]Track, error)
	// OwnedTrackIDs filters a caller-supplied id list down to the ones they own.
	OwnedTrackIDs(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID) ([]uuid.UUID, error)
}

/* ── service ─────────────────────────────────────────────────────── */

func (s *Service) CreatePlaylist(ctx context.Context, ownerID uuid.UUID, name string, description *string) (Playlist, error) {
	name = strings.TrimSpace(name)
	if name == "" || len([]rune(name)) > maxPlaylistName {
		return Playlist{}, ErrPlaylistInvalid
	}
	if s.playlists == nil {
		return Playlist{}, ErrPlaylistNotFound
	}
	return s.playlists.CreatePlaylist(ctx, ownerID, name, description)
}

func (s *Service) ListPlaylists(ctx context.Context, ownerID uuid.UUID) ([]Playlist, error) {
	if s.playlists == nil {
		return nil, nil
	}
	return s.playlists.ListPlaylists(ctx, ownerID)
}

func (s *Service) GetPlaylist(ctx context.Context, ownerID, id uuid.UUID) (Playlist, []Track, error) {
	if s.playlists == nil {
		return Playlist{}, nil, ErrPlaylistNotFound
	}
	p, err := s.playlists.GetPlaylist(ctx, ownerID, id)
	if err != nil {
		return Playlist{}, nil, err
	}
	tracks, err := s.playlists.ListPlaylistTracks(ctx, ownerID, id)
	if err != nil {
		return Playlist{}, nil, err
	}
	return p, tracks, nil
}

func (s *Service) RenamePlaylist(ctx context.Context, ownerID, id uuid.UUID, name *string, setDescription bool, description *string) (Playlist, error) {
	if s.playlists == nil {
		return Playlist{}, ErrPlaylistNotFound
	}
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" || len([]rune(n)) > maxPlaylistName {
			return Playlist{}, ErrPlaylistInvalid
		}
		name = &n
	}
	return s.playlists.RenamePlaylist(ctx, ownerID, id, name, setDescription, description)
}

func (s *Service) DeletePlaylist(ctx context.Context, ownerID, id uuid.UUID) error {
	if s.playlists == nil {
		return ErrPlaylistNotFound
	}
	return s.playlists.DeletePlaylist(ctx, ownerID, id)
}

// AddToPlaylist appends tracks the caller owns. Returns how many landed —
// selecting 30 and adding 28 because two were already there is worth saying.
func (s *Service) AddToPlaylist(ctx context.Context, ownerID, playlistID uuid.UUID, trackIDs []uuid.UUID) (int, error) {
	if s.playlists == nil {
		return 0, ErrPlaylistNotFound
	}
	if len(trackIDs) == 0 {
		return 0, nil
	}
	return s.playlists.AddTracks(ctx, ownerID, playlistID, trackIDs)
}

func (s *Service) RemoveFromPlaylist(ctx context.Context, ownerID, playlistID, trackID uuid.UUID) error {
	if s.playlists == nil {
		return ErrPlaylistNotFound
	}
	return s.playlists.RemoveTrack(ctx, ownerID, playlistID, trackID)
}

/* ── bulk publish ────────────────────────────────────────────────── */

// BulkSetStatus publishes or unpublishes many tracks at once.
//
// The library lists hundreds of imported tracks, and publishing them one HTTP
// call at a time is both slow and a burst the API has no reason to absorb. The
// id list is filtered through OwnedTrackIDs first, so a crafted request can only
// ever move the caller's own tracks — the per-track route's ownership middleware
// has no equivalent here, and "the loop is over ids you sent" is not a check.
//
// Returns the ids that changed, so the UI can report "28 of 30" rather than
// claiming everything worked.
func (s *Service) BulkSetStatus(ctx context.Context, ownerID uuid.UUID, ids []uuid.UUID, status string) ([]uuid.UUID, error) {
	if s.playlists == nil {
		return nil, ErrPlaylistNotFound
	}
	if status != StatusPublished && status != StatusDraft {
		return nil, ErrPlaylistInvalid
	}
	if len(ids) == 0 {
		return nil, nil
	}

	owned, err := s.playlists.OwnedTrackIDs(ctx, ownerID, ids)
	if err != nil {
		return nil, err
	}
	changed := make([]uuid.UUID, 0, len(owned))
	for _, id := range owned {
		t, err := s.repo.SetStatus(ctx, id, status)
		if err != nil {
			// One bad row must not discard the work already done: the caller is
			// told what did change, and can retry the rest.
			return changed, err
		}
		changed = append(changed, id)
		if status == StatusPublished {
			s.emitPublished(ctx, t)
		}
	}
	return changed, nil
}
