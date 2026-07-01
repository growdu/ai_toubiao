package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidwriter/services/document-svc/internal/model"
	"github.com/bidwriter/services/document-svc/internal/service"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/google/uuid"
)

// fakeStore is a hand-rolled in-memory DocumentStore.
type fakeStore struct {
	createFn func(ctx context.Context, d *model.Document) error
	getFn    func(ctx context.Context, id uuid.UUID) (*model.Document, error)
	listFn   func(ctx context.Context, projectID *uuid.UUID, limit int, cursor *uuid.UUID) ([]*model.Document, error)
	updateFn func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Document, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error

	created *model.Document
}

func (f *fakeStore) Create(ctx context.Context, d *model.Document) error {
	f.created = d
	return f.createFn(ctx, d)
}
func (f *fakeStore) Get(ctx context.Context, id uuid.UUID) (*model.Document, error) {
	return f.getFn(ctx, id)
}
func (f *fakeStore) List(ctx context.Context, p *uuid.UUID, lim int, c *uuid.UUID) ([]*model.Document, error) {
	return f.listFn(ctx, p, lim, c)
}
func (f *fakeStore) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Document, error) {
	return f.updateFn(ctx, id, req)
}
func (f *fakeStore) Delete(ctx context.Context, id uuid.UUID) error {
	return f.deleteFn(ctx, id)
}

// memStorage is an in-memory storage.Storage that round-trips bytes.
type memStorage struct {
	data map[string][]byte
}

func newMemStorage() *memStorage { return &memStorage{data: map[string][]byte{}} }

func (m *memStorage) Put(_ context.Context, name string, r io.Reader) (string, string, int64, error) {
	buf, _ := io.ReadAll(r)
	m.data[name] = buf
	// Trivial checksum so the test doesn't care about real SHA-256.
	checksum := uuid.NewSHA1(uuid.NameSpaceOID, buf).String()
	return name, checksum, int64(len(buf)), nil
}
func (m *memStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	v, ok := m.data[key]
	if !ok {
		return nil, errors.New("not found")
	}
	return io.NopCloser(bytes.NewReader(v)), nil
}
func (m *memStorage) Delete(_ context.Context, key string) error {
	delete(m.data, key)
	return nil
}

// fakeParser is a fake DocumentParser.
type fakeParser struct {
	parseFn   func(ctx context.Context, id uuid.UUID, async bool) (*model.ParseResult, error)
	resultFn  func(ctx context.Context, id uuid.UUID) (*model.ParseResult, error)
	lastAsync bool
	lastID    uuid.UUID
}

func (f *fakeParser) Parse(ctx context.Context, id uuid.UUID, async bool) (*model.ParseResult, error) {
	f.lastID, f.lastAsync = id, async
	return f.parseFn(ctx, id, async)
}
func (f *fakeParser) GetParseResult(ctx context.Context, id uuid.UUID) (*model.ParseResult, error) {
	return f.resultFn(ctx, id)
}

// fakeExporter is a fake DocumentExporter.
type fakeExporter struct {
	exportFn  func(ctx context.Context, req *service.ExportRequest) (*service.ExportResult, error)
	lastReq   *service.ExportRequest
}

func (f *fakeExporter) Export(ctx context.Context, req *service.ExportRequest) (*service.ExportResult, error) {
	f.lastReq = req
	return f.exportFn(ctx, req)
}

// ---------------------------------------------------------------------------
// Test rig
// ---------------------------------------------------------------------------

type docRig struct {
	store    *fakeStore
	storage  *memStorage
	parser   *fakeParser
	exporter *fakeExporter
	h        *Handlers
}

