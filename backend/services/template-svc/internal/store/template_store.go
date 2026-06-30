package store

import (
	"context"
	"errors"

	"github.com/bidwriter/services/template-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("template not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Create inserts a new template.
func (s *Store) Create(ctx context.Context, t *model.WordTemplate) error {
	t.ID = uuid.New()
	t.Version = 1
	_, err := s.pool.Exec(ctx, `
		INSERT INTO word_templates (id, tenant_id, name, description, kind, storage_key, size_bytes, checksum, version, is_default, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
	`, t.ID, t.TenantID, t.Name, t.Description, t.Kind, t.StorageKey, t.SizeBytes, t.Checksum, t.Version, t.IsDefault, t.CreatedBy)
	return err
}

// Get retrieves a template by ID.
func (s *Store) Get(ctx context.Context, id uuid.UUID) (*model.WordTemplate, error) {
	var t model.WordTemplate
	err := s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, kind, storage_key, size_bytes, checksum, version, is_default, created_by, created_at, updated_at
		FROM word_templates WHERE id = $1
	`, id).Scan(&t.ID, &t.TenantID, &t.Name, &t.Description, &t.Kind, &t.StorageKey, &t.SizeBytes, &t.Checksum, &t.Version, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// List returns all templates for the current tenant.
func (s *Store) List(ctx context.Context) ([]*model.WordTemplate, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, kind, storage_key, size_bytes, checksum, version, is_default, created_by, created_at, updated_at
		FROM word_templates WHERE tenant_id = $1 ORDER BY created_at DESC
	`, tid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var templates []*model.WordTemplate
	for rows.Next() {
		var t model.WordTemplate
		if err := rows.Scan(&t.ID, &t.TenantID, &t.Name, &t.Description, &t.Kind, &t.StorageKey, &t.SizeBytes, &t.Checksum, &t.Version, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		templates = append(templates, &t)
	}
	return templates, rows.Err()
}

// Update updates template metadata.
func (s *Store) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.WordTemplate, error) {
	// Fetch existing
	t, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Description != nil {
		t.Description = *req.Description
	}
	if req.IsDefault != nil {
		t.IsDefault = *req.IsDefault
	}
	t.Version++

	_, err = s.pool.Exec(ctx, `
		UPDATE word_templates SET name=$1, description=$2, is_default=$3, version=$4, updated_at=NOW()
		WHERE id=$5
	`, t.Name, t.Description, t.IsDefault, t.Version, id)
	if err != nil {
		return nil, err
	}
	return s.Get(ctx, id)
}

// Delete removes a template.
func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM word_templates WHERE id = $1`, id)
	return err
}

// GetDefault returns the default template for the current tenant and kind.
func (s *Store) GetDefault(ctx context.Context, kind string) (*model.WordTemplate, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	var t model.WordTemplate
	err = s.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, kind, storage_key, size_bytes, checksum, version, is_default, created_by, created_at, updated_at
		FROM word_templates WHERE tenant_id = $1 AND kind = $2 AND is_default = true
	`, tid, kind).Scan(&t.ID, &t.TenantID, &t.Name, &t.Description, &t.Kind, &t.StorageKey, &t.SizeBytes, &t.Checksum, &t.Version, &t.IsDefault, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &t, err
}

// ClearDefault unsets default flag for all templates of a kind.
func (s *Store) ClearDefault(ctx context.Context, kind string) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE word_templates SET is_default = false WHERE tenant_id = $1 AND kind = $2
	`, tid, kind)
	return err
}
