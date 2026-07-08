package auditor

import (
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

func TestCheckStarClauses_Unresponded(t *testing.T) {
	a := &Auditor{}
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Spec: core.ChapterSpec{Title: "第一章"}, Content: core.ChapterContent{Markdown: "一些无关内容"}},
	}}
	profile := &core.RFPProfile{StarClauses: []core.StarClause{
		{ID: "st1", Clause: "投标保证金", Severity: "critical"},
	}}
	issues := a.checkStarClauses(pkg, profile)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Severity != "critical" {
		t.Fatalf("expected severity 'critical', got %q", issues[0].Severity)
	}
}

func TestCheckStarClauses_Responded(t *testing.T) {
	a := &Auditor{}
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Content: core.ChapterContent{Markdown: "本章响应了投标保证金要求"}},
	}}
	profile := &core.RFPProfile{StarClauses: []core.StarClause{
		{ID: "st1", Clause: "投标保证金", Severity: "critical"},
	}}
	issues := a.checkStarClauses(pkg, profile)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestCheckScoringCoverage_AllCovered(t *testing.T) {
	a := &Auditor{}
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Spec: core.ChapterSpec{ScoringItemIDs: []string{"s1", "s2"}}},
	}}
	profile := &core.RFPProfile{ScoringTree: []core.ScoringItem{
		{ID: "s1", Name: "技术", Weight: 60},
		{ID: "s2", Name: "商务", Weight: 40},
	}}
	issues := a.checkScoringCoverage(pkg, profile)
	if len(issues) != 0 {
		t.Fatalf("expected 0 issues, got %d", len(issues))
	}
}

func TestCheckScoringCoverage_Missing(t *testing.T) {
	a := &Auditor{}
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Spec: core.ChapterSpec{ScoringItemIDs: []string{"s1"}}},
	}}
	profile := &core.RFPProfile{ScoringTree: []core.ScoringItem{
		{ID: "s1", Name: "技术", Weight: 60},
		{ID: "s2", Name: "商务", Weight: 40},
	}}
	issues := a.checkScoringCoverage(pkg, profile)
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing s2, got %d", len(issues))
	}
	if issues[0].Severity != "major" {
		t.Fatalf("expected severity 'major', got %q", issues[0].Severity)
	}
}

func TestCheckCrossChapter_CompanyName(t *testing.T) {
	a := &Auditor{}
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Spec: core.ChapterSpec{Title: "第一章"}, Content: core.ChapterContent{Markdown: "XX科技有限公司"}},
		{Spec: core.ChapterSpec{Title: "第二章"}, Content: core.ChapterContent{Markdown: "YY科技有限公司"}},
	}}
	issues := a.checkCrossChapter(pkg)
	if len(issues) == 0 {
		t.Fatalf("expected consistency issue for different company names")
	}
}

func TestCheckCrossChapter_Duplicate(t *testing.T) {
	a := &Auditor{}
	dup := "重复内容重复内容重复内容重复内容重复内容重复内容"
	pkg := &core.BidPackage{ID: uuid.New(), Chapters: []core.Chapter{
		{Spec: core.ChapterSpec{Title: "第一章"}, Content: core.ChapterContent{Markdown: dup}},
		{Spec: core.ChapterSpec{Title: "第二章"}, Content: core.ChapterContent{Markdown: dup}},
	}}
	issues := a.checkCrossChapter(pkg)
	if len(issues) == 0 {
		t.Fatalf("expected duplication issue")
	}
}

func TestJaccardSimilarity(t *testing.T) {
	tests := []struct {
		a, b string
		want float64
	}{
		{"hello world", "hello world", 1.0},
		{"hello world", "goodbye world", 0.333},
		{"", "", 0.0},
		{"abc", "xyz", 0.0},
	}
	for _, tt := range tests {
		got := jaccardSimilarity(tt.a, tt.b)
		if tt.want == 0 && got == 0 {
			continue
		}
		if got < tt.want-0.1 || got > tt.want+0.1 {
			t.Errorf("jaccardSimilarity(%q,%q) = %.3f, want ~%.3f", tt.a, tt.b, got, tt.want)
		}
	}
}
