// s3_minio_integration_test.go verifies the S3Backend against a real MinIO
// instance. Skipped unless BIDWRITER_S3_ENDPOINT is set (e.g.
//
//	BIDWRITER_S3_ENDPOINT=127.0.0.1:9100 \
//	BIDWRITER_S3_ACCESS_KEY=minioadmin \
//	BIDWRITER_S3_SECRET_KEY=minioadmin \
//	BIDWRITER_S3_BUCKET=bidwriter-test \
//	  go test ./services/document-svc/internal/storage/ -run Integration -v
//
// This file was added in the "verify MinIO end-to-end" commit to close the
// gap between the existing httptest fake-S3 tests and a real MinIO/S3
// deployment. The httptest tests confirm the protocol; this one confirms
// the wire-level interactions (multipart framing, signature validation,
// bucket auto-discovery via Location, etc.) work too.
package storage

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// s3Env pulls the integration-test MinIO coordinates from the environment.
// Returning the zero value lets tests skip cleanly when not configured.
type s3Env struct {
	endpoint  string
	accessKey string
	secretKey string
	bucket    string
}

func loadS3Env(t *testing.T) s3Env {
	t.Helper()
	e := s3Env{
		endpoint:  os.Getenv("BIDWRITER_S3_ENDPOINT"),
		accessKey: os.Getenv("BIDWRITER_S3_ACCESS_KEY"),
		secretKey: os.Getenv("BIDWRITER_S3_SECRET_KEY"),
		bucket:    os.Getenv("BIDWRITER_S3_BUCKET"),
	}
	if e.endpoint == "" || e.accessKey == "" || e.secretKey == "" || e.bucket == "" {
		t.Skip("BIDWRITER_S3_* not set; skipping integration test")
	}
	return e
}

func TestIntegration_S3Backend_PutGetDelete(t *testing.T) {
	env := loadS3Env(t)
	ctx := context.Background()

	backend, err := NewS3(ctx, env.endpoint, env.accessKey, env.secretKey, env.bucket, "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	// Random 1 MiB payload so two consecutive runs cannot collide on the
	// server side via an accidental key reuse (S3Backend prefixes a UUID
	// already, but a real test should not rely on that).
	payload := make([]byte, 1<<20)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand: %v", err)
	}
	wantSum := sha256.Sum256(payload)

	key, checksum, size, err := backend.Put(ctx, "integration-doc.bin", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if !strings.HasSuffix(key, "-integration-doc.bin") {
		t.Errorf("key %q should end with -integration-doc.bin", key)
	}
	if checksum != hex.EncodeToString(wantSum[:]) {
		t.Errorf("checksum mismatch: want %s, got %s", hex.EncodeToString(wantSum[:]), checksum)
	}
	if size != int64(len(payload)) {
		t.Errorf("size mismatch: want %d, got %d", len(payload), size)
	}
	t.Cleanup(func() {
		_ = backend.Delete(context.Background(), key)
	})

	// Get: must round-trip the bytes byte-for-byte.
	rc, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll after Get: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("round-tripped bytes mismatch against real MinIO")
	}

	// Delete: must succeed and be idempotent.
	if err := backend.Delete(ctx, key); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := backend.Delete(ctx, key); err != nil {
		t.Errorf("Delete idempotency: %v", err)
	}
	if _, err := backend.Get(ctx, key); err == nil {
		t.Error("Get after Delete should fail")
	}
}

func TestIntegration_S3Backend_LargePayload(t *testing.T) {
	// 16 MiB — exercises the buffer-then-Put path past the 5 MiB
	// threshold that the AWS SDK sometimes uses for switching to
	// multipart. We want to make sure the single-Put code path handles
	// payloads in the "large blob, single request" range correctly.
	env := loadS3Env(t)
	ctx := context.Background()

	backend, err := NewS3(ctx, env.endpoint, env.accessKey, env.secretKey, env.bucket, "us-east-1", false)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	const n = 16 << 20
	payload := make([]byte, n)
	// Fill with a deterministic pattern so a mismatch in the middle of
	// the buffer shows up in the diff rather than at the tail.
	for i := range payload {
		payload[i] = byte(i % 251)
	}

	key, checksum, size, err := backend.Put(ctx, fmt.Sprintf("large-%d.bin", n), bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	wantSum := sha256.Sum256(payload)
	if checksum != hex.EncodeToString(wantSum[:]) {
		t.Error("checksum mismatch on large payload")
	}
	if size != int64(n) {
		t.Errorf("size: want %d, got %d", n, size)
	}
	t.Cleanup(func() {
		_ = backend.Delete(context.Background(), key)
	})

	rc, err := backend.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("16 MiB round-trip mismatch")
	}
}