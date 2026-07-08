package store

import (
	"context"
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLite(":memory:")
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	if err := s.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSQLiteStore_ChunkCRUD(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	chunks := []core.Chunk{
		{ID: uuid.New(), FilePath: "/test/a.txt", Category: "rfp", Offset: 0, Text: "招标文件内容"},
		{ID: uuid.New(), FilePath: "/test/a.txt", Category: "rfp", Offset: 1, Text: "第二段"},
		{ID: uuid.New(), FilePath: "/test/b.txt", Category: "technical", Offset: 0, Text: "技术方案"},
	}
	if err := s.SaveChunks(ctx, chunks); err != nil {
		t.Fatalf("SaveChunks: %v", err)
	}

	// ListChunks 全部
	all, err := s.ListChunks(ctx, "")
	if err != nil {
		t.Fatalf("ListChunks: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(all))
	}

	// ListChunks 按分类
	rfp, _ := s.ListChunks(ctx, "rfp")
	if len(rfp) != 2 {
		t.Fatalf("expected 2 rfp chunks, got %d", len(rfp))
	}

	// DeleteChunksByFile
	if err := s.DeleteChunksByFile(ctx, "/test/a.txt"); err != nil {
		t.Fatalf("DeleteChunksByFile: %v", err)
	}
	remaining, _ := s.ListChunks(ctx, "")
	if len(remaining) != 1 {
		t.Fatalf("expected 1 remaining, got %d", len(remaining))
	}
}

func TestSQLiteStore_SearchChunks(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	chunks := []core.Chunk{
		{ID: uuid.New(), FilePath: "/a.txt", Category: "tech", Offset: 0, Text: "Go语言开发",
			Embedding: []float32{1.0, 0.0, 0.0}},
		{ID: uuid.New(), FilePath: "/b.txt", Category: "tech", Offset: 0, Text: "Python数据分析",
			Embedding: []float32{0.0, 1.0, 0.0}},
		{ID: uuid.New(), FilePath: "/c.txt", Category: "tech", Offset: 0, Text: "Go微服务架构",
			Embedding: []float32{0.9, 0.1, 0.0}},
	}
	s.SaveChunks(ctx, chunks)

	// 搜索与 [1,0,0] 最相似的
	results, err := s.SearchChunks(ctx, []float32{1.0, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// 第一条应该是 "Go语言开发"（完全匹配）
	if results[0].Text != "Go语言开发" {
		t.Fatalf("expected first result 'Go语言开发', got %q", results[0].Text)
	}
}

func TestSQLiteStore_RFPProfile(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	profile := &core.RFPProfile{
		ID:          uuid.New(),
		ProjectName: "测试项目",
		Industry:    "IT",
		Issuer:      "测试采购人",
		ScoringTree: []core.ScoringItem{
			{ID: "s1", Name: "技术", Weight: 60},
		},
		StarClauses: []core.StarClause{
			{ID: "st1", Clause: "废标条款1", Severity: "critical"},
		},
		DarkRules: []string{"暗标规则1"},
	}
	if err := s.SaveRFPProfile(ctx, profile); err != nil {
		t.Fatalf("SaveRFPProfile: %v", err)
	}

	got, err := s.GetRFPProfile(ctx, profile.ID)
	if err != nil {
		t.Fatalf("GetRFPProfile: %v", err)
	}
	if got.ProjectName != "测试项目" {
		t.Fatalf("expected project '测试项目', got %q", got.ProjectName)
	}
	if len(got.ScoringTree) != 1 || got.ScoringTree[0].Name != "技术" {
		t.Fatalf("scoring tree mismatch: %+v", got.ScoringTree)
	}
	if len(got.StarClauses) != 1 {
		t.Fatalf("expected 1 star clause, got %d", len(got.StarClauses))
	}
	if len(got.DarkRules) != 1 || got.DarkRules[0] != "暗标规则1" {
		t.Fatalf("dark rules mismatch: %+v", got.DarkRules)
	}
}

func TestSQLiteStore_PatternSearch(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 存 3 个模式
	patterns := []core.BidPattern{
		{ID: uuid.New(), Industry: "IT", RFPType: "公开招标", QualityScore: 90, Label: "won"},
		{ID: uuid.New(), Industry: "IT", RFPType: "公开招标", QualityScore: 70, Label: "lost"},
		{ID: uuid.New(), Industry: "建筑", RFPType: "公开招标", QualityScore: 80, Label: "won"},
	}
	for _, p := range patterns {
		s.SavePattern(ctx, &p)
	}

	// 搜索 IT 行业
	results, err := s.SearchPatterns(ctx, "IT", "公开招标", 5)
	if err != nil {
		t.Fatalf("SearchPatterns: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 IT patterns, got %d", len(results))
	}
	// won 应排在 lost 前面
	if results[0].Label != "won" {
		t.Fatalf("expected first result label 'won', got %q", results[0].Label)
	}
}

func TestSQLiteStore_BidPackage(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	pkg := &core.BidPackage{
		ID:           uuid.New(),
		RFPID:        uuid.New(),
		OutlineID:    uuid.New(),
		QualityScore: 85.5,
		Label:        "draft",
		OutputPath:   "/tmp/test.docx",
	}
	if err := s.SaveBidPackage(ctx, pkg); err != nil {
		t.Fatalf("SaveBidPackage: %v", err)
	}

	got, err := s.GetBidPackage(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("GetBidPackage: %v", err)
	}
	if got.QualityScore != 85.5 {
		t.Fatalf("expected quality 85.5, got %f", got.QualityScore)
	}

	list, _ := s.ListBidPackages(ctx)
	if len(list) != 1 {
		t.Fatalf("expected 1 package, got %d", len(list))
	}
}
