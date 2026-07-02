package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

// Store is the storage contract required by NotifyService. Defined at the
// consumer (service package) so the service can be unit-tested with a fake
// without a live PG. The concrete *store.Store satisfies this interface
// naturally.
type Store interface {
	CreateLog(ctx context.Context, log *model.NotificationLog) error
	UpdateLog(ctx context.Context, id uuid.UUID, status, errorMsg string) error
	CreatePreference(ctx context.Context, p *model.NotificationPreference) error
	ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error)
	UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error
	DeletePreference(ctx context.Context, id uuid.UUID) error
	FindPreferences(ctx context.Context, userID uuid.UUID, notifType model.NotificationType) ([]*model.NotificationPreference, error)
}

// NotifyService routes notification requests through a Notifier (one
// method per delivery channel) and records every attempt in the Store
// log. The Notifier is a required dependency — production wires it
// with real SMTP / webhook transports; tests inject fakes.
type NotifyService struct {
	store    Store
	log      *slog.Logger
	notifier *Notifier
}

// NewNotifyService wires the store + notifier. A nil notifier is allowed
// (so service-level tests can exercise the store path without a channel
// transport); Send returns ErrNotifierNotConfigured in that case.
func NewNotifyService(s Store, n *Notifier) *NotifyService {
	return &NotifyService{store: s, notifier: n}
}

// ErrNotifierNotConfigured is returned by Send when the service was
// constructed without a Notifier. Callers (the HTTP handler) treat it
// as a 500 since it's a wiring error, not a user error.
var ErrNotifierNotConfigured = errors.New("notify-svc: notifier not configured")

// Send persists the notification log, dispatches via the Notifier, and
// updates the log with the outcome. The dispatch is method-dispatched
// (Email/DingTalk/WeCom) rather than table-driven — easier to follow
// than a map of closures, and the dispatch logic is trivial enough
// that a switch is the right amount of indirection.
func (s *NotifyService) Send(ctx context.Context, req *model.SendRequest) error {
	tidStr, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	tid, _ := uuid.Parse(tidStr)

	nlog := &model.NotificationLog{
		TenantID:         tid,
		UserID:           req.UserID,
		Channel:          req.Channel,
		NotificationType: req.Type,
		Subject:          req.Subject,
		Body:             req.Body,
		Status:           "pending",
	}
	if err := s.store.CreateLog(ctx, nlog); err != nil {
		return err
	}

	var sendErr error
	switch req.Channel {
	case model.ChannelEmail:
		if s.notifier == nil {
			sendErr = ErrNotifierNotConfigured
		} else {
			sendErr = s.notifier.Email(ctx, req)
		}
	case model.ChannelDingTalk:
		if s.notifier == nil {
			sendErr = ErrNotifierNotConfigured
		} else {
			sendErr = s.notifier.DingTalk(ctx, req)
		}
	case model.ChannelWeCom:
		if s.notifier == nil {
			sendErr = ErrNotifierNotConfigured
		} else {
			sendErr = s.notifier.WeCom(ctx, req)
		}
	default:
		sendErr = fmt.Errorf("unknown channel: %s", req.Channel)
	}

	status := "sent"
	var errMsg string
	if sendErr != nil {
		status = "failed"
		errMsg = sendErr.Error()
	}
	if err := s.store.UpdateLog(ctx, nlog.ID, status, errMsg); err != nil {
		s.log.Error("failed to update notification log", slog.String("err", err.Error()))
	}
	return sendErr
}

// CreatePreference creates a notification preference.
func (s *NotifyService) CreatePreference(ctx context.Context, userID uuid.UUID, req *model.CreatePreferenceRequest) (*model.NotificationPreference, error) {
	tidStr, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	tid, _ := uuid.Parse(tidStr)
	p := &model.NotificationPreference{
		TenantID:         tid,
		UserID:           userID,
		Channel:          req.Channel,
		NotificationType: req.NotificationType,
		Enabled:          req.Enabled,
		Address:          req.Address,
	}
	if err := s.store.CreatePreference(ctx, p); err != nil {
		return nil, err
	}
	return p, nil
}

// ListPreferences returns all preferences for the current tenant.
func (s *NotifyService) ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error) {
	return s.store.ListPreferences(ctx)
}

// UpdatePreference updates a preference.
func (s *NotifyService) UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error {
	return s.store.UpdatePreference(ctx, id, req)
}

// DeletePreference removes a preference.
func (s *NotifyService) DeletePreference(ctx context.Context, id uuid.UUID) error {
	return s.store.DeletePreference(ctx, id)
}

// NotifyBidGenerated is a convenience method for the bid.generated event.
// It looks up enabled preferences for the user and dispatches a Send for
// each in a background goroutine. The method itself does not surface
// send errors to the caller; persistent failures land in the notification
// log.
func (s *NotifyService) NotifyBidGenerated(ctx context.Context, userID uuid.UUID, bidName string) error {
	prefs, _ := s.store.FindPreferences(ctx, userID, model.TypeBidGenerated)
	for _, p := range prefs {
		if p.Enabled {
			go func(p *model.NotificationPreference) {
				_ = s.Send(context.Background(), &model.SendRequest{
					Type:    model.TypeBidGenerated,
					Channel: p.Channel,
					UserID:  userID,
					Subject: "标书生成完成",
					Body:    fmt.Sprintf("标书「%s」已生成完成，可以开始审计。", bidName),
				})
			}(p)
		}
	}
	return nil
}

// NotifyAuditCompleted notifies about audit completion.
func (s *NotifyService) NotifyAuditCompleted(ctx context.Context, userID uuid.UUID, bidName string, passed bool) error {
	prefs, _ := s.store.FindPreferences(ctx, userID, model.TypeAuditCompleted)
	status := "通过"
	if !passed {
		status = "有严重问题"
	}
	for _, p := range prefs {
		if p.Enabled {
			go func(p *model.NotificationPreference) {
				_ = s.Send(context.Background(), &model.SendRequest{
					Type:    model.TypeAuditCompleted,
					Channel: p.Channel,
					UserID:  userID,
					Subject: "审计完成",
					Body:    fmt.Sprintf("标书「%s」审计完成，结果：%s", bidName, status),
				})
			}(p)
		}
	}
	return nil
}

// NotifyBudgetExhausted notifies when budget is nearly exhausted.
func (s *NotifyService) NotifyBudgetExhausted(ctx context.Context, userID uuid.UUID, percent float64) error {
	prefs, _ := s.store.FindPreferences(ctx, userID, model.TypeBudgetExhausted)
	for _, p := range prefs {
		if p.Enabled {
			go func(p *model.NotificationPreference) {
				_ = s.Send(context.Background(), &model.SendRequest{
					Type:    model.TypeBudgetExhausted,
					Channel: p.Channel,
					UserID:  userID,
					Subject: "预算告警",
					Body:    fmt.Sprintf("您的预算已使用 %.0f%%，请注意控制消费。", percent),
				})
			}(p)
		}
	}
	return nil
}
