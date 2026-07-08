// bidgen 是 doc-gen 模块的 CLI 入口（Phase1）。
// 子命令：generate（生成标书）/ index（建索引）/ learn（学习）/ report（报告）/ theme（换肤）
// 详见 docs/doc-gen/architecture.md 第七节"CLI 工作流"。
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/bidwriter/services/doc-gen/internal/analyzer"
	"github.com/bidwriter/services/doc-gen/internal/assembler"
	"github.com/bidwriter/services/doc-gen/internal/auditor"
	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/generator"
	"github.com/bidwriter/services/doc-gen/internal/illustrator"
	"github.com/bidwriter/services/doc-gen/internal/ingest"
	"github.com/bidwriter/services/doc-gen/internal/learner"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/bidwriter/services/doc-gen/internal/planner"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
	"time"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	cfgPath string
	cfg     *core.RunConfig
	log     *slog.Logger
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "bidgen",
		Short: "AI 标书自动生成工具",
		Long:  "BidWriter 文档生成内核 CLI —— 读取材料目录，自动生成可交付的 Word 标书。",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initConfig()
		},
	}

	rootCmd.PersistentFlags().StringVarP(&cfgPath, "config", "c", "", "配置文件路径（默认 ~/.bidgen/bidgen.yaml）")

	rootCmd.AddCommand(generateCmd())
	rootCmd.AddCommand(indexCmd())
	rootCmd.AddCommand(learnCmd())
	rootCmd.AddCommand(reportCmd())

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// initConfig 加载配置文件和环境变量。
func initConfig() error {
	cfg = core.DefaultRunConfig()

	// 配置文件路径
	if cfgPath == "" {
		home, _ := os.UserHomeDir()
		cfgPath = filepath.Join(home, ".bidgen", "bidgen.yaml")
	}

	// 尝试读取 YAML 配置
	if data, err := os.ReadFile(cfgPath); err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return fmt.Errorf("parse config %s: %w", cfgPath, err)
		}
	}

	// 环境变量覆盖
	if v := os.Getenv("BIDGEN_DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("BIDGEN_ROUTER_URL"); v != "" {
		cfg.RouterURL = v
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("LLM_API_BASE"); v != "" {
		cfg.APIBase = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("LLM_EMBED_MODEL"); v != "" {
		cfg.EmbedModel = v
	}
	// Anthropic 兼容 API
	if v := os.Getenv("ANTHROPIC_AUTH_TOKEN"); v != "" {
		cfg.APIKey = v
	}
	if v := os.Getenv("ANTHROPIC_BASE_URL"); v != "" {
		cfg.APIBase = v
	}
	if v := os.Getenv("ANTHROPIC_MODEL"); v != "" {
		cfg.Model = v
	}

	// 确保 DB 目录存在
	dir := filepath.Dir(cfg.DBPath)
	os.MkdirAll(dir, 0755)

	// 日志
	log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(log)

	return nil
}

// ---- 依赖装配 ----

// openStore 打开并初始化 SQLite Store。
func openStore() (*store.SQLiteStore, error) {
	s, err := store.NewSQLite(cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := s.Init(context.Background()); err != nil {
		return nil, err
	}
	return s, nil
}

// newLLMClient 根据配置创建 LLM 客户端。

func newLLMClient() llm.Client {

	if cfg.RouterURL != "" {

		log.Info("使用 router-svc 作为 LLM 后端", "url", cfg.RouterURL)

		return llm.NewRouterClient(cfg.RouterURL, uuid.Nil)

	}

	// 检测 Anthropic 兼容 API（含 MiniMax 等）

	if cfg.APIKey != "" && (strings.Contains(cfg.APIBase, "anthropic") || os.Getenv("ANTHROPIC_AUTH_TOKEN") != "") {

		log.Info("使用 Anthropic 兼容 API", "base", cfg.APIBase, "model", cfg.Model)

		return llm.NewAnthropicClient(cfg.APIKey, cfg.APIBase, cfg.Model)

	}

	// OpenAI 兼容直连

	if cfg.APIKey != "" {

		log.Info("使用直连 LLM API", "base", cfg.APIBase, "model", cfg.Model)

		return llm.NewDirectClient(cfg.APIKey, cfg.APIBase, cfg.Model, cfg.EmbedModel)

	}

	log.Warn("未配置 LLM，将使用 noop 客户端（生成将失败）")

	return llm.NoopClient{}

}



// buildPipeline 装配完整的 Pipeline。

// puppeteerConfigPath 返回 puppeteer 配置路径，不存在则创建。
func puppeteerConfigPath() string {
	path := filepath.Join(filepath.Dir(cfg.DBPath), "puppeteer.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.WriteFile(path, []byte(`{"args": ["--no-sandbox", "--disable-setuid-sandbox"]}`), 0644)
	}
	return path
}
func buildPipeline(st *store.SQLiteStore) *core.Pipeline {
	client := llm.NewRetryClient(newLLMClient(), 3, log)

	ing := ingest.New(st, client, log)
	ana := analyzer.New(client, log)
	pln := planner.New(client, log)
	gen := generator.New(client, st, log)

	// Illustrator
	var renderers []illustrator.Renderer
renderers = append(renderers, &illustrator.MermaidRenderer{MmdcPath: cfg.MmdcPath, PuppeteerConfig: puppeteerConfigPath(), LLM: client})
	renderers = append(renderers, &illustrator.DataChartRenderer{PythonPath: cfg.PythonPath, LLM: client})
	renderers = append(renderers, &illustrator.AIImageRenderer{LLM: client})
	renderers = append(renderers, &illustrator.TableRenderer{})
	beautifier := &illustrator.Beautifier{}
	il := illustrator.New(renderers, beautifier)

	// Auditor
	aud := auditor.New(log)

	// Assembler
	asm := assembler.New(log)

	// Learner
	lrn := learner.New(st, log)

	// 连接 Planner 和 Learner
	pln.Learner = lrn

	return &core.Pipeline{
		Ingestor:    ing,
		Analyzer:    ana,
		Planner:     pln,
		Generator:   gen,
		Illustrator: il,
		Auditor:     aud,
		Assembler:   asm,
		Learner:     lrn,
		Log:         log,
	}
}

// ---- 子命令 ----

// generateCmd: bidgen generate <材料目录>
func generateCmd() *cobra.Command {
	var (
		rfpPath      string
		outPath      string
		noIllustrate bool
		noAudit      bool
		noLearn      bool
		concurrency  int
		totalBudget  int
	)
	cmd := &cobra.Command{
		Use:   "generate <材料目录>",
		Short: "生成标书",
		Long:  "读取材料目录，自动生成可交付的 Word 标书。\n例: bidgen generate ./投标材料 --rfp 招标文件.pdf --out 标书.docx",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			materialDir := args[0]
			st, err := openStore()
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer st.Close()

			pipe := buildPipeline(st)

			opts := &core.GenerateOptions{
				MaterialDir:  materialDir,
				RFPPath:      rfpPath,
				OutPath:      outPath,
				Theme:        core.DefaultTheme(),
				Concurrency:  concurrency,
				TotalBudget:  totalBudget,
				NoIllustrate: noIllustrate,
				NoAudit:      noAudit,
				NoLearn:      noLearn,
			}

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			result, err := pipe.Run(ctx, opts)
			if err != nil {
				return err
			}

			// 打印结果摘要
			fmt.Println("\n========== 生成完成 ==========")
			fmt.Printf("输出文件: %s\n", result.OutputPath)
			fmt.Printf("总耗时:   %s\n", result.Duration)
			fmt.Println("\n步骤明细:")
			for _, s := range result.Steps {
				status := "✓"
				if s.Skipped {
					status = "⊘ (跳过)"
				}
				if s.Error != nil {
					status = "✗ (错误)"
				}
				fmt.Printf("  %-12s %s  %s\n", s.Name, status, s.Duration)
			}
			if len(result.Issues) > 0 {
				fmt.Printf("\n审计问题: %d 条\n", len(result.Issues))
				for i, iss := range result.Issues {
					if i >= 10 {
						fmt.Printf("  ... 还有 %d 条\n", len(result.Issues)-10)
						break
					}
					fmt.Printf("  [%s] %s: %s\n", iss.Severity, iss.Dimension, iss.Issue)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&rfpPath, "rfp", "", "招标文件路径（为空时自动检测）")
	cmd.Flags().StringVarP(&outPath, "out", "o", "", "输出路径（默认材料目录下 标书.docx）")
	cmd.Flags().BoolVar(&noIllustrate, "no-illustrate", false, "跳过图表渲染")
	cmd.Flags().BoolVar(&noAudit, "no-audit", false, "跳过审计")
	cmd.Flags().BoolVar(&noLearn, "no-learn", false, "跳过学习")
	cmd.Flags().IntVarP(&concurrency, "concurrency", "j", 10, "章节生成并发数")
	cmd.Flags().IntVarP(&totalBudget, "budget", "b", 60000, "总字数预算")
	return cmd
}

// indexCmd: bidgen index <材料目录>
func indexCmd() *cobra.Command {
	var rfpPath string
	cmd := &cobra.Command{
		Use:   "index <材料目录>",
		Short: "仅摄取建索引（增量）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			client := llm.NewRetryClient(newLLMClient(), 3, log)
			ing := ingest.New(st, client, log)

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			chunks, err := ing.Ingest(ctx, args[0], rfpPath)
			if err != nil {
				return err
			}
			fmt.Printf("索引完成: %d 个分块\n", len(chunks))
			return nil
		},
	}
	cmd.Flags().StringVar(&rfpPath, "rfp", "", "招标文件路径")
	return cmd
}

// learnCmd: bidgen learn <历史标书目录>
func learnCmd() *cobra.Command {
	var label string
	var industry string
	cmd := &cobra.Command{
		Use:   "learn <历史标书目录>",
		Short: "把历史标书录入模式库（离线学习）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			client := llm.NewRetryClient(newLLMClient(), 3, log)
			ing := ingest.New(st, client, log)
			ana := analyzer.New(client, log)
			lrn := learner.New(st, log)

			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			// 摄取历史标书
			chunks, err := ing.Ingest(ctx, args[0], "")
			if err != nil {
				return err
			}

			// 提取 RFP 画像（如果有 RFP 文件）
			rfpText := ""
			for _, c := range chunks {
				if c.Category == "rfp" {
					rfpText += c.Text + "\n"
				}
			}
			profile := &core.RFPProfile{
				ID:       uuid.New(),
				Industry: industry,
				RFPType:  "公开招标",
			}
			if rfpText != "" {
				p, err := ana.Analyze(ctx, rfpText)
				if err == nil {
					profile = p
				}
			}
			if industry != "" {
				profile.Industry = industry
			}

			// 构造简易 BidPackage 并学习
			pkg := &core.BidPackage{
				ID:        uuid.New(),
				RFPID:     profile.ID,
				Label:     label,
				CreatedAt: time.Now(),
			}
			for _, c := range chunks {
				if c.Category != "rfp" {
					pkg.Chapters = append(pkg.Chapters, core.Chapter{
						Spec: core.ChapterSpec{ID: uuid.New(), Title: c.FilePath},
						Content: core.ChapterContent{
							ID:        uuid.New(),
							Markdown:  c.Text,
							WordCount: len(c.Text),
						},
					})
				}
			}

			if err := lrn.Learn(ctx, pkg, profile); err != nil {
				return err
			}
			fmt.Printf("学习完成: 标签=%s 行业=%s 模式已入库\n", label, profile.Industry)
			return nil
		},
	}
	cmd.Flags().StringVar(&label, "label", "won", "标签: won/lost/draft")
	cmd.Flags().StringVar(&industry, "industry", "", "行业覆盖")
	return cmd
}

// reportCmd: bidgen report [标书包ID]
func reportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "report",
		Short: "查看生成报告与质量评分",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openStore()
			if err != nil {
				return err
			}
			defer st.Close()

			pkgs, err := st.ListBidPackages(context.Background())
			if err != nil {
				return err
			}
			if len(pkgs) == 0 {
				fmt.Println("暂无标书包记录")
				return nil
			}
			fmt.Printf("标书包列表 (%d):\n", len(pkgs))
			for _, p := range pkgs {
				fmt.Printf("  ID: %s  质量: %.1f  标签: %s  输出: %s\n",
					p.ID, p.QualityScore, p.Label, p.OutputPath)
			}

			// 模式库
			patterns, _ := st.ListPatterns(context.Background())
			if len(patterns) > 0 {
				fmt.Printf("\n模式库 (%d):\n", len(patterns))
				for _, p := range patterns {
					fmt.Printf("  ID: %s  行业: %s  质量: %.1f  标签: %s\n",
						p.ID, p.Industry, p.QualityScore, p.Label)
				}
			}
			return nil
		},
	}
	return cmd
}
