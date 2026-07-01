package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/bidwriter/services/notify-svc/internal/store"
	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// notifyService is the service contract required by Handlers. Defined at the
// consumer (api package) so handlers can be unit-tested with a fake. The
// concrete *service.NotifyService satisfies this interface naturally.
type notifyService interface {
	Send(ctx context.Context, req *model.SendRequest) error
	CreatePreference(ctx context.Context, userID uuid.UUID, req *model.CreatePreferenceRequest) (*model.NotificationPreference, error)
	ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error)
	UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error
	DeletePreference(ctx context.Context, id uuid.UUID) error
}

type Handlers struct {
	Service notifyService
	Log     *slog.Logger
}

func (h *Handlers) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ready"}`))
	})

	r.Route("/api/v1/notifications", func(r chi.Router) {
		r.Post("/send", h.send)
		r.Get("/preferences", h.listPreferences)
		r.Post("/preferences", h.createPreference)
		r.Patch("/preferences/{id}", h.updatePreference)
		r.Delete("/preferences/{id}", h.deletePreference)
	})

	return r
}

// POST /api/v1/notifications/send
func (h *Handlers) send(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	var req model.SendRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	if err := h.Service.Send(r.Context(), &req); err != nil {
		h.Log.Error("send notification", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"message": "notification queued"})
}

// GET /api/v1/notifications/preferences
func (h *Handlers) listPreferences(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	prefs, err := h.Service.ListPreferences(r.Context())
	if err != nil {
		h.Log.Error("list preferences", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": prefs})
}

// POST /api/v1/notifications/preferences
func (h *Handlers) createPreference(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	if _, err := tenant.FromContext(r.Context()); err != nil {
		httperr.Unauthorized(w, rid)
		return
	}

	var req model.CreatePreferenceRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	var userID uuid.UUID
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		userID = uuid.MustParse(uid)
	}

	pref, err := h.Service.CreatePreference(r.Context(), userID, &req)
	if err != nil {
		h.Log.Error("create preference", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"data": pref})
}

// PATCH /api/v1/notifications/preferences/{id}
func (h *Handlers) updatePreference(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	var req model.UpdatePreferenceRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	if err := h.Service.UpdatePreference(r.Context(), id, &req); err != nil {
		h.Log.Error("update preference", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"message": "preference updated"})
}

// DELETE /api/v1/notifications/preferences/{id}
func (h *Handlers) deletePreference(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	if err := h.Service.DeletePreference(r.Context(), id); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			httperr.NotFound(w, rid, "preference")
			return
		}
		h.Log.Error("delete preference", slog.String("err", err.Error()))
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
