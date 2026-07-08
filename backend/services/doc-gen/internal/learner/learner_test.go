package learner

import (
	"math/rand"
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

func TestQualityScore_FullCoverage(t *testing.T) {
	l := &Learner{}
	pkg := &core.BidPackage{
		Chapters: []core.Chapter{
			{Spec: core.ChapterSpec{Title: "技术", TargetWords: 1000, ScoringItemIDs: []string{"s1"},
				FigureSpecs: []core.FigureSpec{{ID: uuid.New()}}},
				Content: core.ChapterContent{WordCount: 1000, EvidenceRefs: []string{"ref1"}, Markdown: "内容"}},
		},
	}
	profile := &core.RFPProfile{ScoringTree: []core.ScoringItem{
		{ID: "s1", Name: "技术", Weight: 100},
	}}
	score := l.qualityScore(pkg, profile)
	if score < 50 {
		t.Fatalf("expected score >= 50, got %.1f", score)
	}
}

func TestQualityScore_ZeroCoverage(t *testing.T) {
	l := &Learner{}
	pkg := &core.BidPackage{
		Chapters: []core.Chapter{
			{Spec: core.ChapterSpec{Title: "无关章节"}, Content: core.ChapterContent{Markdown: "内容"}},
		},
	}
	profile := &core.RFPProfile{ScoringTree: []core.ScoringItem{
		{ID: "s1", Name: "技术", Weight: 100},
	}}
	score := l.qualityScore(pkg, profile)
	if score > 50 {
		t.Fatalf("expected score <= 50 for zero coverage, got %.1f", score)
	}
}

func TestScoringCoverage(t *testing.T) {
	l := &Learner{}
	pkg := &core.BidPackage{
		Chapters: []core.Chapter{
			{Spec: core.ChapterSpec{ScoringItemIDs: []string{"s1", "s2"}}},
		},
	}
	profile := &core.RFPProfile{ScoringTree: []core.ScoringItem{
		{ID: "s1", Name: "A", Weight: 50},
		{ID: "s2", Name: "B", Weight: 30},
		{ID: "s3", Name: "C", Weight: 20},
	}}
	cov := l.scoringCoverage(pkg, profile)
	if cov < 0.65 || cov > 0.69 {
		t.Fatalf("expected coverage ~0.667, got %.3f", cov)
	}
}

func TestWordCountCompliance(t *testing.T) {
	l := &Learner{}
	pkg := &core.BidPackage{
		Chapters: []core.Chapter{
			{Spec: core.ChapterSpec{TargetWords: 1000}, Content: core.ChapterContent{WordCount: 600}},
			{Spec: core.ChapterSpec{TargetWords: 1000}, Content: core.ChapterContent{WordCount: 300}},
		},
	}
	comp := l.wordCountCompliance(pkg)
	// 第一章 600 >= 500(半数) → compliant
	// 第二章 300 < 500 → not compliant
	if comp < 0.49 || comp > 0.51 {
		t.Fatalf("expected compliance ~0.5, got %.3f", comp)
	}
}

func TestBetaSample(t *testing.T) {
	rng := newTestRNG()
	for i := 0; i < 100; i++ {
		s := betaSample(1, 1, rng)
		if s < 0 || s > 1 {
			t.Fatalf("betaSample out of [0,1]: %f", s)
		}
	}
	// Beta(100, 1) 应接近 1
	s := betaSample(100, 1, rng)
	if s < 0.8 {
		t.Fatalf("Beta(100,1) sample too low: %f", s)
	}
}

func TestExtractPattern(t *testing.T) {
	l := &Learner{}
	pkg := &core.BidPackage{
		ID: uuid.New(),
		Chapters: []core.Chapter{
			{Spec: core.ChapterSpec{Title: "技术方案", Level: 1}, Content: core.ChapterContent{WordCount: 5000}},
			{Spec: core.ChapterSpec{Title: "商务方案", Level: 1}, Content: core.ChapterContent{WordCount: 3000}},
		},
		Figures: []core.Illustration{{Status: "ok", RenderEngine: "mmdc"}},
	}
	profile := &core.RFPProfile{Industry: "IT", RFPType: "公开招标"}

	pattern := l.extractPattern(pkg, profile)
	if pattern.Industry != "IT" {
		t.Fatalf("expected industry 'IT', got %q", pattern.Industry)
	}
	if pattern.OutlineTemplate == "" {
		t.Fatalf("expected non-empty outline template")
	}
	if !contains(pattern.ChartDistribution, "mmdc") {
		t.Fatalf("expected chart distribution to contain 'mmdc'")
	}
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newTestRNG() *rand.Rand {
	return rand.New(rand.NewSource(42))
}
