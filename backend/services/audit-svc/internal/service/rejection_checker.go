package service

import (
	"context"
	"strings"

	"github.com/bidwriter/services/audit-svc/internal/model"
	"github.com/bidwriter/services/audit-svc/internal/store"
	"github.com/google/uuid"
)

// RejectionChecker scans for rejection criteria (废标项).
type RejectionChecker struct {
	store *store.Store
}

func NewRejectionChecker(s *store.Store) *RejectionChecker {
	return &RejectionChecker{store: s}
}

// CheckRejectionCriteria checks if content violates any rejection criteria.
func (r *RejectionChecker) CheckRejectionCriteria(ctx context.Context, bidJobID, tenantID uuid.UUID, chapters []store.ChapterInfo) []*model.AuditIssue {
	var issues []*model.AuditIssue

	// Rejection patterns that absolutely cannot appear
	rejectionPatterns := []struct {
		pattern    string
		issue      string
		suggestion string
	}{
		{"伪造", "内容涉嫌伪造或虚构", "请使用真实的资质、业绩和数据"},
		{"假的", "内容存在虚假信息风险", "请确保所有信息真实准确"},
		{"贿赂", "内容涉及商业贿赂", "标书内容必须完全合规，不得包含任何违法内容"},
		{"围标", "内容涉及围标串标", "请确保投标过程完全合规"},
	}

	// ★ or ★★ or ★★★ marked items (starred requirements in RFP)
	starredPatterns := []struct {
		pattern    string
		issue      string
		suggestion string
	}{
		{"★", "RFP中有★号条款未响应", "必须 100% 响应所有★号条款"},
		{"★", "RFP中有★★条款未充分响应", "★★条款需要详细充分说明"},
	}

	for _, ch := range chapters {
		if ch.Content == nil {
			continue
		}
		content := *ch.Content

		// Check rejection patterns
		for _, rp := range rejectionPatterns {
			if strings.Contains(content, rp.pattern) {
				issues = append(issues, &model.AuditIssue{
					BidJobID:     bidJobID,
					TenantID:     tenantID,
					ChapterID:    &ch.ID,
					ChapterTitle: ch.Title,
					Severity:     model.SeverityCritical,
					Dimension:    model.DimensionCompliance,
					Issue:        rp.issue + ": " + rp.pattern,
					Suggestion:   rp.suggestion,
				})
			}
		}

		// Check starred requirements
		// Note: In production, this would compare against the actual RFP parse result
		// which contains starred items. Here we just scan for common patterns.
		for _, sp := range starredPatterns {
			if strings.Contains(content, sp.pattern) {
				// Check if this ★ appears in a responsive context
				if !strings.Contains(content, "已满足") && !strings.Contains(content, "满足") {
					issues = append(issues, &model.AuditIssue{
						BidJobID:     bidJobID,
						TenantID:     tenantID,
						ChapterID:    &ch.ID,
						ChapterTitle: ch.Title,
						Severity:     model.SeverityCritical,
						Dimension:    model.DimensionCompliance,
						Issue:        sp.issue,
						Suggestion:   sp.suggestion,
					})
				}
			}
		}
	}

	return issues
}
