// Package analyzer 实现招标文件分析：LLM 结构化抽取 RFPProfile + 规则校验补全。
// 详见 docs/doc-gen/algorithms.md 第二节"招标文件分析算法"。
package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/google/uuid"
)

// Analyzer 实现 core.Analyzer 接口。
type Analyzer struct {
	LLM llm.Client
	Log *slog.Logger
}

// New 创建 Analyzer。
func New(client llm.Client, log *slog.Logger) *Analyzer {
	return &Analyzer{LLM: client, Log: log}
}

// Analyze 从招标文件文本抽取结构化画像。
// 两阶段：LLM 结构化抽取 → 规则校验与补全。
func (a *Analyzer) Analyze(ctx context.Context, rfpText string) (*core.RFPProfile, error) {
	log := a.Log
	if log == nil {
		log = slog.Default()
	}

	if strings.TrimSpace(rfpText) == "" {
		return nil, fmt.Errorf("analyzer: RFP 文本为空")
	}

	// 截断过长的 RFP 文本（大上下文模型上限 ~100k token）
	truncated := truncateText(rfpText, 80000)

	// 阶段1：LLM 结构化抽取
	profile, err := a.llmExtract(ctx, truncated)
	if err != nil {
		log.Warn("analyzer: LLM 抽取失败，降级为纯规则模式", "err", err)
		profile = &core.RFPProfile{}
	}

	// 阶段2：规则校验与补全
	a.ruleEnrich(profile, rfpText)

	// 最终去重：移除模糊重复的★条款
	profile.StarClauses = dedupStarClauses(profile.StarClauses)
	// 权重归一化
	if len(profile.ScoringTree) > 0 {
		for i := range profile.ScoringTree {
			normalizeWeights(&profile.ScoringTree[i])
		}
	}

	// 确保有 ID
	if profile.ID == uuid.Nil {
		profile.ID = uuid.New()
	}
	profile.RawText = rfpText
	if profile.CreatedAt.IsZero() {
		profile.CreatedAt = time.Now()
	}

	log.Info("analyzer: 完成",
		"project", profile.ProjectName,
		"scoring_items", len(profile.ScoringTree),
		"star_clauses", len(profile.StarClauses),
		"dark_rules", len(profile.DarkRules))

	return profile, nil
}

