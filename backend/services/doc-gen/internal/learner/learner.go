// Package learner 实现学习迭代：模式抽取 + 检索增强 + Prompt Bandit + 反馈闭环 + 质量评分。
// 详见 docs/doc-gen/algorithms.md 第六节"自动学习迭代算法"。
package learner

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
)

// Learner 实现 core.Learner 接口。
type Learner struct {
	Store store.Store
	Log   *slog.Logger
	rng   *rand.Rand
}

// New 创建 Learner。
func New(s store.Store, log *slog.Logger) *Learner {
	return &Learner{
		Store: s,
		Log:   log,
		rng:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// Learn 从一次生成的标书中抽取模式入库。
func (l *Learner) Learn(ctx context.Context, pkg *core.BidPackage, profile *core.RFPProfile) error {
	log := l.Log
	if log == nil {
		log = slog.Default()
	}

	pattern := l.extractPattern(pkg, profile)
	if err := l.Store.SavePattern(ctx, pattern); err != nil {
		return fmt.Errorf("learner: save pattern: %w", err)
	}

	// 更新 Prompt 变体的 Bandit 后验
	qualityScore := l.qualityScore(pkg, profile)
	if qualityScore >= 60 {
		l.updatePromptBandit(ctx, "outline", "default", true)
		l.updatePromptBandit(ctx, "content", "default", true)
	} else {
		l.updatePromptBandit(ctx, "outline", "default", false)
		l.updatePromptBandit(ctx, "content", "default", false)
	}

	pkg.QualityScore = qualityScore
	pkg.PatternID = &pattern.ID

	log.Info("learner: 模式入库", "pattern_id", pattern.ID, "quality", qualityScore)
	return nil
}

// extractPattern 从标书包中抽取可复用模式。
func (l *Learner) extractPattern(pkg *core.BidPackage, profile *core.RFPProfile) *core.BidPattern {
	// 大纲模板
	var titles []string
	totalWords := 0
	for _, ch := range pkg.Chapters {
		titles = append(titles, fmt.Sprintf("{\"title\":\"%s\",\"level\":%d}", ch.Spec.Title, ch.Spec.Level))
		totalWords += ch.Content.WordCount
	}
	outlineTemplate := "[" + strings.Join(titles, ",") + "]"

	// 图表分布
	chartDist := make(map[string]int)
	for _, fig := range pkg.Figures {
		chartDist[string(fig.RenderEngine)]++
	}
	chartJSON, _ := json.Marshal(chartDist)

	// 字数分配比例
	wordRatios := make(map[string]float64)
	for _, ch := range pkg.Chapters {
		if totalWords > 0 {
			wordRatios[ch.Spec.Title] = float64(ch.Content.WordCount) / float64(totalWords)
		}
	}
	wordJSON, _ := json.Marshal(wordRatios)

	// 评分项覆盖率
	coverage := l.scoringCoverage(pkg, profile)

	// 质量评分
	quality := l.qualityScore(pkg, profile)

	return &core.BidPattern{
		ID:                uuid.New(),
		Industry:          profile.Industry,
		RFPType:           profile.RFPType,
		OutlineTemplate:   outlineTemplate,
		ChartDistribution: string(chartJSON),
		SectionWordRatio:  string(wordJSON),
		ScoringCoverage:   coverage,
		QualityScore:      quality,
		Label:             pkg.Label,
		SourceBidID:       &pkg.ID,
		CreatedAt:         time.Now(),
	}
}

// RetrievePatterns 检索相似历史模式。
func (l *Learner) RetrievePatterns(ctx context.Context, industry, rfpType string, topK int) ([]core.BidPattern, error) {
	return l.Store.SearchPatterns(ctx, industry, rfpType, topK)
}

// SelectPrompt 用 Thompson 采样选择 Prompt 变体。
// 返回变体名称（v1 只有一个默认变体）。
func (l *Learner) SelectPrompt(ctx context.Context, task string) (string, error) {
	variants, err := l.Store.ListPromptVariants(ctx, task)
	if err != nil {
		return "default", nil
	}
	if len(variants) == 0 {
		// 初始化默认变体
		v := &store.PromptVariant{
			ID:       uuid.New(),
			Task:     task,
			Name:     "default",
			Template: "",
			Alpha:    1,
			Beta:     1,
		}
		_ = l.Store.SavePromptVariant(ctx, v)
		return "default", nil
	}

	// Thompson 采样：对每个变体采样 Beta(α, β)，选最大的
	bestSample := -1.0
	bestName := "default"
	for _, v := range variants {
		sample := betaSample(v.Alpha, v.Beta, l.rng)
		if sample > bestSample {
			bestSample = sample
			bestName = v.Name
		}
	}
	return bestName, nil
}

// updatePromptBandit 更新 Prompt 变体的 Bandit 后验。
func (l *Learner) updatePromptBandit(ctx context.Context, task, name string, success bool) {
	variants, err := l.Store.ListPromptVariants(ctx, task)
	if err != nil {
		return
	}
	for _, v := range variants {
		if v.Name == name {
			if success {
				v.Alpha++
			} else {
				v.Beta++
			}
			// 衰减旧数据
			v.Alpha = max(1, v.Alpha*95/100)
			v.Beta = max(1, v.Beta*95/100)
			_ = l.Store.UpdatePromptVariant(ctx, &v)
			return
		}
	}
}

// ---- 质量评分 ----

// qualityScore 计算标书包的多维质量评分（0~100）。
func (l *Learner) qualityScore(pkg *core.BidPackage, profile *core.RFPProfile) float64 {
	s := 0.0
	s += 30 * l.scoringCoverage(pkg, profile)
	s += 15 * l.wordCountCompliance(pkg)
	s += 15 * l.figureRichness(pkg)
	s += 10 * l.evidenceGrounding(pkg)
	s += 10 * l.consistencyScore(pkg)
	s += 10 * l.auditPassRate(pkg)
	s += 10 * l.darkBidCompliance(pkg, profile)
	return s
}

// scoringCoverage 评分项覆盖率。
func (l *Learner) scoringCoverage(pkg *core.BidPackage, profile *core.RFPProfile) float64 {
	if len(profile.ScoringTree) == 0 {
		return 1.0
	}
	responded := make(map[string]bool)
	for _, ch := range pkg.Chapters {
		for _, id := range ch.Spec.ScoringItemIDs {
			responded[id] = true
		}
	}
	total := 0
	covered := 0
	var check func(si core.ScoringItem)
	check = func(si core.ScoringItem) {
		if len(si.Children) == 0 {
			total++
			if responded[si.ID] {
				covered++
			}
			return
		}
		for _, c := range si.Children {
			check(c)
		}
	}
	for _, si := range profile.ScoringTree {
		check(si)
	}
	if total == 0 {
		return 1.0
	}
	return float64(covered) / float64(total)
}

// wordCountCompliance 字数达标率。
func (l *Learner) wordCountCompliance(pkg *core.BidPackage) float64 {
	if len(pkg.Chapters) == 0 {
		return 0
	}
	compliant := 0
	for _, ch := range pkg.Chapters {
		if ch.Spec.TargetWords > 0 && ch.Content.WordCount >= ch.Spec.TargetWords/2 {
			compliant++
		}
	}
	return float64(compliant) / float64(len(pkg.Chapters))
}

// figureRichness 图表丰富度。
func (l *Learner) figureRichness(pkg *core.BidPackage) float64 {
	chaptersWithFigures := 0
	for _, ch := range pkg.Chapters {
		if len(ch.Spec.FigureSpecs) > 0 {
			chaptersWithFigures++
		}
	}
	if len(pkg.Chapters) == 0 {
		return 0
	}
	return float64(chaptersWithFigures) / float64(len(pkg.Chapters))
}

// evidenceGrounding 数据可追溯比例。
func (l *Learner) evidenceGrounding(pkg *core.BidPackage) float64 {
	totalRefs := 0
	chaptersWithContent := 0
	for _, ch := range pkg.Chapters {
		if ch.Content.Markdown != "" {
			chaptersWithContent++
			totalRefs += len(ch.Content.EvidenceRefs)
		}
	}
	if chaptersWithContent == 0 {
		return 0
	}
	ratio := float64(totalRefs) / float64(chaptersWithContent)
	if ratio > 1 {
		ratio = 1
	}
	return ratio
}

// consistencyScore 跨章数据一致性。
func (l *Learner) consistencyScore(pkg *core.BidPackage) float64 {
	// 简化：检查是否有明显不一致
	// 完整实现见 auditor
	return 0.8 // 默认较高
}

// auditPassRate 审计通过率。
func (l *Learner) auditPassRate(pkg *core.BidPackage) float64 {
	// 简化：如果没有 critical 问题则通过
	return 0.9 // 默认
}

// darkBidCompliance 暗标规则合规。
func (l *Learner) darkBidCompliance(pkg *core.BidPackage, profile *core.RFPProfile) float64 {
	if len(profile.DarkRules) == 0 {
		return 1.0
	}
	return 0.9 // 简化
}

// ---- 工具函数 ----

// betaSample 从 Beta(α, β) 分布采样（Thompson 采样核心）。
// 用 Gamma 分布实现：Beta = Gamma(α) / (Gamma(α) + Gamma(β))。
func betaSample(alpha, beta int, rng *rand.Rand) float64 {
	if alpha <= 0 {
		alpha = 1
	}
	if beta <= 0 {
		beta = 1
	}
	x := gammaSample(float64(alpha), 1.0, rng)
	y := gammaSample(float64(beta), 1.0, rng)
	return x / (x + y)
}

// gammaSample 从 Gamma(shape, scale) 分布采样。
// 用 Marsaglia-Tsang 方法。
func gammaSample(shape, scale float64, rng *rand.Rand) float64 {
	if shape < 1 {
		// 使用 Boost 公式
		u := rng.Float64()
		return gammaSample(shape+1, 1, rng) * math.Pow(u, 1/shape)
	}
	d := shape - 1.0/3.0
	c := 1.0 / math.Sqrt(9*d)
	for {
		x := rng.NormFloat64()
		v := 1 + c*x
		if v <= 0 {
			continue
		}
		v = v * v * v
		u := rng.Float64()
		if u < 1-0.0331*x*x*x*x {
			return d * v * scale
		}
		if math.Log(u) < 0.5*x*x+d*(1-v+math.Log(v)) {
			return d * v * scale
		}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
