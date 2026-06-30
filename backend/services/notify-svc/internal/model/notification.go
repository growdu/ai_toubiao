package model

import (
	"time"

	"github.com/google/uuid"
)

// NotificationType is the event type that triggers a notification.
type NotificationType string

const (
	TypeBidGenerated     NotificationType = "bid_generated"
	TypeAuditCompleted   NotificationType = "audit_completed"
	TypeAuditFailed      NotificationType = "audit_failed"
	TypeBudgetExhausted  NotificationType = "budget_exhausted"
	TypeChapterCompleted NotificationType = "chapter_completed"
)

// Channel is the delivery channel.
type Channel string

const (
	ChannelEmail    Channel = "email"
	ChannelDingTalk Channel = "dingtalk"
	ChannelWeCom    Channel = "wecom"
)

// NotificationPreference stores per-tenant channel settings.
type NotificationPreference struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	Channel       Channel    `json:"channel" db:"channel"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	Enabled       bool       `json:"enabled" db:"enabled"`
	Address       string     `json:"address" db:"address"` // email address or webhook URL
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// NotificationTemplate stores message templates.
type NotificationTemplate struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	Channel       Channel    `json:"channel" db:"channel"`
	Subject       string     `json:"subject" db:"subject"`
	BodyTemplate  string     `json:"body_template" db:"body_template"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at" db:"updated_at"`
}

// NotificationLog records sent notifications.
type NotificationLog struct {
	ID            uuid.UUID  `json:"id" db:"id"`
	TenantID      uuid.UUID  `json:"tenant_id" db:"tenant_id"`
	UserID        uuid.UUID  `json:"user_id" db:"user_id"`
	Channel       Channel    `json:"channel" db:"channel"`
	NotificationType NotificationType `json:"notification_type" db:"notification_type"`
	Subject       string     `json:"subject" db:"subject"`
	Body          string     `json:"body" db:"body"`
	Status        string     `json:"status" db:"status"` // "pending", "sent", "failed"
	ErrorMessage  string     `json:"error_message,omitempty" db:"error_message"`
	SentAt        *time.Time `json:"sent_at,omitempty" db:"sent_at"`
	CreatedAt     time.Time  `json:"created_at" db:"created_at"`
}

// SendRequest for triggering a notification.
type SendRequest struct {
	Type      NotificationType `json:"type" validate:"required"`
	Channel   Channel          `json:"channel" validate:"required"`
	UserID    uuid.UUID       `json:"user_id" validate:"required"`
	Subject   string          `json:"subject"`
	Body      string          `json:"body" validate:"required"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// UpdatePreferenceRequest for updating channel preferences.
type UpdatePreferenceRequest struct {
	Enabled bool   `json:"enabled"`
	Address string `json:"address,omitempty"`
}

// CreatePreferenceRequest for creating a new preference.
type CreatePreferenceRequest struct {
	Channel         Channel               `json:"channel" validate:"required"`
	NotificationType NotificationType     `json:"notification_type" validate:"required"`
	Enabled         bool                  `json:"enabled"`
	Address         string                `json:"address,omitempty" validate:"required"`
}
