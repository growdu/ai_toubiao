package planner

import (
	"testing"

	"github.com/google/uuid"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

func TestRuleBasedPlan(t *testing.T) {
	p := New(nil, nil)
	profile := &core.RFPProfile{
		ScoringTree: []core.ScoringItem{
			{ID: "s1", Name: "技术方案", Weight: 60},
			{ID: "s2", Name: "商务方案", Weight: 20, Children: []core.ScoringItem{
				{ID: "s2a", Name: "企业资质", Weight: 10},
				{ID: "s2b", Name: "类似业绩", Weight: 10},
			}},
		},
	}
	chapters := p.ruleBasedPlan(profile)
	if len(chapters) < 5 { // 封面+目录+公司简介+技术方案+商务方案+企业资质+类似业绩
		t.Fatalf("expected >=5 chapters, got %d", len(chapters))
	}
	// 检查有前置章节
	if chapters[0].Title != "封面" {
		t.Fatalf("expected first chapter '封面', got %q", chapters[0].Title)
	}
}

func TestAllocateWords(t *testing.T) {
	p := New(nil, nil)
	p.TotalBudget = 60000
	chapters := []core.ChapterSpec{
		{ID: newUUID(), Title: "封面", Level: 1, Order: 0},
		{ID: newUUID(), Title: "技术方案", Level: 1, Order: 1, ScoringItemIDs: []string{"s1"}},
		{ID: newUUID(), Title: "商务方案", Level: 1, Order: 2, ScoringItemIDs: []string{"s2", "s3"}},
		{ID: newUUID(), Title: "目录", Level: 1, Order: 3},
	}
	p.allocateWords(chapters)
	// 前置章节应该有固定少量字数
	if chapters[0].TargetWords != 500 {
		t.Fatalf("expected front matter 500 words, got %d", chapters[0].TargetWords)
	}
	// 正文章节应该有更多字数
	if chapters[1].TargetWords < 800 {
		t.Fatalf("expected technical chapter >=800 words, got %d", chapters[1].TargetWords)
	}
}

func TestInferFigureNeed(t *testing.T) {
	tests := []struct {
		title string
		want  string
	}{
		{"技术方案", "方案流程图"},
		{"组织架构", "组织架构图"},
		{"进度计划", "进度计划表"},
		{"设备清单", "设备清单表"},
		{"公司简介", ""},
	}
	profile := &core.RFPProfile{}
	for _, tt := range tests {
		got := inferFigureNeed(tt.title, profile)
		if got != tt.want {
			t.Errorf("inferFigureNeed(%q) = %q, want %q", tt.title, got, tt.want)
		}
	}
}

func TestInferFigureType(t *testing.T) {
	tests := []struct {
		req  string
		want core.FigureType
	}{
		{"流程图", core.FigureMermaid},
		{"架构图", core.FigureMermaid},
		{"对比表", core.FigureTable},
		{"清单表", core.FigureTable},
		{"趋势图", core.FigureDataChart},
	}
	for _, tt := range tests {
		got := inferFigureType(tt.req)
		if got != tt.want {
			t.Errorf("inferFigureType(%q) = %q, want %q", tt.req, got, tt.want)
		}
	}
}

func TestIsFrontMatter(t *testing.T) {
	tests := []struct {
		title string
		want  bool
	}{
		{"封面", true},
		{"目录", true},
		{"技术方案", false},
		{"投标声明", true},
	}
	for _, tt := range tests {
		got := isFrontMatter(tt.title)
		if got != tt.want {
			t.Errorf("isFrontMatter(%q) = %v, want %v", tt.title, got, tt.want)
		}
	}
}

func TestExtractJSONArray(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`text [{"a":1}] more`, `[{"a":1}]`},
		{`[1,2,3]`, `[1,2,3]`},
		{`no array`, ``},
	}
	for _, tt := range tests {
		got := extractJSONArray(tt.input)
		if got != tt.want {
			t.Errorf("extractJSONArray(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func newUUID() uuid.UUID { return uuid.New() }
