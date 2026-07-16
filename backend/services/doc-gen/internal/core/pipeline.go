// Package core 的 Pipeline 编排全流程：
// Ingest → Analyze → Plan → Generate → Illustrate → Audit → Assemble → Learn
// Pipeline 持有各组件接口，组件通过依赖注入接入，内核不含 IO 绑定。
package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

// ---- 组件接口 ----

// Ingestor 摄取材料目录，建立索引。
type Ingestor interface {
	Ingest(ctx context.Context, dir string, rfpPath string) ([]Chunk, error)
}

// Analyzer 分析招标文件，产出 RFPProfile。
type Analyzer interface {
	Analyze(ctx context.Context, rfpText string) (*RFPProfile, error)
}

// Planner 规划大纲。
type Planner interface {
	Plan(ctx context.Context, profile *RFPProfile) (*Outline, error)
}

// Generator 生成章节正文。
type Generator interface {
	Generate(ctx context.Context, outline *Outline, profile *RFPProfile) ([]Chapter, error)
}

// Illustrator 渲染图表。
type Illustrator interface {
	Illustrate(ctx context.Context, chapters []Chapter, theme *Theme) ([]Illustration, error)
}

// Auditor 内审。
type Auditor interface {
	Audit(ctx context.Context, pkg *BidPackage, profile *RFPProfile) ([]AuditIssue, error)
}

// Assembler 组装文档。
type Assembler interface {
	Assemble(ctx context.Context, pkg *BidPackage, theme *Theme) (string, error)
}

// Learner 学习迭代。
type Learner interface {
	Learn(ctx context.Context, pkg *BidPackage, profile *RFPProfile) error
	RetrievePatterns(ctx context.Context, industry, rfpType string, topK int) ([]BidPattern, error)
	SelectPrompt(ctx context.Context, task string) (string, error)
}

// ---- Pipeline ----

// Pipeline 是全流程编排器。
type Pipeline struct {
	Ingestor    Ingestor
	Analyzer    Analyzer
	Planner     Planner
	Generator   Generator
	Illustrator Illustrator
	Auditor     Auditor
	Assembler   Assembler
	Learner     Learner
	Log         *slog.Logger
}

// PipelineResult 是一次 generate 的完整结果。
type PipelineResult struct {
	Package    *BidPackage
	Issues     []AuditIssue
	OutputPath string
	Duration   time.Duration
	Steps      []StepResult
}

// StepResult 记录单步执行情况。
type StepResult struct {
	Name     string
	Duration time.Duration
	Skipped  bool
	Error    error
}

