package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/store"
	"github.com/google/uuid"
)

// TestExportWordHandler_Success: full path through exportWordHandler with
// a fake backend that returns a typical workflow. Verifies 200 OK, the
// body is a non-empty DOCX (PK magic), and the Content-Type is the
// Office Open XML MIME type.
func TestExportWordHandler_Success(t *testing.T) {
	wfID := uuid.New()
	be := &fakeBackend{
		getFn: func(_ context.Context, _ uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: wfID, ProjectID: uuid.New(), Status: model.StateDone, Version: 1}, nil
		},
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/"+wfID.String()+"/export/word", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Errorf("body is not a ZIP/DOCX; got %d bytes, first 4: % x",
			w.Body.Len(), w.Body.Bytes()[:min(w.Body.Len(), 4)])
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("Content-Type = %q, want docx", ct)
	}
}

// TestExportDocumentHandler_Success: POST /export lets the caller supply
// a custom chapters list. Verifies the handler returns 200 + a DOCX when
// the backend is healthy.
func TestExportDocumentHandler_Success(t *testing.T) {
	wfID := uuid.New()
	be := &fakeBackend{
		getFn: func(_ context.Context, _ uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: wfID, ProjectID: uuid.New(), Status: model.StateDone, Version: 1}, nil
		},
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	body := map[string]any{
		"title":    "测试项目",
		"chapters": []map[string]string{{"title": "第一章", "content": "项目背景与目标"}},
	}
	w := doRequest(t, h.Routes(), http.MethodPost,
		"/api/v1/bids/"+wfID.String()+"/export", body)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if !bytes.HasPrefix(w.Body.Bytes(), []byte("PK\x03\x04")) {
		t.Errorf("body is not a DOCX; first 4: % x", w.Body.Bytes()[:min(w.Body.Len(), 4)])
	}
}

// TestExportWordHandler_InvalidID: bad UUID in path should 400, not 500.
func TestExportWordHandler_InvalidID(t *testing.T) {
	h := newTestHandler(&fakeBackend{})
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/not-a-uuid/export/word", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// TestExportDocumentHandler_NotFound: POST /export against a missing
// workflow yields 404. Confirms error path of the new endpoint.
func TestExportDocumentHandler_NotFound(t *testing.T) {
	be := &fakeBackend{
		getFn: func(_ context.Context, _ uuid.UUID) (*model.Workflow, error) {
			return nil, store.ErrNotFound
		},
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodPost,
		"/api/v1/bids/"+uuid.NewString()+"/export",
		map[string]any{"title": "x", "chapters": []any{}})
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

// TestExportDocumentHandler_InvalidJSON: malformed body should 400.
// Note: the handler calls Store.Get BEFORE decoding the body, so we
// must supply a getFn that returns a valid workflow first.
func TestExportDocumentHandler_InvalidJSON(t *testing.T) {
	wfID := uuid.New()
	be := &fakeBackend{
		getFn: func(_ context.Context, _ uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: wfID, Status: model.StateDone, Version: 1}, nil
		},
	}
	h := newTestHandler(be)
	req := newRawRequest(t, http.MethodPost, "/api/v1/bids/"+wfID.String()+"/export", "not-json")
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

// newRawRequest builds an HTTP request with a raw body string and the
// required tenant + X-User-ID headers (no JSON marshaling). Used by
// malformed-body tests.
func newRawRequest(t *testing.T, method, path, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(method, path, bytes.NewBufferString(body)).WithContext(ctxWithTenant())
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-User-ID", uuid.NewString())
	return req
}