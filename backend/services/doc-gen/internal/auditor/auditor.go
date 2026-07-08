// Package auditor 实现内审：合规规则 + 跨章一致性 + 图表引用核对。
// 详见 docs/doc-gen/algorithms.md 相关章节。
package auditor

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

// Auditor 实现 core.Auditor 接口。
type Auditor struct {
	Log *slog.Logger
}

// New 创建 Auditor。
func New(log *slog.Logger) *Auditor {
	return &Auditor{Log: log}
}

// Audit 执行完整审计，返回所有发现的问题。
func (a *Auditor) Audit(ctx context.Context, pkg *core.BidPackage, profile *core.RFPProfile) ([]core.AuditIssue, error) {
	log := a.Log
	if log == nil {
		log = slog.Default()
	}

	var issues []core.AuditIssue

	// 1. 废标条款合规检查
	issues = append(issues, a.checkStarClauses(pkg, profile)...)

	// 2. 评分项覆盖率检查
	issues = append(issues, a.checkScoringCoverage(pkg, profile)...)

	// 3. 跨章一致性检查
	issues = append(issues, a.checkCrossChapter(pkg)...)

	// 4. 图表引用核对
	issues = append(issues, a.checkFigureRefs(pkg)...)

	// 5. 暗标规则检查
	issues = append(issues, a.checkDarkRules(pkg, profile)...)

	log.Info("auditor: 完成", "issues", len(issues))
	return issues, nil
}

// checkStarClauses 检查★号废标条款是否被响应。
func (a *Auditor) checkStarClauses(pkg *core.BidPackage, profile *core.RFPProfile) []core.AuditIssue {
	var issues []core.AuditIssue
	for _, sc := range profile.StarClauses {
		found := false
		for _, ch := range pkg.Chapters {
			if strings.Contains(ch.Content.Markdown, sc.Clause) ||
				strings.Contains(ch.Content.Markdown, sc.ID) {
				found = true
				break
			}
		}
		if !found {
			issues = append(issues, core.AuditIssue{
				ID:           uuid.New(),
				BidID:        pkg.ID,
				ChapterTitle: "全局",
				Severity:     sc.Severity,
				Dimension:    "compliance",
				Issue:        fmt.Sprintf("★号废标条款未被响应: %s", sc.Clause),
				Suggestion:   "必须在标书中明确响应此废标条款，否则投标将被否决",
				Evidence:     sc.Section,
			})
		}
	}
	return issues
}

// checkScoringCoverage 检查评分项是否被覆盖。
func (a *Auditor) checkScoringCoverage(pkg *core.BidPackage, profile *core.RFPProfile) []core.AuditIssue {
	var issues []core.AuditIssue
	// 收集所有已响应的评分项 ID
	responded := make(map[string]bool)
	for _, ch := range pkg.Chapters {
		for _, id := range ch.Spec.ScoringItemIDs {
			responded[id] = true
		}
	}
	// 检查每个叶子评分项
	var checkItem func(si core.ScoringItem)
	checkItem = func(si core.ScoringItem) {
		if len(si.Children) == 0 {
			if !responded[si.ID] {
				issues = append(issues, core.AuditIssue{
					ID:           uuid.New(),
					BidID:        pkg.ID,
					ChapterTitle: "全局",
					Severity:     "major",
					Dimension:    "completeness",
					Issue:        fmt.Sprintf("评分项未被响应: %s (权重%.1f)", si.Name, si.Weight),
					Suggestion:   fmt.Sprintf("建议增加章节响应此评分项，权重%.1f", si.Weight),
				})
			}
			return
		}
		for _, c := range si.Children {
			checkItem(c)
		}
	}
	for _, si := range profile.ScoringTree {
		checkItem(si)
	}
	return issues
}

