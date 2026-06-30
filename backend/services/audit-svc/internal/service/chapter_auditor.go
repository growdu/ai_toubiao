package service

import (
	"context"
	"strconv"
	"strings"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/rules"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/google/uuid"
)

// ChapterAuditor performs chapter-level content audit.
type ChapterAuditor struct {
	store *store.Store
}

func NewChapterAuditor(s *store.Store) *ChapterAuditor {
	return &ChapterAuditor{store: s}
}

// AuditChapter reviews a single chapter for issues.
func (a *ChapterAuditor) AuditChapter(ctx context.Context, bidJobID, tenantID uuid.UUID, chapter *store.ChapterInfo) []*model.AuditIssue {
	var issues []*model.AuditIssue

	if chapter.Content == nil || strings.TrimSpace(*chapter.Content) == "" {
		if chapter.Status != "skipped" {
			issues = append(issues, &model.AuditIssue{
				BidJobID:     bidJobID,
				TenantID:     tenantID,
				ChapterID:    &chapter.ID,
				ChapterTitle: chapter.Title,
				Severity:     model.SeverityCritical,
				Dimension:    model.DimensionCompleteness,
				Issue:        "章节内容为空",
				Suggestion:   "请补充章节内容或标记为跳过",
			})
		}
		return issues
	}

	content := *chapter.Content

	// Check word count
	wordCount := countWords(content)
	if wordCount < chapter.MinWordCount {
		issues = append(issues, &model.AuditIssue{
			BidJobID:     bidJobID,
			TenantID:     tenantID,
			ChapterID:    &chapter.ID,
			ChapterTitle: chapter.Title,
			Severity:     model.SeverityMajor,
			Dimension:    model.DimensionCompleteness,
			Issue:        "章节字数不足，最少需要 " + itoa(chapter.MinWordCount) + " 字，当前 " + itoa(wordCount) + " 字",
			Suggestion:   "请扩充章节内容以满足最低字数要求",
		})
	}

	// Check compliance rules
	for _, rule := range rules.ComplianceRules {
		if issuesFound := rule.Check(chapter.Title, content); issuesFound != "" {
			issues = append(issues, &model.AuditIssue{
				BidJobID:     bidJobID,
				TenantID:     tenantID,
				ChapterID:    &chapter.ID,
				ChapterTitle: chapter.Title,
				Severity:     model.SeverityCritical,
				Dimension:    model.DimensionCompliance,
				Issue:        issuesFound,
				Suggestion:   rule.Suggestion,
				Evidence:     rule.EvidenceSample,
			})
		}
	}

	return issues
}

func countWords(s string) int {
	words := strings.Fields(s)
	return len(words)
}

func itoa(i int) string {
	return strconv.Itoa(i)
}
