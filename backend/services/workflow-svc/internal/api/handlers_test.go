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

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/store"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// fakeBackend is a hand-rolled in-memory WorkflowBackend for handler tests.
type fakeBackend struct {
	createFn       func(context.Context, *model.CreateRequest, uuid.UUID) (*model.Workflow, error)
	getFn          func(context.Context, uuid.UUID) (*model.Workflow, error)
	listFn         func(context.Context, *uuid.UUID, *model.State, int) ([]*model.Workflow, error)
	transitionFn   func(context.Context, uuid.UUID, *model.TransitionRequest, int, uuid.UUID) (*model.Workflow, error)
	listStepsFn    func(context.Context, uuid.UUID) ([]*model.Step, error)
	listEventsFn   func(context.Context, uuid.UUID, int) ([]*model.Event, error)
}

func (f *fakeBackend) Create(ctx context.Context, req *model.CreateRequest, actorID uuid.UUID) (*model.Workflow, error) {
	return f.createFn(ctx, req, actorID)
}
func (f *fakeBackend) Get(ctx context.Context, id uuid.UUID) (*model.Workflow, error) {
	return f.getFn(ctx, id)
}
func (f *fakeBackend) List(ctx context.Context, p *uuid.UUID, s *model.State, l int) ([]*model.Workflow, error) {
	return f.listFn(ctx, p, s, l)
}
func (f *fakeBackend) Transition(ctx context.Context, id uuid.UUID, r *model.TransitionRequest, v int, a uuid.UUID) (*model.Workflow, error) {
	return f.transitionFn(ctx, id, r, v, a)
}
func (f *fakeBackend) ListSteps(ctx context.Context, id uuid.UUID) ([]*model.Step, error) {
	return f.listStepsFn(ctx, id)
}
func (f *fakeBackend) ListEvents(ctx context.Context, id uuid.UUID, l int) ([]*model.Event, error) {
	return f.listEventsFn(ctx, id, l)
}

