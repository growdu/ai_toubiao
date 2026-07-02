// Package storage holds the MinIO/S3 backend used by KB Ingest to
// materialise file-backed materials. The struct purposefully implements
// the service-side ObjectStore seam so Ingest does not need to know about
// minio-go.
package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// fetcher is the seam that abstracts minio-go's GetObject+Stat combo
// (which we cannot easily mock at the HTTP layer — minio-go speaks the
// full S3 protocol including SigV4 / chunked encoding). Production uses
// minioFetcher; tests inject fakeFetcher.
type fetcher interface {
	fetch(ctx context.Context, bucket, key string) (io.ReadCloser, error)
}

// MinIOStore is a thin MinIO / S3 wrapper. It implements
// service.ObjectStore so Ingest can fetch objects without learning about
// AWS Signature V4.
type MinIOStore struct {
	f   fetcher
	log *slog.Logger
}

// NewMinIO builds a MinIO client pointed at endpoint (host:port, no
// scheme). region is required by the signature; MinIO ignores it.
// useSSL controls whether the underlying transport is HTTPS.
func NewMinIO(endpoint, accessKey, secretKey, region string, useSSL bool) (*MinIOStore, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("minio endpoint required")
	}
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
		Region: region,
	})
	if err != nil {
		return nil, fmt.Errorf("minio new: %w", err)
	}
	return &MinIOStore{
		f:   &minioFetcher{client: cli},
		log: slog.Default().With(slog.String("component", "kb-minio")),
	}, nil
}

// newMinIOWithFetcher is the test seam used by minio_test.go. It builds
// a MinIOStore without touching a real *minio.Client.
func newMinIOWithFetcher(f fetcher) *MinIOStore {
	return &MinIOStore{
		f:   f,
		log: slog.Default().With(slog.String("component", "kb-minio")),
	}
}

// Get fetches the object as a stream. The caller is responsible for
// Close().
func (s *MinIOStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	rc, err := s.f.fetch(ctx, bucket, key)
	if err != nil {
		return nil, fmt.Errorf("minio get %q: %w", key, err)
	}
	s.log.Debug("minio get ok", slog.String("bucket", bucket), slog.String("key", key))
	return rc, nil
}

// minioFetcher is the production implementation of the fetcher seam.
type minioFetcher struct {
	client *minio.Client
}

func (f *minioFetcher) fetch(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, err := f.client.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// GetObject is lazy; Stat now to surface "not found" early.
	if _, statErr := obj.Stat(); statErr != nil {
		_ = obj.Close()
		return nil, statErr
	}
	return obj, nil
}