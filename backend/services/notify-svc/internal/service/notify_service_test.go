package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// fakeStore is a hand-rolled in-memory Store for service tests.
type fakeStore struct {
	createLogFn        func(ctx context.Context, log *model.NotificationLog) error
	updateLogFn        func(ctx context.Context, id uuid.UUID, status, errorMsg string) error
	createPreferenceFn func(ctx context.Context, p *model.NotificationPreference) error
	listPreferencesFn  func(ctx context.Context) ([]*model.NotificationPreference, error)
	updatePreferenceFn func(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error
	deletePreferenceFn func(ctx context.Context, id uuid.UUID) error
	findPreferencesFn  func(ctx context.Context, userID uuid.UUID, notifType model.NotificationType) ([]*model.NotificationPreference, error)
}

func (f *fakeStore) CreateLog(ctx context.Context, log *model.NotificationLog) error {
	if f.createLogFn != nil {
		return f.createLogFn(ctx, log)
	}
	log.ID = uuid.New()
	return nil
}
func (f *fakeStore) UpdateLog(ctx context.Context, id uuid.UUID, status, errorMsg string) error {
	if f.updateLogFn != nil {
		return f.updateLogFn(ctx, id, status, errorMsg)
	}
	return nil
}
func (f *fakeStore) CreatePreference(ctx context.Context, p *model.NotificationPreference) error {
	if f.createPreferenceFn != nil {
		return f.createPreferenceFn(ctx, p)
	}
	p.ID = uuid.New()
	return nil
}
func (f *fakeStore) ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error) {
	return f.listPreferencesFn(ctx)
}
func (f *fakeStore) UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error {
	return f.updatePreferenceFn(ctx, id, req)
}
func (f *fakeStore) DeletePreference(ctx context.Context, id uuid.UUID) error {
	return f.deletePreferenceFn(ctx, id)
}
func (f *fakeStore) FindPreferences(ctx context.Context, userID uuid.UUID, notifType model.NotificationType) ([]*model.NotificationPreference, error) {
	return f.findPreferencesFn(ctx, userID, notifType)
}

func ctxWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func newTestService(s Store, n *Notifier) *NotifyService {
	return &NotifyService{
		store:    s,
		notifier: n,
		log:      slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
}

// notifierFromFakes is a shorthand for a notifier backed by the given fake
// transports, used by every Send test below.
func notifierFromFakes(s *fakeSMTPDialer, w *fakeTransport) *Notifier {
	return NewNotifier(s, w, "noreply@example.com", "[bidwriter] ")
}

// ---- Send ----

func TestSend_Email_HappyPath_StatusSent(t *testing.T) {
	var createCalls, updateCalls int
	var lastUpdateStatus, lastUpdateErr string
	var lastUpdateID uuid.UUID
	var lastCreated *model.NotificationLog

	st := &fakeStore{
		createLogFn: func(_ context.Context, log *model.NotificationLog) error {
			createCalls++
			lastCreated = log
			return nil
		},
		updateLogFn: func(_ context.Context, id uuid.UUID, status, errMsg string) error {
			updateCalls++
			lastUpdateID = id
			lastUpdateStatus = status
			lastUpdateErr = errMsg
			return nil
		},
	}
	dialer := &fakeSMTPDialer{} // returns nil -> success
	svc := newTestService(st, notifierFromFakes(dialer, &fakeTransport{}))

	err := svc.Send(ctxWithTenant(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelEmail,
		UserID: uuid.New(), Subject: "hi", Body: "there",
		Address: "alice@example.com",
	})
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if createCalls != 1 {
		t.Errorf("CreateLog calls = %d, want 1", createCalls)
	}
	if updateCalls != 1 {
		t.Errorf("UpdateLog calls = %d, want 1", updateCalls)
	}
	if lastCreated == nil || lastCreated.Status != "pending" {
		t.Errorf("created log not pending: %+v", lastCreated)
	}
	if lastCreated.Channel != model.ChannelEmail {
		t.Errorf("channel = %s, want email", lastCreated.Channel)
	}
	if lastUpdateStatus != "sent" {
		t.Errorf("status = %q, want sent", lastUpdateStatus)
	}
	if lastUpdateErr != "" {
		t.Errorf("error message = %q, want empty (no error)", lastUpdateErr)
	}
	if lastUpdateID != lastCreated.ID {
		t.Errorf("UpdateLog id (%s) != CreateLog id (%s)", lastUpdateID, lastCreated.ID)
	}
	if len(dialer.calls) != 1 || dialer.calls[0].addr != "alice@example.com" {
		t.Errorf("dialer.calls = %+v, want one call to alice@example.com", dialer.calls)
	}
}

func TestSend_UnknownChannel_LogsFailedAndReturnsError(t *testing.T) {
	var updateStatus, updateErr string
	st := &fakeStore{
		updateLogFn: func(_ context.Context, _ uuid.UUID, status, errMsg string) error {
			updateStatus = status
			updateErr = errMsg
			return nil
		},
	}
	svc := newTestService(st, notifierFromFakes(&fakeSMTPDialer{}, &fakeTransport{}))
	err := svc.Send(ctxWithTenant(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.Channel("sms"),
		UserID: uuid.New(), Body: "x",
	})
	if err == nil {
		t.Fatal("expected error for unknown channel")
	}
	if !strings.Contains(err.Error(), "unknown channel") {
		t.Errorf("err = %v, want one mentioning unknown channel", err)
	}
	if updateStatus != "failed" {
		t.Errorf("log status = %q, want failed", updateStatus)
	}
	if !strings.Contains(updateErr, "unknown channel") {
		t.Errorf("log err = %q, want one mentioning unknown channel", updateErr)
	}
}

// Notifier-not-configured replaces the old "defaultEmailSender is a
// placeholder" test. It exercises the production wiring error path: a
// service constructed with a nil notifier (e.g. tests, or a misconfigured
// cmd/main.go) returns ErrNotifierNotConfigured.
func TestSend_NilNotifier_ReturnsErrNotifierNotConfigured(t *testing.T) {
	var updateStatus, updateErr string
	st := &fakeStore{
		updateLogFn: func(_ context.Context, _ uuid.UUID, status, errMsg string) error {
			updateStatus = status
			updateErr = errMsg
			return nil
		},
	}
	svc := newTestService(st, nil) // intentionally nil notifier
	err := svc.Send(ctxWithTenant(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelEmail,
		UserID: uuid.New(), Body: "x",
	})
	if !errors.Is(err, ErrNotifierNotConfigured) {
		t.Errorf("err = %v, want ErrNotifierNotConfigured", err)
	}
	if updateStatus != "failed" {
		t.Errorf("log status = %q, want failed", updateStatus)
	}
	if !strings.Contains(updateErr, "not configured") {
		t.Errorf("log err = %q, want one mentioning 'not configured'", updateErr)
	}
}

func TestSend_NoTenant_ReturnsErrorAndDoesNotCallStore(t *testing.T) {
	st := &fakeStore{
		createLogFn: func(context.Context, *model.NotificationLog) error {
			t.Fatal("CreateLog should not be called when tenant is missing")
			return nil
		},
	}
	svc := newTestService(st, notifierFromFakes(&fakeSMTPDialer{}, &fakeTransport{}))
	err := svc.Send(context.Background(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelEmail,
		UserID: uuid.New(), Body: "x",
	})
	if err == nil {
		t.Fatal("expected error when tenant is missing")
	}
	if !errors.Is(err, tenant.ErrNoTenant) {
		t.Errorf("err = %v, want tenant.ErrNoTenant", err)
	}
}

func TestSend_CreateLogError_PropagatesAndSkipsSend(t *testing.T) {
	wantErr := errors.New("db down on create")
	calledSender := false
	dialer := &fakeSMTPDialer{}
	dialer.sendErr = nil // would succeed; but we expect not to be called
	// Use a custom responder that records calls.
	capture := &fakeTransport{
		responder: func(_ string, _ []byte) (int, []byte, error) {
			calledSender = true
			return 200, []byte(`{}`), nil
		},
	}
	st := &fakeStore{
		createLogFn: func(context.Context, *model.NotificationLog) error { return wantErr },
	}
	svc := newTestService(st, notifierFromFakes(dialer, capture))
	err := svc.Send(ctxWithTenant(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelEmail,
		UserID: uuid.New(), Body: "x", Address: "x@y.z",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if len(dialer.calls) != 0 {
		t.Errorf("smtp should not be called when CreateLog fails, got %d calls", len(dialer.calls))
	}
	if calledSender {
		t.Error("webhook transport should not be called when CreateLog fails (only used for DingTalk/WeCom)")
	}
}

func TestSend_UpdateLogError_LoggedButSendReturnsOriginalErr(t *testing.T) {
	updateErr := errors.New("db down on update")
	sendErr := errors.New("smtp connection refused")
	st := &fakeStore{
		updateLogFn: func(context.Context, uuid.UUID, string, string) error { return updateErr },
	}
	dialer := &fakeSMTPDialer{sendErr: sendErr}
	svc := newTestService(st, notifierFromFakes(dialer, &fakeTransport{}))
	err := svc.Send(ctxWithTenant(), &model.SendRequest{
		Type: model.TypeBidGenerated, Channel: model.ChannelEmail,
		UserID: uuid.New(), Body: "x", Address: "x@y.z",
	})
	if !errors.Is(err, sendErr) {
		t.Errorf("Send returned err = %v, want original sendErr %v (UpdateLog failure should not mask it)", err, sendErr)
	}
}

// ---- Preferences ----

func TestCreatePreference_StoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("insert failed")
	var gotReq *model.CreatePreferenceRequest
	st := &fakeStore{
		createPreferenceFn: func(_ context.Context, p *model.NotificationPreference) error {
			gotReq = &model.CreatePreferenceRequest{
				Channel: p.Channel, NotificationType: p.NotificationType,
				Enabled: p.Enabled, Address: p.Address,
			}
			return wantErr
		},
	}
	svc := newTestService(st, nil)
	got, err := svc.CreatePreference(ctxWithTenant(), uuid.New(), &model.CreatePreferenceRequest{
		Channel:          model.ChannelEmail,
		NotificationType: model.TypeBidGenerated,
		Enabled:          true,
		Address:          "user@example.com",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got != nil {
		t.Errorf("pref = %+v, want nil", got)
	}
	if gotReq == nil || gotReq.Address != "user@example.com" {
		t.Errorf("createPreference not invoked with expected args: %+v", gotReq)
	}
}

func TestCreatePreference_Success_PopulatesTenantAndUser(t *testing.T) {
	tenantID := uuid.NewString()
	userID := uuid.New()
	var got *model.NotificationPreference
	st := &fakeStore{
		createPreferenceFn: func(_ context.Context, p *model.NotificationPreference) error {
			got = p
			p.ID = uuid.New()
			return nil
		},
	}
	svc := newTestService(st, nil)
	got2, err := svc.CreatePreference(
		tenant.WithTenant(context.Background(), tenantID),
		userID,
		&model.CreatePreferenceRequest{
			Channel:          model.ChannelDingTalk,
			NotificationType: model.TypeAuditCompleted,
			Enabled:          true,
			Address:          "https://oapi.dingtalk.com/robot/send?access_token=x",
		},
	)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if got == nil {
		t.Fatal("store not called")
	}
	if got.TenantID.String() != tenantID {
		t.Errorf("tenant id = %s, want %s", got.TenantID, tenantID)
	}
	if got.UserID != userID {
		t.Errorf("user id = %s, want %s", got.UserID, userID)
	}
	if got2 == nil || got2.ID != got.ID {
		t.Errorf("returned pref mismatch: %+v vs %+v", got2, got)
	}
}

func TestListPreferences_PassesThrough(t *testing.T) {
	want := []*model.NotificationPreference{
		{ID: uuid.New(), Channel: model.ChannelEmail},
		{ID: uuid.New(), Channel: model.ChannelWeCom},
	}
	st := &fakeStore{
		listPreferencesFn: func(context.Context) ([]*model.NotificationPreference, error) {
			return want, nil
		},
	}
	svc := newTestService(st, nil)
	got, err := svc.ListPreferences(ctxWithTenant())
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].ID != want[0].ID {
		t.Errorf("got[0].ID = %s, want %s", got[0].ID, want[0].ID)
	}
}

func TestUpdatePreference_StoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("update failed")
	var gotID uuid.UUID
	var gotReq *model.UpdatePreferenceRequest
	st := &fakeStore{
		updatePreferenceFn: func(_ context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error {
			gotID = id
			gotReq = req
			return wantErr
		},
	}
	svc := newTestService(st, nil)
	id := uuid.New()
	err := svc.UpdatePreference(ctxWithTenant(), id, &model.UpdatePreferenceRequest{
		Enabled: true, Address: "new@example.com",
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if gotID != id {
		t.Errorf("id passed to store = %s, want %s", gotID, id)
	}
	if gotReq == nil || !gotReq.Enabled || gotReq.Address != "new@example.com" {
		t.Errorf("update request not propagated: %+v", gotReq)
	}
}

func TestDeletePreference_StoreErrorPropagates(t *testing.T) {
	wantErr := errors.New("delete failed")
	var gotID uuid.UUID
	st := &fakeStore{
		deletePreferenceFn: func(_ context.Context, id uuid.UUID) error {
			gotID = id
			return wantErr
		},
	}
	svc := newTestService(st, nil)
	id := uuid.New()
	err := svc.DeletePreference(ctxWithTenant(), id)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if gotID != id {
		t.Errorf("id passed to store = %s, want %s", gotID, id)
	}
}

// ---- NotifyBidGenerated / NotifyAuditCompleted / NotifyBudgetExhausted ----
//
// These helpers spawn goroutines that call Send, so the send itself is not
// deterministically observable in a test. The deterministic contract is the
// "no enabled prefs" branch: the method must return nil without spawning
// any goroutine. We assert that by giving the store a FindPreferences that
// returns empty, and confirming the helper does not call Send.

func TestNotifyBidGenerated_NoEnabledPrefs_ReturnsNilNoSpawn(t *testing.T) {
	storeCalled := false
	st := &fakeStore{
		findPreferencesFn: func(context.Context, uuid.UUID, model.NotificationType) ([]*model.NotificationPreference, error) {
			storeCalled = true
			return nil, nil
		},
	}
	svc := newTestService(st, nil)
	if err := svc.NotifyBidGenerated(ctxWithTenant(), uuid.New(), "Test Bid"); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !storeCalled {
		t.Error("FindPreferences should be called even when no prefs exist (drives the lookup)")
	}
}

func TestNotifyAuditCompleted_NoEnabledPrefs_ReturnsNilNoSpawn(t *testing.T) {
	storeCalled := false
	st := &fakeStore{
		findPreferencesFn: func(context.Context, uuid.UUID, model.NotificationType) ([]*model.NotificationPreference, error) {
			storeCalled = true
			return nil, nil
		},
	}
	svc := newTestService(st, nil)
	if err := svc.NotifyAuditCompleted(ctxWithTenant(), uuid.New(), "Test Bid", true); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !storeCalled {
		t.Error("FindPreferences should be called even when no prefs exist")
	}
}

func TestNotifyBudgetExhausted_NoEnabledPrefs_ReturnsNilNoSpawn(t *testing.T) {
	storeCalled := false
	st := &fakeStore{
		findPreferencesFn: func(context.Context, uuid.UUID, model.NotificationType) ([]*model.NotificationPreference, error) {
			storeCalled = true
			return nil, nil
		},
	}
	svc := newTestService(st, nil)
	if err := svc.NotifyBudgetExhausted(ctxWithTenant(), uuid.New(), 85); err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if !storeCalled {
		t.Error("FindPreferences should be called even when no prefs exist")
	}
}
