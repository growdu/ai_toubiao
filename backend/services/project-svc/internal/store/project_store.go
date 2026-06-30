// Package store is the data access layer.
// All queries MUST filter by tenant_id (see ADR-0001).
package store

import (
	"context"
	"errors"

	"github.com/bidwriter/services/project-svc/internal/model"
	"github.com/bidwriter/shared/pkg/db"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a project doesn't exist or belongs to another tenant.
var ErrNotFound = errors.New("project not found")

// ErrVersionConflict is returned on optimistic-lock failure.
var ErrVersionConflict = errors.New("version conflict")

// Store provides CRUD operations on projects.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a new Store.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, p *model.Project) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	p.TenantID = uuid.MustParse(tid)
	p.ID = uuid.New()
	p.Status = model.StatusDraft
	p.Version = 1

	const q = `
		INSERT INTO projects (
			id, tenant_id, name, description, industry, template_id,
			status, estimated_value, currency, deadline, owner_id, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at`

	return s.pool.QueryRow(ctx, q,
		p.ID, p.TenantID, p.Name, p.Description, p.Industry, p.TemplateID,
		p.Status, p.EstimatedValue, p.Currency, p.Deadline, p.OwnerID, p.Version,
	).Scan(&p.CreatedAt, &p.UpdatedAt)
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*model.Project, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	const q = `
		SELECT id, tenant_id, name, description, industry, template_id,
		       status, estimated_value, currency, deadline, owner_id,
		       version, created_at, updated_at
		FROM projects
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`

	var p model.Project
	err = s.pool.QueryRow(ctx, q, id, tid).Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Industry, &p.TemplateID,
		&p.Status, &p.EstimatedValue, &p.Currency, &p.Deadline, &p.OwnerID,
		&p.Version, &p.CreatedAt, &p.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &p, err
}

func (s *Store) List(ctx context.Context, limit int, cursor *uuid.UUID) ([]*model.Project, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	const q = `
		SELECT id, tenant_id, name, description, industry, template_id,
		       status, estimated_value, currency, deadline, owner_id,
		       version, created_at, updated_at
		FROM projects
		WHERE tenant_id = $1 AND deleted_at IS NULL
		  AND ($2::uuid IS NULL OR id < $2)
		ORDER BY id DESC
		LIMIT $3`

	rows, err := s.pool.Query(ctx, q, tid, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Project
	for rows.Next() {
		var p model.Project
		if err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Industry, &p.TemplateID,
			&p.Status, &p.EstimatedValue, &p.Currency, &p.Deadline, &p.OwnerID,
			&p.Version, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &p)
	}
	return out, rows.Err()
}

// Update applies a partial update with optimistic locking.
func (s *Store) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Project, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	var updated *model.Project
	err = db.InTx(ctx, s.pool, func(tx pgx.Tx) error {
		// Lock the row
		var current model.Project
		err := tx.QueryRow(ctx, `
			SELECT id, tenant_id, name, status, version
			FROM projects
			WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
			FOR UPDATE`, id, tid).Scan(
			&current.ID, &current.TenantID, &current.Name, &current.Status, &current.Version,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}

		// Optimistic-lock check
		if current.Version != req.Version {
			return ErrVersionConflict
		}

		// Apply partial update
		const q = `
			UPDATE projects
			SET name = COALESCE($3, name),
			    description = COALESCE($4, description),
			    industry = COALESCE($5, industry),
			    status = COALESCE($6, status),
			    estimated_value = COALESCE($7, estimated_value),
			    currency = COALESCE($8, currency),
			    deadline = COALESCE($9, deadline),
			    version = version + 1,
			    updated_at = NOW()
			WHERE id = $1 AND tenant_id = $2 AND version = $10
			RETURNING id, tenant_id, name, description, industry, template_id,
			          status, estimated_value, currency, deadline, owner_id,
			          version, created_at, updated_at`

		var p model.Project
		var statusArg *string
		if req.Status != nil {
			s := string(*req.Status)
			statusArg = &s
		}
		err = tx.QueryRow(ctx, q,
			id, tid, req.Name, req.Description, req.Industry,
			statusArg, req.EstimatedValue, req.Currency, req.Deadline, req.Version,
		).Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.Industry, &p.TemplateID,
			&p.Status, &p.EstimatedValue, &p.Currency, &p.Deadline, &p.OwnerID,
			&p.Version, &p.CreatedAt, &p.UpdatedAt,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVersionConflict
		}
		if err != nil {
			return err
		}
		updated = &p
		return nil
	})
	return updated, err
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE projects SET deleted_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`,
		id, tid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}