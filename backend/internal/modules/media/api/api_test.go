package api

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// A module wired without a signer must refuse, not return a valid-looking
// empty URL: the stub used to answer ("", nil), and a caller that built an
// <audio src=""> from it would have had nothing to debug.
func TestSignedURLWithoutASignerIsAnError(t *testing.T) {
	impl := NewImpl(nil, nil, nil, nil, nil, nil)
	url, err := impl.SignedURL(context.Background(), uuid.New(), time.Minute)
	if err == nil {
		t.Fatal("SignedURL with no signer returned nil error")
	}
	if url != "" {
		t.Fatalf("SignedURL with no signer returned %q, want empty", url)
	}
}
