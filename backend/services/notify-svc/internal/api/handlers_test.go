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
	"testing"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/bidwriter/services/notify-svc/internal/store"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// fakeService is a hand-rolled in-memory notifyService for handler tests.
type fakeService struct {
	sendFn             func(ctx context.Context, req *model.SendRequest) error
	createPreferenceFn func(ctx context.Context, userID uuid.UUID, req *model.CreatePreferenceRequest) (*model.NotificationPreference, error)
	listPreferencesFn  func(ctx context.Context) ([]*model.NotificationPreference, error)
	updatePreferenceFn func(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error
	deletePreferenceFn func(ctx context.Context, id uuid.UUID) error
}

func (f *fakeService) Send(ctx context.Context, req *model.SendRequest) error {
	return f.sendFn(ctx, req)
}
func (f *fakeService) CreatePreference(ctx context.Context, userID uuid.UUID, req *model.CreatePreferenceRequest) (*model.NotificationPreference, error) {
	return f.createPreferenceFn(ctx, userID, req)
}
func (f *fakeService) ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error) {
	return f.listPreferencesFn(ctx)
}
func (f *fakeService) UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error {
	return f.updatePreferenceFn(ctx, id, req)
}
func (f *fakeService) DeletePreference(ctx context.Context, id uuid.UUID) error {
	return f.deletePreferenceFn(ctx, id)
}

