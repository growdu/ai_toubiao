// Package api implements the HTTP layer for knowledge-svc.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/services/knowledge-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	// KBService is the dependency the handlers need. Declared as an interface
	// (implemented by *service.KBService) so tests can pass a fake without
	// pulling in pgvector, the embedding router, or any LLM client.
	KBService KBAPI
	Log       *slog.Logger
}

// KBAPI is the surface area the HTTP layer needs from the KB service.
// Adding a method here is a contract change for every fake in tests.
type KBAPI interface {
	CreateMaterial(ctx context.Context, req *model.CreateMaterialRequest) (*model.KBMaterial, error)
	ListMaterials(ctx context.Context, category string, limit, offset int) ([]*model.KBMaterial, error)
	GetMaterial(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error)
	DeleteMaterial(ctx context.Context, id uuid.UUID) error
	Search(ctx context.Context, tenantID uuid.UUID, req *model.SearchRequest) (*model.SearchResponse, error)
	Ingest(ctx context.Context, req *model.IngestRequest) error
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/kb", func(r chi.Router) {
		// Materials CRUD
		r.Post("/materials", h.createMaterial)
		r.Get("/materials", h.listMaterials)
		r.Get("/materials/{id}", h.getMaterial)
		r.Delete("/materials/{id}", h.deleteMaterial)

		// Search
		r.Post("/search", h.search)

		// Ingest
		r.Post("/ingest", h.ingest)
	})

	return r
}

// POST /api/v1/kb/materials
func (h *Handlers) createMaterial(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	var req model.CreateMaterialRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	m, err := h.KBService.CreateMaterial(r.Context(), &req)
	if err != nil {
		h.Log.Error("create material", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": m})
}

// GET /api/v1/kb/materials?category=&limit=&offset=
func (h *Handlers) listMaterials(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	q := r.URL.Query()

	category := q.Get("category")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	materials, err := h.KBService.ListMaterials(r.Context(), category, limit, offset)
	if err != nil {
		h.Log.Error("list materials", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": materials, "meta": map[string]any{"count": len(materials)}})
}

// GET /api/v1/kb/materials/{id}
func (h *Handlers) getMaterial(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	m, err := h.KBService.GetMaterial(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "素材")
		return
	}
	if err != nil {
		h.Log.Error("get material", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": m})
}

// DELETE /api/v1/kb/materials/{id}
func (h *Handlers) deleteMaterial(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	if err := h.KBService.DeleteMaterial(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w, rid, "素材")
			return
		}
		h.Log.Error("delete material", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// POST /api/v1/kb/search
func (h *Handlers) search(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())

	// Get tenant ID from header (set by api-gateway)
	tidStr := r.Header.Get("X-Tenant-ID")
	if tidStr == "" {
		httperr.InvalidInput(w, rid, "X-Tenant-ID header required", nil)
		return
	}
	tid, err := uuid.Parse(tidStr)
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid X-Tenant-ID", nil)
		return
	}

	var req model.SearchRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	result, err := h.KBService.Search(r.Context(), tid, &req)
	if err != nil {
		h.Log.Error("search", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// POST /api/v1/kb/ingest
func (h *Handlers) ingest(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	var req model.IngestRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	if err := h.KBService.Ingest(r.Context(), &req); err != nil {
		h.Log.Error("ingest", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": "ingest started"})
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