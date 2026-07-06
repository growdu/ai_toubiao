package workers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/hibiken/asynq"
	"github.com/google/uuid"
)

// ============================================================================
// fakes
// ============================================================================

type fakeAuditTrigger struct {
	gotBidJob   uuid.UUID
	gotTenant   uuid.UUID
	status      string
	err         error
	calledTimes int
}

func (f *fakeAuditTrigger) TriggerSyncAudit(ctx context.Context, bidJobID, tenantID uuid.UUID) (string, error) {
	f.calledTimes++
	f.gotBidJob = bidJobID
	f.gotTenant = tenantID
	return f.status, f.err
}

type fakeAutoAdvanceEnqueuer struct {
	called       bool
	gotWorkflow  uuid.UUID
	gotBidJob    uuid.UUID
	gotFormat    string
	returnsErr   error
}

func (f *fakeAutoAdvanceEnqueuer) EnqueueExport(ctx context.Context, workflowID, bidJobID, tenantID uuid.UUID, format string, templateID uuid.UUID) error {
	f.called = true
	f.gotWorkflow = workflowID
	f.gotBidJob = bidJobID
	f.gotFormat = format
	return f.returnsErr
}

type fakeAutoTransitioner struct {
	called     bool
	gotFrom    model.State
	gotTo      model.State
	gotVersion int
	returnsErr error
}

func (f *fakeAutoTransitioner) Transition(ctx context.Context, workflowID uuid.UUID, fromState, toState model.State, expectedVersion int, actorID uuid.UUID) error {
	f.called = true
	f.gotFrom = fromState
	f.gotTo = toState
	f.gotVersion = expectedVersion
	return f.returnsErr
}

type fakeWorkflowReader struct {
	version int
	err     error
}

func (f *fakeWorkflowReader) GetWorkflow(ctx context.Context, workflowID uuid.UUID) (int, error) {
	return f.version, f.err
}

// taskFromPayload is a tiny helper to turn an AuditPayload into *asynq.Task.
func taskFromPayload(t *testing.T, payload AuditPayload) *asynq.Task {
	t.Helper()
	buf, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return asynq.NewTask(TaskAudit, buf)
}

func silentLogger() *slog.Logger { //nolint:unused // duplicate kept for readability in this file
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// ============================================================================
// Process — synchronous audit + auto-advance
// ============================================================================

func TestAuditorWorker_Process_SuccessAdvancesAndEnqueuesExport(t *testing.T) {
	wfID := uuid.New()
	bidID := uuid.New()
	tenantID := uuid.New()
	actor := uuid.New()

	trig := &fakeAuditTrigger{status: "done"}
	enq := &fakeAutoAdvanceEnqueuer{}
	tr := &fakeAutoTransitioner{}
	rdr := &fakeWorkflowReader{version: 11}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, rdr, enq, actor)

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: wfID, BidJobID: bidID, TenantID: tenantID,
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}

	if trig.calledTimes != 1 {
		t.Errorf("trigger should be called once, got %d", trig.calledTimes)
	}
	if trig.gotBidJob != bidID || trig.gotTenant != tenantID {
		t.Errorf("trigger got bid=%s tenant=%s, want %s/%s",
			trig.gotBidJob, trig.gotTenant, bidID, tenantID)
	}
	if !tr.called {
		t.Fatal("transitioner should have been called")
	}
	if tr.gotFrom != model.StateAuditing || tr.gotTo != model.StateExporting {
		t.Errorf("expected auditing->exporting, got %s->%s", tr.gotFrom, tr.gotTo)
	}
	if tr.gotVersion != 11 {
		t.Errorf("expected version=11, got %d", tr.gotVersion)
	}
	if !enq.called {
		t.Fatal("enqueuer should have been called")
	}
	if enq.gotWorkflow != wfID || enq.gotBidJob != bidID {
		t.Errorf("enqueue args mismatch: %+v", enq)
	}
	if enq.gotFormat != "word" {
		t.Errorf("expected format=word, got %q", enq.gotFormat)
	}
}

func TestAuditorWorker_Process_TriggerFailureStopsWithoutAdvance(t *testing.T) {
	trig := &fakeAuditTrigger{err: errors.New("audit-svc 500")}
	enq := &fakeAutoAdvanceEnqueuer{}
	tr := &fakeAutoTransitioner{}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, &fakeWorkflowReader{version: 1}, enq, uuid.New())

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process should not bubble trigger err: %v", err)
	}
	if tr.called {
		t.Error("transitioner must not be called when trigger fails")
	}
	if enq.called {
		t.Error("enqueuer must not be called when trigger fails")
	}
}

func TestAuditorWorker_Process_NonDoneStatusSkipsAdvance(t *testing.T) {
	trig := &fakeAuditTrigger{status: "failed"}
	tr := &fakeAutoTransitioner{}
	enq := &fakeAutoAdvanceEnqueuer{}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, &fakeWorkflowReader{version: 1}, enq, uuid.New())

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}
	if tr.called || enq.called {
		t.Error("auto-advance must be skipped when status != done")
	}
}

func TestAuditorWorker_Process_NoAutoAdvanceFallsBackToLegacyMode(t *testing.T) {
	trig := &fakeAuditTrigger{status: "done"}
	// All auto-advance deps nil → legacy mode (just log audit completion).
	w := NewAuditorWorker(silentLogger(), Config{}).WithTrigger(trig)

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}
	if trig.calledTimes != 1 {
		t.Errorf("trigger should still be called, got %d", trig.calledTimes)
	}
}

