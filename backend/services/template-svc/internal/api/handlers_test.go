package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bidwriter/services/template-svc/internal/model"
	"github.com/bidwriter/services/template-svc/internal/store"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// ---- fake service ----

type fakeService struct {
	listFn     func(ctx context.Context) ([]*model.WordTemplate, error)
	uploadFn   func(ctx context.Context, userID uuid.UUID, req *model.CreateRequest, file io.Reader, filename string, size int64) (*model.WordTemplate, error)
	getFn      func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error)
	updateFn   func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error)
	deleteFn   func(ctx context.Context, id uuid.UUID) error
	downloadFn func(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.WordTemplate, error)
}

func (f *fakeService) List(ctx context.Context) ([]*model.WordTemplate, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return nil, nil
}
func (f *fakeService) Upload(ctx context.Context, userID uuid.UUID, req *model.CreateRequest, file io.Reader, filename string, size int64) (*model.WordTemplate, error) {
	if f.uploadFn != nil {
		return f.uploadFn(ctx, userID, req, file, filename, size)
	}
	return &model.WordTemplate{ID: uuid.New(), Name: req.Name, Kind: req.Kind, IsDefault: req.IsDefault}, nil
}
func (f *fakeService) Get(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
	if f.getFn != nil {
		return f.getFn(ctx, id)
	}
	return nil, store.ErrNotFound
}
func (f *fakeService) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
	if f.updateFn != nil {
		return f.updateFn(ctx, id, req)
	}
	return nil, nil
}
func (f *fakeService) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}
func (f *fakeService) Download(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.WordTemplate, error) {
	if f.downloadFn != nil {
		return f.downloadFn(ctx, id)
	}
	return nil, nil, store.ErrNotFound
}

// ---- helpers ----

