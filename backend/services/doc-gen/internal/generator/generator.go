// Package generator 实现章节生成：RAG 接地 + 并发执行 + 自审限2轮。
// 详见 docs/doc-gen/algorithms.md 第四节"章节生成算法"。
package generator

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
)

// Generator 实现 core.Generator 接口。
type Generator struct {
	LLM         llm.Client
	Store       store.Store
	Log         *slog.Logger
	Concurrency int  // 并发数，默认 10
	MaxRevisions int // 自审最大修订轮次，默认 2
}

// New 创建 Generator。
func New(client llm.Client, s store.Store, log *slog.Logger) *Generator {
	return &Generator{
		LLM:          client,
		Store:        s,
		Log:          log,
		Concurrency:  10,
		MaxRevisions: 2,
	}
}

// Generate 并发生成所有章节正文。
func (g *Generator) Generate(ctx context.Context, outline *core.Outline, profile *core.RFPProfile) ([]core.Chapter, error) {
	log := g.Log
	if log == nil {
		log = slog.Default()
	}

	conc := g.Concurrency
	if conc <= 0 {
		conc = 10
	}
	if conc > len(outline.Chapters) {
		conc = len(outline.Chapters)
	}

	// 为每个章节设置 OutlineID
	for i := range outline.Chapters {
		outline.Chapters[i].OutlineID = outline.ID
	}

	type result struct {
		index   int
		chapter core.Chapter
		err     error
	}

	results := make(chan result, len(outline.Chapters))
	sem := make(chan struct{}, conc)
	var wg sync.WaitGroup

	for i, spec := range outline.Chapters {
		wg.Add(1)
		go func(idx int, s core.ChapterSpec) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ch, err := g.generateOne(ctx, s, profile)
			results <- result{index: idx, chapter: ch, err: err}
		}(i, spec)
	}

	wg.Wait()
	close(results)

	chapters := make([]core.Chapter, len(outline.Chapters))
	for r := range results {
		if r.err != nil {
			log.Warn("generator: 章节生成失败，使用占位", "chapter", outline.Chapters[r.index].Title, "err", r.err)
			chapters[r.index] = core.Chapter{
				Spec: outline.Chapters[r.index],
				Content: core.ChapterContent{
					ID:        uuid.New(),
					ChapterID: outline.Chapters[r.index].ID,
					Markdown:  fmt.Sprintf("## %s\n\n[本章节生成失败，需手动补充]\n\n错误: %v\n", outline.Chapters[r.index].Title, r.err),
					WordCount: 0,
				},
			}
		} else {
			chapters[r.index] = r.chapter
		}
	}

	log.Info("generator: 完成", "chapters", len(chapters))
	return chapters, nil
}

// generateOne 生成单个章节，含 RAG 接地和自审。
func (g *Generator) generateOne(ctx context.Context, spec core.ChapterSpec, profile *core.RFPProfile) (core.Chapter, error) {
	log := g.Log

	// RAG 检索：用章节标题 + 评分项作为查询
	query := spec.Title
	if len(spec.ScoringItemIDs) > 0 {
		query += " " + strings.Join(spec.ScoringItemIDs, " ")
	}
	evidence, err := g.retrieveEvidence(ctx, query, 5)
	if err != nil {
		if log != nil {
			log.Warn("generator: RAG 检索失败，无接地证据", "chapter", spec.Title, "err", err)
		}
	}

	// 生成正文
	md, promptVariant, err := g.llmGenerate(ctx, spec, profile, evidence)
	if err != nil {
		return core.Chapter{}, fmt.Errorf("generate %s: %w", spec.Title, err)
	}

	// 自审：检查字数和评分项覆盖
	for rev := 0; rev < g.MaxRevisions; rev++ {
		issues := g.selfAudit(md, spec)
		if len(issues) == 0 {
			break
		}
		if log != nil {
			log.Info("generator: 自审发现问题，修订", "chapter", spec.Title, "revision", rev+1, "issues", len(issues))
		}
		md, err = g.llmRevise(ctx, spec, md, issues)
		if err != nil {
			break
		}
	}

	wordCount := countWords(md)

	return core.Chapter{
		Spec: spec,
		Content: core.ChapterContent{
			ID:            uuid.New(),
			ChapterID:     spec.ID,
			Markdown:      md,
			WordCount:     wordCount,
			EvidenceRefs:  evidence,
			PromptVariant: promptVariant,
			Revision:      0,
			CreatedAt:     time.Now(),
		},
	}, nil
}

