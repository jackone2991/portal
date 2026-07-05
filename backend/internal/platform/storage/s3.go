package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// S3 is the S3-compatible implementation of Storage (MinIO, R2, AWS S3).
type S3 struct {
	client  *s3.Client
	presign *s3.PresignClient
	bucket  string
}

// Compile-time assurance S3 satisfies the interface.
var _ Storage = (*S3)(nil)

// NewS3 builds an S3 client. The endpoint + path-style toggle are what make the
// same code target MinIO (dev) or R2/S3 (prod); nothing else changes.
func NewS3(cfg Config) (*S3, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" {
		return nil, errors.New("storage: endpoint and bucket are required")
	}
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}

	client := s3.New(s3.Options{
		Region:       region,
		Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, ""),
		BaseEndpoint: aws.String(cfg.Endpoint), // MinIO/R2 endpoint
		UsePathStyle: cfg.UsePathStyle,         // MinIO: true · R2/S3: false
	})

	return &S3{
		client:  client,
		presign: s3.NewPresignClient(client),
		bucket:  cfg.Bucket,
	}, nil
}

func (s *S3) Bucket() string { return s.bucket }

func (s *S3) PresignPut(ctx context.Context, key, contentType string, ttl time.Duration) (*PresignedRequest, error) {
	in := &s3.PutObjectInput{Bucket: &s.bucket, Key: &key}
	if contentType != "" {
		in.ContentType = &contentType
	}
	req, err := s.presign.PresignPutObject(ctx, in, s3.WithPresignExpires(ttl))
	if err != nil {
		return nil, fmt.Errorf("storage: presign put %q: %w", key, err)
	}
	return toPresigned(req.URL, req.Method, req.SignedHeader, ttl), nil
}

func (s *S3) PresignGet(ctx context.Context, key string, ttl time.Duration) (*PresignedRequest, error) {
	req, err := s.presign.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key}, s3.WithPresignExpires(ttl))
	if err != nil {
		return nil, fmt.Errorf("storage: presign get %q: %w", key, err)
	}
	return toPresigned(req.URL, req.Method, req.SignedHeader, ttl), nil
}

func (s *S3) Put(ctx context.Context, key string, body io.Reader, contentType string) error {
	in := &s3.PutObjectInput{Bucket: &s.bucket, Key: &key, Body: body}
	if contentType != "" {
		in.ContentType = &contentType
	}
	if _, err := s.client.PutObject(ctx, in); err != nil {
		return fmt.Errorf("storage: put %q: %w", key, err)
	}
	return nil
}

func (s *S3) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("storage: get %q: %w", key, err)
	}
	return out.Body, nil
}

func (s *S3) Delete(ctx context.Context, key string) error {
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: &s.bucket, Key: &key}); err != nil {
		return fmt.Errorf("storage: delete %q: %w", key, err)
	}
	return nil
}

func (s *S3) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &s.bucket, Key: &key})
	if err != nil {
		if isNotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("storage: head %q: %w", key, err)
	}
	return true, nil
}

func toPresigned(url, method string, signed map[string][]string, ttl time.Duration) *PresignedRequest {
	headers := make(map[string]string, len(signed))
	for k, v := range signed {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}
	return &PresignedRequest{
		URL:     url,
		Method:  method,
		Headers: headers,
		Expires: time.Now().Add(ttl),
	}
}

// isNotFound normalises the several shapes S3-compatible backends use for a
// missing object: typed NoSuchKey/NotFound, or a smithy APIError with a 404-ish code.
func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	var nf *types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
