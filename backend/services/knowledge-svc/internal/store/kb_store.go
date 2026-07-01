// Package store is the data access layer for knowledge-svc.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgvector "github.com/pgvector/pgvector-go"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
)

var ErrNotFound = errors.New("material not found")

// Store is the persistence layer for knowledge-svc. It owns a *pgxpool.Pool
// and binds pgvector codecs on the connection(s) it touches so that
// []float32 embeddings can be encoded into PostgreSQL's vector(1536) column
// without going through the broken "vector is of type vector but expression
// is of type float[]" path that raw []float32 would produce.
//
// Codec registration is best-effort: see the comment on New for details.
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store bound to the given pool. It also tries to register the
// pgvector type codecs (vector / halfvec / sparsevec) on a sample connection
// from the pool so that values of type pgvector.Vector can be encoded and
// decoded in binary format on that connection.
//
// Why a sample connection, not the pool itself?
//   - The pgvector-go pgx subpackage exposes RegisterTypes(ctx, *pgx.Conn),
//     which mutates a single connection's TypeMap. pgxpool.Pool does not
//     expose a "register on all" hook after construction.
//   - The recommended way to register codecs on every connection is to set
//     pgxpool.Config.AfterConnect to call pgxvec.RegisterTypes at the time
//     the pool is created. We cannot do that here because we only receive
//     an already-built pool; see the pgvector-go README for the snippet.
//
// What happens on connections that did NOT get RegisterTypes called?
//   - For encoding (INSERT / query parameters), pgx falls back to the
//     database/sql/driver.Valuer interface when no codec matches the
//     parameter OID. pgvector.Vector.Value() returns the wire-format string
//     like "[0.1,0.2,...]", which PostgreSQL parses as a vector literal.
//   - For decoding (SELECT into *pgvector.Vector), the codec is required.
//     This store currently does not scan vector columns (see SearchChunks
//     comment); if it ever does, the caller should also set AfterConnect
//     when building the pool to guarantee every connection has the codec.
//
// If the pool is nil (e.g. in unit tests that don't touch the database) the
// call is a no-op. If a connection cannot be acquired, we log and proceed:
// the driver.Valuer fallback above keeps the write path functional.
func New(pool *pgxpool.Pool) *Store {
	s := &Store{pool: pool}
	if pool == nil {
		return s
	}
	// Use a fresh context (not the caller's) because New is a constructor and
	// should not be tied to a request lifetime.
	ctx, cancel := context.WithTimeout(context.Background(), pgRegisterTimeout)
	defer cancel()
	conn, err := pool.Acquire(ctx)
	if err != nil {
		slog.Warn("pgvector: cannot acquire connection to register types; write path will rely on driver.Valuer fallback",
			slog.String("err", err.Error()))
		return s
	}
	defer conn.Release()
	if err := pgxvec.RegisterTypes(ctx, conn.Conn()); err != nil {
		slog.Warn("pgvector: RegisterTypes failed; write path will rely on driver.Valuer fallback",
			slog.String("err", err.Error()))
	}
	return s
}

// pgRegisterTimeout caps how long New will wait while trying to register the
// pgvector codec on a sample connection. Kept short on purpose: the worst
// case is that we fall back to the driver.Valuer path, which still works.
const pgRegisterTimeout = 5 * time.Second

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
//
// How the []float32 -> vector(1536) wire transition works:
//  1. The query embeds the vector as a parameter; pgx needs to know how to
//     turn the Go value into a Postgres parameter.
//  2. If we hand pgx a raw []float32, pgx sends it as a float8[] (the default
//     Go codec) and PostgreSQL responds with
//     "column \"content_vec\" is of type vector but expression is of type
//     float[]". That is the bug ADR-0009 fixes.
//  3. Wrapping the slice in pgvector.NewVector(embedding) gives pgx a
//     concrete value that implements database/sql/driver.Valuer. pgx
//     recognizes that interface and encodes the value via Value(), which
//     returns the wire-format string "[0.1,0.2,...]". PostgreSQL parses
//     that as a vector literal and the <=> / <-> operators work.
//  4. If the connection had pgxvec.RegisterTypes applied (see New), the
//     binary codec kicks in and we send the compact binary representation
//     instead. Either way, the call site is identical.
//
// The Scan side here only reads scalar columns (id, material_id, title,
// content, chunk_index, score) and does not pull the vector column back,
// so we do not need *pgvector.Vector on the receive side yet. If a future
// query needs to read content_vec out, the destination must be
// *pgvector.Vector and the connection must have had RegisterTypes called.
func (s *Store) SearchChunks(ctx context.Context, tenantID uuid.UUID, queryEmbedding []float32, topK int, category string) ([]model.KBSearchResult, error) {
	if topK <= 0 {
		topK = 10
	}
	if topK > 50 {
		topK = 50
	}

	// Wrap the embedding once so the same pgvector.Vector value flows into
	// both the parameter slot and (in some PG builds) the operator argument.
	// pgvector.NewVector is allocation-cheap; it just stores the slice.
	qVec := pgvector.NewVector(queryEmbedding)

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
		args = []any{tenantID, qVec, category, topK}
	} else {
		q = `
			SELECT c.id, c.material_id, m.title, c.content, c.chunk_index,
			       1 - (c.content_vec <=> $2) AS score
			FROM kb_chunks c
			JOIN kb_materials m ON m.id = c.material_id
			WHERE c.tenant_id = $1 AND c.content_vec IS NOT NULL
			ORDER BY c.content_vec <=> $2
			LIMIT $3`
		args = []any{tenantID, qVec, topK}
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

// CreateChunkWithVec creates a chunk with its embedding vector.
//
// The vec []float32 is wrapped in pgvector.NewVector before being passed to
// pgx.Exec. See the comment on SearchChunks for the full rationale; the
// short version is that pgvector.Vector implements driver.Valuer and (if
// the connection was registered via New) pgtype.Codec, so pgx can encode
// it as a Postgres vector(1536) instead of a float8[].
func (s *Store) CreateChunkWithVec(ctx context.Context, c *model.KBChunk, vec []float32) error {
	c.ID = uuid.New()
	_, err := s.pool.Exec(ctx, `
		INSERT INTO kb_chunks (id, material_id, tenant_id, content, chunk_index, char_start, char_end, source_location, hit_count, used_count, metadata, content_vec)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, c.ID, c.MaterialID, c.TenantID, c.Content, c.ChunkIndex, c.CharStart, c.CharEnd, c.SourceLocation, c.HitCount, c.UsedCount, c.Metadata, pgvector.NewVector(vec))
	return err
}

// UpdateMaterialIndexed updates indexed_at and chunk_count after ingestion.
func (s *Store) UpdateMaterialIndexed(ctx context.Context, id uuid.UUID, chunkCount int) error {
	_, err := s.pool.Exec(ctx, `UPDATE kb_materials SET indexed_at = NOW(), chunk_count = $1, updated_at = NOW() WHERE id = $2`, chunkCount, id)
	return err
}
