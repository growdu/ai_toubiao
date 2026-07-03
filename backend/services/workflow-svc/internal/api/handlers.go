// Package api implements the HTTP layer for workflow-svc.
package api

import (
	"context"
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
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/google/uuid"
)

// WorkflowBackend is the storage contract required by Handlers. Defined at the
// consumer (api package) so handlers can be unit-tested with a fake. The
// concrete *store.Store satisfies this interface naturally.
type WorkflowBackend interface {
	Create(ctx context.Context, req *model.CreateRequest, actorID uuid.UUID) (*model.Workflow, error)
	Get(ctx context.Context, id uuid.UUID) (*model.Workflow, error)
	List(ctx context.Context, projectID *uuid.UUID, status *model.State, limit int) ([]*model.Workflow, error)
	Transition(ctx context.Context, id uuid.UUID, req *model.TransitionRequest, expectedVersion int, actorID uuid.UUID) (*model.Workflow, error)
	ListSteps(ctx context.Context, workflowID uuid.UUID) ([]*model.Step, error)
	ListEvents(ctx context.Context, workflowID uuid.UUID, limit int) ([]*model.Event, error)
	Pause(ctx context.Context, id uuid.UUID, req *model.PauseRequest, actorID uuid.UUID) (*model.Workflow, error)
	Resume(ctx context.Context, id uuid.UUID, req *model.ResumeRequest, actorID uuid.UUID) (*model.Workflow, error)
}

type Handlers struct {
	Store        WorkflowBackend
	Log          *slog.Logger
	DocBuilder   DocBuilder   // optional; defaults to ooxmlBuilder{}
	PDFConverter PDFConverter // optional; nil means PDF endpoints fall back to DOCX
	Enqueuer     Enqueuer     // optional; nil means transitions don't enqueue tasks
	ChapterPool  *pgxpool.Pool // optional; enables chapter CRUD endpoints
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/bids", func(r chi.Router) {
		r.Post("/", h.create)
		r.Get("/", h.list)
		r.Route("/{id}", func(r chi.Router) {
			r.Get("/", h.get)
			r.Post("/transition", h.transition)
			r.Get("/steps", h.listSteps)
			r.Get("/events", h.listEvents)
			// HIL endpoints (human-in-the-loop pause/resume)
			r.Post("/pause", h.pause)
			r.Post("/resume", h.resume)
			// Export endpoints
			r.Get("/export/word", h.exportWordHandler)
			r.Get("/export/pdf", h.exportPDFHandler)
			r.Post("/export", h.exportDocumentHandler)
		// Chapter CRUD endpoints
		if h.ChapterPool != nil {
			ch := &ChapterHandlers{Pool: h.ChapterPool, Log: h.Log}
			ch.ChapterRoutes(r)
		}
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

	// Auto-create a bid_job linking the project and workflow so chapter
	// endpoints work immediately without a separate API call.
	if h.ChapterPool != nil {
		var projectName string
		h.ChapterPool.QueryRow(r.Context(),
			`SELECT name FROM projects WHERE id = $1`, req.ProjectID).Scan(&projectName)
		_, err := h.ChapterPool.Exec(r.Context(), `
			INSERT INTO bid_jobs (tenant_id, project_id, workflow_id, status, project_name)
			VALUES ($1, $2, $3, 'pending', $4)`,
			wf.TenantID, req.ProjectID, wf.ID, projectName)
		if err != nil {
			h.Log.Warn("auto-create bid_job failed",
				slog.String("err", err.Error()),
				slog.String("workflow_id", wf.ID.String()))
		}
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
		// Best-effort: enqueue async pipeline tasks for the new state.
		h.dispatchOnTransition(r.Context(), wf, h.Log)
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

// pause moves an active workflow to StatePaused and records the previous
// state in metadata.paused_from so the matching resume call knows where to
// go back to. Terminal / failed / already-paused workflows are rejected by
// the underlying state machine (ErrInvalidState → 422).
func (h *Handlers) pause(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, false)
}

// resume returns a paused workflow to its previous state (or to the
// explicit `to` in the request body). Rejects anything not currently
// paused (ErrInvalidState → 422).
func (h *Handlers) resume(w http.ResponseWriter, r *http.Request) {
	h.lifecycleAction(w, r, true)
}

// lifecycleAction is the shared pause/resume dispatcher. It validates the
// workflow id, parses the request body, requires X-User-ID, and translates
// domain errors to HTTP envelopes. The store is responsible for the
// metadata merge and the actual state transition.
func (h *Handlers) lifecycleAction(w http.ResponseWriter, r *http.Request, isResume bool) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	actorID := r.Header.Get("X-User-ID")
	if actorID == "" {
		httperr.Write(w, http.StatusBadRequest, httperr.CodeInvalidInput, "缺少 X-User-ID", rid, nil)
		return
	}

	var (
		wf *model.Workflow
		e  error
	)
	if isResume {
		// Resume accepts an optional `to`; an empty body is fine — the store
		// falls back to metadata.paused_from.
		var req model.ResumeRequest
		if r.ContentLength != 0 {
			if err := readJSON(r.Body, &req); err != nil {
				httperr.InvalidInput(w, rid, "invalid JSON", nil)
				return
			}
		}
		if err := validator.Validate(&req); err != nil {
			httperr.InvalidInput(w, rid, err.Error(), nil)
			return
		}
		wf, e = h.Store.Resume(r.Context(), id, &req, uuid.MustParse(actorID))
	} else {
		var req model.PauseRequest
		if r.ContentLength != 0 {
			if err := readJSON(r.Body, &req); err != nil {
				httperr.InvalidInput(w, rid, "invalid JSON", nil)
				return
			}
		}
		if err := validator.Validate(&req); err != nil {
			httperr.InvalidInput(w, rid, err.Error(), nil)
			return
		}
		wf, e = h.Store.Pause(r.Context(), id, &req, uuid.MustParse(actorID))
	}

	switch {
	case errors.Is(e, store.ErrNotFound):
		httperr.NotFound(w, rid, "workflow")
	case errors.Is(e, store.ErrInvalidState):
		msg := "工作流当前状态不允许暂停"
		if isResume {
			msg = "工作流未处于暂停状态,无法恢复"
		}
		httperr.Write(w, http.StatusUnprocessableEntity, httperr.CodeInvalidInput, msg, rid, nil)
	case errors.Is(e, store.ErrVersionConflict):
		httperr.Write(w, http.StatusConflict, httperr.CodeVersionConflict, "版本冲突", rid, nil)
	case e != nil:
		h.Log.Error("lifecycle", slog.String("err", e.Error()))
		httperr.InternalError(w, rid)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"data": wf})
	}
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