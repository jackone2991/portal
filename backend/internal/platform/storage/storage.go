// Package storage is the object-storage abstraction for Portal. It speaks S3, so
// the same code runs against MinIO in dev and Cloudflare R2 / AWS S3 in prod —
// the only difference is the endpoint + path-style flag (see Config).
//
// Large media never streams through the API: the browser uploads directly to the
// bucket with a PresignPut URL, and plays back via a PresignGet URL. Put/Get on
// the server are for internal use (e.g. the transcode worker writing HLS variants).
package storage

import (
	"context"
	"errors"
	"io"
	"time"
)

// ErrNotFound is returned by Get/Exists when the object does not exist.
var ErrNotFound = errors.New("storage: object not found")

// Config is the S3-compatible connection config, sourced from the S3_* env vars.
//
//	MinIO (dev):  Endpoint=http://minio:9000            UsePathStyle=true   Region=us-east-1
//	R2   (prod):  Endpoint=https://<acct>.r2...com      UsePathStyle=false  Region=auto
type Config struct {
	Endpoint     string
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool
}

// PresignedRequest is a time-limited URL the client uses to talk to the bucket
// directly. Headers are the request headers that were folded into the signature
// and MUST be replayed verbatim by the client (e.g. Content-Type on a PUT).
type PresignedRequest struct {
	URL     string
	Method  string
	Headers map[string]string
	Expires time.Time
}

// Storage is the object-storage surface the rest of the app depends on.
type Storage interface {
	// Bucket returns the configured bucket name.
	Bucket() string

	// PresignPut returns a URL the client can PUT an object to directly.
	PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (*PresignedRequest, error)
	// PresignGet returns a URL the client can GET an object from directly.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (*PresignedRequest, error)

	// Put uploads an object from the server. body should be seekable (e.g. a
	// *bytes.Reader or *os.File) so the SDK can sign the payload.
	Put(ctx context.Context, key string, body io.Reader, contentType string) error
	// Get streams an object back. The caller MUST close the returned reader.
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// Delete removes an object. Deleting a missing key is not an error.
	Delete(ctx context.Context, key string) error
	// Exists reports whether an object is present.
	Exists(ctx context.Context, key string) (bool, error)
}