// Run 执行完整的生成流程。
func (p *Pipeline) Run(ctx context.Context, opts *GenerateOptions) (*PipelineResult, error) {
	start := time.Now()
	result := &PipelineResult{}
	log := p.Log
	if log == nil {
		log = slog.Default()
	}

	theme := opts.Theme
	if theme == nil {
		theme = DefaultTheme()
	}

	// Step 1: Ingest
	s := StepResult{Name: "ingest"}
	t0 := time.Now()
	chunks, err := p.Ingestor.Ingest(ctx, opts.MaterialDir, opts.RFPPath)
	s.Duration = time.Since(t0)
	if err != nil {
		s.Error = err
		result.Steps = append(result.Steps, s)
		return nil, fmt.Errorf("ingest: %w", err)
	}
	log.Info("ingest 完成", "chunks", len(chunks))
	result.Steps = append(result.Steps, s)

	// 拼接 RFP 全文
	rfpText := ""
	for _, c := range chunks {
		if c.Category == "rfp" {
			rfpText += c.Text + "\n"
		}
	}
	if rfpText == "" && opts.RFPPath != "" {
		data, rerr := os.ReadFile(opts.RFPPath)
		if rerr == nil {
			rfpText = string(data)
		}
	}

	// Step 2: Analyze
	s = StepResult{Name: "analyze"}
	t0 = time.Now()
	profile, err := p.Analyzer.Analyze(ctx, rfpText)
	s.Duration = time.Since(t0)
	if err != nil {
		s.Error = err
		result.Steps = append(result.Steps, s)
		return nil, fmt.Errorf("analyze: %w", err)
	}
	log.Info("analyze 完成", "project", profile.ProjectName, "scoring_items", len(profile.ScoringTree))
	result.Steps = append(result.Steps, s)

	// Step 3: Plan
	s = StepResult{Name: "plan"}
	t0 = time.Now()
	outline, err := p.Planner.Plan(ctx, profile)
	s.Duration = time.Since(t0)
	if err != nil {
		s.Error = err
		result.Steps = append(result.Steps, s)
		return nil, fmt.Errorf("plan: %w", err)
	}
	log.Info("plan 完成", "chapters", len(outline.Chapters))
	result.Steps = append(result.Steps, s)

	// Step 4: Generate
	s = StepResult{Name: "generate"}
	t0 = time.Now()
	chapters, err := p.Generator.Generate(ctx, outline, profile)
	s.Duration = time.Since(t0)
	if err != nil {
		s.Error = err
		result.Steps = append(result.Steps, s)
		return nil, fmt.Errorf("generate: %w", err)
	}
	log.Info("generate 完成", "chapters", len(chapters))
	result.Steps = append(result.Steps, s)

	// 组装 BidPackage
	pkg := &BidPackage{
		ID:          uuid.New(),
		RFPID:       profile.ID,
		ProjectName: profile.ProjectName,
		OutlineID:   outline.ID,
		Chapters:    chapters,
		Label:       "draft",
		CreatedAt:   time.Now(),
	}

	// Step 5: Illustrate
	s = StepResult{Name: "illustrate"}
	if opts.NoIllustrate {
		s.Skipped = true
		result.Steps = append(result.Steps, s)
	} else {
		t0 = time.Now()
		figures, ierr := p.Illustrator.Illustrate(ctx, chapters, theme)
		s.Duration = time.Since(t0)
		if ierr != nil {
			log.Warn("illustrate 失败，降级跳过", "err", ierr)
			s.Error = ierr
			s.Skipped = true
		} else {
			pkg.Figures = figures
			log.Info("illustrate 完成", "figures", len(figures))
		}
		result.Steps = append(result.Steps, s)
	}

	// Step 6: Audit
	s = StepResult{Name: "audit"}
	if opts.NoAudit {
		s.Skipped = true
		result.Steps = append(result.Steps, s)
	} else {
		t0 = time.Now()
		issues, aerr := p.Auditor.Audit(ctx, pkg, profile)
		s.Duration = time.Since(t0)
		if aerr != nil {
			log.Warn("audit 失败，降级跳过", "err", aerr)
			s.Error = aerr
			s.Skipped = true
		} else {
			result.Issues = issues
			log.Info("audit 完成", "issues", len(issues))
		}
		result.Steps = append(result.Steps, s)
	}

	// Step 7: Assemble
	s = StepResult{Name: "assemble"}
	t0 = time.Now()
	outPath := opts.OutPath
	if outPath == "" {
		outPath = filepath.Join(opts.MaterialDir, "标书.docx")
	}
	pkg.OutputPath = outPath
	out, err := p.Assembler.Assemble(ctx, pkg, theme)
	s.Duration = time.Since(t0)
	if err != nil {
		s.Error = err
		result.Steps = append(result.Steps, s)
		return nil, fmt.Errorf("assemble: %w", err)
	}
	result.OutputPath = out
	log.Info("assemble 完成", "output", out)
	result.Steps = append(result.Steps, s)

	// Step 8: Learn
	s = StepResult{Name: "learn"}
	if opts.NoLearn || p.Learner == nil {
		s.Skipped = true
		result.Steps = append(result.Steps, s)
	} else {
		t0 = time.Now()
		if lerr := p.Learner.Learn(ctx, pkg, profile); lerr != nil {
			log.Warn("learn 失败，降级跳过", "err", lerr)
			s.Error = lerr
		} else {
			log.Info("learn 完成")
		}
		s.Duration = time.Since(t0)
		result.Steps = append(result.Steps, s)
	}

	result.Package = pkg
	result.Duration = time.Since(start)
	log.Info("pipeline 完成", "duration", result.Duration, "output", result.OutputPath)
	return result, nil
}
