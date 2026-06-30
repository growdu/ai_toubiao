package storage

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"testing"
)

func TestLocalFilesystem_PutGetDelete(t *testing.T) {
	dir := t.TempDir()
	s, err := NewLocal(dir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	payload := []byte("hello bidwriter")
	key, checksum, size, err := s.Put(t.Context(), "test.txt", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("Put: %v", err)
	}
	if key == "" || checksum == "" || size != int64(len(payload)) {
		t.Errorf("bad Put result: key=%s checksum=%s size=%d", key, checksum, size)
	}

	// Verify SHA256 matches
	want := sha256.Sum256(payload)
	if got := hex.EncodeToString(want[:]); got != checksum {
		t.Errorf("checksum mismatch: want %s, got %s", got, checksum)
	}

	// Get should return the same content
	rc, err := s.Get(t.Context(), key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("content mismatch: got %q", got)
	}

	// Delete should be idempotent
	if err := s.Delete(t.Context(), key); err != nil {
		t.Errorf("Delete: %v", err)
	}
	if err := s.Delete(t.Context(), key); err != nil {
		t.Errorf("Delete (idempotent): %v", err)
	}
}