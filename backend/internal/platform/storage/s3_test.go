package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"testing"
	"time"
)

// TestS3RoundTrip exercises the full surface against a live S3-compatible
// backend. It is skipped unless S3_ENDPOINT is set, so `go test ./...` stays
// green without MinIO. Run it against the dev stack with:
//
//	S3_ENDPOINT=http://minio:9000 S3_BUCKET=portal-media \
//	S3_ACCESS_KEY=portal S3_SECRET_KEY=... S3_USE_PATH_STYLE=true \
//	go test ./internal/platform/storage -run TestS3RoundTrip -v
func TestS3RoundTrip(t *testing.T) {
	endpoint := os.Getenv("S3_ENDPOINT")
	if endpoint == "" {
		t.Skip("S3_ENDPOINT not set — skipping storage integration test")
	}

	st, err := NewS3(Config{
		Endpoint:     endpoint,
		Region:       envOr("S3_REGION", "us-east-1"),
		Bucket:       os.Getenv("S3_BUCKET"),
		AccessKey:    os.Getenv("S3_ACCESS_KEY"),
		SecretKey:    os.Getenv("S3_SECRET_KEY"),
		UsePathStyle: os.Getenv("S3_USE_PATH_STYLE") != "false",
	})
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	ctx := context.Background()
	prefix := fmt.Sprintf("selftest/%d", time.Now().UnixNano())
	key := prefix + "/put.txt"
	presignKey := prefix + "/presign.txt"
	payload := []byte("portal storage round-trip ✓")
	const ct = "text/plain; charset=utf-8"

	t.Cleanup(func() {
		_ = st.Delete(ctx, key)
		_ = st.Delete(ctx, presignKey)
	})

	// ── server-side Put → Exists → Get ──────────────────────────────
	if err := st.Put(ctx, key, bytes.NewReader(payload), ct); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if ok, err := st.Exists(ctx, key); err != nil || !ok {
		t.Fatalf("Exists after Put = %v, %v; want true, nil", ok, err)
	}
	got := mustRead(t, st, ctx, key)
	if !bytes.Equal(got, payload) {
		t.Fatalf("Get mismatch: %q != %q", got, payload)
	}

	// ── PresignGet → fetch over HTTP ─────────────────────────────────
	pg, err := st.PresignGet(ctx, key, time.Minute)
	if err != nil {
		t.Fatalf("PresignGet: %v", err)
	}
	if body := httpDo(t, http.MethodGet, pg, nil); !bytes.Equal(body, payload) {
		t.Fatalf("presigned GET mismatch: %q != %q", body, payload)
	}

	// ── PresignPut → upload over HTTP → verify server-side ───────────
	pp, err := st.PresignPut(ctx, presignKey, ct, time.Minute)
	if err != nil {
		t.Fatalf("PresignPut: %v", err)
	}
	httpDo(t, http.MethodPut, pp, payload)
	if ok, err := st.Exists(ctx, presignKey); err != nil || !ok {
		t.Fatalf("Exists after presigned PUT = %v, %v; want true, nil", ok, err)
	}
	if got := mustRead(t, st, ctx, presignKey); !bytes.Equal(got, payload) {
		t.Fatalf("presigned PUT content mismatch: %q != %q", got, payload)
	}

	// ── Delete → Exists false → Get ErrNotFound ──────────────────────
	if err := st.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if ok, err := st.Exists(ctx, key); err != nil || ok {
		t.Fatalf("Exists after Delete = %v, %v; want false, nil", ok, err)
	}
	if _, err := st.Get(ctx, key); err != ErrNotFound {
		t.Fatalf("Get after Delete = %v; want ErrNotFound", err)
	}
}

func mustRead(t *testing.T, st *S3, ctx context.Context, key string) []byte {
	t.Helper()
	rc, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get %q: %v", key, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read %q: %v", key, err)
	}
	return b
}

func httpDo(t *testing.T, method string, pr *PresignedRequest, body []byte) []byte {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, pr.URL, rdr)
	if err != nil {
		t.Fatalf("build %s request: %v", method, err)
	}
	for k, v := range pr.Headers {
		req.Header.Set(k, v)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s presigned URL: %v", method, err)
	}
	defer res.Body.Close()
	out, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		t.Fatalf("%s presigned URL: status %d: %s", method, res.StatusCode, out)
	}
	return out
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
