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
	"sync"
	"testing"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------------

type fakeStore struct {
	mu sync.Mutex

	createReportFn   func(ctx context.Context, r *model.AuditReport) error
	getReportFn      func(ctx context.Context, bidJobID uuid.UUID) (*model.AuditReport, error)
	updateReportFn   func(ctx context.Context, r *model.AuditReport) error
	batchInsertFn    func(ctx context.Context, issues []*model.AuditIssue) error
	getIssuesFn      func(ctx context.Context, reportID uuid.UUID) ([]*model.AuditIssue, error)
	resolveIssueFn   func(ctx context.Context, issueID, userID uuid.UUID, decision string) error
	getBidJobFn      func(ctx context.Context, bidJobID uuid.UUID) (*store.BidJobWithChapters, error)

	created     *model.AuditReport
	updated     *model.AuditReport
	resolvedID  uuid.UUID
	resolvedBy  uuid.UUID
	resolvedDec string
}

func (f *fakeStore) CreateReport(ctx context.Context, r *model.AuditReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.created = r
	return f.createReportFn(ctx, r)
}
func (f *fakeStore) GetReport(ctx context.Context, bidJobID uuid.UUID) (*model.AuditReport, error) {
	return f.getReportFn(ctx, bidJobID)
}
func (f *fakeStore) UpdateReport(ctx context.Context, r *model.AuditReport) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.updated = r
	return f.updateReportFn(ctx, r)
}
func (f *fakeStore) BatchInsertIssues(ctx context.Context, issues []*model.AuditIssue) error {
	return f.batchInsertFn(ctx, issues)
}
func (f *fakeStore) GetIssuesByReportID(ctx context.Context, reportID uuid.UUID) ([]*model.AuditIssue, error) {
	return f.getIssuesFn(ctx, reportID)
}
func (f *fakeStore) ResolveIssue(ctx context.Context, issueID, userID uuid.UUID, decision string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolvedID, f.resolvedBy, f.resolvedDec = issueID, userID, decision
	return f.resolveIssueFn(ctx, issueID, userID, decision)
}
func (f *fakeStore) GetBidJobWithChapters(ctx context.Context, bidJobID uuid.UUID) (*store.BidJobWithChapters, error) {
	return f.getBidJobFn(ctx, bidJobID)
}

type fakeAuditor struct {
	fn func(ctx context.Context, bidJobID, tenantID uuid.UUID, ch *store.ChapterInfo) []*model.AuditIssue
}

func (a *fakeAuditor) AuditChapter(ctx context.Context, bidJobID, tenantID uuid.UUID, ch *store.ChapterInfo) []*model.AuditIssue {
	return a.fn(ctx, bidJobID, tenantID, ch)
}

