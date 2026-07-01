package service

import (
	"context"
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

// channelSenderFn is the per-channel dispatch function used by Send. It is
// looked up in the package-level channelSenders map (a test seam).
type channelSenderFn func(*model.SendRequest) error

// channelSenders is the dispatch table for Send. The default entries return
// the "not yet implemented" error for the corresponding channel. Tests can
// override individual entries (or replace the whole map) to inject success
// or failure responses without a live SMTP / webhook client.
var channelSenders = map[model.Channel]channelSenderFn{
	model.ChannelEmail:    defaultEmailSender,
	model.ChannelDingTalk: defaultDingTalkSender,
	model.ChannelWeCom:    defaultWeComSender,
}

// NotifyService handles sending notifications.
type NotifyService struct {
	store Store
	log   *slog.Logger
}

func NewNotifyService(s Store) *NotifyService {
	return &NotifyService{store: s}
}

// Send sends a notification to a user via the specified channel.
func (s *NotifyService) Send(ctx context.Context, req *model.SendRequest) error {
	tidStr, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	tid, _ := uuid.Parse(tidStr) // tenant context is UUID string

	// Log the notification
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

	// Send via the appropriate channel. The dispatch is table-driven through
	// channelSenders so tests can replace individual entries.
	var sendErr error
	if sender, ok := channelSenders[req.Channel]; ok {
		sendErr = sender(req)
	} else {
		sendErr = fmt.Errorf("unknown channel: %s", req.Channel)
	}

	// Update log status. sendErr may be nil on success, in which case the
	// error_message column stays empty.
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

// defaultEmailSender is the production email dispatcher. It currently
// returns "not yet implemented" and exists as a package-level function so it
// can be replaced in tests via channelSenders[model.ChannelEmail] = ...
func defaultEmailSender(req *model.SendRequest) error {
	// TODO: implement SMTP email sending
	return fmt.Errorf("email sending not yet implemented")
}

// defaultDingTalkSender is the production DingTalk dispatcher.
func defaultDingTalkSender(req *model.SendRequest) error {
	// TODO: implement DingTalk webhook
	return fmt.Errorf("dingtalk sending not yet implemented")
}

// defaultWeComSender is the production WeCom dispatcher.
func defaultWeComSender(req *model.SendRequest) error {
	// TODO: implement WeCom webhook
	return fmt.Errorf("wecom sending not yet implemented")
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
