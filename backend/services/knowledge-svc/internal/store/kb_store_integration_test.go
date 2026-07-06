// kb_store_integration_test.go verifies the kb_store against a real PostgreSQL
// instance. Skipped unless BIDWRITER_TEST_DSN is set (e.g.
//
//	BIDWRITER_TEST_DSN=postgres://postgres:postgres@localhost:5434/bidwriter?sslmode=disable \
//	  go test ./services/knowledge-svc/internal/store/ -run Integration -v
//
// This file was added in the "fix kb_chunks id type" commit to lock in that:
//   - InsertChunk with the Go uuid.UUID model maps to a UUID PK column
//     (the schema after migration 00013 must be UUID, not BIGSERIAL)
//   - SearchChunks reads the UUID PK back into uuid.UUID without Scan errors
//   - CreateMaterial + Insert + Search round-trip works against pgvector
package store

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bidwriter/services/knowledge-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvec "github.com/pgvector/pgvector-go/pgx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMain wires pgvector's binary codec into the pgx pool BEFORE any test
// acquires a connection. Without this, SearchChunks's <-> operator works
// but decoding a vector(1536) into pgvector.Vector returns the wrong type.
func TestMain(m *testing.M) {
	dsn := os.Getenv("BIDWRITER_TEST_DSN")
	if dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		pcfg, err := pgxpool.ParseConfig(dsn)
		if err == nil {
			pcfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
				return pgxvec.RegisterTypes(ctx, conn)
			}
			pool, err := pgxpool.NewWithConfig(ctx, pcfg)
			if err == nil {
				intPool = pool
			}
		}
	}
	os.Exit(m.Run())
}

var intPool *pgxpool.Pool

// TestIntegration_ChunkInsertAndSearch is the original chunk-insert + vector
// round-trip smoke test. It locks in the UUID PK + pgvector type-codec fix.
func TestIntegration_ChunkInsertAndSearch(t *testing.T) {
	if intPool == nil {
		t.Skip("BIDWRITER_TEST_DSN not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Tenant fixture.
	tid := uuid.New()
	_, err := intPool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		tid, "test-"+tid.String()[:8], "test-"+tid.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(),
			`DELETE FROM tenants WHERE id = $1`, tid)
	})
	ctx = tenant.WithTenant(ctx, tid.String())

	st := New(intPool)

	// Create a material.
	mat := &model.KBMaterial{
		Category: "case",
		Title:    "Integration test material " + tid.String()[:8],
		Content:  "Some long body text that we will chunk into embedding-friendly pieces.",
	}
	require.NoError(t, st.CreateMaterial(ctx, mat))
	require.NotEqual(t, uuid.Nil, mat.ID)
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(),
			`DELETE FROM kb_materials WHERE id = $1`, mat.ID)
	})

	// Insert two chunks with deterministic embeddings.
	// Chunk A is the "needle"; chunk B is unrelated noise.
	embA := make([]float32, 1536)
	embA[0] = 1.0 // distinctive spike
	embB := make([]float32, 1536)
	embB[1] = 1.0

	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "needle", ChunkIndex: 0,
	}, embA))
	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "noise", ChunkIndex: 1,
	}, embB))
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(),
			`DELETE FROM kb_chunks WHERE material_id = $1`, mat.ID)
	})

	// Query with the same distinctive spike — chunk A must win.
	q := make([]float32, 1536)
	q[0] = 1.0
	hits, err := st.SearchChunks(ctx, tid, q, 5, "")
	require.NoError(t, err)
	require.NotEmpty(t, hits, "should find at least one chunk")
	assert.Equal(t, "needle", hits[0].Content, "needle embedding should rank #1")
	assert.Greater(t, hits[0].Score, 0.99, "cosine sim of identical vectors ~ 1.0")
}
// TestIntegration_BM25Search verifies the new tsvector-backed full-text
// search path. The trigger from migration 00013 must populate content_tsv
// automatically; we assert that here by inserting chunks, waiting for the
// trigger to fire, and then querying with a natural-language query.
//
// We deliberately do NOT use an embedding service for this test — BM25 is
// pure keyword matching and the trigger fills content_tsv at insert time.
func TestIntegration_BM25Search(t *testing.T) {
	if intPool == nil {
		t.Skip("BIDWRITER_TEST_DSN not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tid := uuid.New()
	_, err := intPool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		tid, "test-bm25-"+tid.String()[:8], "test-bm25-"+tid.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tid)
	})
	ctx = tenant.WithTenant(ctx, tid.String())

	st := New(intPool)

	mat := &model.KBMaterial{
		Category: "case",
		Title:    "ISO9001 quality management system case " + tid.String()[:8],
		Content:  "We implement ISO 9001 across all business units.",
	}
	require.NoError(t, st.CreateMaterial(ctx, mat))
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM kb_materials WHERE id = $1`, mat.ID)
	})

	// Two contrasting chunks. Only one mentions ISO.
	emb := make([]float32, 1536)
	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "Our quality system is ISO 9001 certified since 2018.", ChunkIndex: 0,
	}, emb))
	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "We deliver hardware to telecom operators.", ChunkIndex: 1,
	}, emb))
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM kb_chunks WHERE material_id = $1`, mat.ID)
	})

	hits, err := st.SearchChunksBM25(ctx, tid, "ISO 9001 quality", 5, "")
	require.NoError(t, err)
	require.NotEmpty(t, hits, "BM25 should match at least one chunk")
	require.Equal(t, 1, len(hits), "BM25 should match exactly one chunk (the ISO one)")
	assert.Contains(t, hits[0].Content, "ISO", "ranked chunk should contain ISO")
	assert.Greater(t, hits[0].Score, 0.0, "score should be positive")
}

