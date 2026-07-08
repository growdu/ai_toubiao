package analyzer

import (
	"strings"
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

func TestNormalizeWeights_ChildrenSum(t *testing.T) {
	node := &core.ScoringItem{
		Weight: 60,
		Children: []core.ScoringItem{
			{ID: "a", Weight: 30},
			{ID: "b", Weight: 10},
			{ID: "c", Weight: 20},
		},
	}
	normalizeWeights(node)
	sum := 0.0
	for _, c := range node.Children {
		sum += c.Weight
	}
	// 子项之和应等于父项权重
	if sum < 59.9 || sum > 60.1 {
		t.Fatalf("children weights sum = %.2f, expected 60", sum)
	}
}

func TestNormalizeWeights_Nested(t *testing.T) {
	node := &core.ScoringItem{
		Weight: 100,
		Children: []core.ScoringItem{
			{ID: "a", Weight: 40, Children: []core.ScoringItem{
				{ID: "a1", Weight: 10},
				{ID: "a2", Weight: 30},
			}},
			{ID: "b", Weight: 60},
		},
	}
	normalizeWeights(node)
	// a 的子项应归一化到 40
	sumA := node.Children[0].Children[0].Weight + node.Children[0].Children[1].Weight
	if sumA < 39.9 || sumA > 40.1 {
		t.Fatalf("children of 'a' sum = %.2f, expected 40", sumA)
	}
}

func TestRuleEnrich_StarClauses(t *testing.T) {
	a := &Analyzer{}
	profile := &core.RFPProfile{}
	text := `一些文本内容
★1. 投标保证金须按规定缴纳，否则废标
★2. 投标文件须密封递交，否则废标
普通文本
`
	a.ruleEnrich(profile, text)
	if len(profile.StarClauses) < 2 {
		t.Fatalf("expected >=2 star clauses, got %d", len(profile.StarClauses))
	}
	// 检查是否包含废标条款
	found := false
	for _, sc := range profile.StarClauses {
		if strings.Contains(sc.Clause, "投标保证金") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find '投标保证金' star clause")
	}
}

func TestRuleEnrich_DarkRules(t *testing.T) {
	a := &Analyzer{}
	profile := &core.RFPProfile{}
	text := "本项目采用暗标评审方式"
	a.ruleEnrich(profile, text)
	if len(profile.DarkRules) == 0 {
		t.Fatalf("expected dark rules to be detected")
	}
}

func TestRuleEnrich_IndustryInference(t *testing.T) {
	a := &Analyzer{}
	profile := &core.RFPProfile{}
	text := "本项目为信息化系统建设，包含软件开发和系统集成"
	a.ruleEnrich(profile, text)
	if profile.Industry != "IT" {
		t.Fatalf("expected industry 'IT', got %q", profile.Industry)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{`some text {"key":"value"} more text`, `{"key":"value"}`},
		{`{"a":1,"b":[2,3]}`, `{"a":1,"b":[2,3]}`},
		{`no json here`, ``},
		{`incomplete {`, ``},
	}
	for _, tt := range tests {
		got := extractJSON(tt.input)
		if got != tt.want {
			t.Errorf("extractJSON(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncateText(t *testing.T) {
	text := strings.Repeat("中", 200)
	truncated := truncateText(text, 100)
	if len([]rune(truncated)) > 110 {
		t.Fatalf("truncated text too long: %d runes", len([]rune(truncated)))
	}
	if !strings.Contains(truncated, "截断") {
		t.Fatalf("expected truncation marker")
	}
}
