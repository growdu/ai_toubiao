// Package analyzer 的去重辅助函数。
// LLM 和正则可能返回略有差异的★条款文本，需模糊匹配去重。
package analyzer

import (
	"strings"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// normalizeClause 归一化条款文本：去除★号、序号、空白，用于比较。
func normalizeClause(s string) string {
	s = strings.TrimSpace(s)
	// 去除开头的 ★ 和序号（如 "★1." "1." "（1）"）
	s = strings.TrimLeft(s, "★*·")
	s = strings.TrimLeft(s, "0123456789.")
	s = strings.TrimLeft(s, "（）()")
	s = strings.TrimSpace(s)
	// 截取前 30 个字符做比较（足够区分）
	runes := []rune(s)
	if len(runes) > 30 {
		s = string(runes[:30])
	}
	return s
}

// clauseExists 模糊检查条款是否已存在（包含关系）。
func clauseExists(existing []core.StarClause, newClause string) bool {
	norm := normalizeClause(newClause)
	if norm == "" {
		return true // 空条款视为已存在，跳过
	}
	for _, sc := range existing {
		existingNorm := normalizeClause(sc.Clause)
		// 包含关系：任一方包含另一方
		if strings.Contains(existingNorm, norm) || strings.Contains(norm, existingNorm) {
			return true
		}
	}
	return false
}

// dedupStarClauses 对条款列表做最终去重，保留首次出现的。
func dedupStarClauses(clauses []core.StarClause) []core.StarClause {
	var result []core.StarClause
	for _, c := range clauses {
		if !clauseExists(result, c.Clause) {
			result = append(result, c)
		}
	}
	return result
}

// isClauseHeader 判断匹配文本是否是条款标题/说明而非具体条款。
// 例如 "★废标条款（带★号为实质性要求，不满足将废标）" 是标题，不是条款。
func isClauseHeader(text string) bool {
	// 标题特征：包含"条款"+"要求"/"说明"/"带★号"等元描述词
	headerKeywords := []string{"带★号", "条款说明", "实质性要求", "废标条款（", "星号条款"}
	for _, kw := range headerKeywords {
		if strings.Contains(text, kw) {
			return true
		}
	}
	// 太短（<10 字）或太长（>150 字）的匹配可能是误报
	runes := []rune(text)
	if len(runes) < 10 || len(runes) > 150 {
		return true
	}
	return false
}