func newTestHandler(b WorkflowBackend) *Handlers {
	return &Handlers{Store: b, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func ctxWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	return doRequestWithHeaders(t, h, method, path, body, nil)
}

func doRequestWithHeaders(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr).WithContext(ctxWithTenant())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Default to a fresh UUID so the X-User-ID-required write paths
	// pass; tests that need a specific value (or absence) pass it via
	// `headers` and/or set req.Header directly.
	if _, ok := headers["X-User-ID"]; !ok {
		req.Header.Set("X-User-ID", uuid.NewString())
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func TestCreateWorkflow_Success(t *testing.T) {
	projectID := uuid.New()
	actor := uuid.New()
	wf := &model.Workflow{ID: uuid.New(), ProjectID: projectID, Status: model.StatePending, Version: 1}
	be := &fakeBackend{
		createFn: func(_ context.Context, req *model.CreateRequest, a uuid.UUID) (*model.Workflow, error) {
			if req.ProjectID != projectID {
				t.Errorf("project_id mismatch: got %s want %s", req.ProjectID, projectID)
			}
			if a != actor {
				t.Errorf("actor mismatch: got %s want %s", a, actor)
			}
			return wf, nil
		},
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	w := doRequestWithHeaders(t, h.Routes(), http.MethodPost, "/api/v1/bids/",
		map[string]any{"project_id": projectID.String()},
		map[string]string{"X-User-ID": actor.String()})

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data *model.Workflow `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data == nil || got.Data.ID != wf.ID {
		t.Errorf("unexpected response: %+v", got)
	}
}

func TestCreateWorkflow_ActiveExistsConflict(t *testing.T) {
	be := &fakeBackend{
		createFn: func(context.Context, *model.CreateRequest, uuid.UUID) (*model.Workflow, error) {
			return nil, store.ErrActiveExists
		},
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/bids/",
		map[string]any{"project_id": uuid.NewString()})
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("ALREADY_EXISTS")) {
		t.Errorf("expected ALREADY_EXISTS code in body, got %s", w.Body.String())
	}
}

func TestCreateWorkflow_MissingUserID(t *testing.T) {
	be := &fakeBackend{
		listStepsFn:  func(context.Context, uuid.UUID) ([]*model.Step, error) { return nil, nil },
		listEventsFn: func(context.Context, uuid.UUID, int) ([]*model.Event, error) { return nil, nil },
	}
	h := newTestHandler(be)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/bids/",
		bytes.NewBufferString(`{"project_id":"`+uuid.NewString()+`"}`)).WithContext(ctxWithTenant())
	req.Header.Set("Content-Type", "application/json")
	// Intentionally do NOT set X-User-ID.
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
}

func TestGetWorkflow_NotFound(t *testing.T) {
	be := &fakeBackend{
		getFn: func(context.Context, uuid.UUID) (*model.Workflow, error) { return nil, store.ErrNotFound },
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/"+uuid.NewString(), nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404; body=%s", w.Code, w.Body.String())
	}
}

func TestGetWorkflow_InvalidID(t *testing.T) {
	h := newTestHandler(&fakeBackend{})
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/not-a-uuid", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestListWorkflows_EmptyReturnsZeroCount(t *testing.T) {
	be := &fakeBackend{
		listFn: func(context.Context, *uuid.UUID, *model.State, int) ([]*model.Workflow, error) {
			return nil, nil
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data []*model.Workflow `json:"data"`
		Meta map[string]any    `json:"meta"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &got)
	if got.Meta["count"] != float64(0) {
		t.Errorf("expected count=0, got %v", got.Meta["count"])
	}
}

func TestListWorkflows_InvalidProjectID(t *testing.T) {
	h := newTestHandler(&fakeBackend{})
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/?project_id=garbage", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTransition_Success(t *testing.T) {
	id := uuid.New()
	updated := &model.Workflow{ID: id, Status: model.StateParsing, Version: 2}
	be := &fakeBackend{
		transitionFn: func(_ context.Context, _ uuid.UUID, r *model.TransitionRequest, v int, _ uuid.UUID) (*model.Workflow, error) {
			if r.To != model.StateParsing {
				t.Errorf("to = %s, want parsing", r.To)
			}
			if v != 1 {
				t.Errorf("version = %d, want 1", v)
			}
			return updated, nil
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/bids/"+id.String()+"/transition",
		map[string]any{"to": "parsing", "reason": "start"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestTransition_VersionConflict(t *testing.T) {
	be := &fakeBackend{
		transitionFn: func(context.Context, uuid.UUID, *model.TransitionRequest, int, uuid.UUID) (*model.Workflow, error) {
			return nil, store.ErrVersionConflict
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/bids/"+uuid.NewString()+"/transition",
		map[string]any{"to": "parsing"})
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("VERSION_CONFLICT")) {
		t.Errorf("expected VERSION_CONFLICT code, got %s", w.Body.String())
	}
}

func TestTransition_InvalidState(t *testing.T) {
	be := &fakeBackend{
		transitionFn: func(context.Context, uuid.UUID, *model.TransitionRequest, int, uuid.UUID) (*model.Workflow, error) {
			return nil, store.ErrInvalidState
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/bids/"+uuid.NewString()+"/transition",
		map[string]any{"to": "parsing"})
	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422; body=%s", w.Code, w.Body.String())
	}
}

func TestListSteps_BubblesUpBackendError(t *testing.T) {
	be := &fakeBackend{
		listStepsFn: func(context.Context, uuid.UUID) ([]*model.Step, error) {
			return nil, errors.New("db down")
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/bids/"+uuid.NewString()+"/steps", nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
}

func TestListEvents_Success(t *testing.T) {
	be := &fakeBackend{
		listEventsFn: func(_ context.Context, _ uuid.UUID, l int) ([]*model.Event, error) {
			if l != 50 {
				t.Errorf("limit = %d, want 50", l)
			}
			return []*model.Event{{ID: 1, ToState: model.StateParsing}}, nil
		},
	}
	h := newTestHandler(be)
	w := doRequest(t, h.Routes(), http.MethodGet,
		"/api/v1/bids/"+uuid.NewString()+"/events?limit=50", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	h := newTestHandler(&fakeBackend{})
	w := doRequest(t, h.Routes(), http.MethodGet, "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"ok"`)) {
		t.Errorf("body = %s", w.Body.String())
	}
}