func TestAuditorWorker_Process_ReaderErrorStopsAfterTrigger(t *testing.T) {
	trig := &fakeAuditTrigger{status: "done"}
	tr := &fakeAutoTransitioner{}
	enq := &fakeAutoAdvanceEnqueuer{}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, &fakeWorkflowReader{err: errors.New("boom")}, enq, uuid.New())

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}
	if tr.called {
		t.Error("transitioner should not be called if reader errors")
	}
	if enq.called {
		t.Error("enqueuer should not be called if reader errors")
	}
}

func TestAuditorWorker_Process_TransitionerErrorStopsBeforeEnqueue(t *testing.T) {
	trig := &fakeAuditTrigger{status: "done"}
	tr := &fakeAutoTransitioner{returnsErr: ErrTransitionConflict}
	enq := &fakeAutoAdvanceEnqueuer{}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, &fakeWorkflowReader{version: 5}, enq, uuid.New())

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}
	if !tr.called {
		t.Fatal("transitioner should have been called")
	}
	if enq.called {
		t.Error("enqueuer must not be called when transition fails")
	}
}

func TestAuditorWorker_Process_EnqueueErrorDoesNotPanic(t *testing.T) {
	trig := &fakeAuditTrigger{status: "done"}
	tr := &fakeAutoTransitioner{}
	enq := &fakeAutoAdvanceEnqueuer{returnsErr: errors.New("redis down")}

	w := NewAuditorWorker(silentLogger(), Config{}).
		WithTrigger(trig).
		WithAutoAdvance(tr, &fakeWorkflowReader{version: 5}, enq, uuid.New())

	task := taskFromPayload(t, AuditPayload{
		WorkflowID: uuid.New(), BidJobID: uuid.New(), TenantID: uuid.New(),
	})
	if err := w.Process(context.Background(), task); err != nil {
		t.Fatalf("Process err: %v", err)
	}
	if !enq.called {
		t.Error("enqueuer should be called even if it errors")
	}
}

func TestAuditorWorker_Process_InvalidPayload(t *testing.T) {
	w := NewAuditorWorker(silentLogger(), Config{}).WithTrigger(&fakeAuditTrigger{status: "done"})
	bad := asynq.NewTask(TaskAudit, []byte("not json"))
	err := w.Process(context.Background(), bad)
	if err == nil {
		t.Fatal("expected unmarshal err")
	}
}

// ============================================================================
// TriggerSyncAudit — HTTP-level test with httptest
// ============================================================================

func TestAuditClient_TriggerSyncAudit_PostsAsyncFalseAndDecodesStatus(t *testing.T) {
	var gotAsyncHeader bool
	var gotAsyncContentType bool
	var gotPath string
	// Reading r.Body before WriteHeader triggers a chunked-encoding
	// deadlock: the http server won't flush response headers until the
	// handler returns, the http client won't send more request body
	// until the response headers arrive, and both sides block forever.
	// Two ways out:
	//   (a) declare a Content-Length up front so the client knows when
	//       to stop sending — what the previous attempt did. But the
	//       client here uses chunked transfer too, and chunked output
	//       from the server still won't be flushed until the handler
	//       returns.
	//   (b) Drain the body in a goroutine that writes into a channel,
	//       and respond immediately. The body read continues in the
	//       background; the assertion that uses it happens after
	//       server shutdown.
	// We use (b) because it matches the real-world pattern of services
	// that respond before fully consuming the request body (audit-svc
	// itself reads the body in a worker goroutine).
	bodyCh := make(chan string, 1)
	responseBody := []byte(`{"data":{"report":{"status":"done","passed":true},"issues":[]}}`)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAsyncContentType = strings.HasPrefix(r.Header.Get("Content-Type"), "application/json")
		gotAsyncHeader = r.Header.Get("X-Tenant-ID") != ""
		gotPath = r.URL.Path
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		// Drain body off the request goroutine so the response can flush.
		go func() {
			buf, _ := io.ReadAll(r.Body)
			bodyCh <- string(buf)
		}()
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Length", strconv.Itoa(len(responseBody)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(responseBody)
	}))
	defer srv.Close()
	// bodyCh is buffered; if the handler goroutine never gets to send
	// (e.g. because the client hung up), the channel is simply GC'd
	// along with the test function — no close needed, and closing
	// before the goroutine sends would panic.

	bidID := uuid.New()
	tenantID := uuid.New()
	c := NewAuditClient(srv.URL)
	status, err := c.TriggerSyncAudit(context.Background(), bidID, tenantID)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if status != "done" {
		t.Errorf("expected status=done, got %s", status)
	}
	if !gotAsyncContentType {
		t.Error("missing JSON content-type")
	}
	if !gotAsyncHeader {
		t.Error("missing X-Tenant-ID")
	}

	// Body assertions happen after the response was delivered, so the
	// background drain is guaranteed to have finished (or we wait briefly
	// for it). A select on bodyCh keeps the test bounded if the goroutine
	// hangs for any reason.
	var gotBody string
	select {
	case gotBody = <-bodyCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for request body drain")
	}
	if !strings.Contains(gotBody, `"async":false`) {
		t.Errorf("expected async:false in body, got %s", gotBody)
	}
	// Sanity check on routing: the URL the server actually saw should
	// contain the bid ID we sent (the previous version of this test used
	// uuid.New() in the suffix and never matched anything — a silent bug
	// that this assertion now pins).
	wantSuffix := "/api/v1/audit/bidjobs/" + bidID.String() + "/report"
	if !strings.HasSuffix(gotPath, wantSuffix) {
		t.Errorf("unexpected path: got %q, want suffix %q", gotPath, wantSuffix)
	}
}

func TestAuditClient_TriggerSyncAudit_HTTPErrorPropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	}))
	defer srv.Close()
	c := NewAuditClient(srv.URL)
	_, err := c.TriggerSyncAudit(context.Background(), uuid.New(), uuid.New())
	if err == nil {
		t.Fatal("expected err on HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("expected 500 in err, got %v", err)
	}
}
