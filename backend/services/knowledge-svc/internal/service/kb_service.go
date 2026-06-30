// Package service implements business logic for knowledge-svc.
package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/services/knowledge-svc/internal/store"
	"github.com/google/uuid"
)

// KBService handles knowledge base operations.
type KBService struct {
	store       *store.Store
	log         *slog.Logger
	routerClient *RouterClient
}

// NewKBService creates a KBService.
func NewKBService(s *store.Store, log *slog.Logger, routerURL string) *KBService {
	return &KBService{store: s, log: log, routerClient: NewRouterClient(routerURL)}
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

	// 1. Get query embedding from router-svc.
	embResp, err := s.routerClient.Embed(ctx, tenantID, []string{req.Query}, "text-embedding-3-small")
	if err != nil {
		s.log.Warn("embed call failed, returning empty results", slog.Any("error", err))
		return &model.SearchResponse{Hits: []model.KBSearchResult{}, Total: 0}, nil
	}
	if len(embResp.Embeddings) == 0 {
		return &model.SearchResponse{Hits: []model.KBSearchResult{}, Total: 0}, nil
	}

	queryEmbedding := embResp.Embeddings[0]

	// 2. Vector search in pgvector.
	hits, err := s.store.SearchChunks(ctx, tenantID, queryEmbedding, req.TopK, req.Category)
	if err != nil {
		return nil, err
	}

	return &model.SearchResponse{
		Hits:  hits,
		Total: len(hits),
	}, nil
}

// chunkContent splits text into chunks of approximately maxTokens tokens.
// Uses simple paragraph + character-based splitting.
func chunkContent(text string, maxTokens int) []string {
	if text == "" {
		return nil
	}

	// Rough estimate: 1 token ≈ 4 chars for Chinese/English mixed text.
	charLimit := maxTokens * 4
	if charLimit < 100 {
		charLimit = 200
	}

	var chunks []string
	paragraphs := strings.Split(text, "\n")

	var current strings.Builder
	currentLen := 0
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		paraLen := len(para)
		if currentLen+paraLen+1 > charLimit && current.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(current.String()))
			current.Reset()
			currentLen = 0
		}
		current.WriteString(para)
		current.WriteString("\n")
		currentLen += paraLen + 1
	}
	if current.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(current.String()))
	}
	if len(chunks) == 0 {
		chunks = append(chunks, text)
	}
	return chunks
}

// Ingest triggers indexing of a material (chunking + embedding).
func (s *KBService) Ingest(ctx context.Context, req *model.IngestRequest) error {
	m, err := s.store.GetMaterial(ctx, req.MaterialID)
	if err != nil {
		return err
	}

	content := m.Content
	if content == "" && m.FilePath != "" {
		// TODO: Load content from file_path (S3 or local FS)
		s.log.Warn("file_path content loading not implemented", slog.String("file_path", m.FilePath))
		return nil
	}
	if content == "" {
		s.log.Warn("no content to ingest", slog.String("material_id", m.ID.String()))
		return nil
	}

	// 1. Chunk the content.
	chunks := chunkContent(content, 512)
	if len(chunks) == 0 {
		return nil
	}

	// 2. Batch embed chunks via router-svc.
	var allEmbeddings [][]float32
	for i := 0; i < len(chunks); i += 20 {
		end := i + 20
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		embResp, err := s.routerClient.Embed(ctx, m.TenantID, batch, "text-embedding-3-small")
		if err != nil {
			s.log.Warn("embed batch failed", slog.Any("error", err), slog.Int("batch_start", i))
			continue
		}
		allEmbeddings = append(allEmbeddings, embResp.Embeddings...)
	}

	// 3. Store chunks with embeddings.
	for i, chunk := range chunks {
		if i >= len(allEmbeddings) || allEmbeddings[i] == nil {
			continue
		}
		c := &model.KBChunk{
			MaterialID: m.ID,
			TenantID:   m.TenantID,
			Content:    chunk,
			ChunkIndex: i,
		}
		if err := s.store.CreateChunkWithVec(ctx, c, allEmbeddings[i]); err != nil {
			s.log.Warn("create chunk failed", slog.Any("error", err), slog.Int("chunk_index", i))
		}
	}

	s.log.Info("ingest completed",
		slog.String("material_id", m.ID.String()),
		slog.Int("chunks_created", len(allEmbeddings)))

	return s.store.UpdateMaterialIndexed(ctx, m.ID, len(allEmbeddings))
}