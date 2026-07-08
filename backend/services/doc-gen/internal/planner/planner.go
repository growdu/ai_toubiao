// Package planner 实现大纲规划：权重→字数分配 + 学习增强 + 图表需求推断。
// 详见 docs/doc-gen/algorithms.md 第三节"大纲规划算法"。
package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/google/uuid"
)

// Planner 实现 core.Planner 接口。
type Planner struct {
	LLM     llm.Client
	Log     *slog.Logger
	TotalBudget int  // 总字数预算，默认 60000
	Learner core.Learner  // 可选：学习增强
}

// New 创建 Planner。
func New(client llm.Client, log *slog.Logger) *Planner {
	return &Planner{
		LLM:         client,
		Log:         log,
		TotalBudget: 60000,
	}
}

// Plan 基于 RFP 画像规划章节大纲。
func (p *Planner) Plan(ctx context.Context, profile *core.RFPProfile) (*core.Outline, error) {
	log := p.Log
	if log == nil {
		log = slog.Default()
	}

	// 检索相似历史模式（学习增强）
	var patterns []core.BidPattern
	if p.Learner != nil {
		var err error
		patterns, err = p.Learner.RetrievePatterns(ctx, profile.Industry, profile.RFPType, 3)
		if err != nil {
			log.Warn("planner: 检索模式失败，退化为纯 LLM 规划", "err", err)
		}
	}
	if len(patterns) > 0 {
		log.Info("planner: 学习增强", "patterns", len(patterns))
	}

	// LLM 生成大纲
	chapters, err := p.llmPlan(ctx, profile, patterns)
	if err != nil {
		log.Warn("planner: LLM 规划失败，降级为规则模式", "err", err)
		chapters = p.ruleBasedPlan(profile)
	}

	// 权重→字数分配
	p.allocateWords(chapters)

	// 图表需求推断
	for i := range chapters {
		chapters[i].FigureRequirements = inferFigureNeed(chapters[i].Title, profile)
	}

	// 为有图表需求的章节创建 FigureSpec
	for i := range chapters {
		if chapters[i].FigureRequirements != "" {
			figType := inferFigureType(chapters[i].FigureRequirements)
			chapters[i].FigureSpecs = append(chapters[i].FigureSpecs, core.FigureSpec{
				ID:        uuid.New(),
				ChapterID: chapters[i].ID,
				Type:      figType,
				Caption:   chapters[i].Title + " - " + chapters[i].FigureRequirements,
			})
		}
	}

	outline := &core.Outline{
		ID:        uuid.New(),
		RFPID:     profile.ID,
		Chapters:  chapters,
		CreatedAt: time.Now(),
	}

	log.Info("planner: 完成", "chapters", len(chapters))
	return outline, nil
}

