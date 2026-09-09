package music

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
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

// MediaAPI also gained OpenOriginal, for the enrichment pass that re-reads an
// audio file to pull the cover art out of it.
func (f *fakeMedia) OpenOriginal(context.Context, uuid.UUID, uuid.UUID) (io.ReadCloser, string, error) {
	return nil, "", errNotImplementedInFake
}

/* ── enrichment ───────────────────────────────────────────────────── */

// Enrichment runs long after the import, so anything already on the track may be
// something the user typed. Overwriting it with whatever the file's tags happen
// to say would be worse than adding nothing at all.

func str(s string) *string { return &s }

func TestEnrichPatchFillsOnlyEmptyFields(t *testing.T) {
	cover := uuid.New()
	tags := map[string]string{"artist": "From Tags", "album": "Tag Album"}

	// Everything already set: nothing to do, and above all nothing to replace.
	full := Track{ID: uuid.New(), Artist: str("Typed"), Album: str("Typed Album"), CoverAssetID: &cover}
	if _, changed := enrichPatch(full, tags, &cover); changed {
		t.Error("enrichPatch reported a change on a fully populated track")
	}

	// Empty artist gets filled; a set album is left alone.
	partial := Track{ID: uuid.New(), Album: str("Typed Album")}
	patch, changed := enrichPatch(partial, tags, nil)
	if !changed {
		t.Fatal("enrichPatch = no change, want the empty artist filled")
	}
	if !patch.SetArtist || patch.Artist == nil || *patch.Artist != "From Tags" {
		t.Errorf("artist not filled from tags: %+v", patch)
	}
	if patch.SetAlbum {
		t.Error("album was overwritten — enrichment must never replace an existing value")
	}
}

// An empty string counts as empty, not as a value worth preserving: a track
// whose artist is "" is one nobody has filled in.
func TestEnrichPatchTreatsBlankAsMissing(t *testing.T) {
	patch, changed := enrichPatch(
		Track{ID: uuid.New(), Artist: str("")},
		map[string]string{"artist": "From Tags"},
		nil,
	)
	if !changed || !patch.SetArtist {
		t.Errorf("blank artist not treated as missing: %+v", patch)
	}
}

func TestEnrichPatchAttachesACoverOnlyWhenThereIsNone(t *testing.T) {
	existing, found := uuid.New(), uuid.New()

	patch, changed := enrichPatch(Track{ID: uuid.New()}, nil, &found)
	if !changed || !patch.SetCover || patch.CoverAssetID == nil || *patch.CoverAssetID != found {
		t.Errorf("cover not attached to a track without one: %+v", patch)
	}

	if _, changed := enrichPatch(Track{ID: uuid.New(), CoverAssetID: &existing}, nil, &found); changed {
		t.Error("an existing cover was replaced")
	}
}

// A file with no art and no tags leaves the track exactly as it was — the common
// case, and it must not produce a pointless write.
func TestEnrichPatchIsANoOpWithNothingToAdd(t *testing.T) {
	if _, changed := enrichPatch(Track{ID: uuid.New()}, nil, nil); changed {
		t.Error("enrichPatch reported a change with no tags and no cover")
	}
}

func TestCoverMimeMatchesTheExtractedExtension(t *testing.T) {
	for ext, want := range map[string]string{
		".jpg":  "image/jpeg",
		".png":  "image/png",
		".webp": "image/webp",
		"":      "image/jpeg", // ffmpeg's default for an attached picture
	} {
		if got := coverMime(ext); got != want {
			t.Errorf("coverMime(%q) = %q, want %q", ext, got, want)
		}
	}
}

// Repository gained the two lookup writes; the track tests never touch them.
func (f *fakeRepo) MarkLookupPending(context.Context, uuid.UUID) error {
	return errNotImplementedInFake
}
func (f *fakeRepo) SetLookupResult(context.Context, SetLookupInput) error {
	return errNotImplementedInFake
}
