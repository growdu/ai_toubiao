// Package service implements business logic for knowledge-svc.
package service

import (
	"context"
	"log/slog"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/services/knowledge-svc/internal/store"
	"github.com/google/uuid"
)

// KBService handles knowledge base operations.
type KBService struct {
	store *store.Store
	log   *slog.Logger
}

// NewKBService creates a KBService.
func NewKBService(s *store.Store, log *slog.Logger) *KBService {
	return &KBService{store: s, log: log}
}

// CreateMaterial creates a new KB material.
func (s *KBService) CreateMaterial(ctx context.Context, req *model.CreateMaterialRequest) (*model.KBMaterial, error) {
	m := &model.KBMaterial{
		Category: req.Category,
		Title:    req.Title,
		Content:  req.Content,
		FilePath: req.FilePath,
		MimeType: req.MimeType,
		Metadata: req.Metadata,
	}
	if err := s.store.CreateMaterial(ctx, m); err != nil {
		return nil, err
	}
	s.log.Info("material created", slog.String("id", m.ID.String()))
	return m, nil
}

// GetMaterial retrieves a material by ID.
func (s *KBService) GetMaterial(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error) {
	return s.store.GetMaterial(ctx, id)
}

// ListMaterials lists materials for the tenant.
func (s *KBService) ListMaterials(ctx context.Context, category string, limit, offset int) ([]*model.KBMaterial, error) {
	return s.store.ListMaterials(ctx, category, limit, offset)
}

// DeleteMaterial deletes a material.
func (s *KBService) DeleteMaterial(ctx context.Context, id uuid.UUID) error {
	return s.store.DeleteMaterial(ctx, id)
}

// Search performs semantic search across knowledge base.
func (s *KBService) Search(ctx context.Context, tenantID uuid.UUID, req *model.SearchRequest) (*model.SearchResponse, error) {
	if req.TopK <= 0 {
		req.TopK = 10
	}

	// TODO: In Phase 1.2, this will call router-svc to get embedding for the query.
	// For now, return a placeholder that indicates embedding is needed.
	// The actual implementation:
	// 1. Call router-svc embed task to get query embedding
	// 2. Call store.SearchChunks with the embedding

	s.log.Info("search requested", slog.String("query", req.Query), slog.Int("top_k", req.TopK))

	// Placeholder: return empty results until embedding is implemented
	return &model.SearchResponse{
		Hits:  []model.KBSearchResult{},
		Total: 0,
	}, nil
}

// Ingest triggers indexing of a material (chunking + embedding).
func (s *KBService) Ingest(ctx context.Context, req *model.IngestRequest) error {
	m, err := s.store.GetMaterial(ctx, req.MaterialID)
	if err != nil {
		return err
	}

	// TODO: In Phase 1.2:
	// 1. If content is empty, read from file_path (S3)
	// 2. Chunk the content (simple paragraph splitting or smarter)
	// 3. Call router-svc embed task for each chunk
	// 4. Store chunks with embeddings in kb_chunks
	// 5. Update material indexed_at and chunk_count

	s.log.Info("ingest triggered", slog.String("material_id", m.ID.String()), slog.Bool("force", req.Force))

	// Placeholder: mark as indexed with 0 chunks
	return s.store.UpdateMaterialIndexed(ctx, m.ID, 0)
}