// llmPlan 用 LLM 生成大纲。
func (p *Planner) llmPlan(ctx context.Context, profile *core.RFPProfile, patterns []core.BidPattern) ([]core.ChapterSpec, error) {
	// 构造评分项摘要
	scoringJSON, _ := json.Marshal(profile.ScoringTree)

	// 构造历史模式 few-shot
	patternHint := ""
	if len(patterns) > 0 {
		var outlines []string
		for _, pat := range patterns {
			if pat.OutlineTemplate != "" {
				outlines = append(outlines, pat.OutlineTemplate)
			}
		}
		if len(outlines) > 0 {
			patternHint = "\n\n以下是同行业中标标书的大纲参考：\n" + strings.Join(outlines, "\n---\n")
		}
	}

	prompt := fmt.Sprintf(`你是标书撰写专家。基于以下招标文件评分项，规划标书章节大纲。

项目名称：%s
行业：%s
评分项：%s
★号废标条款数：%d
%s

请返回 JSON 数组，每个元素是一个章节：
[
  {"title": "章节标题", "level": 1, "scoring_item_ids": ["s1","s2"]},
  {"title": "子章节标题", "level": 2, "scoring_item_ids": ["s3"]}
]

要求：
1. 覆盖所有评分项（每个评分项至少有一个章节响应）
2. 包含必要的非评分章节（如封面、目录、公司简介等）
3. level 1 为主章节，level 2 为子章节
4. 只返回 JSON 数组，不要其他文字`, profile.ProjectName, profile.Industry, string(scoringJSON), len(profile.StarClauses), patternHint)

	resp, err := p.LLM.Chat(ctx, &core.LLMRequest{
		Task: "outline_generate",
		Messages: []core.Message{
			{Role: "system", Content: "你是标书撰写专家，只返回合法的 JSON 数组。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
	})
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSONArray(resp.Content)
	if jsonStr == "" {
		return nil, fmt.Errorf("planner: 响应中无有效 JSON")
	}

	var rawChapters []struct {
		Title          string   `json:"title"`
		Level          int      `json:"level"`
		ScoringItemIDs []string `json:"scoring_item_ids"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &rawChapters); err != nil {
		return nil, fmt.Errorf("planner: unmarshal: %w", err)
	}

	chapters := make([]core.ChapterSpec, len(rawChapters))
	for i, rc := range rawChapters {
		level := rc.Level
		if level == 0 {
			level = 1
		}
		chapters[i] = core.ChapterSpec{
			ID:             uuid.New(),
			Title:          rc.Title,
			Level:          level,
			Order:          i,
			ScoringItemIDs: rc.ScoringItemIDs,
		}
	}

	return chapters, nil
}

// ruleBasedPlan 规则兜底：从评分项直接映射章节。
func (p *Planner) ruleBasedPlan(profile *core.RFPProfile) []core.ChapterSpec {
	var chapters []core.ChapterSpec
	order := 0

	// 必要的前置章节
	chapters = append(chapters, core.ChapterSpec{
		ID:    uuid.New(),
		Title: "封面",
		Level: 1,
		Order: order,
	})
	order++
	chapters = append(chapters, core.ChapterSpec{
		ID:    uuid.New(),
		Title: "目录",
		Level: 1,
		Order: order,
	})
	order++
	chapters = append(chapters, core.ChapterSpec{
		ID:    uuid.New(),
		Title: "公司简介",
		Level: 1,
		Order: order,
	})
	order++

	// 从评分项映射
	for _, si := range profile.ScoringTree {
		if len(si.Children) > 0 {
			// 父项作为一级章节
			chapters = append(chapters, core.ChapterSpec{
				ID:             uuid.New(),
				Title:          si.Name,
				Level:          1,
				Order:          order,
				ScoringItemIDs: []string{si.ID},
			})
			order++
			// 子项作为二级章节
			for _, child := range si.Children {
				chapters = append(chapters, core.ChapterSpec{
					ID:             uuid.New(),
					Title:          child.Name,
					Level:          2,
					Order:          order,
					ScoringItemIDs: []string{child.ID},
				})
				order++
			}
		} else {
			chapters = append(chapters, core.ChapterSpec{
				ID:             uuid.New(),
				Title:          si.Name,
				Level:          1,
				Order:          order,
				ScoringItemIDs: []string{si.ID},
			})
			order++
		}
	}

	return chapters
}

// allocateWords 按评分权重分配目标字数。
func (p *Planner) allocateWords(chapters []core.ChapterSpec) {
	budget := p.TotalBudget
	if budget <= 0 {
		budget = 60000
	}

	// 只给 level 1 的正文章节分配（跳过封面/目录）
	var weighted []int
	totalWeight := 0.0
	for i := range chapters {
		if chapters[i].Level == 1 && !isFrontMatter(chapters[i].Title) {
			w := 1.0
			if len(chapters[i].ScoringItemIDs) > 0 {
				w = float64(len(chapters[i].ScoringItemIDs)) + 1
			}
			weighted = append(weighted, i)
			totalWeight += w
			chapters[i].TargetWords = int(w) // 临时存权重
		} else {
			chapters[i].TargetWords = 500 // 固定少量
		}
	}

	// 分配
	for _, idx := range weighted {
		w := float64(chapters[idx].TargetWords)
		words := int(float64(budget) * w / totalWeight)
		if words < 800 {
			words = 800
		}
		if words > int(float64(budget)*0.4) {
			words = int(float64(budget) * 0.4)
		}
		chapters[idx].TargetWords = words
	}
}

// inferFigureNeed 推断章节是否需要图表。
func inferFigureNeed(title string, profile *core.RFPProfile) string {
	lower := strings.ToLower(title)
	// 有序列表：更具体的模式在前，避免"架构"先于"组织架构"匹配
	keywords := []struct{ kw, fig string }{
		{"组织", "组织架构图"},
		{"流程", "流程图"},
		{"架构", "架构图"},
		{"进度", "进度计划表"},
		{"计划", "计划表"},
		{"对比", "对比表"},
		{"方案", "方案流程图"},
		{"质量", "质量管理流程图"},
		{"安全", "安全管理体系图"},
		{"人员", "人员配置表"},
		{"设备", "设备清单表"},
		{"业绩", "业绩一览表"},
		{"价格", "报价表"},
		{"商务", "商务条款响应表"},
	}
	for _, kf := range keywords {
		if strings.Contains(lower, kf.kw) {
			return kf.fig
		}
	}
	return ""
}

// inferFigureType 从图表需求推断图表类型。
func inferFigureType(requirement string) core.FigureType {
	lower := strings.ToLower(requirement)
	switch {
	case strings.Contains(lower, "流程") || strings.Contains(lower, "架构") || strings.Contains(lower, "体系"):
		return core.FigureMermaid
	case strings.Contains(lower, "表") || strings.Contains(lower, "清单") || strings.Contains(lower, "矩阵"):
		return core.FigureTable
	case strings.Contains(lower, "对比") || strings.Contains(lower, "趋势") || strings.Contains(lower, "分布"):
		return core.FigureDataChart
	default:
		return core.FigureMermaid
	}
}

// isFrontMatter 判断是否为前置章节（封面/目录等）。
func isFrontMatter(title string) bool {
	lower := strings.ToLower(title)
	return strings.Contains(lower, "封面") || strings.Contains(lower, "目录") ||
		strings.Contains(lower, "声明") || strings.Contains(lower, "授权")
}

// extractJSONArray 从文本中提取 JSON 数组。
func extractJSONArray(s string) string {
	start := strings.Index(s, "[")
	if start < 0 {
		return ""
	}
	end := strings.LastIndex(s, "]")
	if end < start {
		return ""
	}
	return s[start : end+1]
}
