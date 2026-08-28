package music

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// The importer's judgement calls — what counts as a song, and what a song is
// called when nothing tells you — are the parts a user notices when they get it
// wrong: a library where every track is named "01" or where cover.jpg shows up
// as a failed import.

func TestTitleFromFilenameSplitsArtistAndTitle(t *testing.T) {
	cases := map[string]trackMeta{
		// The near-universal download conventions.
		"Radiohead - Creep.mp3":       {Artist: "Radiohead", Title: "Creep"},
		"01 - Radiohead - Creep.mp3":  {Artist: "Radiohead", Title: "Creep"},
		"1. Radiohead - Creep.flac":   {Artist: "Radiohead", Title: "Creep"},
		"003 - Radiohead - Creep.m4a": {Artist: "Radiohead", Title: "Creep"},
		// A hyphen inside the title must stay in the title — splitting on every
		// separator would truncate half the songs with a dash in their name.
		"Radiohead - Paranoid - Android.mp3": {Artist: "Radiohead", Title: "Paranoid - Android"},
		// Nothing to split: the stem is the title, not a guess.
		"Creep.mp3":      {Title: "Creep"},
		"01 - Creep.mp3": {Title: "Creep"},
		// A number followed only by a space is NOT a track number — mangling a
		// real title is worse than leaving a position marker in one.
		"99 Luftballons.mp3": {Title: "99 Luftballons"},
		"01 Creep.mp3":       {Title: "01 Creep"},
		// Underscore separators show up in ripped libraries too.
		"04_Radiohead - Creep.mp3": {Artist: "Radiohead", Title: "Creep"},
	}
	for name, want := range cases {
		got := titleFromFilename(name)
		if got.Title != want.Title || got.Artist != want.Artist {
			t.Errorf("titleFromFilename(%q) = {artist:%q title:%q}, want {artist:%q title:%q}",
				name, got.Artist, got.Title, want.Artist, want.Title)
		}
	}
}

// The separator is what makes a leading number a track number. Without one it is
// part of the title, and stripping it would rename "99 Luftballons" to
// "Luftballons" — a silent corruption of somebody's library.
func TestStripTrackNumberNeedsASeparator(t *testing.T) {
	strip := map[string]string{
		"01 - Creep": "Creep",
		"1. Creep":   "Creep",
		"003_Creep":  "Creep",
		"12.Creep":   "Creep",
	}
	for in, want := range strip {
		if got := stripTrackNumber(in); got != want {
			t.Errorf("stripTrackNumber(%q) = %q, want %q", in, got, want)
		}
	}
	for _, keep := range []string{"99 Luftballons", "01 Creep", "Creep", "2 Unlimited", "0001 - X", "12"} {
		if got := stripTrackNumber(keep); got != keep {
			t.Errorf("stripTrackNumber(%q) = %q, want it untouched", keep, got)
		}
	}
}

func TestAudioEntriesPicksOnlyAudioInStableOrder(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range []string{
		"album/02 - Second.mp3",
		"album/01 - First.mp3",
		"album/cover.jpg",                 // artwork: skipped, not a failure
		"album/notes.txt",                 // ditto
		"__MACOSX/album/._01 - First.mp3", // macOS shadow tree
		// The same shadow tree as a Windows zip tool writes it: backslashes
		// instead of the forward slash the zip spec calls for.
		"__MACOSX\\album\\._99 - Windows.mp3",
		"album/._02 - Second.mp3", // AppleDouble stub
		"album/",                  // directory entry
		"album/Third.FLAC",        // extension case must not matter
	} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("x"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}

	got := audioEntries(zr)
	want := []string{"album/01 - First.mp3", "album/02 - Second.mp3", "album/Third.FLAC"}
	if len(got) != len(want) {
		names := make([]string, len(got))
		for i, f := range got {
			names[i] = f.Name
		}
		t.Fatalf("audioEntries = %v, want %v", names, want)
	}
	for i, w := range want {
		if got[i].Name != w {
			t.Errorf("entry %d = %q, want %q (sorted so a re-run reports the same order)", i, got[i].Name, w)
		}
	}
}

func TestFirstTagPrefersTitleThenFallbacks(t *testing.T) {
	tags := map[string]string{"album_artist": "Various", "performer": "Nobody"}
	if got := firstTag(tags, "artist", "album_artist", "performer"); got != "Various" {
		t.Errorf("firstTag = %q, want %q — the fallback order is the preference order", got, "Various")
	}
	if got := firstTag(tags, "artist"); got != "" {
		t.Errorf("firstTag = %q, want empty when no key matches", got)
	}
}

func TestHumanBytesReadsAsASize(t *testing.T) {
	cases := map[int64]string{
		512:            "512 B",
		2048:           "2.0 KB",
		5 << 20:        "5.0 MB",
		int64(4) << 30: "4.0 GB",
	}
	for n, want := range cases {
		if got := humanBytes(n); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", n, got, want)
		}
	}
}

// ── Repository stubs for the existing fakeRepo ─────────────────────────────
//
// The import surface joined music.Repository, so the shared fake in
// music_test.go has to satisfy it. These are no-ops on purpose: nothing in the
// existing track tests touches an import, and a fake that pretended to would
// only invite tests to assert on fiction.

func (f *fakeRepo) CreateImport(context.Context, uuid.UUID) (ImportJob, error) {
	return ImportJob{}, errNotImplementedInFake
}
func (f *fakeRepo) GetImport(context.Context, uuid.UUID) (ImportJob, error) {
	return ImportJob{}, errNotImplementedInFake
}
func (f *fakeRepo) ListImports(context.Context, uuid.UUID, int) ([]ImportJob, error) {
	return nil, errNotImplementedInFake
}
func (f *fakeRepo) SetImportUpload(context.Context, uuid.UUID, string) (ImportJob, error) {
	return ImportJob{}, errNotImplementedInFake
}
func (f *fakeRepo) StartImport(context.Context, uuid.UUID, int) error {
	return errNotImplementedInFake
}
func (f *fakeRepo) FinishImport(context.Context, uuid.UUID, string, int, int, string, string) error {
	return errNotImplementedInFake
}

var errNotImplementedInFake = errors.New("music: import not implemented in this fake")

// MediaAPI gained Ingest for the zip import; the track tests only ever validate
// asset references, so the fake declines rather than pretends.
func (f *fakeMedia) Ingest(context.Context, uuid.UUID, string, string, []byte) (uuid.UUID, error) {
	return uuid.Nil, errNotImplementedInFake
}
