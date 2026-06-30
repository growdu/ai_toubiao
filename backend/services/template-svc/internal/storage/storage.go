package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
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

// MinIOStorage is a Storage backed by MinIO/S3.
type MinIOStorage struct {
	endpoint  string
	bucket    string
	accessKey string
	secretKey string
}

func NewMinIO(ctx context.Context, endpoint, bucket, accessKey, secretKey string) (*MinIOStorage, error) {
	// TODO: implement MinIO client
	// In production, use minio/minio-go
	return &MinIOStorage{
		endpoint:  endpoint,
		bucket:    bucket,
		accessKey: accessKey,
		secretKey: secretKey,
	}, nil
}

func (m *MinIOStorage) Put(ctx context.Context, name string, r io.Reader) (string, string, int64, error) {
	return "", "", 0, fmt.Errorf("not yet implemented: MinIO storage")
}

func (m *MinIOStorage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("not yet implemented: MinIO storage")
}

func (m *MinIOStorage) Delete(ctx context.Context, key string) error {
	return fmt.Errorf("not yet implemented: MinIO storage")
}
