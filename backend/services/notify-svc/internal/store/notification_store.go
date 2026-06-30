package store

import (
	"context"
	"errors"
	"time"

	"github.com/bidwriter/services/notify-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreatePreference creates a new notification preference.
func (s *Store) CreatePreference(ctx context.Context, p *model.NotificationPreference) error {
	p.ID = uuid.New()
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification_preferences (id, tenant_id, user_id, channel, notification_type, enabled, address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, p.ID, p.TenantID, p.UserID, p.Channel, p.NotificationType, p.Enabled, p.Address, p.CreatedAt, p.UpdatedAt)
	return err
}

// GetPreference retrieves a preference by ID.
func (s *Store) GetPreference(ctx context.Context, id uuid.UUID) (*model.NotificationPreference, error) {
	var p model.NotificationPreference
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, user_id, channel, notification_type, enabled, address, created_at, updated_at
		FROM notification_preferences WHERE id = $1
	`, id).Scan(&p.ID, &p.TenantID, &p.UserID, &p.Channel, &p.NotificationType, &p.Enabled, &p.Address, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

// ListPreferences returns all preferences for a tenant.
func (s *Store) ListPreferences(ctx context.Context) ([]*model.NotificationPreference, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, channel, notification_type, enabled, address, created_at, updated_at
		FROM notification_preferences WHERE tenant_id = $1 ORDER BY created_at DESC
	`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []*model.NotificationPreference
	for rows.Next() {
		var p model.NotificationPreference
		if err := rows.Scan(&p.ID, &p.TenantID, &p.UserID, &p.Channel, &p.NotificationType, &p.Enabled, &p.Address, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, &p)
	}
	return prefs, rows.Err()
}

// UpdatePreference updates a preference.
func (s *Store) UpdatePreference(ctx context.Context, id uuid.UUID, req *model.UpdatePreferenceRequest) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_preferences SET enabled=$1, address=$2, updated_at=NOW()
		WHERE id=$3
	`, req.Enabled, req.Address, id)
	return err
}

// DeletePreference removes a preference.
func (s *Store) DeletePreference(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM notification_preferences WHERE id = $1`, id)
	return err
}

// FindPreferences finds enabled preferences for a user and notification type.
func (s *Store) FindPreferences(ctx context.Context, userID uuid.UUID, notifType model.NotificationType) ([]*model.NotificationPreference, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, user_id, channel, notification_type, enabled, address, created_at, updated_at
		FROM notification_preferences
		WHERE tenant_id = $1 AND user_id = $2 AND notification_type = $3 AND enabled = true
	`, tid, userID, notifType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var prefs []*model.NotificationPreference
	for rows.Next() {
		var p model.NotificationPreference
		if err := rows.Scan(&p.ID, &p.TenantID, &p.UserID, &p.Channel, &p.NotificationType, &p.Enabled, &p.Address, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		prefs = append(prefs, &p)
	}
	return prefs, rows.Err()
}

// CreateLog creates a notification log entry.
func (s *Store) CreateLog(ctx context.Context, log *model.NotificationLog) error {
	log.ID = uuid.New()
	log.CreatedAt = time.Now()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO notification_logs (id, tenant_id, user_id, channel, notification_type, subject, body, status, error_message, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, log.ID, log.TenantID, log.UserID, log.Channel, log.NotificationType, log.Subject, log.Body, log.Status, log.ErrorMessage, log.CreatedAt)
	return err
}

// UpdateLog updates log status after sending.
func (s *Store) UpdateLog(ctx context.Context, id uuid.UUID, status, errorMsg string) error {
	var sentAt *time.Time
	if status == "sent" {
		now := time.Now()
		sentAt = &now
	}
	_, err := s.pool.Exec(ctx, `
		UPDATE notification_logs SET status=$1, error_message=$2, sent_at=$3
		WHERE id=$4
	`, status, errorMsg, sentAt, id)
	return err
}
