package service

import (
	"strings"
	"testing"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/google/uuid"
)

func strPtr(s string) *string { return &s }

func TestChapterAuditor_EmptyContent(t *testing.T) {
	a := &ChapterAuditor{}
	chapter := &store.ChapterInfo{
		ID:    uuid.New(),
		Title: "Ch1",
		// Content nil → empty-content issue
	}
	issues := a.AuditChapter(t.Context(), uuid.New(), uuid.New(), chapter)
	if len(issues) != 1 {
		t.Fatalf("len=%d, want 1", len(issues))
	}
	if issues[0].Severity != model.SeverityCritical {
		t.Errorf("severity=%s, want critical", issues[0].Severity)
	}
	if !strings.Contains(issues[0].Issue, "章节内容为空") {
		t.Errorf("issue text unexpected: %s", issues[0].Issue)
	}
}

func TestChapterAuditor_EmptyContentSkipped(t *testing.T) {
	a := &ChapterAuditor{}
	chapter := &store.ChapterInfo{ID: uuid.New(), Title: "Ch1", Status: "skipped"}
	issues := a.AuditChapter(t.Context(), uuid.New(), uuid.New(), chapter)
	if len(issues) != 0 {
		t.Errorf("skipped chapters should produce no issues, got %d", len(issues))
	}
}

func TestChapterAuditor_ShortContent(t *testing.T) {
	a := &ChapterAuditor{}
	chapter := &store.ChapterInfo{
		ID:           uuid.New(),
		Title:        "Ch1",
		Content:      strPtr("太短了"),
		MinWordCount: 500,
	}
	issues := a.AuditChapter(t.Context(), uuid.New(), uuid.New(), chapter)
	if len(issues) == 0 {
		t.Fatal("expected at least one issue (low word count)")
	}
	found := false
	for _, is := range issues {
		if strings.Contains(is.Issue, "字数不足") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 字数不足 issue, got %#v", issues)
	}
}

func TestChapterAuditor_TriggersDarkBidRule(t *testing.T) {
	a := &ChapterAuditor{}
	// 100+ words so word count passes; dark-bid content triggers compliance.
	words := make([]string, 200)
	for i := range words {
		words[i] = "字"
	}
	content := strings.Join(words, "") + " 期望中标"
	chapter := &store.ChapterInfo{
		ID: uuid.New(), Title: "Ch1",
		Content:      &content,
		MinWordCount: 0,
	}
	issues := a.AuditChapter(t.Context(), uuid.New(), uuid.New(), chapter)
	if len(issues) == 0 {
		t.Fatal("expected compliance issue")
	}
}

func TestRejectionChecker_FlagsForbidden(t *testing.T) {
	r := &RejectionChecker{}
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "Ch1", Content: strPtr("本章节内容完全合规，没有伪造数据。")},
	}
	issues := r.CheckRejectionCriteria(t.Context(), uuid.New(), uuid.New(), ch)
	if len(issues) == 0 {
		t.Fatal("expected flag for 伪造")
	}
}

func TestRejectionChecker_NegativeSkipped(t *testing.T) {
	r := &RejectionChecker{}
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "Ch1", Content: strPtr("本章节内容完全合规。")},
	}
	if got := r.CheckRejectionCriteria(t.Context(), uuid.New(), uuid.New(), ch); len(got) != 0 {
		t.Errorf("expected 0 issues, got %d", len(got))
	}
}

func TestRejectionChecker_StarredWithResponseOK(t *testing.T) {
	r := &RejectionChecker{}
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "Ch1", Content: strPtr("★项已满足响应要求。")},
	}
	// "已满足" should suppress the starred warning.
	got := r.CheckRejectionCriteria(t.Context(), uuid.New(), uuid.New(), ch)
	for _, is := range got {
		if strings.Contains(is.Issue, "★号条款") || strings.Contains(is.Issue, "★条款") {
			t.Errorf("expected no starred warning when 已满足 present, got %q", is.Issue)
		}
	}
}

func TestRejectionChecker_NilContentSkipped(t *testing.T) {
	r := &RejectionChecker{}
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "Ch1", Content: nil},
	}
	if got := r.CheckRejectionCriteria(t.Context(), uuid.New(), uuid.New(), ch); len(got) != 0 {
		t.Errorf("expected 0 issues for nil content, got %d", len(got))
	}
}

func TestCrossAuditor_DetectsDuplicateContent(t *testing.T) {
	a := &CrossAuditor{}
	long := strings.Repeat("投标技术方案包括系统架构模块设计实施步骤质量保证措施。", 5)
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "A", Content: &long},
		{ID: uuid.New(), Title: "B", Content: &long},
	}
	issues := a.AuditCrossChapter(t.Context(), uuid.New(), uuid.New(), ch)
	if len(issues) == 0 {
		t.Fatal("expected duplicate-content issue")
	}
}

func TestCrossAuditor_ShortContentNotCompared(t *testing.T) {
	a := &CrossAuditor{}
	short := "短"
	ch := []store.ChapterInfo{
		{ID: uuid.New(), Title: "A", Content: &short},
		{ID: uuid.New(), Title: "B", Content: &short},
	}
	if got := a.AuditCrossChapter(t.Context(), uuid.New(), uuid.New(), ch); len(got) != 0 {
		t.Errorf("expected 0 issues, got %d", len(got))
	}
}

func TestSimilarStrings(t *testing.T) {
	cases := []struct {
		a, b string
		min  float64
	}{
		{"", "", 0},
		{"a b c", "a b c", 0.99},
		{"a b c", "a b d", 0.4},
		{"hello world", "goodbye world", 0.2},
	}
	for _, c := range cases {
		got := similarStrings(c.a, c.b)
		if got < c.min {
			t.Errorf("similarStrings(%q, %q) = %f, want >= %f", c.a, c.b, got, c.min)
		}
	}
}
