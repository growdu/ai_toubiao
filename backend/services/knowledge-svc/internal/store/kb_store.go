// Package store is the data access layer for knowledge-svc.
package store

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("material not found")

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// CreateMaterial creates a new KB material.
func (s *Store) CreateMaterial(ctx context.Context, m *model.KBMaterial) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	m.ID = uuid.New()
	m.TenantID = uuid.MustParse(tid)
	m.Status = "active"
	if m.Metadata == nil {
		m.Metadata = json.RawMessage(`{}`)
	}

	const q = `
		INSERT INTO kb_materials (id, tenant_id, category, title, summary, content, file_path, file_size, mime_type, status, metadata, chunk_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at`
	return s.pool.QueryRow(ctx, q,
		m.ID, m.TenantID, m.Category, m.Title, m.Summary, m.Content, m.FilePath, m.FileSize, m.MimeType, m.Status, m.Metadata, m.ChunkCount,
	).Scan(&m.CreatedAt, &m.UpdatedAt)
}

// GetMaterial retrieves a material by ID.
func (s *Store) GetMaterial(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	const q = `
		SELECT id, tenant_id, category, title, summary, content, file_path, file_size, mime_type, status, metadata, chunk_count, indexed_at, created_at, updated_at
		FROM kb_materials WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
	var m model.KBMaterial
	err = s.pool.QueryRow(ctx, q, id, tid).Scan(
		&m.ID, &m.TenantID, &m.Category, &m.Title, &m.Summary, &m.Content, &m.FilePath, &m.FileSize, &m.MimeType, &m.Status, &m.Metadata, &m.ChunkCount, &m.IndexedAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return &m, err
}

// ListMaterials lists materials for a tenant.
func (s *Store) ListMaterials(ctx context.Context, category string, limit, offset int) ([]*model.KBMaterial, error) {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	var q string
	var args []any
	if category != "" {
		q = `SELECT id, tenant_id, category, title, summary, content, file_path, file_size, mime_type, status, metadata, chunk_count, indexed_at, created_at, updated_at
			FROM kb_materials WHERE tenant_id = $1 AND category = $2 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		args = []any{tid, category, limit, offset}
	} else {
		q = `SELECT id, tenant_id, category, title, summary, content, file_path, file_size, mime_type, status, metadata, chunk_count, indexed_at, created_at, updated_at
			FROM kb_materials WHERE tenant_id = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		args = []any{tid, limit, offset}
	}
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.KBMaterial
	for rows.Next() {
		var m model.KBMaterial
		if err := rows.Scan(&m.ID, &m.TenantID, &m.Category, &m.Title, &m.Summary, &m.Content, &m.FilePath, &m.FileSize, &m.MimeType, &m.Status, &m.Metadata, &m.ChunkCount, &m.IndexedAt, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, &m)
	}
	return out, rows.Err()
}

// DeleteMaterial soft-deletes a material.
func (s *Store) DeleteMaterial(ctx context.Context, id uuid.UUID) error {
	tid, err := tenant.FromContext(ctx)
	if err != nil {
		return err
	}
	tag, err := s.pool.Exec(ctx, `UPDATE kb_materials SET deleted_at = NOW() WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`, id, tid)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SearchChunks performs vector similarity search.
func (s *Store) SearchChunks(ctx context.Context, tenantID uuid.UUID, queryEmbedding []float32, topK int, category string) ([]model.KBSearchResult, error) {
	if topK <= 0 {
		topK = 10
	}
	if topK > 50 {
		topK = 50
	}

	// Note: requires pgvector extension. The query uses <=> (cosine distance) operator.
	var q string
	var args []any
	if category != "" {
		q = `
			SELECT c.id, c.material_id, m.title, c.content, c.chunk_index,
			       1 - (c.content_vec <=> $2) AS score
			FROM kb_chunks c
			JOIN kb_materials m ON m.id = c.material_id
			WHERE c.tenant_id = $1 AND m.category = $3 AND c.content_vec IS NOT NULL
			ORDER BY c.content_vec <=> $2
			LIMIT $4`
		args = []any{tenantID, queryEmbedding, category, topK}
	} else {
		q = `
			SELECT c.id, c.material_id, m.title, c.content, c.chunk_index,
			       1 - (c.content_vec <=> $2) AS score
			FROM kb_chunks c
			JOIN kb_materials m ON m.id = c.material_id
			WHERE c.tenant_id = $1 AND c.content_vec IS NOT NULL
			ORDER BY c.content_vec <=> $2
			LIMIT $3`
		args = []any{tenantID, queryEmbedding, topK}
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []model.KBSearchResult
	for rows.Next() {
		var r model.KBSearchResult
		if err := rows.Scan(&r.ChunkID, &r.MaterialID, &r.MaterialTitle, &r.Content, &r.ChunkIndex, &r.Score); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

// CreateChunk creates a new chunk.
func (s *Store) CreateChunk(ctx context.Context, c *model.KBChunk) error {
	c.ID = uuid.New()
	const q = `
		INSERT INTO kb_chunks (id, material_id, tenant_id, content, chunk_index, char_start, char_end, source_location, hit_count, used_count, metadata)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING created_at`
	return s.pool.QueryRow(ctx, q, c.ID, c.MaterialID, c.TenantID, c.Content, c.ChunkIndex, c.CharStart, c.CharEnd, c.SourceLocation, c.HitCount, c.UsedCount, c.Metadata).Scan(&c.CreatedAt)
}

// UpdateMaterialIndexed updates indexed_at and chunk_count after ingestion.
func (s *Store) UpdateMaterialIndexed(ctx context.Context, id uuid.UUID, chunkCount int) error {
	_, err := s.pool.Exec(ctx, `UPDATE kb_materials SET indexed_at = NOW(), chunk_count = $1, updated_at = NOW() WHERE id = $2`, chunkCount, id)
	return err
}