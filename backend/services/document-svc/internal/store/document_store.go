// Package store is the data access layer for documents.
// All queries MUST filter by tenant_id (see ADR-0001).
package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bidwriter/services/document-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")
var ErrVersionConflict = errors.New("version conflict")
var ErrDuplicate = errors.New("duplicate document (same checksum already exists for this project)")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Create(ctx context.Context, d *model.Document) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	d.TenantID = uuid.MustParse(tid)
	d.ID = uuid.New()
	d.Status = model.StatusUploading
	d.Version = 1
	if len(d.Metadata) == 0 {
		d.Metadata = json.RawMessage(`{}`)
	}

	const q = `
		INSERT INTO documents (
			id, tenant_id, project_id, name, kind, mime_type, size_bytes,
			storage_key, checksum_sha256, status, parse_status, metadata,
			uploaded_by, version
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at, updated_at`

	return s.pool.QueryRow(ctx, q,
		d.ID, d.TenantID, d.ProjectID, d.Name, d.Kind, d.MimeType, d.SizeBytes,
		d.StorageKey, d.ChecksumSHA256, d.Status, d.ParseStatus, d.Metadata,
		d.UploadedBy, d.Version,
	).Scan(&d.CreatedAt, &d.UpdatedAt)
}

func (s *Store) Get(ctx context.Context, id uuid.UUID) (*model.Document, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	const q = `
		SELECT id, tenant_id, project_id, name, kind, mime_type, size_bytes,
		       storage_key, checksum_sha256, status, parse_status, metadata,
		       uploaded_by, version, created_at, updated_at
		FROM documents
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`

	var d model.Document
	err = s.pool.QueryRow(ctx, q, id, tid).Scan(
		&d.ID, &d.TenantID, &d.ProjectID, &d.Name, &d.Kind, &d.MimeType, &d.SizeBytes,
		&d.StorageKey, &d.ChecksumSHA256, &d.Status, &d.ParseStatus, &d.Metadata,
		&d.UploadedBy, &d.Version, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &d, err
}

// List returns documents for a tenant, optionally filtered by project_id.
func (s *Store) List(ctx context.Context, projectID *uuid.UUID, limit int, cursor *uuid.UUID) ([]*model.Document, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	const q = `
		SELECT id, tenant_id, project_id, name, kind, mime_type, size_bytes,
		       storage_key, checksum_sha256, status, parse_status, metadata,
		       uploaded_by, version, created_at, updated_at
		FROM documents
		WHERE tenant_id = $1
		  AND ($2::uuid IS NULL OR project_id = $2)
		  AND deleted_at IS NULL
		  AND ($3::uuid IS NULL OR id < $3)
		ORDER BY id DESC
		LIMIT $4`

	rows, err := s.pool.Query(ctx, q, tid, projectID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Document
	for rows.Next() {
		var d model.Document
		if err := rows.Scan(
			&d.ID, &d.TenantID, &d.ProjectID, &d.Name, &d.Kind, &d.MimeType, &d.SizeBytes,
			&d.StorageKey, &d.ChecksumSHA256, &d.Status, &d.ParseStatus, &d.Metadata,
			&d.UploadedBy, &d.Version, &d.CreatedAt, &d.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &d)
	}
	return out, rows.Err()
}

func (s *Store) Update(ctx context.Context, id uuid.UUID, req *model.UpdateRequest) (*model.Document, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}

	// Lock + version check
	var currentVersion int
	err = s.pool.QueryRow(ctx, `
		SELECT version FROM documents
		WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL
		FOR UPDATE`, id, tid).Scan(&currentVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if currentVersion != req.Version {
		return nil, ErrVersionConflict
	}

	const q = `
		UPDATE documents
		SET name = COALESCE($3, name),
		    kind = COALESCE($4, kind),
		    status = COALESCE($5, status),
		    parse_status = COALESCE($6, parse_status),
		    metadata = COALESCE($7, metadata),
		    version = version + 1,
		    updated_at = NOW()
		WHERE id = $1 AND tenant_id = $2 AND version = $8
		RETURNING id, tenant_id, project_id, name, kind, mime_type, size_bytes,
		          storage_key, checksum_sha256, status, parse_status, metadata,
		          uploaded_by, version, created_at, updated_at`

	var d model.Document
	var kindArg, statusArg *string
	if req.Kind != nil {
		s := string(*req.Kind)
		kindArg = &s
	}
	if req.Status != nil {
		s := string(*req.Status)
		statusArg = &s
	}

	err = s.pool.QueryRow(ctx, q,
		id, tid, req.Name, kindArg, statusArg, req.ParseStatus, req.Metadata, req.Version,
	).Scan(
		&d.ID, &d.TenantID, &d.ProjectID, &d.Name, &d.Kind, &d.MimeType, &d.SizeBytes,
		&d.StorageKey, &d.ChecksumSHA256, &d.Status, &d.ParseStatus, &d.Metadata,
		&d.UploadedBy, &d.Version, &d.CreatedAt, &d.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrVersionConflict
	}
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) Delete(ctx context.Context, id uuid.UUID) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `
		UPDATE documents SET deleted_at = NOW()
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