type fakeCrossAuditor struct {
	fn func(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue
}

func (a *fakeCrossAuditor) AuditCrossChapter(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue {
	return a.fn(ctx, bidJobID, tenantID, chapters)
}

type fakeRejectionChecker struct {
	fn func(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue
}

func (r *fakeRejectionChecker) CheckRejectionCriteria(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue {
	return r.fn(ctx, bidJobID, tenantID, chapters)
}

// ---------------------------------------------------------------------------
// Test rig
// ---------------------------------------------------------------------------

type auditRig struct {
	store    *fakeStore
	chapter  *fakeAuditor
	cross    *fakeCrossAuditor
	reject   *fakeRejectionChecker
	h        *Handlers
}

func newAuditRig() *auditRig {
	bidJobID := uuid.New()
	tenantID := uuid.New()
	fs := &fakeStore{
		createReportFn: func(context.Context, *model.AuditReport) error { return nil },
		getReportFn: func(context.Context, uuid.UUID) (*model.AuditReport, error) {
			return nil, store.ErrNotFound
		},
		updateReportFn:   func(context.Context, *model.AuditReport) error { return nil },
		batchInsertFn:    func(context.Context, []*model.AuditIssue) error { return nil },
		getIssuesFn:      func(context.Context, uuid.UUID) ([]*model.AuditIssue, error) { return nil, nil },
		resolveIssueFn:   func(context.Context, uuid.UUID, uuid.UUID, string) error { return nil },
		getBidJobFn: func(_ context.Context, _ uuid.UUID) (*store.BidJobWithChapters, error) {
			hello := "hello"
			return &store.BidJobWithChapters{
				BidJob: store.BidJobInfo{ID: bidJobID, TenantID: tenantID},
				Chapters: []store.ChapterInfo{{ID: uuid.New(), Title: "ch1", Content: &hello}},
			}, nil
		},
	}
	fa := &fakeAuditor{fn: func(context.Context, uuid.UUID, uuid.UUID, *store.ChapterInfo) []*model.AuditIssue { return nil }}
	fc := &fakeCrossAuditor{fn: func(context.Context, uuid.UUID, uuid.UUID, []store.ChapterInfo) []*model.AuditIssue { return nil }}
	fr := &fakeRejectionChecker{fn: func(context.Context, uuid.UUID, uuid.UUID, []store.ChapterInfo) []*model.AuditIssue { return nil }}
	h := &Handlers{
		Store:            fs,
		ChapterAuditor:   fa,
		CrossAuditor:     fc,
		RejectionChecker: fr,
		Log:              slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return &auditRig{store: fs, chapter: fa, cross: fc, reject: fr, h: h}
}

func (r *auditRig) do(method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var rdr io.Reader
	if body != nil {
		switch v := body.(type) {
		case string:
			rdr = bytes.NewBufferString(v)
		case []byte:
			rdr = bytes.NewReader(v)
		default:
			b, _ := json.Marshal(body)
			rdr = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.h.Routes().ServeHTTP(w, req)
	return w
}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

func TestAuditHealthz(t *testing.T) {
	if w := newAuditRig().do(http.MethodGet, "/healthz", nil, nil); w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
}

// ---------------------------------------------------------------------------
// triggerReport (POST /api/v1/audit/bidjobs/{id}/report)
// ---------------------------------------------------------------------------

func TestTrigger_InvalidBidJobID(t *testing.T) {
	w := newAuditRig().do(http.MethodPost, "/api/v1/audit/bidjobs/not-a-uuid/report",
		map[string]any{"async": false}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestTrigger_AsyncReturns202(t *testing.T) {
	w := newAuditRig().do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report",
		map[string]any{"async": true}, nil)
	if w.Code != http.StatusAccepted {
		t.Errorf("status = %d, want 202", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("audit started in background")) {
		t.Errorf("body missing background message: %s", w.Body.String())
	}
}

func TestTrigger_SyncRunsAuditAndCountsCritical(t *testing.T) {
	r := newAuditRig()
	// Inject two critical issues from the chapter auditor.
	r.chapter.fn = func(_ context.Context, _, _ uuid.UUID, _ *store.ChapterInfo) []*model.AuditIssue {
		return []*model.AuditIssue{
			{Severity: model.SeverityCritical},
			{Severity: model.SeverityCritical},
		}
	}
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report",
		map[string]any{"async": false}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if r.store.updated == nil {
		t.Fatal("UpdateReport was not called")
	}
	if r.store.updated.Critical != 2 {
		t.Errorf("critical count = %d, want 2", r.store.updated.Critical)
	}
	if r.store.updated.Passed {
		t.Errorf("Passed should be false when critical issues exist")
	}
}

func TestTrigger_SyncRunsAuditCountsMajorAndMinor(t *testing.T) {
	r := newAuditRig()
	r.cross.fn = func(context.Context, uuid.UUID, uuid.UUID, []store.ChapterInfo) []*model.AuditIssue {
		return []*model.AuditIssue{
			{Severity: model.SeverityMajor},
			{Severity: model.SeverityMajor},
			{Severity: model.SeverityMinor},
		}
	}
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report",
		map[string]any{"async": false}, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if r.store.updated.Major != 2 || r.store.updated.Minor != 1 {
		t.Errorf("major=%d minor=%d, want 2/1", r.store.updated.Major, r.store.updated.Minor)
	}
	if !r.store.updated.Passed {
		t.Errorf("Passed should be true when no critical issues")
	}
}

func TestTrigger_BidJobNotFoundReturns500(t *testing.T) {
	r := newAuditRig()
	r.store.getBidJobFn = func(context.Context, uuid.UUID) (*store.BidJobWithChapters, error) {
		return nil, store.ErrNotFound
	}
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report",
		map[string]any{"async": false}, nil)
	// runAudit surfaces the error; handler maps to 500.
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestTrigger_StoreUpdateErrorStillReturnsResult(t *testing.T) {
	r := newAuditRig()
	r.store.updateReportFn = func(context.Context, *model.AuditReport) error { return errors.New("db gone") }
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report",
		map[string]any{"async": false}, nil)
	// runAudit logs the update error but returns the result; handler returns 200.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (result still returned)", w.Code)
	}
}

// ---------------------------------------------------------------------------
// getReport (GET /api/v1/audit/bidjobs/{id}/report)
// ---------------------------------------------------------------------------

func TestGetReport_NotFound(t *testing.T) {
	w := newAuditRig().do(http.MethodGet, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report", nil, nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestGetReport_DoneIncludesIssues(t *testing.T) {
	r := newAuditRig()
	reportID := uuid.New()
	r.store.getReportFn = func(context.Context, uuid.UUID) (*model.AuditReport, error) {
		return &model.AuditReport{ID: reportID, Status: "done", TotalIssues: 1}, nil
	}
	r.store.getIssuesFn = func(context.Context, uuid.UUID) ([]*model.AuditIssue, error) {
		return []*model.AuditIssue{{ID: uuid.New(), Severity: model.SeverityCritical}}, nil
	}
	w := r.do(http.MethodGet, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"severity":"critical"`)) {
		t.Errorf("body should contain severity, got %s", w.Body.String())
	}
}

func TestGetReport_RunningExcludesIssues(t *testing.T) {
	r := newAuditRig()
	r.store.getReportFn = func(context.Context, uuid.UUID) (*model.AuditReport, error) {
		return &model.AuditReport{ID: uuid.New(), Status: "running"}, nil
	}
	called := false
	r.store.getIssuesFn = func(context.Context, uuid.UUID) ([]*model.AuditIssue, error) {
		called = true
		return nil, nil
	}
	w := r.do(http.MethodGet, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/report", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if called {
		t.Errorf("GetIssuesByReportID should not be called for non-done report")
	}
}

func TestGetReport_InvalidID(t *testing.T) {
	w := newAuditRig().do(http.MethodGet, "/api/v1/audit/bidjobs/not-a-uuid/report", nil, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---------------------------------------------------------------------------
// resolveIssue (POST /api/v1/audit/bidjobs/{id}/resolve)
// ---------------------------------------------------------------------------

func TestResolve_InvalidJSON(t *testing.T) {
	w := newAuditRig().do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/resolve",
		"not-json", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

func TestResolve_Success(t *testing.T) {
	r := newAuditRig()
	issueID := uuid.New()
	userID := uuid.New()
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/resolve",
		map[string]any{"issue_id": issueID.String(), "decision": "fixed"},
		map[string]string{"X-User-ID": userID.String()})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	if r.store.resolvedID != issueID {
		t.Errorf("resolvedID = %s, want %s", r.store.resolvedID, issueID)
	}
	if r.store.resolvedBy != userID {
		t.Errorf("resolvedBy = %s, want %s", r.store.resolvedBy, userID)
	}
	if r.store.resolvedDec != "fixed" {
		t.Errorf("decision = %q, want \"fixed\"", r.store.resolvedDec)
	}
}

func TestResolve_StoreErrorReturns500(t *testing.T) {
	r := newAuditRig()
	r.store.resolveIssueFn = func(context.Context, uuid.UUID, uuid.UUID, string) error {
		return errors.New("db gone")
	}
	w := r.do(http.MethodPost, "/api/v1/audit/bidjobs/"+uuid.NewString()+"/resolve",
		map[string]any{"issue_id": uuid.NewString(), "decision": "wontfix"}, nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

func TestResolve_InvalidID(t *testing.T) {
	w := newAuditRig().do(http.MethodPost, "/api/v1/audit/bidjobs/not-a-uuid/resolve",
		map[string]any{"issue_id": uuid.NewString()}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}