// Package storage holds the local and MinIO/S3 backends used by
// template-svc for rendering artefact persistence.
package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Storage is the abstraction over object stores.
type Storage interface {
	Put(ctx context.Context, name string, r io.Reader) (key string, checksum string, size int64, err error)
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

// LocalFilesystem is a Storage backed by a local directory.
type LocalFilesystem struct {
	BaseDir string
}

func NewLocal(baseDir string) (*LocalFilesystem, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &LocalFilesystem{BaseDir: baseDir}, nil
}

func (l *LocalFilesystem) Put(_ context.Context, name string, r io.Reader) (string, string, int64, error) {
	key := uuid.NewString() + "-" + filepath.Base(name)
	full := filepath.Join(l.BaseDir, key)

	f, err := os.Create(full)
	if err != nil {
		return "", "", 0, fmt.Errorf("create: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	mw := io.MultiWriter(f, h)
	n, err := io.Copy(mw, r)
	if err != nil {
		_ = os.Remove(full)
		return "", "", 0, fmt.Errorf("write: %w", err)
	}
	return key, hex.EncodeToString(h.Sum(nil)), n, nil
}

func (l *LocalFilesystem) Get(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(l.BaseDir, key))
}

func (l *LocalFilesystem) Delete(_ context.Context, key string) error {
	err := os.Remove(filepath.Join(l.BaseDir, key))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// minioClient is the subset of *minio.Client that MinIOStorage uses.
// Defining it at the consumer lets tests inject a fake without bringing
// up a real minio server (the full S3 protocol — SigV4, chunked
// transfer — is impractical to fake via httptest).
type minioClient interface {
	Put(ctx context.Context, bucket, key, contentType string, body []byte) (int64, error)
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, bucket, key string) error
}

// MinIOStorage is a Storage backed by MinIO / S3.
type MinIOStorage struct {
	client minioClient
	bucket string
	log    *slog.Logger
}

// NewMinIO builds a MinIO client pointed at endpoint (host:port, no
// scheme). region is required by the S3 signature; MinIO ignores it.
// useSSL controls HTTPS vs HTTP.
func NewMinIO(ctx context.Context, endpoint, bucket, accessKey, secretKey string) (*MinIOStorage, error) {
	_ = ctx
	if endpoint == "" {
		return nil, fmt.Errorf("minio endpoint required")
	}
	cli, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
		Region: "us-east-1",
	})
	if err != nil {
		return nil, fmt.Errorf("minio new: %w", err)
	}
	return &MinIOStorage{
		client: &realMinioClient{cli: cli, useSSL: false},
		bucket: bucket,
		log:    slog.Default().With(slog.String("component", "tpl-minio")),
	}, nil
}

// newMinIOWithClient is the test seam used by minio_test.go.
func newMinIOWithClient(c minioClient, bucket string) *MinIOStorage {
	return &MinIOStorage{
		client: c,
		bucket: bucket,
		log:    slog.Default().With(slog.String("component", "tpl-minio")),
	}
}

func (m *MinIOStorage) Put(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
	// Mirror LocalFilesystem.Put: uuid-prefixed key with the original
	// basename so that callers moving artefacts between backends don't
	// see surprising renames.
	key := uuid.NewString() + "-" + filepath.Base(name)

	// Buffer to disk first so we can compute SHA256 + size before the
	// Put call. Templates are KB-to-low-MB; this is acceptable.
	buf, err := io.ReadAll(io.LimitReader(r, 64<<20))
	if err != nil {
		return "", "", 0, fmt.Errorf("buffer %q: %w", name, err)
	}
	hasher := sha256.New()
	hasher.Write(buf)
	checksum := hex.EncodeToString(hasher.Sum(nil))

	n, err := m.client.Put(ctx, m.bucket, key, "application/octet-stream", buf)
	if err != nil {
		return "", "", 0, fmt.Errorf("minio put %q: %w", key, err)
	}
	m.log.Debug("minio put ok",
		slog.String("bucket", m.bucket),
		slog.String("key", key),
		slog.Int64("size", n),
	)
	return key, checksum, n, nil
}

func (m *MinIOStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	rc, err := m.client.Get(ctx, m.bucket, key)
	if err != nil {
		return nil, fmt.Errorf("minio get %q: %w", key, err)
	}
	m.log.Debug("minio get ok", slog.String("bucket", m.bucket), slog.String("key", key))
	return rc, nil
}

func (m *MinIOStorage) Delete(ctx context.Context, key string) error {
	if err := m.client.Delete(ctx, m.bucket, key); err != nil {
		return fmt.Errorf("minio delete %q: %w", key, err)
	}
	m.log.Debug("minio delete ok", slog.String("bucket", m.bucket), slog.String("key", key))
	return nil
}

// realMinioClient adapts *minio.Client to the minioClient seam used by
// tests. It exists in production only — tests substitute fakeMinioClient.
type realMinioClient struct {
	cli    *minio.Client
	useSSL bool
}

func (r *realMinioClient) Put(ctx context.Context, bucket, key, contentType string, body []byte) (int64, error) {
	info, err := r.cli.PutObject(ctx, bucket, key, bytesReader(body), int64(len(body)), minio.PutObjectOptions{
		ContentType:          contentType,
		DisableContentSha256: true,
	})
	if err != nil {
		return 0, err
	}
	return info.Size, nil
}

func (r *realMinioClient) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	obj, err := r.cli.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// Pre-Stat to surface "not found" immediately rather than at first Read.
	if _, statErr := obj.Stat(); statErr != nil {
		_ = obj.Close()
		return nil, statErr
	}
	return obj, nil
}

func (r *realMinioClient) Delete(ctx context.Context, bucket, key string) error {
	return r.cli.RemoveObject(ctx, bucket, key, minio.RemoveObjectOptions{})
}

// bytesReader avoids importing bytes for a single use.
type bytesReaderImpl struct {
	b []byte
	i int
}

func bytesReader(b []byte) io.Reader { return &bytesReaderImpl{b: b} }

func (r *bytesReaderImpl) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}