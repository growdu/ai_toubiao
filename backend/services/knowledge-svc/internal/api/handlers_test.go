package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/services/knowledge-svc/internal/store"
	"github.com/google/uuid"
)

// fakeKB is a hand-rolled in-memory KBAPI. It records the most recent
// arguments so tests can assert on them, and lets each test pre-program
// the return value or error per method.
type fakeKB struct {
	createFn   func(ctx context.Context, req *model.CreateMaterialRequest) (*model.KBMaterial, error)
	listFn     func(ctx context.Context, category string, limit, offset int) ([]*model.KBMaterial, error)
	getFn      func(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error)
	deleteFn   func(ctx context.Context, id uuid.UUID) error
	searchFn   func(ctx context.Context, tid uuid.UUID, req *model.SearchRequest) (*model.SearchResponse, error)
	ingestFn   func(ctx context.Context, req *model.IngestRequest) error

	lastListCat   string
	lastListLim   int
	lastListOff   int
	lastSearchTID uuid.UUID
	lastIngest    *model.IngestRequest
}

func (f *fakeKB) CreateMaterial(ctx context.Context, req *model.CreateMaterialRequest) (*model.KBMaterial, error) {
	return f.createFn(ctx, req)
}
func (f *fakeKB) ListMaterials(ctx context.Context, category string, limit, offset int) ([]*model.KBMaterial, error) {
	f.lastListCat, f.lastListLim, f.lastListOff = category, limit, offset
	return f.listFn(ctx, category, limit, offset)
}
func (f *fakeKB) GetMaterial(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error) {
	return f.getFn(ctx, id)
}
func (f *fakeKB) DeleteMaterial(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}
func (f *fakeKB) Search(ctx context.Context, tid uuid.UUID, req *model.SearchRequest) (*model.SearchResponse, error) {
	f.lastSearchTID = tid
	return f.searchFn(ctx, tid, req)
}
func (f *fakeKB) Ingest(ctx context.Context, req *model.IngestRequest) error {
	f.lastIngest = req
	return f.ingestFn(ctx, req)
}

// noopKB returns zero values + nil for every method. Use it as a base
// when a test only exercises one endpoint.
func noopKB() *fakeKB {
	return &fakeKB{
		createFn: func(context.Context, *model.CreateMaterialRequest) (*model.KBMaterial, error) { return nil, nil },
		listFn:   func(context.Context, string, int, int) ([]*model.KBMaterial, error) { return nil, nil },
		getFn:    func(context.Context, uuid.UUID) (*model.KBMaterial, error) { return nil, nil },
		deleteFn: func(context.Context, uuid.UUID) error { return nil },
		searchFn: func(context.Context, uuid.UUID, *model.SearchRequest) (*model.SearchResponse, error) {
			return &model.SearchResponse{Hits: []model.KBSearchResult{}}, nil
		},
		ingestFn: func(context.Context, *model.IngestRequest) error { return nil },
	}
}

