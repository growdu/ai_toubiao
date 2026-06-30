// Package api implements the HTTP layer for project-svc.
// Routes match docs/api/rest.md.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/bidwriter/services/project-svc/internal/middleware"
	"github.com/bidwriter/services/project-svc/internal/model"
	"github.com/bidwriter/services/project-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"log/slog"
)

// Handlers holds dependencies for the HTTP handlers.
type Handlers struct {
	Store *store.Store
	Log   *slog.Logger
}

// Routes wires up the router. Auth middleware is applied at the router level.
func (h *Handlers) Routes(authMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	// Health endpoints (unauthenticated)
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	})

	// Project endpoints (authenticated)
	r.Group(func(r chi.Router) {
		r.Use(authMW)
		r.Route("/api/v1/projects", func(r chi.Router) {
			r.Get("/", h.list)
			r.Post("/", h.create)
			r.Get("/{id}", h.get)
			r.Patch("/{id}", h.update)
			r.Delete("/{id}", h.delete)
		})
	})

	return r
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconvAtoi(q.Get("limit"))
	var cursor *uuid.UUID
	if c := q.Get("cursor"); c != "" {
		u, err := uuid.Parse(c)
		if err != nil {
			httperr.InvalidInput(w, rid, "invalid cursor", nil)
			return
		}
		cursor = &u
	}
	projects, err := h.Store.List(r.Context(), limit, cursor)
	if err != nil {
		h.Log.Error("list projects", slog.String("err", err.Error()), slog.String("rid", rid))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": projects,
		"meta": map[string]any{"count": len(projects)},
	})
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	var req model.CreateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON: "+err.Error(), nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	p := &model.Project{
		Name:           req.Name,
		Description:    req.Description,
		Industry:       req.Industry,
		TemplateID:     req.TemplateID,
		EstimatedValue: req.EstimatedValue,
		Currency:       req.Currency,
		Deadline:       req.Deadline,
	}
	if p.Currency == "" {
		p.Currency = "CNY"
	}
	if uid := middleware.UserID(r.Context()); uid != "" {
		p.OwnerID = uuid.MustParse(uid)
	}
	if err := h.Store.Create(r.Context(), p); err != nil {
		h.Log.Error("create project", slog.String("err", err.Error()), slog.String("rid", rid))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": p})
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	p, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "项目")
		return
	}
	if err != nil {
		h.Log.Error("get project", slog.String("err", err.Error()), slog.String("rid", rid))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func (h *Handlers) update(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	var req model.UpdateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON: "+err.Error(), nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	p, err := h.Store.Update(r.Context(), id, &req)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "项目")
		return
	}
	if errors.Is(err, store.ErrVersionConflict) {
		httperr.Write(w, http.StatusConflict, httperr.CodeVersionConflict,
			"版本冲突，请刷新后重试", rid, nil)
		return
	}
	if err != nil {
		h.Log.Error("update project", slog.String("err", err.Error()), slog.String("rid", rid))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": p})
}

func (h *Handlers) delete(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	if err := h.Store.Delete(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w, rid, "项目")
			return
		}
		h.Log.Error("delete project", slog.String("err", err.Error()), slog.String("rid", rid))
		httperr.InternalError(w, rid)
		return
	}
	w.WriteHeader(http.StatusNoContent)
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

func strconvAtoi(s string) (int, error) {
	if s == "" {
		return 0, nil
	}
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, errors.New("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// unused — silences "time" import if needed in future
var _ = time.Now