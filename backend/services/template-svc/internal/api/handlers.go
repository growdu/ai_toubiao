package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidwriter/services/template-svc/internal/model"
	"github.com/bidwriter/services/template-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const maxUploadBytes = 50 << 20 // 50 MB

type Handlers struct {
	// Service is the consumer-defined service interface. *service.TemplateService
	// satisfies it; tests use a fake to avoid spinning up PG + filesystem.
	Service Service
	Log     *slog.Logger
}

// Service is the contract Handlers needs. Declared at the consumer (api
// package) so the api layer can be unit-tested with a fake.
type Service interface {
	List(ctx context.Context) ([]*model.WordTemplate, error)
	Upload(ctx context.Context, userID uuid.UUID, req *model.CreateRequest, file io.Reader, filename string, size int64) (*model.WordTemplate, error)
	Get(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error)
	Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
	Download(ctx context.Context, id uuid.UUID) (io.ReadCloser, *model.WordTemplate, error)
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/templates", func(r chi.Router) {
		r.Get("/", h.list)
		r.Post("/", h.upload)
		r.Get("/{id}", h.get)
		r.Patch("/{id}", h.update)
		r.Delete("/{id}", h.delete)
		r.Get("/{id}/download", h.download)
	})

	return r
}

// GET /api/v1/templates
func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	templates, err := h.Service.List(r.Context())
	if err != nil {
		h.Log.Error("list templates", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": templates})
}

// POST /api/v1/templates — multipart upload
func (h *Handlers) upload(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		httperr.InvalidInput(w, rid, "invalid multipart form: "+err.Error(), nil)
		return
	}

	var userID uuid.UUID
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		userID = uuid.MustParse(uid)
	}

	req := &model.CreateRequest{
		Name:        r.FormValue("name"),
		Description: r.FormValue("description"),
		Kind:        r.FormValue("kind"),
		IsDefault:   r.FormValue("is_default") == "true",
	}
	if req.Kind == "" {
		req.Kind = "standard"
	}

	fh, header, err := r.FormFile("file")
	if err != nil {
		httperr.InvalidInput(w, rid, "file is required", nil)
		return
	}
	defer fh.Close()

	t, err := h.Service.Upload(r.Context(), userID, req, fh, header.Filename, header.Size)
	if err != nil {
		h.Log.Error("upload template", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": t})
}

// GET /api/v1/templates/{id}
func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	t, err := h.Service.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "template")
		return
	}
	if err != nil {
		h.Log.Error("get template", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": t})
}

// PATCH /api/v1/templates/{id}
func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	var req model.UpdateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	t, err := h.Service.Update(r.Context(), id, &req)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "template")
		return
	}
	if err != nil {
		h.Log.Error("update template", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": t})
}

// DELETE /api/v1/templates/{id}
func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	if err := h.Service.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w, rid, "template")
			return
		}
		h.Log.Error("delete template", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// GET /api/v1/templates/{id}/download
func (h *Handlers) download(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	rc, t, err := h.Service.Download(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "template")
		return
	}
	if err != nil {
		h.Log.Error("download template", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", `attachment; filename="`+t.Name+`.docx"`)
	_, _ = io.Copy(w, rc)
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func readJSON(body io.ReadCloser, v any) error {
	if body == nil {
		return errors.New("empty body")
	}
	defer body.Close()
	dec := json.NewDecoder(io.LimitReader(body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// avoid unused import
var _ = strconv.Quote
