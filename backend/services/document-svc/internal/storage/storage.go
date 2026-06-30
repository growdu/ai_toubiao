// Package storage abstracts object storage (S3/MinIO) so the rest of the service
// doesn't care about the backend.
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
	// Put writes data to the store. Returns the storage key and SHA256 checksum.
	Put(ctx context.Context, name string, r io.Reader) (key string, checksum string, size int64, err error)

	// Get returns a reader for the object at the given key.
	Get(ctx context.Context, key string) (io.ReadCloser, error)

	// Delete removes the object. Idempotent.
	Delete(ctx context.Context, key string) error
}

// LocalFilesystem is a Storage backed by a local directory.
// Suitable for dev/test; in production use S3 or MinIO.
type LocalFilesystem struct {
	BaseDir string
}

// NewLocal creates a LocalFilesystem rooted at baseDir.
func NewLocal(baseDir string) (*LocalFilesystem, error) {
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir: %w", err)
	}
	return &LocalFilesystem{BaseDir: baseDir}, nil
}

func (l *LocalFilesystem) Put(_ context.Context, name string, r io.Reader) (string, string, int64, error) {
	// Use a UUID-prefixed path to avoid collisions and leakage.
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