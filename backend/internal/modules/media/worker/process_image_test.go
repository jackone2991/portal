package worker

// The image pipeline's rules (SPEC-01 P0.1; backlog #9). The admission and
// scaling rules are pure and pinned as tables; the encode and probe paths run
// the real ffmpeg/ffprobe against fixtures drawn here with image/png and
// image/gif — no binary fixtures in the repo. The fixture test skips locally
// when ffmpeg is not on PATH and fails under CI, where the `backend` job
// installs it, so the pipeline's contract cannot lapse silently.

import (
	"context"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCheckImageDims(t *testing.T) {
	cases := []struct {
		name       string
		w, h, fr   int
		wantReject bool
	}{
		{"ordinary photo", 4000, 3000, 1, false},
		{"square at the area cap", 8000, 8000, 1, false},
		{"one pixel over the area cap", 8001, 8000, 1, true},
		{"tall webtoon strip — small area, long side", 704, 18000, 1, false},
		{"long side past the sanity ceiling", 100, 30001, 1, true},
		{"width past the sanity ceiling", 30001, 100, 1, true},
		{"animated — two frames", 100, 100, 2, true},
		{"unknown frame count reads as one", 100, 100, 1, false},
		{"zero width", 0, 100, 1, true},
		{"zero height", 100, 0, 1, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := checkImageDims(c.w, c.h, c.fr)
			if (err != nil) != c.wantReject {
				t.Fatalf("checkImageDims(%d, %d, %d) = %v, want reject=%v", c.w, c.h, c.fr, err, c.wantReject)
			}
		})
	}
}

func TestScaledDims(t *testing.T) {
	cases := []struct {
		name         string
		w, h, maxW   int
		wantW, wantH int
	}{
		{"never upscaled", 100, 80, thumbMaxWidth, 100, 80},
		{"width-limited, aspect kept", 4000, 3000, mediumMaxWidth, 1280, 960},
		{"thumb of a landscape", 4000, 3000, thumbMaxWidth, 320, 240},
		{"portrait under the width cap keeps its size", 600, 1000, mediumMaxWidth, 600, 1000},
		{"webtoon strip: width fits, height clamped for libwebp", 704, 18000, mediumMaxWidth, 625, variantMaxHeight},
		{"webtoon strip thumb: width scaling alone brings the height under the clamp", 704, 18000, thumbMaxWidth, 320, 8181},
		{"extreme ribbon never collapses to zero width", 10, 30000, mediumMaxWidth, 5, variantMaxHeight},
		{"extreme banner never collapses to zero height", 30000, 10, mediumMaxWidth, 1280, 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			gw, gh := scaledDims(c.w, c.h, c.maxW)
			if gw != c.wantW || gh != c.wantH {
				t.Fatalf("scaledDims(%d, %d, %d) = %d×%d, want %d×%d", c.w, c.h, c.maxW, gw, gh, c.wantW, c.wantH)
			}
			if gw > c.maxW || gh > variantMaxHeight || gw < 1 || gh < 1 {
				t.Fatalf("scaledDims(%d, %d, %d) = %d×%d breaks the box", c.w, c.h, c.maxW, gw, gh)
			}
		})
	}
}

// requireFFmpeg is the fixture tests' gate: skip on a box without the
// binaries, fail under CI (the `backend` job installs ffmpeg for this).
func requireFFmpeg(t *testing.T) {
	t.Helper()
	for _, bin := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(bin); err != nil {
			if os.Getenv("CI") != "" {
				t.Fatalf("%s not on PATH but CI is — the image-pipeline fixture tests must run in CI, not skip", bin)
			}
			t.Skipf("%s not on PATH — skipping the image-pipeline fixture tests", bin)
		}
	}
}

func writePNG(t *testing.T, dir, name string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeAnimatedGIF(t *testing.T, dir string, frames int) string {
	t.Helper()
	g := &gif.GIF{}
	for i := 0; i < frames; i++ {
		fr := image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.Black, color.White})
		fr.SetColorIndex(i%16, i%16, 1)
		g.Image = append(g.Image, fr)
		g.Delay = append(g.Delay, 10)
	}
	p := filepath.Join(dir, "anim.gif")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := gif.EncodeAll(f, g); err != nil {
		t.Fatal(err)
	}
	return p
}

// The real ffmpeg/ffprobe on fixtures drawn here: the probe reads what the
// fixture is, the encoder produces the variant the scaling rule predicts —
// thumb and medium for a landscape photo, a tall strip that comes out under
// libwebp's height limit rather than failing — and an animated input reads
// as the frame count the admission rule refuses.
func TestPipelineAgainstFixtures(t *testing.T) {
	requireFFmpeg(t)
	ctx := context.Background()
	dir := t.TempDir()

	t.Run("photo: probe and both variants", func(t *testing.T) {
		photo := writePNG(t, dir, "photo.png", 1600, 1200)
		w, h, frames, err := probeImage(ctx, photo)
		if err != nil || w != 1600 || h != 1200 || frames != 1 {
			t.Fatalf("probe photo = %d×%d ×%d, %v", w, h, frames, err)
		}
		for _, v := range []struct {
			name string
			maxW int
		}{{"thumb", thumbMaxWidth}, {"medium", mediumMaxWidth}} {
			out := filepath.Join(dir, v.name+".webp")
			if err := encodeWebP(ctx, photo, out, v.maxW); err != nil {
				t.Fatalf("encode %s: %v", v.name, err)
			}
			gw, gh, _, err := probeImage(ctx, out)
			if err != nil {
				t.Fatalf("probe %s: %v", v.name, err)
			}
			// 1600×1200 scales to whole pixels at both widths, so exact is real here.
			if ww, wh := scaledDims(w, h, v.maxW); gw != ww || gh != wh {
				t.Fatalf("%s variant = %d×%d, scaledDims says %d×%d", v.name, gw, gh, ww, wh)
			}
		}
	})

	t.Run("tall strip: shrunk under the libwebp limit, not refused", func(t *testing.T) {
		// The bug that made webtoon chapters un-importable.
		strip := writePNG(t, dir, "strip.png", 200, 18000)
		out := filepath.Join(dir, "strip.webp")
		if err := encodeWebP(ctx, strip, out, mediumMaxWidth); err != nil {
			t.Fatalf("encode tall strip: %v", err)
		}
		gw, gh, _, err := probeImage(ctx, out)
		if err != nil || gh > variantMaxHeight || gw > 200 {
			t.Fatalf("tall strip variant = %d×%d (%v), want height ≤ %d and no upscale", gw, gh, err, variantMaxHeight)
		}
		// ffmpeg's scale box truncates the scaled width (177 where the rule says
		// 178); the row is metadata, the object is truth — one pixel, no more.
		if ww, wh := scaledDims(200, 18000, mediumMaxWidth); abs(gw-ww) > 1 || abs(gh-wh) > 1 {
			t.Fatalf("tall strip variant = %d×%d, scaledDims says %d×%d", gw, gh, ww, wh)
		}
	})

	t.Run("animated gif: probed as many frames, refused", func(t *testing.T) {
		anim := writeAnimatedGIF(t, dir, 3)
		_, _, frames, err := probeImage(ctx, anim)
		if err != nil || frames < 2 {
			t.Fatalf("probe animated gif = %d frames, %v; want ≥ 2", frames, err)
		}
		if err := checkImageDims(16, 16, frames); err == nil {
			t.Fatal("an animated image was admitted")
		}
	})
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