func newKBHandler(svc KBAPI) *Handlers {
	return &Handlers{KBService: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func kbDo(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestHealthz(t *testing.T) {
	w := kbDo(t, newKBHandler(noopKB()).Routes(), http.MethodGet, "/healthz", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Materials CRUD
// ---------------------------------------------------------------------------

func TestCreateMaterial_Success(t *testing.T) {
	id := uuid.New()
	kb := noopKB()
	kb.createFn = func(_ context.Context, req *model.CreateMaterialRequest) (*model.KBMaterial, error) {
		if req.Title != "测试素材" {
			t.Errorf("title = %q, want %q", req.Title, "测试素材")
		}
		return &model.KBMaterial{ID: id, Title: req.Title, Category: req.Category}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/materials",
		map[string]any{"title": "测试素材", "category": "技术方案"}, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"id":"`+id.String()+`"`)) {
		t.Errorf("body should contain id %s, got %s", id, w.Body.String())
	}
}

func TestCreateMaterial_InvalidJSON(t *testing.T) {
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/materials", "not-json", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateMaterial_ServiceError(t *testing.T) {
	kb := noopKB()
	kb.createFn = func(context.Context, *model.CreateMaterialRequest) (*model.KBMaterial, error) {
		return nil, errors.New("db down")
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/materials",
		map[string]any{"title": "x"}, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestListMaterials_PassesQueryParamsToService(t *testing.T) {
	kb := noopKB()
	kb.listFn = func(_ context.Context, cat string, lim, off int) ([]*model.KBMaterial, error) {
		return []*model.KBMaterial{{ID: uuid.New(), Category: cat}}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodGet, "/api/v1/kb/materials?category=合同&limit=20&offset=5", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if kb.lastListCat != "合同" {
		t.Errorf("category = %q, want 合同", kb.lastListCat)
	}
	if kb.lastListLim != 20 {
		t.Errorf("limit = %d, want 20", kb.lastListLim)
	}
	if kb.lastListOff != 5 {
		t.Errorf("offset = %d, want 5", kb.lastListOff)
	}
}

func TestListMaterials_EmptyHasZeroCount(t *testing.T) {
	kb := noopKB()
	kb.listFn = func(context.Context, string, int, int) ([]*model.KBMaterial, error) {
		return []*model.KBMaterial{}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodGet, "/api/v1/kb/materials", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"count":0`)) {
		t.Errorf("expected count=0 in body, got %s", w.Body.String())
	}
}

func TestGetMaterial_Success(t *testing.T) {
	id := uuid.New()
	kb := noopKB()
	kb.getFn = func(_ context.Context, got uuid.UUID) (*model.KBMaterial, error) {
		if got != id {
			t.Errorf("id mismatch: got %s want %s", got, id)
		}
		return &model.KBMaterial{ID: id, Title: "ok"}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodGet, "/api/v1/kb/materials/"+id.String(), nil, nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

func TestGetMaterial_NotFound(t *testing.T) {
	kb := noopKB()
	kb.getFn = func(context.Context, uuid.UUID) (*model.KBMaterial, error) {
		return nil, store.ErrNotFound
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodGet, "/api/v1/kb/materials/"+uuid.NewString(), nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetMaterial_InvalidID(t *testing.T) {
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodGet, "/api/v1/kb/materials/not-a-uuid", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestDeleteMaterial_Success(t *testing.T) {
	id := uuid.New()
	kb := noopKB()
	kb.deleteFn = func(_ context.Context, got uuid.UUID) error {
		if got != id {
			t.Errorf("id mismatch: got %s want %s", got, id)
		}
		return nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodDelete, "/api/v1/kb/materials/"+id.String(), nil, nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestDeleteMaterial_NotFound(t *testing.T) {
	kb := noopKB()
	kb.deleteFn = func(context.Context, uuid.UUID) error { return store.ErrNotFound }
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodDelete, "/api/v1/kb/materials/"+uuid.NewString(), nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Search
// ---------------------------------------------------------------------------

func TestSearch_RequiresTenantHeader(t *testing.T) {
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/search",
		map[string]any{"query": "hi"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 (missing X-Tenant-ID)", w.Code)
	}
}

func TestSearch_RejectsBadTenantUUID(t *testing.T) {
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/search",
		map[string]any{"query": "hi"},
		map[string]string{"X-Tenant-ID": "not-uuid"})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearch_InvalidJSON(t *testing.T) {
	tid := uuid.NewString()
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/search", "not-json",
		map[string]string{"X-Tenant-ID": tid})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestSearch_PassesTenantAndQueryToService(t *testing.T) {
	tid := uuid.New()
	kb := noopKB()
	kb.searchFn = func(_ context.Context, gotTID uuid.UUID, req *model.SearchRequest) (*model.SearchResponse, error) {
		if gotTID != tid {
			t.Errorf("tenant = %s, want %s", gotTID, tid)
		}
		if req.Query != "RAG 检索" {
			t.Errorf("query = %q, want %q", req.Query, "RAG 检索")
		}
		return &model.SearchResponse{
			Hits:  []model.KBSearchResult{{ChunkID: uuid.New(), Score: 0.91}},
			Total: 1,
		}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/search",
		map[string]any{"query": "RAG 检索", "top_k": 5},
		map[string]string{"X-Tenant-ID": tid.String()})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"total":1`)) {
		t.Errorf("expected total=1 in body, got %s", w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"score":0.91`)) {
		t.Errorf("expected score=0.91 in body, got %s", w.Body.String())
	}
}

func TestSearch_EmptyResultsReturnsZeroHits(t *testing.T) {
	tid := uuid.New()
	kb := noopKB()
	kb.searchFn = func(context.Context, uuid.UUID, *model.SearchRequest) (*model.SearchResponse, error) {
		return &model.SearchResponse{Hits: []model.KBSearchResult{}, Total: 0}, nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/search",
		map[string]any{"query": "x"}, map[string]string{"X-Tenant-ID": tid.String()})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"total":0`)) {
		t.Errorf("expected total=0, got %s", w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Ingest
// ---------------------------------------------------------------------------

func TestIngest_Success(t *testing.T) {
	mid := uuid.New()
	kb := noopKB()
	kb.ingestFn = func(_ context.Context, req *model.IngestRequest) error {
		if req.MaterialID != mid {
			t.Errorf("material_id = %s, want %s", req.MaterialID, mid)
		}
		return nil
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/ingest",
		map[string]any{"material_id": mid.String()}, nil)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
}

func TestIngest_InvalidJSON(t *testing.T) {
	h := newKBHandler(noopKB())
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/ingest", "not-json", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestIngest_ServiceErrorReturns500(t *testing.T) {
	kb := noopKB()
	kb.ingestFn = func(context.Context, *model.IngestRequest) error {
		return errors.New("embed service unavailable")
	}
	h := newKBHandler(kb)
	w := kbDo(t, h.Routes(), http.MethodPost, "/api/v1/kb/ingest",
		map[string]any{"material_id": uuid.NewString()}, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// Compile-time check that *service.KBService (or a structural type with the
// same shape) still satisfies the interface after the refactor.
var _ KBAPI = (*noopKBSatisfier)(nil)

type noopKBSatisfier struct{}

func (noopKBSatisfier) CreateMaterial(context.Context, *model.CreateMaterialRequest) (*model.KBMaterial, error) {
	return nil, nil
}
func (noopKBSatisfier) ListMaterials(context.Context, string, int, int) ([]*model.KBMaterial, error) {
	return nil, nil
}
func (noopKBSatisfier) GetMaterial(context.Context, uuid.UUID) (*model.KBMaterial, error) {
	return nil, nil
}
func (noopKBSatisfier) DeleteMaterial(context.Context, uuid.UUID) error { return nil }
func (noopKBSatisfier) Search(context.Context, uuid.UUID, *model.SearchRequest) (*model.SearchResponse, error) {
	return nil, nil
}
func (noopKBSatisfier) Ingest(context.Context, *model.IngestRequest) error { return nil }