// retrieveEvidence 从知识库检索相关证据。
func (g *Generator) retrieveEvidence(ctx context.Context, query string, topK int) ([]string, error) {
	// Phase1：无向量检索时用关键词匹配
	chunks, err := g.Store.ListChunks(ctx, "")
	if err != nil {
		return nil, err
	}
	// 简单关键词匹配
	var matched []string
	queryLower := strings.ToLower(query)
	for _, c := range chunks {
		if c.Category == "rfp" {
			continue // 跳过 RFP 本身
		}
		if strings.Contains(strings.ToLower(c.Text), queryLower) || containsAny(c.Text, strings.Fields(queryLower)) {
			matched = append(matched, c.Text)
			if len(matched) >= topK {
				break
			}
		}
	}
	return matched, nil
}

// llmGenerate 用 LLM 生成章节正文。
func (g *Generator) llmGenerate(ctx context.Context, spec core.ChapterSpec, profile *core.RFPProfile, evidence []string) (string, string, error) {
	evidenceText := "无可用证据材料"
	if len(evidence) > 0 {
		evidenceText = strings.Join(evidence, "\n---\n")
	}

	scoringInfo := ""
	if len(spec.ScoringItemIDs) > 0 {
		for _, si := range profile.ScoringTree {
			for _, id := range spec.ScoringItemIDs {
				if si.ID == id {
					scoringInfo += fmt.Sprintf("评分项：%s（权重%.1f）\n", si.Name, si.Weight)
				}
			}
		}
	}

	prompt := fmt.Sprintf(`你是标书撰写专家。请撰写以下章节的正文内容。

章节标题：%s
目标字数：%d
%s
证据材料：
%s

要求：
1. 所有数据必须来自证据材料，无证据则标注[需补充]
2. 使用 Markdown 格式
3. 如果需要图表，使用占位符格式：[!figure:type caption=图表标题]
4. 内容专业、具体，避免空话套话
5. 确保覆盖相关评分项的要求`, spec.Title, spec.TargetWords, scoringInfo, evidenceText)

	resp, err := g.LLM.Chat(ctx, &core.LLMRequest{
		Task: "content_generate",
		Messages: []core.Message{
			{Role: "system", Content: "你是标书撰写专家，输出 Markdown 格式的标书章节正文。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.7,
	})
	if err != nil {
		return "", "", err
	}

	return resp.Content, "default", nil
}

// llmRevise 用 LLM 修订章节。
func (g *Generator) llmRevise(ctx context.Context, spec core.ChapterSpec, md string, issues []string) (string, error) {
	prompt := fmt.Sprintf(`请修订以下标书章节内容。

章节标题：%s
当前内容：
%s

发现问题：
%s

请修正以上问题，返回修订后的完整内容（Markdown格式）。`, spec.Title, md, strings.Join(issues, "\n"))

	resp, err := g.LLM.Chat(ctx, &core.LLMRequest{
		Task: "content_revise",
		Messages: []core.Message{
			{Role: "system", Content: "你是标书审阅专家，修正问题并返回完整修订内容。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
	})
	if err != nil {
		return md, err // 修订失败返回原文
	}
	return resp.Content, nil
}

// selfAudit 自审：检查字数和内容质量。
func (g *Generator) selfAudit(md string, spec core.ChapterSpec) []string {
	var issues []string
	wc := countWords(md)
	if spec.TargetWords > 0 && wc < spec.TargetWords/2 {
		issues = append(issues, fmt.Sprintf("字数不足：当前%d字，目标%d字", wc, spec.TargetWords))
	}
	if strings.Contains(md, "[需补充]") && wc < spec.TargetWords {
		issues = append(issues, "存在未补充的占位符，请尽量填充")
	}
	return issues
}

// countWords 统计字数（中文按字计，英文按词计）。
func countWords(text string) int {
	count := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			count++
		}
	}
	// 英文单词数
	words := strings.Fields(text)
	for _, w := range words {
		if !isAllCJK(w) {
			count++
		}
	}
	return count
}

func isAllCJK(s string) bool {
	for _, r := range s {
		if r < 0x4E00 || r > 0x9FFF {
			return false
		}
	}
	return true
}

// containsAny 检查文本是否包含任一关键词。
func containsAny(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if len(kw) > 1 && strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
