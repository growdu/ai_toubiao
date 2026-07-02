// kb_service_test.go covers the KBService business logic that does not
// require a live PostgreSQL / object-store backend. Anything that touches
// the store / object store / router goes through hand-rolled fakes.
//
// Scope:
//   - Ingest content vs. FilePath branches (MinIO load path is the gap
//     this commit closes)
//   - chunkContent edge cases (pure function, easy to lock in)
//   - NotifyPrefLookup patterns (deferred — listed in the existing
//     store/router tests already)
package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/google/uuid"
)

// ---- test fakes ----

// fakeStore is a stand-in for *store.Store for the Ingest path. Only the
// methods Ingest actually touches need to be implemented; the others get
// a panic so they're easy to spot if the test grows beyond expectations.
type fakeStore struct {
	materialsByID map[uuid.UUID]*model.KBMaterial
	chunks        []*model.KBChunk
	indexedCalls  []indexedCall
	getErr        error
	createChunkFn func(ctx context.Context, c *model.KBChunk, emb []float32) error
}

type indexedCall struct {
	id         uuid.UUID
	chunkCount int
}

func (f *fakeStore) GetMaterial(_ context.Context, id uuid.UUID) (*model.KBMaterial, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if m, ok := f.materialsByID[id]; ok {
		return m, nil
	}
	return nil, ErrMaterialNotFoundForTest
}
func (f *fakeStore) CreateChunkWithVec(ctx context.Context, c *model.KBChunk, emb []float32) error {
	if f.createChunkFn != nil {
		return f.createChunkFn(ctx, c, emb)
	}
	f.chunks = append(f.chunks, c)
	return nil
}
func (f *fakeStore) UpdateMaterialIndexed(_ context.Context, id uuid.UUID, count int) error {
	f.indexedCalls = append(f.indexedCalls, indexedCall{id: id, chunkCount: count})
	return nil
}

var ErrMaterialNotFoundForTest = errors.New("material not found")

// fakeObjectStore records the (bucket, key) pairs it was asked for and
// returns the (content, err) the test pre-programmed.
type fakeObjectStore struct {
	getCalls []objectCall
	responder func(ctx context.Context, bucket, key string) (io.ReadCloser, error)
}

type objectCall struct {
	bucket, key string
}

func (f *fakeObjectStore) Get(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	f.getCalls = append(f.getCalls, objectCall{bucket: bucket, key: key})
	if f.responder != nil {
		return f.responder(ctx, bucket, key)
	}
	return nil, errors.New("object not found")
}

// stubRouterClient avoids real HTTP. Embeddings are derived deterministically
// from chunk text length so the test can assert chunk count without
// depending on a live router-svc.
type stubRouterClient struct {
	embedCalls int
	embedding  []float32
	embedErr   error
}

func (s *stubRouterClient) Embed(_ context.Context, _ uuid.UUID, texts []string, _ string) (*EmbedResponse, error) {
	s.embedCalls++
	if s.embedErr != nil {
		return nil, s.embedErr
	}
	out := make([][]float32, len(texts))
	emb := s.embedding
	if emb == nil {
		emb = []float32{0.1, 0.2, 0.3}
	}
	for i := range texts {
		out[i] = emb
	}
	return &EmbedResponse{Embeddings: out}, nil
}

func silentLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---- Ingest ----

func TestIngest_EmptyContent_FilePath_PullsFromObjectStore(t *testing.T) {
	id := uuid.New()
	content := "This is the file content from MinIO. It is short.\n\nSecond paragraph."
	fs := &fakeStore{
		materialsByID: map[uuid.UUID]*model.KBMaterial{
			id: {ID: id, TenantID: uuid.New(), Content: "", FilePath: "kb-materials/material.txt", Status: "pending"},
		},
	}
	obj := &fakeObjectStore{
		responder: func(_ context.Context, bucket, key string) (io.ReadCloser, error) {
			if bucket != "kb" {
				t.Errorf("bucket = %q, want kb", bucket)
			}
			if key != "kb-materials/material.txt" {
				t.Errorf("key = %q, want kb-materials/material.txt", key)
			}
			return io.NopCloser(strings.NewReader(content)), nil
		},
	}
	router := &stubRouterClient{}
	svc := &KBService{
		store:        nil, // unused on this path (only store methods we override)
		log:          silentLogger(),
		routerClient: nil,
		objectStore:  obj,
		kbBucket:     "kb",
		embedder:     router,
	}
	if err := svc.ingestWithDeps(context.Background(), fs, id); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(obj.getCalls) != 1 {
		t.Errorf("ObjectStore.Get calls = %d, want 1", len(obj.getCalls))
	}
	if router.embedCalls == 0 {
		t.Error("expected router.Embed to be called")
	}
	if len(fs.chunks) == 0 {
		t.Error("expected chunks to be created")
	}
}

