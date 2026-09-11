package media

// A lazy io.ReadSeeker over an object in storage.
//
// It exists so the original-file route can be served with http.ServeContent,
// which is what makes a media element seekable. Without a ReadSeeker the handler
// can only io.Copy the whole body: no `Accept-Ranges`, no `206`, no
// `Content-Range` — and a browser that cannot range-request an <audio> or
// <video> source reports `seekable` as [0,0] and silently refuses every scrub.
// The progress bar still moves, the click does nothing, and it looks like a
// front-end bug.
//
// "Lazy" is the whole trick: Seek moves a counter and costs nothing, and the
// ranged GET is only issued on the first Read after it. ServeContent's opening
// move is Seek(0, End) to size the body, which would otherwise mean fetching the
// entire object just to be told how long it is.

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/portal/backend/internal/platform/storage"
)

// objectReader reads an object through ranged GETs. Not safe for concurrent use;
// one request owns one reader.
type objectReader struct {
	ctx   context.Context
	store storage.Storage
	key   string
	size  int64

	pos int64
	rc  io.ReadCloser // open only while reading; nil after a Seek
}

// newObjectReader wraps an object of known size. The size comes from the asset
// row rather than a HEAD — it is already there, and one fewer round trip per
// media request matters when a browser opens several to scrub.
func newObjectReader(ctx context.Context, store storage.Storage, key string, size int64) *objectReader {
	return &objectReader{ctx: ctx, store: store, key: key, size: size}
}

func (o *objectReader) Read(p []byte) (int, error) {
	if o.pos >= o.size {
		return 0, io.EOF
	}
	if o.rc == nil {
		rc, err := o.store.GetByteRange(o.ctx, o.key, o.pos, -1)
		if err != nil {
			if errors.Is(err, storage.ErrNotFound) {
				return 0, io.EOF
			}
			return 0, err
		}
		o.rc = rc
	}
	n, err := o.rc.Read(p)
	o.pos += int64(n)
	return n, err
}

// Seek only moves the cursor. The open body is dropped so the next Read starts a
// fresh ranged GET from the new position — continuing to read the old stream
// after a seek is the bug this avoids.
func (o *objectReader) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = o.pos + offset
	case io.SeekEnd:
		next = o.size + offset
	default:
		return 0, fmt.Errorf("media: invalid whence %d", whence)
	}
	if next < 0 {
		return 0, fmt.Errorf("media: negative seek to %d", next)
	}
	if next != o.pos {
		o.closeBody()
		o.pos = next
	}
	return o.pos, nil
}

func (o *objectReader) Close() error {
	o.closeBody()
	return nil
}

func (o *objectReader) closeBody() {
	if o.rc != nil {
		_ = o.rc.Close()
		o.rc = nil
	}
}