func newTestHandler(svc notifyService) *Handlers {
	return &Handlers{Service: svc, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
}

func ctxWithTenant() context.Context {
	return tenant.WithTenant(context.Background(), uuid.NewString())
}

func doRequest(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		if s, ok := body.(string); ok {
			rdr = bytes.NewBufferString(s)
		} else {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			rdr = bytes.NewReader(b)
		}
	}
	req := httptest.NewRequest(method, path, rdr).WithContext(ctxWithTenant())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func doRequestNoTenant(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

func doRequestWithHeaders(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		rdr = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, rdr).WithContext(ctxWithTenant())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	return w
}

// ---- send ----

func TestSend_Success_202(t *testing.T) {
	svc := &fakeService{
		sendFn: func(_ context.Context, req *model.SendRequest) error {
			if req.Channel != model.ChannelEmail {
				t.Errorf("channel = %s, want email", req.Channel)
			}
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/notifications/send",
		map[string]any{
			"type":    "bid_generated",
			"channel": "email",
			"user_id": uuid.NewString(),
			"subject": "hi",
			"body":    "there",
		})
	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("notification queued")) {
		t.Errorf("expected 'notification queued' in body, got %s", w.Body.String())
	}
}

func TestSend_NoTenant_401(t *testing.T) {
	svc := &fakeService{
		sendFn: func(context.Context, *model.SendRequest) error {
			t.Fatal("service should not be called when tenant is missing")
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequestNoTenant(t, h.Routes(), http.MethodPost, "/api/v1/notifications/send",
		map[string]any{"channel": "email", "body": "x"})
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("UNAUTHORIZED")) {
		t.Errorf("expected UNAUTHORIZED in body, got %s", w.Body.String())
	}
}

func TestSend_BadJSON_400(t *testing.T) {
	svc := &fakeService{
		sendFn: func(context.Context, *model.SendRequest) error {
			t.Fatal("service should not be called when JSON is malformed")
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/notifications/send", "{not-valid-json")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("INVALID_INPUT")) {
		t.Errorf("expected INVALID_INPUT in body, got %s", w.Body.String())
	}
}

func TestSend_ServiceError_500(t *testing.T) {
	svc := &fakeService{
		sendFn: func(context.Context, *model.SendRequest) error {
			return errors.New("send failed")
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/notifications/send",
		map[string]any{"channel": "email", "body": "x"})
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", w.Code, w.Body.String())
	}
	if !bytes.Contains(w.Body.Bytes(), []byte("INTERNAL_ERROR")) {
		t.Errorf("expected INTERNAL_ERROR in body, got %s", w.Body.String())
	}
}

// ---- listPreferences ----

func TestListPreferences_Success_200(t *testing.T) {
	want := []*model.NotificationPreference{
		{ID: uuid.New(), Channel: model.ChannelEmail, NotificationType: model.TypeBidGenerated},
		{ID: uuid.New(), Channel: model.ChannelDingTalk, NotificationType: model.TypeAuditCompleted},
	}
	svc := &fakeService{
		listPreferencesFn: func(context.Context) ([]*model.NotificationPreference, error) {
			return want, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/notifications/preferences", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data []*model.NotificationPreference `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Data) != 2 {
		t.Fatalf("len(data) = %d, want 2", len(got.Data))
	}
	if got.Data[0].ID != want[0].ID {
		t.Errorf("data[0].ID = %s, want %s", got.Data[0].ID, want[0].ID)
	}
}

func TestListPreferences_NoTenant_401(t *testing.T) {
	svc := &fakeService{
		listPreferencesFn: func(context.Context) ([]*model.NotificationPreference, error) {
			t.Fatal("service should not be called when tenant is missing")
			return nil, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequestNoTenant(t, h.Routes(), http.MethodGet, "/api/v1/notifications/preferences", nil)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestListPreferences_ServiceError_500(t *testing.T) {
	svc := &fakeService{
		listPreferencesFn: func(context.Context) ([]*model.NotificationPreference, error) {
			return nil, errors.New("db down")
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodGet, "/api/v1/notifications/preferences", nil)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// ---- createPreference ----

func TestCreatePreference_Success_201(t *testing.T) {
	userID := uuid.New()
	svc := &fakeService{
		createPreferenceFn: func(_ context.Context, uid uuid.UUID, req *model.CreatePreferenceRequest) (*model.NotificationPreference, error) {
			if uid != userID {
				t.Errorf("userID = %s, want %s", uid, userID)
			}
			if req.Channel != model.ChannelEmail {
				t.Errorf("channel = %s, want email", req.Channel)
			}
			return &model.NotificationPreference{
				ID:               uuid.New(),
				UserID:           uid,
				Channel:          req.Channel,
				NotificationType: req.NotificationType,
				Enabled:          req.Enabled,
				Address:          req.Address,
			}, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequestWithHeaders(t, h.Routes(), http.MethodPost, "/api/v1/notifications/preferences",
		map[string]any{
			"channel":           "email",
			"notification_type": "bid_generated",
			"enabled":           true,
			"address":           "user@example.com",
		},
		map[string]string{"X-User-ID": userID.String()},
	)
	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", w.Code, w.Body.String())
	}
	var got struct {
		Data *model.NotificationPreference `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Data == nil || got.Data.UserID != userID {
		t.Errorf("unexpected data: %+v", got.Data)
	}
}

func TestCreatePreference_NoTenant_401(t *testing.T) {
	svc := &fakeService{
		createPreferenceFn: func(context.Context, uuid.UUID, *model.CreatePreferenceRequest) (*model.NotificationPreference, error) {
			t.Fatal("service should not be called when tenant is missing")
			return nil, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequestNoTenant(t, h.Routes(), http.MethodPost, "/api/v1/notifications/preferences",
		map[string]any{"channel": "email", "address": "a"})
	if w.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", w.Code)
	}
}

func TestCreatePreference_BadJSON_400(t *testing.T) {
	svc := &fakeService{
		createPreferenceFn: func(context.Context, uuid.UUID, *model.CreatePreferenceRequest) (*model.NotificationPreference, error) {
			t.Fatal("service should not be called when JSON is malformed")
			return nil, nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPost, "/api/v1/notifications/preferences", "{bad")
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- updatePreference ----

func TestUpdatePreference_Success_200(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{
		updatePreferenceFn: func(_ context.Context, gotID uuid.UUID, req *model.UpdatePreferenceRequest) error {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			if !req.Enabled {
				t.Error("expected enabled=true to be propagated")
			}
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPatch, "/api/v1/notifications/preferences/"+id.String(),
		map[string]any{"enabled": true, "address": "new@example.com"})
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", w.Code, w.Body.String())
	}
}

func TestUpdatePreference_InvalidID_400(t *testing.T) {
	svc := &fakeService{
		updatePreferenceFn: func(context.Context, uuid.UUID, *model.UpdatePreferenceRequest) error {
			t.Fatal("service should not be called with invalid id")
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodPatch, "/api/v1/notifications/preferences/not-a-uuid",
		map[string]any{"enabled": true})
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- deletePreference ----

func TestDeletePreference_Success_204(t *testing.T) {
	id := uuid.New()
	svc := &fakeService{
		deletePreferenceFn: func(_ context.Context, gotID uuid.UUID) error {
			if gotID != id {
				t.Errorf("id = %s, want %s", gotID, id)
			}
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodDelete, "/api/v1/notifications/preferences/"+id.String(), nil)
	if w.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", w.Code)
	}
}

func TestDeletePreference_NotFound_404(t *testing.T) {
	svc := &fakeService{
		deletePreferenceFn: func(context.Context, uuid.UUID) error {
			return store.ErrNotFound
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodDelete, "/api/v1/notifications/preferences/"+uuid.NewString(), nil)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestDeletePreference_InvalidID_400(t *testing.T) {
	svc := &fakeService{
		deletePreferenceFn: func(context.Context, uuid.UUID) error {
			t.Fatal("service should not be called with invalid id")
			return nil
		},
	}
	h := newTestHandler(svc)
	w := doRequest(t, h.Routes(), http.MethodDelete, "/api/v1/notifications/preferences/not-a-uuid", nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", w.Code)
	}
}

// ---- health ----

func TestHealthz(t *testing.T) {
	h := newTestHandler(&fakeService{})
	w := doRequest(t, h.Routes(), http.MethodGet, "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !bytes.Contains(w.Body.Bytes(), []byte(`"ok"`)) {
		t.Errorf("body = %s", w.Body.String())
	}
}
