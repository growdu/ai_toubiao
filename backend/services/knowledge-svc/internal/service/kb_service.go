// Package service implements business logic for knowledge-svc.
package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/services/knowledge-svc/internal/store"
	"github.com/google/uuid"
)

// ObjectStore is the seam used by Ingest to load file-backed materials
// from MinIO / S3 (or any compatible store). Production wires
// *MinIOStore; tests inject *fakeObjectStore to verify the load path
// without an actual server.
type ObjectStore interface {
	Get(ctx context.Context, bucket, key string) (io.ReadCloser, error)
}

// Embedder is the seam used by Ingest / Search to obtain embeddings.
// *RouterClient implements it; tests inject a stub.
type Embedder interface {
	Embed(ctx context.Context, tenantID uuid.UUID, texts []string, model string) (*EmbedResponse, error)
}

// ingestStore is the storage contract Ingest actually needs (subset of
// *store.Store). Defined at the consumer so tests can inject a fake
// without bringing up a real PG.
type ingestStore interface {
	GetMaterial(ctx context.Context, id uuid.UUID) (*model.KBMaterial, error)
	CreateChunkWithVec(ctx context.Context, c *model.KBChunk, emb []float32) error
	UpdateMaterialIndexed(ctx context.Context, id uuid.UUID, count int) error
}

// KBService handles knowledge base operations.
type KBService struct {
	store        *store.Store
	log          *slog.Logger
	routerClient *RouterClient
	objectStore  ObjectStore
	kbBucket     string
	embedder     Embedder
}

// NewKBService creates a KBService wired against the production store and
// the supplied object store (nil disables the file-path code path).
// bucket is the MinIO bucket name that file-backed KB materials are
// stored under. routerURL is kept for backward compatibility — passing
// an empty string skips the router-backed Search path.
func NewKBService(s *store.Store, log *slog.Logger, routerURL string, obj ObjectStore, bucket string) *KBService {
	rc := NewRouterClient(routerURL)
	return &KBService{
		store:        s,
		log:          log,
		routerClient: rc,
		embedder:     rc,
		objectStore:  obj,
		kbBucket:     bucket,
	}
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
// Public entry point used by the HTTP handler. Internally calls
// ingestWithDeps so the same logic is unit-testable.
func (s *KBService) Ingest(ctx context.Context, req *model.IngestRequest) error {
	return s.ingestWithDeps(ctx, s.store, req.MaterialID)
}

// ingestWithDeps is the testable core of Ingest. The store arg is the
// storage seam (production = *store.Store, tests = fakeStore); other deps
// come from the receiver.
func (s *KBService) ingestWithDeps(ctx context.Context, st ingestStore, materialID uuid.UUID) error {
	m, err := st.GetMaterial(ctx, materialID)
	if err != nil {
		return err
	}

	content := m.Content
	if content == "" && m.FilePath != "" {
		if s.objectStore == nil {
			s.log.Warn("file_path set but no object store configured; skipping",
				slog.String("material_id", m.ID.String()),
				slog.String("file_path", m.FilePath))
			return nil
		}
		rc, err := s.objectStore.Get(ctx, s.kbBucket, m.FilePath)
		if err != nil {
			return fmt.Errorf("load %q from bucket %q: %w", m.FilePath, s.kbBucket, err)
		}
		defer rc.Close()
		// Cap the read at 50 MiB to defend against a runaway object
		// DoS-ing the service. KB materials are RFP excerpts / sample
		// docs — anything past this is almost certainly wrong.
		buf, err := io.ReadAll(io.LimitReader(rc, 50<<20))
		if err != nil {
			return fmt.Errorf("read object %q: %w", m.FilePath, err)
		}
		content = string(buf)
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

	// 2. Batch embed chunks via the router.
	var allEmbeddings [][]float32
	for i := 0; i < len(chunks); i += 20 {
		end := i + 20
		if end > len(chunks) {
			end = len(chunks)
		}
		batch := chunks[i:end]
		embResp, err := s.embedder.Embed(ctx, m.TenantID, batch, "text-embedding-3-small")
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
		if err := st.CreateChunkWithVec(ctx, c, allEmbeddings[i]); err != nil {
			s.log.Warn("create chunk failed", slog.Any("error", err), slog.Int("chunk_index", i))
		}
	}

	s.log.Info("ingest completed",
		slog.String("material_id", m.ID.String()),
		slog.Int("chunks_created", len(allEmbeddings)))

	return st.UpdateMaterialIndexed(ctx, m.ID, len(allEmbeddings))
}