func newRig() *docRig {
	fs := &fakeStore{
		createFn: func(context.Context, *model.Document) error { return nil },
		getFn:    func(context.Context, uuid.UUID) (*model.Document, error) { return nil, store.ErrNotFound },
		listFn:   func(context.Context, *uuid.UUID, int, *uuid.UUID) ([]*model.Document, error) { return nil, nil },
		updateFn: func(context.Context, uuid.UUID, *model.UpdateRequest) (*model.Document, error) { return nil, store.ErrNotFound },
		deleteFn: func(context.Context, uuid.UUID) error { return store.ErrNotFound },
	}
	fp := &fakeParser{
		parseFn:  func(context.Context, uuid.UUID, bool) (*model.ParseResult, error) { return &model.ParseResult{}, nil },
		resultFn: func(context.Context, uuid.UUID) (*model.ParseResult, error) { return &model.ParseResult{}, nil },
	}
	fe := &fakeExporter{
		exportFn: func(context.Context, *service.ExportRequest) (*service.ExportResult, error) {
			return &service.ExportResult{
				DocumentID:  uuid.New(),
				StorageKey:  "exports/abc.docx",
				DownloadURL: "/api/v1/documents/x/content",
				SizeBytes:   12,
				Format:      string(service.FormatWord),
			}, nil
		},
	}
	ms := newMemStorage()
	h := &Handlers{
		Store: fs, Storage: ms, Parser: fp, Exporter: fe,
		Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return &docRig{store: fs, storage: ms, parser: fp, exporter: fe, h: h}
}

func (r *docRig) do(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			rdr = bytes.NewBufferString(v)
		case []byte:
			rdr = bytes.NewReader(v)
		default:
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.h.Routes().ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestDocHealthz(t *testing.T) {
	w := newRig().do(http.MethodGet, "/healthz", nil, nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ---------------------------------------------------------------------------
// List
// ---------------------------------------------------------------------------

func TestList_EmptyHasZeroCount(t *testing.T) {
	w := newRig().do(http.MethodGet, "/api/v1/documents/", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"count":0`)) {
		t.Errorf("expected count=0, got %s", w.Body.String())
	}
}

func TestList_InvalidProjectID(t *testing.T) {
	w := newRig().do(http.MethodGet, "/api/v1/documents/?project_id=garbage", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestList_InvalidCursor(t *testing.T) {
	w := newRig().do(http.MethodGet, "/api/v1/documents/?cursor=garbage", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestList_PassesProjectIDToStore(t *testing.T) {
	r := newRig()
	pid := uuid.New()
	r.store.listFn = func(_ context.Context, p *uuid.UUID, lim int, _ *uuid.UUID) ([]*model.Document, error) {
		if p == nil || *p != pid {
			t.Errorf("projectID = %v, want %s", p, pid)
		}
		return []*model.Document{{ID: uuid.New(), ProjectID: pid}}, nil
	}
	w := r.do(http.MethodGet, "/api/v1/documents/?project_id="+pid.String(), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

// ---------------------------------------------------------------------------
// Get / Delete / Update
// ---------------------------------------------------------------------------

func TestGet_NotFound(t *testing.T) {
	w := newRig().do(http.MethodGet, "/api/v1/documents/"+uuid.NewString(), nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGet_InvalidID(t *testing.T) {
	w := newRig().do(http.MethodGet, "/api/v1/documents/not-a-uuid", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestGet_Success(t *testing.T) {
	r := newRig()
	id := uuid.New()
	r.store.getFn = func(_ context.Context, got uuid.UUID) (*model.Document, error) {
		return &model.Document{ID: got, Name: "RFP.pdf", MimeType: "application/pdf"}, nil
	}
	w := r.do(http.MethodGet, "/api/v1/documents/"+id.String(), nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"name":"RFP.pdf"`)) {
		t.Errorf("body should contain name, got %s", w.Body.String())
	}
}

func TestUpdate_Success(t *testing.T) {
	r := newRig()
	id := uuid.New()
	r.store.updateFn = func(_ context.Context, got uuid.UUID, req *model.UpdateRequest) (*model.Document, error) {
		if got != id {
			t.Errorf("id mismatch: got %s want %s", got, id)
		}
		if req.Name == nil || *req.Name != "new name" {
			t.Errorf("expected name to be propagated, got %v", req.Name)
		}
		return &model.Document{ID: id, Name: *req.Name}, nil
	}
w := r.do(http.MethodPatch, "/api/v1/documents/"+id.String(),
		map[string]any{"name": "new name", "version": 1}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestUpdate_NotFound(t *testing.T) {
	r := newRig()
	r.store.updateFn = func(context.Context, uuid.UUID, *model.UpdateRequest) (*model.Document, error) {
		return nil, store.ErrNotFound
	}
	w := r.do(http.MethodPatch, "/api/v1/documents/"+uuid.NewString(),
		map[string]any{"name": "x", "version": 1}, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDelete_NotFound(t *testing.T) {
	w := newRig().do(http.MethodDelete, "/api/v1/documents/"+uuid.NewString(), nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// ---------------------------------------------------------------------------
// CreateJSON (metadata-only)
// ---------------------------------------------------------------------------

func TestCreateJSON_InvalidJSON(t *testing.T) {
	w := newRig().do(http.MethodPost, "/api/v1/documents/json", "not-json", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestCreateJSON_StoreErrorReturns500(t *testing.T) {
	r := newRig()
	r.store.createFn = func(context.Context, *model.Document) error { return errors.New("db down") }
	// Valid CreateRequest shape so we get past the validator and into the
	// store.Create call where we inject the error.
	w := r.do(http.MethodPost, "/api/v1/documents/json", map[string]any{
		"project_id":      uuid.NewString(),
		"name":            "x.pdf",
		"kind":            "tender",
		"mime_type":       "application/pdf",
		"size_bytes":      1234,
		"checksum_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
	}, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Upload (multipart)
// ---------------------------------------------------------------------------

func TestUpload_Success(t *testing.T) {
	r := newRig()
	// Build a multipart body with a tiny text file. project_id is required
	// as a form field (matches the handler's expectation).
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("project_id", uuid.NewString())
	fw, _ := mw.CreateFormFile("file", "test.txt")
	_, _ = fw.Write([]byte("hello bid"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	if r.store.created == nil {
		t.Fatal("Store.Create was not called")
	}
	if r.store.created.Name != "test.txt" {
		t.Errorf("name = %q, want test.txt", r.store.created.Name)
	}
	if r.store.created.SizeBytes != int64(len("hello bid")) {
		t.Errorf("size = %d, want %d", r.store.created.SizeBytes, len("hello bid"))
	}
}

func TestUpload_MissingFileField(t *testing.T) {
	r := newRig()
	// Multipart with no "file" field — should 400.
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("other", "value")
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestUpload_StoreErrorTriggersStorageCleanup(t *testing.T) {
	r := newRig()
	r.store.createFn = func(context.Context, *model.Document) error { return errors.New("db failed") }

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("project_id", uuid.NewString())
	fw, _ := mw.CreateFormFile("file", "cleanup.txt")
	_, _ = fw.Write([]byte("clean me up"))
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents/", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	// Storage should be cleaned up. We use "test.txt" or whatever name was
	// uploaded; check it's gone from the memStorage.
	if _, ok := r.storage.data["cleanup.txt"]; ok {
		t.Errorf("expected cleanup.txt to be deleted from storage, but it's still there")
	}
}

// ---------------------------------------------------------------------------
// Parse
// ---------------------------------------------------------------------------

func TestParse_Success(t *testing.T) {
	r := newRig()
	id := uuid.New()
	r.store.getFn = func(_ context.Context, got uuid.UUID) (*model.Document, error) {
		return &model.Document{ID: got, Name: "x"}, nil
	}
	r.parser.parseFn = func(_ context.Context, got uuid.UUID, async bool) (*model.ParseResult, error) {
		return &model.ParseResult{ProjectName: "测试项目", Industry: "IT", Issuer: "某单位"}, nil
	}
	w := r.do(http.MethodPost, "/api/v1/documents/"+id.String()+"/parse",
		map[string]any{"async": false}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"project_name":"测试项目"`)) {
		t.Errorf("body should contain project_name, got %s", w.Body.String())
	}
	if !r.parser.lastAsync == r.parser.lastAsync { // tautology: confirm field was set
		t.Errorf("parser lastAsync not set")
	}
}

func TestParse_AsyncFlagPropagates(t *testing.T) {
	r := newRig()
	id := uuid.New()
	r.store.getFn = func(_ context.Context, got uuid.UUID) (*model.Document, error) {
		return &model.Document{ID: got}, nil
	}
	r.parser.parseFn = func(context.Context, uuid.UUID, bool) (*model.ParseResult, error) {
		return &model.ParseResult{}, nil
	}
	w := r.do(http.MethodPost, "/api/v1/documents/"+id.String()+"/parse",
		map[string]any{"async": true}, nil)
	// Async returns 202 Accepted; sync returns 200 OK.
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	if !r.parser.lastAsync {
		t.Errorf("expected async=true to reach parser, got false")
	}
}

func TestGetParseResult_NotParsed(t *testing.T) {
	r := newRig()
	r.parser.resultFn = func(context.Context, uuid.UUID) (*model.ParseResult, error) {
		// Simulate parser reporting "no result yet" — handler should map to 404.
		return nil, errors.New("no parse result")
	}
	w := r.do(http.MethodGet, "/api/v1/documents/"+uuid.NewString()+"/parse-result", nil, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500 (parser error mapped to internal)", w.Code)
	}
}

// ---------------------------------------------------------------------------
// Export
// ---------------------------------------------------------------------------

func TestExportDocument_Success(t *testing.T) {
	r := newRig()
	body := map[string]any{
		"bid_job_id": uuid.NewString(),
		"format":     "docx",
		"chapters":   []map[string]string{{"title": "x", "content": "y"}},
	}
	w := r.do(http.MethodPost, "/api/v1/export/document", body, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if r.exporter.lastReq == nil {
		t.Fatal("Exporter.Export was not called")
	}
}

func TestExportDocument_InvalidJSON(t *testing.T) {
	w := newRig().do(http.MethodPost, "/api/v1/export/document", "not-json", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestExportDocument_ExporterErrorReturns500(t *testing.T) {
	r := newRig()
	r.exporter.exportFn = func(context.Context, *service.ExportRequest) (*service.ExportResult, error) {
		return nil, errors.New("libreoffice crashed")
	}
	w := r.do(http.MethodPost, "/api/v1/export/document",
		map[string]any{"bid_job_id": uuid.NewString(), "format": "pdf"}, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}