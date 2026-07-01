package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bidwriter/services/project-svc/internal/auth"
	"github.com/bidwriter/services/project-svc/internal/middleware"
	"github.com/bidwriter/services/project-svc/internal/model"
	"github.com/bidwriter/services/project-svc/internal/store"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/google/uuid"
)

// ---- fake store ----

type fakeStore struct {
	createFn func(ctx context.Context, p *model.Project) error
	getFn    func(ctx context.Context, id uuid.UUID) (*model.Project, error)
	listFn   func(ctx context.Context, limit int, cursor *uuid.UUID) ([]*model.Project, error)
	updateFn func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Project, error)
	deleteFn func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeStore) Create(ctx context.Context, p *model.Project) error {
	if f.createFn != nil {
		return f.createFn(ctx, p)
	}
	return nil
}
func (f *fakeStore) Get(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, store.ErrNotFound
}
func (f *fakeStore) List(ctx context.Context, limit int, cursor *uuid.UUID) ([]*model.Project, error) {
	if f.listFn != nil {
		return f.listFn(ctx, limit, cursor)
	}
	return nil, nil
}
func (f *fakeStore) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Project, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, req)
	}
	return nil, nil
}
func (f *fakeStore) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

// ---- helpers ----

const testJWTSecret = "test-jwt-secret"