func newTestHandlers(svc Service) *Handlers {
	return &Handlers{Service: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func ctxWithTenant(t *testing.T) context.Context {
	t.Helper()
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func doReq(t *testing.T, h http.Handler, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, body)
	if body != nil {
		r.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// newRouterWithCtx mounts handlers.Routes() and optionally injects a tenant
// ctx so the api layer can be unit-tested without the project-wide middleware.
func ctxWithTenantSilent() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func newRouterWithCtx(h *Handlers, injectTenant bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if injectTenant {
			r = r.WithContext(ctxWithTenantSilent())
		}
		h.Routes().ServeHTTP(w, r)
	})
}

// ---- tests ----

func TestList_Success(t *testing.T) {
	svc := &fakeService{
		listFn: func(ctx context.Context) ([]*model.WordTemplate, error) {
			return []*model.WordTemplate{{ID: uuid.New(), Name: "A"}, {ID: uuid.New(), Name: "B"}}, nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)

	w := doReq(t, r, http.MethodGet, "/api/v1/templates/", nil, nil)
	if w.Code != 200 {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []*model.WordTemplate `json:"data"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 templates, got %d", len(resp.Data))
	}
}

func TestList_NoTenant_401(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, false)
	w := doReq(t, r, http.MethodGet, "/api/v1/templates/", nil, nil)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestGet_Success(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{
		getFn: func(ctx context.Context, got uuid.UUID) (*model.WordTemplate, error) {
			if got != id {
				t.Errorf("wrong id passed to service: got %v want %v", got, id)
			}
			return &model.WordTemplate{ID: id, Name: "A"}, nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)

	w := doReq(t, r, http.MethodGet, "/api/v1/templates/"+id.String(), nil, nil)
	if w.Code != 200 {
		t.Errorf("status: got %d", w.Code)
	}
}

func TestGet_NotFound_404(t *testing.T) {
	svc := &fakeService{
		getFn: func(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
			return nil, store.ErrNotFound
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/templates/"+uuid.NewString(), nil, nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGet_BadID_400(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/templates/not-a-uuid", nil, nil)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDelete_Success_204(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{
		deleteFn: func(ctx context.Context, got uuid.UUID) error {
			if got != id {
				t.Errorf("wrong id: got %v", got)
			}
			return nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodDelete, "/api/v1/templates/"+id.String(), nil, nil)
	if w.Code != 204 {
		t.Errorf("expected 204, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDelete_NotFound_404(t *testing.T) {
	svc := &fakeService{
		deleteFn: func(ctx context.Context, id uuid.UUID) error { return store.ErrNotFound },
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodDelete, "/api/v1/templates/"+uuid.NewString(), nil, nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdate_Success_200(t *testing.T) {
	id := uuid.New()
	name := "renamed"
	svc := &fakeService{
		updateFn: func(ctx context.Context, got uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
			if req.Name == nil || *req.Name != name {
				t.Errorf("name not propagated: %+v", req)
			}
			return &model.WordTemplate{ID: id, Name: name}, nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodPatch, "/api/v1/templates/"+id.String(),
		strings.NewReader(`{"name":"renamed"}`), nil)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdate_NotFound_404(t *testing.T) {
	svc := &fakeService{
		updateFn: func(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
			return nil, store.ErrNotFound
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodPatch, "/api/v1/templates/"+uuid.NewString(),
		strings.NewReader(`{"name":"x"}`), nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpdate_BadJSON_400(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodPatch, "/api/v1/templates/"+uuid.NewString(),
		strings.NewReader("not json"), nil)
	if w.Code != 400 {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestDownload_ContentTypeAndDisposition(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{
		downloadFn: func(ctx context.Context, got uuid.UUID) (io.ReadCloser, *model.WordTemplate, error) {
			return io.NopCloser(bytes.NewReader([]byte("DOCXBYTES"))), &model.WordTemplate{ID: id, Name: "My Tpl"}, nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/templates/"+id.String()+"/download", nil, nil)
	if w.Code != 200 {
		t.Fatalf("status: got %d body=%s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "officedocument.wordprocessingml") {
		t.Errorf("Content-Type mismatch: %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, `filename="My Tpl.docx"`) {
		t.Errorf("Content-Disposition mismatch: %q", cd)
	}
	if w.Body.String() != "DOCXBYTES" {
		t.Errorf("body not streamed: %q", w.Body.String())
	}
}

func TestDownload_NotFound_404(t *testing.T) {
	svc := &fakeService{
		downloadFn: func(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.WordTemplate, error) {
			return nil, nil, store.ErrNotFound
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)
	w := doReq(t, r, http.MethodGet, "/api/v1/templates/"+uuid.NewString()+"/download", nil, nil)
	if w.Code != 404 {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestUpload_Multipart_Success(t *testing.T) {
	var capturedFilename string
	var capturedSize int64
	svc := &fakeService{
		uploadFn: func(ctx context.Context, userID uuid.UUID, req *model.CreateRequest, file io.Reader, filename string, size int64) (*model.WordTemplate, error) {
			capturedFilename = filename
			capturedSize = size
			if req.Name != "MyTpl" {
				t.Errorf("name not propagated: %q", req.Name)
			}
			if req.Kind != "standard" {
				t.Errorf("kind not propagated: %q", req.Kind)
			}
			return &model.WordTemplate{ID: uuid.New(), Name: "MyTpl", Kind: "standard"}, nil
		},
	}
	h := newTestHandlers(svc)
	r := newRouterWithCtx(h, true)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "MyTpl")
	_ = mw.WriteField("description", "d")
	_ = mw.WriteField("kind", "standard")
	_ = mw.WriteField("is_default", "false")
	fw, _ := mw.CreateFormFile("file", "tpl.docx")
	_, _ = fw.Write([]byte("DOCXDATA"))
	mw.Close()

	w := doReq(t, r, http.MethodPost, "/api/v1/templates/", &buf, map[string]string{
		"Content-Type": mw.FormDataContentType(),
	})
	if w.Code != 201 {
		t.Fatalf("expected 201, got %d body=%s", w.Code, w.Body.String())
	}
	if capturedFilename != "tpl.docx" {
		t.Errorf("filename not propagated: %q", capturedFilename)
	}
	if capturedSize != int64(len("DOCXDATA")) {
		t.Errorf("size not propagated: %d", capturedSize)
	}
}

func TestUpload_NoFile_400(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, true)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("name", "X")
	_ = mw.WriteField("kind", "standard")
	mw.Close()

	w := doReq(t, r, http.MethodPost, "/api/v1/templates/", &buf, map[string]string{
		"Content-Type": mw.FormDataContentType(),
	})
	if w.Code != 400 {
		t.Errorf("expected 400 when no file, got %d", w.Code)
	}
}

func TestUpload_NoTenant_401(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, false)
	w := doReq(t, r, http.MethodPost, "/api/v1/templates/", strings.NewReader(""), nil)
	if w.Code != 401 {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	h := newTestHandlers(&fakeService{})
	r := newRouterWithCtx(h, false)
	w := doReq(t, r, http.MethodGet, "/healthz", nil, nil)
	if w.Code != 200 {
		t.Errorf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("body: %q", w.Body.String())
	}
}

