package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/service"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type Handlers struct {
	Store            *store.Store
	ChapterAuditor   *service.ChapterAuditor
	CrossAuditor     *service.CrossAuditor
	RejectionChecker *service.RejectionChecker
	Log              *slog.Logger
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/audit", func(r chi.Router) {
		r.Post("/bidjobs/{id}/report", h.triggerReport)
		r.Get("/bidjobs/{id}/report", h.getReport)
		r.Post("/bidjobs/{id}/resolve", h.resolveIssue)
	})

	return r
}

// POST /api/v1/audit/bidjobs/{id}/report — trigger audit
func (h *Handlers) triggerReport(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	bidJobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid bid job id", nil)
		return
	}

	var req model.TriggerAuditRequest
	if err := readJSON(r.Body, &req); err != nil {
		req.Async = true // default to async
	}

	if req.Async {
		go h.runAudit(r.Context(), bidJobID)
		writeJSON(w, http.StatusAccepted, map[string]any{
			"message":   "audit started in background",
			"bid_job_id": bidJobID,
		})
		return
	}

	result, err := h.runAudit(r.Context(), bidJobID)
	if err != nil {
		h.Log.Error("audit failed", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": result})
}

// runAudit executes the full audit pipeline.
func (h *Handlers) runAudit(ctx context.Context, bidJobID uuid.UUID) (*model.AuditResult, error) {
	ctx = context.Background()

	// Get bid job and chapters
	data, err := h.Store.GetBidJobWithChapters(ctx, bidJobID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, err
	}
	if err != nil {
		return nil, err
	}

	tenantID := data.BidJob.TenantID

	// Create report
	report := &model.AuditReport{
		BidJobID: bidJobID,
		TenantID: tenantID,
		Status:   "running",
	}
	now := time.Now()
	report.StartedAt = &now

	if err := h.Store.CreateReport(ctx, report); err != nil {
		return nil, err
	}

	// Run all auditors
	var allIssues []*model.AuditIssue

	// 1. Chapter-level audit
	for _, ch := range data.Chapters {
		issues := h.ChapterAuditor.AuditChapter(ctx, bidJobID, tenantID, &ch)
		allIssues = append(allIssues, issues...)
	}

	// 2. Cross-chapter consistency
	crossIssues := h.CrossAuditor.AuditCrossChapter(ctx, bidJobID, tenantID, data.Chapters)
	allIssues = append(allIssues, crossIssues...)

	// 3. Rejection criteria check
	rejectionIssues := h.RejectionChecker.CheckRejectionCriteria(ctx, bidJobID, tenantID, data.Chapters)
	allIssues = append(allIssues, rejectionIssues...)

	// Save issues
	if err := h.Store.BatchInsertIssues(ctx, allIssues); err != nil {
		h.Log.Error("batch insert issues", slog.String("err", err.Error()))
	}

	// Update report
	report.Status = "done"
	finished := time.Now()
	report.FinishedAt = &finished
	for _, iss := range allIssues {
		switch iss.Severity {
		case model.SeverityCritical:
			report.Critical++
		case model.SeverityMajor:
			report.Major++
		default:
			report.Minor++
		}
	}
	report.TotalIssues = len(allIssues)
	report.Passed = report.Critical == 0

	if err := h.Store.UpdateReport(ctx, report); err != nil {
		h.Log.Error("update report", slog.String("err", err.Error()))
	}

	return &model.AuditResult{Report: report, Issues: allIssues}, nil
}

// GET /api/v1/audit/bidjobs/{id}/report — get audit report
func (h *Handlers) getReport(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	bidJobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid bid job id", nil)
		return
	}

	report, err := h.Store.GetReport(r.Context(), bidJobID)
	if errors.Is(err, store.ErrNotFound) {
		httperr.NotFound(w, rid, "audit report")
		return
	}
	if err != nil {
		h.Log.Error("get report", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	var issues []*model.AuditIssue
	if report.Status == "done" {
		issues, _ = h.Store.GetIssuesByReportID(r.Context(), report.ID)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"report": report,
			"issues": issues,
		},
	})
}

// POST /api/v1/audit/bidjobs/{id}/resolve — resolve an issue
func (h *Handlers) resolveIssue(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	bidJobID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid bid job id", nil)
		return
	}

	var req model.ResolveIssueRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	var userID uuid.UUID
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		userID = uuid.MustParse(uid)
	}

	if err := h.Store.ResolveIssue(r.Context(), req.IssueID, userID, req.Decision); err != nil {
		h.Log.Error("resolve issue", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	_ = bidJobID // unused but part of route
	writeJSON(w, http.StatusOK, map[string]any{"message": "issue resolved"})
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