func TestIngest_EmptyContent_FilePath_ObjectStoreError_Propagates(t *testing.T) {
	id := uuid.New()
	wantErr := errors.New("minio down")
	fs := &fakeStore{
		materialsByID: map[uuid.UUID]*model.KBMaterial{
			id: {ID: id, Content: "", FilePath: "kb-materials/x"},
		},
	}
	obj := &fakeObjectStore{
		responder: func(context.Context, string, string) (io.ReadCloser, error) {
			return nil, wantErr
		},
	}
	router := &stubRouterClient{}
	svc := &KBService{
		log:         silentLogger(),
		objectStore: obj,
		kbBucket:    "kb",
		embedder:    router,
	}
	err := svc.ingestWithDeps(context.Background(), fs, id)
	if !errors.Is(err, wantErr) {
		t.Errorf("err = %v, want %v", err, wantErr)
	}
	if len(fs.chunks) != 0 {
		t.Errorf("no chunks should be created on object store error, got %d", len(fs.chunks))
	}
}

func TestIngest_NonEmptyContent_SkipsObjectStore(t *testing.T) {
	id := uuid.New()
	fs := &fakeStore{
		materialsByID: map[uuid.UUID]*model.KBMaterial{
			id: {ID: id, Content: "inline content", FilePath: "ignored.txt"},
		},
	}
	obj := &fakeObjectStore{
		responder: func(context.Context, string, string) (io.ReadCloser, error) {
			t.Fatal("ObjectStore.Get must NOT be called when content is non-empty")
			return nil, nil
		},
	}
	router := &stubRouterClient{}
	svc := &KBService{
		log:         silentLogger(),
		objectStore: obj,
		kbBucket:    "kb",
		embedder:    router,
	}
	if err := svc.ingestWithDeps(context.Background(), fs, id); err != nil {
		t.Fatalf("ingest: %v", err)
	}
	if len(obj.getCalls) != 0 {
		t.Errorf("ObjectStore.Get calls = %d, want 0", len(obj.getCalls))
	}
}

func TestIngest_NeitherContentNorPath_LogsAndReturnsNil(t *testing.T) {
	id := uuid.New()
	fs := &fakeStore{
		materialsByID: map[uuid.UUID]*model.KBMaterial{
			id: {ID: id, Content: "", FilePath: ""},
		},
	}
	obj := &fakeObjectStore{
		responder: func(context.Context, string, string) (io.ReadCloser, error) {
			t.Fatal("ObjectStore.Get must not be called when no source is available")
			return nil, nil
		},
	}
	svc := &KBService{
		log:         silentLogger(),
		objectStore: obj,
		kbBucket:    "kb",
		embedder:    &stubRouterClient{},
	}
	if err := svc.ingestWithDeps(context.Background(), fs, id); err != nil {
		t.Errorf("err = %v, want nil (no-op)", err)
	}
}

// ---- chunkContent (pure-function coverage; behavior locks the contract the
// Ingest path depends on) ----

func TestChunkContent_Empty(t *testing.T) {
	if got := chunkContent("", 512); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}

func TestChunkContent_SingleParagraph(t *testing.T) {
	got := chunkContent("Hello, world.", 512)
	if len(got) != 1 || got[0] != "Hello, world." {
		t.Errorf("got %v, want [Hello, world.]", got)
	}
}

func TestChunkContent_SplitsLongMultiParagraph(t *testing.T) {
	// Build 10 paragraphs of 100 chars each → ~1000 chars total; limit at
	// 200 chars (~50 tokens) → should split into multiple chunks.
	var paras []string
	for i := 0; i < 10; i++ {
		paras = append(paras, strings.Repeat("x", 100))
	}
	got := chunkContent(strings.Join(paras, "\n\n"), 50)
	if len(got) < 2 {
		t.Errorf("got %d chunks, want >=2", len(got))
	}
	for _, chunk := range got {
		if strings.Contains(chunk, "\n\n\n") {
			t.Errorf("chunk has triple newline (paragraph boundaries mangled): %q", chunk)
		}
	}
}

func TestChunkContent_PreservesAllText(t *testing.T) {
	in := "alpha\nbravo\ncharlie"
	got := chunkContent(in, 1) // tiny limit → forces splits
	joined := strings.Join(got, "")
	for _, want := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in reconstructed chunks", want)
		}
	}
}

// ---- ObjectStore interface assertion ----

// Compile-time check that fakeObjectStore satisfies the seam.
var _ ObjectStore = (*fakeObjectStore)(nil)

// Sanity: it should be possible to call io.ReadAll on what Get returns
// (object stores are expected to return io.ReadCloser; tests rely on this
// being true without panicking).
func TestFakeObjectStore_GetReturnsReadCloser(t *testing.T) {
	obj := &fakeObjectStore{
		responder: func(context.Context, string, string) (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("hello"))), nil
		},
	}
	rc, err := obj.Get(context.Background(), "b", "k")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want hello", data)
	}
}
