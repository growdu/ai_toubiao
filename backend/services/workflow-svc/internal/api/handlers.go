// Package api implements the HTTP layer for workflow-svc.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/validator"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	Store *store.Store
	Log   *slog.Logger
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/workflows", func(r chi.Router) {
		r.Post("/", h.create)
		r.Get("/", h.list)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Post("/transition", h.transition)
			r.Get("/steps", h.listSteps)
			r.Get("/events", h.listEvents)
		})
	})
	return r
}

func (h *Handlers) create(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	var req model.CreateRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	actorID := r.Header.Get("X-User-ID")
	if actorID == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidInput, "缺少 X-User-ID", rid, nil)
		return
	}
	actor := uuid.MustParse(actorID)

	wf, err := h.Store.Create(r.Context(), &req, actor)
	if errors.Is(err, store.ErrActiveExists) {
		httperr.Write(w, http.StatusConflict, httperr.CodeAlreadyExists,
			"该项目已有进行中的工作流", rid, nil)
		return
	}
	if err != nil {
		h.Log.Error("create workflow", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": wf})
}

func (h *Handlers) get(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	wf, err := h.Store.Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "workflow")
		return
	}
	if err != nil {
		h.Log.Error("get workflow", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": wf})
}

func (h *Handlers) list(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	var projectID *uuid.UUID
	if p := q.Get("project_id"); p != "" {
		u, err := uuid.Parse(p)
		if err != nil {
			httperr.InvalidInput(w, rid, "invalid project_id", nil)
			return
		}
		projectID = &u
	}
	var status *model.State
	if s := q.Get("status"); s != "" {
		st := model.State(s)
		status = &st
	}
	out, err := h.Store.List(r.Context(), projectID, status, limit)
	if err != nil {
		h.Log.Error("list workflows", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": out,
		"meta": map[string]any{"count": len(out)},
	})
}

func (h *Handlers) transition(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	var req model.TransitionRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if err := validator.Validate(&req); err != nil {
		httperr.InvalidInput(w, rid, err.Error(), nil)
		return
	}
	actorID := r.Header.Get("X-User-ID")
	if actorID == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidInput, "缺少 X-User-ID", rid, nil)
		return
	}
	expectedVersion := 1
	if v := r.URL.Query().Get("version"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			expectedVersion = n
		}
	}

	wf, err := h.Store.Transition(r.Context(), id, &req, expectedVersion, uuid.MustParse(actorID))
	switch {
	case errors.Is(err, store.ErrNotFound):
		httperr.NotFound(w, rid, "workflow")
	case errors.Is(err, store.ErrVersionConflict):
		httperr.Write(w, http.StatusConflict, httperr.CodeVersionConflict,
			"版本冲突", rid, nil)
	case errors.Is(err, store.ErrInvalidState):
		httperr.Write(w, http.StatusUnprocessableEntity, httperr.CodeInvalidInput,
			"非法状态转换: "+string(req.To), rid, nil)
	case err != nil:
		h.Log.Error("transition", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"data": wf})
	}
}

func (h *Handlers) listSteps(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	steps, err := h.Store.ListSteps(r.Context(), id)
	if err != nil {
		h.Log.Error("list steps", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": steps,
		"meta": map[string]any{"count": len(steps)},
	})
}

func (h *Handlers) listEvents(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.Store.ListEvents(r.Context(), id, limit)
	if err != nil {
		h.Log.Error("list events", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"data": events,
		"meta": map[string]any{"count": len(events)},
	})
}

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