// llmExtract 用 LLM 做结构化抽取。
func (a *Analyzer) llmExtract(ctx context.Context, text string) (*core.RFPProfile, error) {
	prompt := `你是一个招标文件分析专家。请从以下招标文件文本中抽取结构化信息，返回 JSON 格式。

JSON 结构如下：
{
  "project_name": "项目名称",
  "industry": "行业（IT/建筑/医疗/制造等）",
  "issuer": "招标方/采购人",
  "bid_deadline": "投标截止时间（ISO格式，如不确定则留空）",
  "rfp_type": "招标类型（公开招标/邀请招标/竞争性谈判/单一来源等）",
  "scoring_tree": [
    {
      "id": "s1",
      "category": "技术/商务/资格/价格",
      "name": "评分项名称",
      "weight": 10.0,
      "chapter_mapping": ["建议章节1"],
      "children": []
    }
  ],
  "star_clauses": [
    {"id": "st1", "clause": "★号条款原文", "section": "所在章节", "severity": "critical"}
  ],
  "dark_rules": ["暗标规则1", "暗标规则2"],
  "qualifications": ["资质要求1", "资质要求2"]
}

注意：
1. scoring_tree 的 weight 是百分制，所有同级项之和应为100
2. star_clauses 是带★号或标注"否则废标"的强制性条款
3. dark_rules 是暗标格式要求（如不得出现公司名称等）
4. 只返回 JSON，不要其他文字

招标文件文本：
` + text

	resp, err := a.LLM.Chat(ctx, &core.LLMRequest{
		Task: "rfp_parse",
		Messages: []core.Message{
			{Role: "system", Content: "你是一个招标文件分析专家，只返回合法的 JSON。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   8192,
		Temperature: 0.1,
	})
	if err != nil {
		return nil, fmt.Errorf("llm extract: %w", err)
	}

	// 从响应中提取 JSON
	jsonStr := extractJSON(resp.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("llm extract: 响应中无有效 JSON")
	}

	// 用临时结构接收，BidDeadline 作为字符串处理（LLM 返回的时间格式不固定）
	var raw struct {
		ProjectName    string             `json:"project_name"`
		Industry       string             `json:"industry"`
		Issuer         string             `json:"issuer"`
		BidDeadline    string             `json:"bid_deadline"`
		ScoringTree    []core.ScoringItem `json:"scoring_tree"`
		StarClauses    []core.StarClause  `json:"star_clauses"`
		DarkRules      []string           `json:"dark_rules"`
		Qualifications []string           `json:"qualifications"`
		RFPType        string             `json:"rfp_type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("llm extract: unmarshal: %w", err)
	}
	profile := &core.RFPProfile{
		ID:             uuid.New(),
		ProjectName:    raw.ProjectName,
		Industry:       raw.Industry,
		Issuer:         raw.Issuer,
		ScoringTree:    raw.ScoringTree,
		StarClauses:    raw.StarClauses,
		DarkRules:      raw.DarkRules,
		Qualifications: raw.Qualifications,
		RFPType:        raw.RFPType,
		CreatedAt:      time.Now(),
	}
	// 尝试多种时间格式解析
	if raw.BidDeadline != "" {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
			if t, err := time.Parse(layout, raw.BidDeadline); err == nil {
				profile.BidDeadline = &t
				break
			}
		}
	}
	return profile, nil

}

// ruleEnrich 用规则补全 LLM 可能遗漏的信息。
func (a *Analyzer) ruleEnrich(profile *core.RFPProfile, fullText string) {
	// ★号条款正则扫描
	starPattern := regexp.MustCompile(`★\s*\d+[\.、）)]\s*[^\n。；;]{5,150}`)
	matches := starPattern.FindAllString(fullText, -1)
	existing := make(map[string]bool)
	for _, sc := range profile.StarClauses {
		existing[sc.Clause] = true
	}
	for i, m := range matches {
		m = strings.TrimSpace(m)
		if isClauseHeader(m) {
			continue
		}
		if clauseExists(profile.StarClauses, m) {
			continue
		}
		profile.StarClauses = append(profile.StarClauses, core.StarClause{
			ID:       fmt.Sprintf("rule_st_%d", i),
			Clause:   m,
			Severity: "critical",
		})
	}

	// "否则废标"模式
	rejectPattern := regexp.MustCompile(`[^\n。]{10,80}否则[^\n。]{0,15}废标`)
	rejectMatches := rejectPattern.FindAllString(fullText, -1)
	for i, m := range rejectMatches {
		m = strings.TrimSpace(m)
		if isClauseHeader(m) {
			continue
		}
		if clauseExists(profile.StarClauses, m) {
			continue
		}
		profile.StarClauses = append(profile.StarClauses, core.StarClause{
			ID:       fmt.Sprintf("rule_reject_%d", i),
			Clause:   m,
			Severity: "critical",
		})
	}

	// 暗标规则检测
	if strings.Contains(fullText, "暗标") || strings.Contains(fullText, "匿名") {
		if len(profile.DarkRules) == 0 {
			profile.DarkRules = append(profile.DarkRules, "本招标包含暗标要求，请注意格式规范")
		}
	}

	// 行业推断（如果 LLM 没给出）
	if profile.Industry == "" {
		profile.Industry = inferIndustry(fullText)
	}

	// RFP 类型推断
	if profile.RFPType == "" {
		profile.RFPType = inferRFPType(fullText)
	}
}

// normalizeWeights 递归归一化评分项权重，使同级子项之和等于父项权重。
func normalizeWeights(node *core.ScoringItem) {
	if len(node.Children) == 0 {
		return
	}
	// 先递归子节点
	for i := range node.Children {
		normalizeWeights(&node.Children[i])
	}
	// 计算子项原始权重之和
	rawSum := 0.0
	for _, c := range node.Children {
		rawSum += c.Weight
	}
	if rawSum <= 0 {
		return
	}
	// 按比例分配父项权重
	for i := range node.Children {
		node.Children[i].Weight = node.Children[i].Weight / rawSum * node.Weight
	}
}

// extractJSON 从 LLM 响应中提取 JSON 字符串。
func extractJSON(s string) string {
	// 尝试找到第一个 { 和最后一个 }
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(s, "}")
	if end < start {
		return ""
	}
	return s[start : end+1]
}

// truncateText 截断文本到大约 maxChars 字符。
func truncateText(text string, maxChars int) string {
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text
	}
	return string(runes[:maxChars]) + "\n...（截断）"
}

// inferIndustry 从文本推断行业。
func inferIndustry(text string) string {
	lower := strings.ToLower(text)
	switch {
	case strings.Contains(lower, "软件") || strings.Contains(lower, "信息化") || strings.Contains(lower, "系统"):
		return "IT"
	case strings.Contains(lower, "施工") || strings.Contains(lower, "工程") || strings.Contains(lower, "建筑"):
		return "建筑"
	case strings.Contains(lower, "医疗") || strings.Contains(lower, "器械") || strings.Contains(lower, "医院"):
		return "医疗"
	case strings.Contains(lower, "设备") || strings.Contains(lower, "制造") || strings.Contains(lower, "生产"):
		return "制造"
	default:
		return "综合"
	}
}

// inferRFPType 从文本推断招标类型。
func inferRFPType(text string) string {
	switch {
	case strings.Contains(text, "公开招标"):
		return "公开招标"
	case strings.Contains(text, "邀请招标"):
		return "邀请招标"
	case strings.Contains(text, "竞争性谈判"):
		return "竞争性谈判"
	case strings.Contains(text, "单一来源"):
		return "单一来源"
	default:
		return "公开招标"
	}
}
