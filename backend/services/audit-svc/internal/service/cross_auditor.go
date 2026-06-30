package service

import (
	"context"
	"strings"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/google/uuid"
)

// CrossAuditor checks consistency across chapters.
type CrossAuditor struct {
	store *store.Store
}

func NewCrossAuditor(s *store.Store) *CrossAuditor {
	return &CrossAuditor{store: s}
}

// AuditCrossChapter checks consistency between chapters.
func (a *CrossAuditor) AuditCrossChapter(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue {
	var issues []*model.AuditIssue

	// Build a map of company names mentioned across chapters
	var allNames []string
	var nameOccurrences = make(map[string][]string) // name -> chapter titles

	for _, ch := range chapters {
		if ch.Content == nil {
			continue
		}
		content := *ch.Content
		// Extract potential organization names (simplified heuristic)
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			if strings.Contains(line, "公司") || strings.Contains(line, "有限公司") || strings.Contains(line, "集团") {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 0 && len(trimmed) < 100 {
					allNames = append(allNames, trimmed)
					nameOccurrences[trimmed] = append(nameOccurrences[trimmed], ch.Title)
				}
			}
		}
	}

	// Check for inconsistent company name mentions
	if len(allNames) > 1 {
		firstName := allNames[0]
		for _, name := range allNames[1:] {
			if name != firstName && !strings.Contains(name, firstName) && !strings.Contains(firstName, name) {
				chapters1 := nameOccurrences[firstName]
				chapters2 := nameOccurrences[name]
				issues = append(issues, &model.AuditIssue{
					BidJobID:     bidJobID,
					TenantID:     tenantID,
					ChapterTitle: "跨章节一致性",
					Severity:     model.SeverityMajor,
					Dimension:    model.DimensionConsistency,
					Issue:        "公司名称不一致: \"" + firstName + "\" vs \"" + name + "\"",
					Suggestion:   "请确认正确的公司全称，确保所有章节使用统一的名称",
					Evidence:     "出现在: " + strings.Join(chapters1, ", ") + " vs " + strings.Join(chapters2, ", "),
				})
			}
		}
	}

	// Check for duplicate content across chapters (simple similarity check)
	for i := 0; i < len(chapters); i++ {
		if chapters[i].Content == nil {
			continue
		}
		for j := i + 1; j < len(chapters); j++ {
			if chapters[j].Content == nil {
				continue
			}
			c1 := *chapters[i].Content
			c2 := *chapters[j].Content
			if len(c1) > 100 && len(c2) > 100 && similarStrings(c1, c2) > 0.8 {
				issues = append(issues, &model.AuditIssue{
					BidJobID:     bidJobID,
					TenantID:     tenantID,
					ChapterTitle: "跨章节一致性",
					Severity:     model.SeverityMajor,
					Dimension:    model.DimensionConsistency,
					Issue:        "章节 \"" + chapters[i].Title + "\" 和 \"" + chapters[j].Title + "\" 内容重复度较高",
					Suggestion:   "请检查两个章节的内容，确保各有侧重点，避免简单重复",
				})
			}
		}
	}

	return issues
}

// similarStrings returns a rough similarity ratio between two strings (0 to 1).
func similarStrings(a, b string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	// Simple word-based Jaccard similarity
	wordsA := make(map[string]bool)
	for _, w := range strings.Fields(a) {
		wordsA[w] = true
	}
	wordsB := make(map[string]bool)
	for _, w := range strings.Fields(b) {
		wordsB[w] = true
	}
	var intersection, union int
	for w := range wordsA {
		if wordsB[w] {
			intersection++
		}
		union++
	}
	for w := range wordsB {
		if !wordsA[w] {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