func newTestRouter(t *testing.T, s ProjectStore, injectAuth bool) http.Handler {
	t.Helper()
	h := &Handlers{Store: s, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	authMW := func(next http.Handler) http.Handler { return next }
	if injectAuth {
		v := auth.NewVerifier(testJWTSecret)
		authMW = middleware.Auth(v)
	}
	// Wrap with request-id middleware so logger.RequestIDFrom works in
	// handlers and httperr responses.
	return middleware.RequestID(h.Routes(authMW))
}

func mustToken(t *testing.T, tenantID, userID string) string {
	t.Helper()
	tok, err := auth.IssueToken(testJWTSecret, tenantID, userID, "owner", 1*time.Hour)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	return tok
}

func doReq(t *testing.T, h http.Handler, method, path, token string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func envelope(t *testing.T, body []byte) map[string]json.RawMessage {
	t.Helper()
	var out map[string]json.RawMessage
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("envelope unmarshal: %v body=%s", err, string(body))
	}
	return out
}

// ---- tests ----

func TestList_Success_WithAuth(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	want := []*model.Project{{ID: uuid.New(), TenantID: uuid.MustParse(tid), Name: "P1"}}

	s := &fakeStore{
		listFn: func(ctx context.Context, limit int, cursor *uuid.UUID) ([]*model.Project, error) {
			return want, nil
		},
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/?limit=10", mustToken(t, tid, uid), nil)
	if w.Code != 200 {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	env := envelope(t, w.Body.Bytes())
	if _, ok := env["data"]; !ok {
		t.Errorf("response missing 'data' key: %s", w.Body.String())
	}
	if _, ok := env["meta"]; !ok {
		t.Errorf("response missing 'meta' key: %s", w.Body.String())
	}
}

func TestList_BadCursor_400(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	r := newTestRouter(t, &fakeStore{}, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/?cursor=not-a-uuid", mustToken(t, tid, uid), nil)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestList_StoreError_500(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		listFn: func(ctx context.Context, limit int, cursor *uuid.UUID) ([]*model.Project, error) {
			return nil, errors.New("db down")
		},
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/", mustToken(t, tid, uid), nil)
	if w.Code != 500 {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestList_NoAuth_401(t *testing.T) {
	r := newTestRouter(t, &fakeStore{}, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/", "", nil)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestCreate_Success_201(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	var created *model.Project
	s := &fakeStore{
		createFn: func(ctx context.Context, p *model.Project) error {
			created = p
			return nil
		},
	}
	r := newTestRouter(t, s, true)
	body := strings.NewReader(`{"name":"Project A","description":"d","industry":"it"}`)
	w := doReq(t, r, http.MethodPost, "/api/v1/projects/", mustToken(t, tid, uid), body)
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if created == nil {
		t.Fatal("createFn was not called")
	}
	if created.OwnerID != uuid.MustParse(uid) {
		t.Errorf("owner_id not set from auth context: got %v", created.OwnerID)
	}
	if created.Currency != "CNY" {
		t.Errorf("default currency not applied: %q", created.Currency)
	}
}

func TestCreate_ValidationError_400(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	r := newTestRouter(t, &fakeStore{}, true)
	body := strings.NewReader(`{"name":""}`) // violates required
	w := doReq(t, r, http.MethodPost, "/api/v1/projects/", mustToken(t, tid, uid), body)
	if w.Code != 400 {
		t.Errorf("expected 400 on empty name, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCreate_BadJSON_400(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	r := newTestRouter(t, &fakeStore{}, true)
	w := doReq(t, r, http.MethodPost, "/api/v1/projects/", mustToken(t, tid, uid),
		strings.NewReader("not json"))
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestGet_Success_200(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	id := uuid.New()
	s := &fakeStore{
		getFn: func(ctx context.Context, got uuid.UUID) (*model.Project, error) {
			if got != id {
				t.Errorf("wrong id: got %v want %v", got, id)
			}
			return &model.Project{ID: id, Name: "X"}, nil
		},
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/"+id.String(), mustToken(t, tid, uid), nil)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestGet_NotFound_404(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.Project, error) {
			return nil, store.ErrNotFound
		},
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/"+uuid.NewString(), mustToken(t, tid, uid), nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGet_BadID_400(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	r := newTestRouter(t, &fakeStore{}, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/projects/not-a-uuid", mustToken(t, tid, uid), nil)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestUpdate_Success_200(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	id := uuid.New()
	s := &fakeStore{
		updateFn: func(ctx context.Context, got uuid.UUID, req *model.UpdateRequest) (*model.Project, error) {
			return &model.Project{ID: got, Name: "renamed", Version: 2}, nil
		},
	}
	r := newTestRouter(t, s, true)
	body := strings.NewReader(`{"name":"renamed","version":1}`)
	w := doReq(t, r, http.MethodPatch, "/api/v1/projects/"+id.String(), mustToken(t, tid, uid), body)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdate_VersionConflict_409(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		updateFn: func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Project, error) {
			return nil, store.ErrVersionConflict
		},
	}
	r := newTestRouter(t, s, true)
	body := strings.NewReader(`{"name":"x","version":99}`)
	w := doReq(t, r, http.MethodPatch, "/api/v1/projects/"+uuid.NewString(), mustToken(t, tid, uid), body)
	if w.Code != 409 {
		t.Errorf("expected 409, got %d", w.Code)
	}
}

func TestUpdate_NotFound_404(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		updateFn: func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Project, error) {
			return nil, store.ErrNotFound
		},
	}
	r := newTestRouter(t, s, true)
	body := strings.NewReader(`{"name":"x","version":1}`)
	w := doReq(t, r, http.MethodPatch, "/api/v1/projects/"+uuid.NewString(), mustToken(t, tid, uid), body)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDelete_Success_204(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		deleteFn: func(ctx context.Context, id uuid.UUID) error { return nil },
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodDelete, "/api/v1/projects/"+uuid.NewString(), mustToken(t, tid, uid), nil)
	if w.Code != 204 {
		t.Errorf("expected 204, got %d", w.Code)
	}
}

func TestDelete_NotFound_404(t *testing.T) {
	tid := uuid.NewString()
	uid := uuid.NewString()
	s := &fakeStore{
		deleteFn: func(ctx context.Context, id uuid.UUID) error { return store.ErrNotFound },
	}
	r := newTestRouter(t, s, true)
	w := doReq(t, r, http.MethodDelete, "/api/v1/projects/"+uuid.NewString(), mustToken(t, tid, uid), nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestHealthz_NoAuth_200(t *testing.T) {
	r := newTestRouter(t, &fakeStore{}, true) // auth is on the protected group
	w := doReq(t, r, http.MethodGet, "/healthz", "", nil)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestStrconvAtoi(t *testing.T) {
	cases := []struct {
		in   string
		want int
		err  bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"42", 42, false},
		{"abc", 0, true},
		{"12x", 0, true},
	}
	for _, c := range cases {
		got, err := strconvAtoi(c.in)
		if (err != nil) != c.err {
			t.Errorf("strconvAtoi(%q) err=%v want err=%v", c.in, err, c.err)
		}
		if got != c.want {
			t.Errorf("strconvAtoi(%q) = %d want %d", c.in, got, c.want)
		}
	}
}

// Ensure import isn't reported as unused if extracted helpers come and go.
var _ = logger.WithRequest
