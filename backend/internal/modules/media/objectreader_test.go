package media

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/portal/backend/internal/platform/storage/storagetest"
)

// objectReader is what makes a media file seekable over HTTP. The properties
// worth pinning down are the ones http.ServeContent relies on — it sizes the
// body with Seek(0, End) before reading a byte, and a reader that fetched the
// whole object to answer that would defeat the point.

func newTestReader(t *testing.T, body []byte) *objectReader {
	t.Helper()
	store := storagetest.New()
	store.Seed("k", body, "")
	return newObjectReader(context.Background(), store, "k", int64(len(body)))
}

func TestObjectReaderSeekEndReportsSizeWithoutReading(t *testing.T) {
	body := []byte("0123456789")
	r := newTestReader(t, body)

	n, err := r.Seek(0, io.SeekEnd)
	if err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if n != int64(len(body)) {
		t.Errorf("Seek(0, End) = %d, want %d", n, len(body))
	}
	// ServeContent does exactly this before serving anything; if it cost a full
	// object fetch, every range request would download the whole file first.
	if r.rc != nil {
		t.Error("sizing the object opened a body — the seek must stay lazy")
	}
}

func TestObjectReaderReadsFromTheSeekedOffset(t *testing.T) {
	r := newTestReader(t, []byte("0123456789"))

	if _, err := r.Seek(4, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(got) != "456789" {
		t.Errorf("read %q, want %q", got, "456789")
	}
}

// A seek mid-stream has to abandon the open body: continuing to read the old
// one would return bytes from the previous position and silently corrupt the
// response.
func TestObjectReaderDropsTheOpenBodyOnSeek(t *testing.T) {
	r := newTestReader(t, []byte("0123456789"))

	buf := make([]byte, 3)
	if _, err := r.Read(buf); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(buf) != "012" {
		t.Fatalf("read %q, want %q", buf, "012")
	}
	if r.rc == nil {
		t.Fatal("expected an open body after Read")
	}

	if _, err := r.Seek(8, io.SeekStart); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if r.rc != nil {
		t.Error("the previous body survived a seek")
	}
	rest, _ := io.ReadAll(r)
	if string(rest) != "89" {
		t.Errorf("read %q after seeking to 8, want %q", rest, "89")
	}
}

func TestObjectReaderSeekCurrentAndEOF(t *testing.T) {
	r := newTestReader(t, []byte("0123456789"))

	if _, err := r.Read(make([]byte, 2)); err != nil {
		t.Fatalf("Read: %v", err)
	}
	if pos, err := r.Seek(3, io.SeekCurrent); err != nil || pos != 5 {
		t.Errorf("Seek(3, Current) = (%d, %v), want (5, nil)", pos, err)
	}

	if _, err := r.Seek(0, io.SeekEnd); err != nil {
		t.Fatalf("Seek: %v", err)
	}
	if _, err := r.Read(make([]byte, 1)); err != io.EOF {
		t.Errorf("Read at EOF = %v, want io.EOF", err)
	}
	if _, err := r.Seek(-1, io.SeekStart); err == nil {
		t.Error("a negative seek should be refused")
	}
}

// The end-to-end property: served through http.ServeContent, the route answers
// a Range request with 206 + Content-Range. Without that a browser reports the
// media as seekable [0,0] and refuses every scrub — which is what a user sees as
// "the progress bar does nothing".
func TestServeContentAnswersRangeRequests(t *testing.T) {
	body := []byte("0123456789abcdef")
	r := newTestReader(t, body)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.ServeContent(w, req, "track.mp3", time.Unix(0, 0), r)
	}))
	defer srv.Close()

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=4-7")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", resp.StatusCode)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "bytes 4-7/16" {
		t.Errorf("Content-Range = %q, want %q", cr, "bytes 4-7/16")
	}
	if ar := resp.Header.Get("Accept-Ranges"); ar != "bytes" {
		t.Errorf("Accept-Ranges = %q, want %q — this header is what makes the element seekable", ar, "bytes")
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, []byte("4567")) {
		t.Errorf("body = %q, want %q", got, "4567")
	}
}