// TestIntegration_HybridRRFOrder verifies the hybrid merger:
//   - We insert two chunks: A is the "vector needle" (sharp embedding), B
//     is the "BM25 needle" (text that matches the query lexically).
//   - The vector path ranks A first, the BM25 path ranks B first.
//   - The fused ranking should place whichever chunk each list picked at
//     the top, and the SECOND place should also contain the other chunk.
//     A strict ordering test would be flaky because RRF ties depend on the
//     score spread; we check that BOTH chunks appear in the result.
func TestIntegration_HybridRRFOrder(t *testing.T) {
	if intPool == nil {
		t.Skip("BIDWRITER_TEST_DSN not set; skipping integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tid := uuid.New()
	_, err := intPool.Exec(ctx,
		`INSERT INTO tenants (id, name, slug) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO NOTHING`,
		tid, "test-rrf-"+tid.String()[:8], "test-rrf-"+tid.String()[:8])
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, tid)
	})
	ctx = tenant.WithTenant(ctx, tid.String())

	st := New(intPool)
	mat := &model.KBMaterial{
		Category: "case",
		Title:    "Hybrid RRF " + tid.String()[:8],
		Content:  "Hybrid retrieval smoke test material.",
	}
	require.NoError(t, st.CreateMaterial(ctx, mat))
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM kb_materials WHERE id = $1`, mat.ID)
	})

	// A: vector needle (axis 0 spike)
	a := make([]float32, 1536)
	a[0] = 1.0
	// B: BM25 needle (text contains the query terms "antenna pattern gain")
	// Use a random embedding so B does not match the vector query at all.
	b := make([]float32, 1536)
	b[1] = 1.0

	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "vector-only needle", ChunkIndex: 0,
	}, a))
	require.NoError(t, st.CreateChunkWithVec(ctx, &model.KBChunk{
		MaterialID: mat.ID, TenantID: tid, Content: "antenna pattern gain specifications", ChunkIndex: 1,
	}, b))
	t.Cleanup(func() {
		_, _ = intPool.Exec(context.Background(), `DELETE FROM kb_chunks WHERE material_id = $1`, mat.ID)
	})

	// Vector query: should pick A only.
	qVec := make([]float32, 1536)
	qVec[0] = 1.0
	vecHits, err := st.SearchChunks(ctx, tid, qVec, 5, "")
	require.NoError(t, err)
	require.NotEmpty(t, vecHits)

	// BM25 query: should pick B only.
	bmHits, err := st.SearchChunksBM25(ctx, tid, "antenna pattern gain", 5, "")
	require.NoError(t, err)
	require.NotEmpty(t, bmHits)

	// Fusion: both A and B should appear in the merged list.
	merged := st.RRFuse(vecHits, bmHits, 5)
	require.GreaterOrEqual(t, len(merged), 2, "RRF should preserve both chunks")

	gotA, gotB := false, false
	for _, h := range merged {
		if h.Content == "vector-only needle" {
			gotA = true
		}
		if h.Content == "antenna pattern gain specifications" {
			gotB = true
		}
	}
	assert.True(t, gotA, "merged list should contain the vector-only chunk")
	assert.True(t, gotB, "merged list should contain the BM25-only chunk")
}