// checkCrossChapter 检查跨章一致性。
func (a *Auditor) checkCrossChapter(pkg *core.BidPackage) []core.AuditIssue {
	var issues []core.AuditIssue

	// 公司名称一致性
	var companyNames []string
	nameMap := make(map[string][]string)
	for _, ch := range pkg.Chapters {
		for _, line := range strings.Split(ch.Content.Markdown, "\n") {
			if strings.Contains(line, "公司") || strings.Contains(line, "有限公司") || strings.Contains(line, "集团") {
				trimmed := strings.TrimSpace(line)
				if len(trimmed) > 0 && len(trimmed) < 100 {
					companyNames = append(companyNames, trimmed)
					nameMap[trimmed] = append(nameMap[trimmed], ch.Spec.Title)
				}
			}
		}
	}
	if len(companyNames) > 1 {
		first := companyNames[0]
		for _, name := range companyNames[1:] {
			if name != first && !strings.Contains(name, first) && !strings.Contains(first, name) {
				issues = append(issues, core.AuditIssue{
					ID:           uuid.New(),
					BidID:        pkg.ID,
					ChapterTitle: "跨章节一致性",
					Severity:     "major",
					Dimension:    "consistency",
					Issue:        fmt.Sprintf("公司名称不一致: \"%s\" vs \"%s\"", first, name),
					Suggestion:   "请确认正确的公司全称，确保所有章节使用统一的名称",
					Evidence:     fmt.Sprintf("出现在: %s vs %s", nameMap[first], nameMap[name]),
				})
			}
		}
	}

	// 内容重复度检查
	for i := 0; i < len(pkg.Chapters); i++ {
		for j := i + 1; j < len(pkg.Chapters); j++ {
			sim := jaccardSimilarity(pkg.Chapters[i].Content.Markdown, pkg.Chapters[j].Content.Markdown)
			if sim > 0.8 {
				issues = append(issues, core.AuditIssue{
					ID:           uuid.New(),
					BidID:        pkg.ID,
					ChapterTitle: "跨章节一致性",
					Severity:     "major",
					Dimension:    "consistency",
					Issue:        fmt.Sprintf("章节 \"%s\" 和 \"%s\" 内容重复度较高 (%.0f%%)", pkg.Chapters[i].Spec.Title, pkg.Chapters[j].Spec.Title, sim*100),
					Suggestion:   "请检查两个章节的内容，确保各有侧重点",
				})
			}
		}
	}

	return issues
}

// checkFigureRefs 检查图表占位符是否都有对应的渲染结果。
func (a *Auditor) checkFigureRefs(pkg *core.BidPackage) []core.AuditIssue {
	var issues []core.AuditIssue
	// 收集所有渲染成功的图表 spec ID
	rendered := make(map[uuid.UUID]bool)
	for _, fig := range pkg.Figures {
		if fig.Status == "ok" {
			rendered[fig.SpecID] = true
		}
	}
	// 检查章节中的图表占位符
	for _, ch := range pkg.Chapters {
		for _, spec := range ch.Spec.FigureSpecs {
			if !rendered[spec.ID] {
				issues = append(issues, core.AuditIssue{
					ID:           uuid.New(),
					BidID:        pkg.ID,
					ChapterTitle: ch.Spec.Title,
					Severity:     "minor",
					Dimension:    "format",
					Issue:        fmt.Sprintf("图表未渲染或渲染失败: %s", spec.Caption),
					Suggestion:   "请检查渲染依赖是否安装，或手动补充图表",
				})
			}
		}
	}
	return issues
}

// checkDarkRules 检查暗标规则。
func (a *Auditor) checkDarkRules(pkg *core.BidPackage, profile *core.RFPProfile) []core.AuditIssue {
	var issues []core.AuditIssue
	for _, rule := range profile.DarkRules {
		// 如果暗标规则要求不出现公司名称，检查正文
		if strings.Contains(rule, "公司名称") || strings.Contains(rule, "匿名") {
			for _, ch := range pkg.Chapters {
				for _, line := range strings.Split(ch.Content.Markdown, "\n") {
					if (strings.Contains(line, "公司") || strings.Contains(line, "有限")) &&
						strings.Contains(rule, "不得") {
						issues = append(issues, core.AuditIssue{
							ID:           uuid.New(),
							BidID:        pkg.ID,
							ChapterTitle: ch.Spec.Title,
							Severity:     "critical",
							Dimension:    "compliance",
							Issue:        fmt.Sprintf("暗标规则违反: %s（在正文中发现了可能的公司名称）", rule),
							Suggestion:   "暗标部分不得出现公司名称，请使用[投标人]等通用称谓",
							Evidence:     strings.TrimSpace(line),
						})
					}
				}
			}
		}
	}
	return issues
}

// jaccardSimilarity 计算两个字符串的 Jaccard 相似度。
func jaccardSimilarity(a, b string) float64 {
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
