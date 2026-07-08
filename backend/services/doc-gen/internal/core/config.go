// Package core 的配置定义：主题、运行时配置、生成选项。
package core

import (
	"os"
	"path/filepath"
)

// Theme 是图表与文档的视觉主题。
type Theme struct {
	Name    string         `yaml:"name"`
	Palette []string       `yaml:"palette"`
	Font    FontConfig     `yaml:"font"`
	Chart   ChartConfig    `yaml:"chart"`
	Mermaid MermaidConfig  `yaml:"mermaid"`
	Table   TableConfig    `yaml:"table"`
}

type FontConfig struct {
	Family string `yaml:"family"`
	SizePT int    `yaml:"size_pt"`
}

type ChartConfig struct {
	DPI        int     `yaml:"dpi"`
	GridStyle  string  `yaml:"grid_style"`
	FigureSize [2]float64 `yaml:"figure_size"`
	WidthPx    int     `yaml:"width_px"`
}

type MermaidConfig struct {
	Theme          string            `yaml:"theme"`
	ThemeVariables map[string]string `yaml:"theme_variables"`
}

type TableConfig struct {
	HeaderFill      string `yaml:"header_fill"`
	HeaderFontColor string `yaml:"header_font_color"`
	Zebra           bool   `yaml:"zebra"`
}

// DefaultTheme 返回内置默认主题。
func DefaultTheme() *Theme {
	return &Theme{
		Name:    "default",
		Palette: []string{"#1F4E79", "#2E75B6", "#9DC3E6", "#A9D18E", "#FFC000"},
		Font:    FontConfig{Family: "微软雅黑", SizePT: 10},
		Chart: ChartConfig{
			DPI:        300,
			GridStyle:  "dashed",
			FigureSize: [2]float64{10, 5.6},
			WidthPx:    1600,
		},
		Mermaid: MermaidConfig{
			Theme: "default",
			ThemeVariables: map[string]string{
				"fontSize":     "14px",
				"primaryColor": "#1F4E79",
			},
		},
		Table: TableConfig{
			HeaderFill:      "1F4E79",
			HeaderFontColor: "FFFFFF",
			Zebra:           true,
		},
	}
}

// GenerateOptions 是生成标书时的运行时选项。
type GenerateOptions struct {
	MaterialDir  string  // 材料目录
	RFPPath      string  // 招标文件路径（可选，为空时自动检测）
	OutPath      string  // 输出路径
	RefPath      string  // 参考标书路径（迭代模式）
	FeedbackPath string  // 反馈文件路径
	Theme        *Theme  // 视觉主题
	Interactive  bool    // 交互式审查大纲
	Concurrency  int     // 章节生成并发数
	TotalBudget  int     // 总字数预算
	NoIllustrate bool    // 跳过图表渲染（调试用）
	NoAudit      bool    // 跳过审计（调试用）
	NoLearn      bool    // 跳过学习（调试用）
}

// RunConfig 是 CLI 运行时配置，从 YAML + 环境变量加载。
type RunConfig struct {
	DBPath       string `yaml:"db_path"`        // SQLite 数据库路径
	RouterURL    string `yaml:"router_url"`     // router-svc 地址（为空则直连）
	APIKey       string `yaml:"-"`              // LLM API key（仅 env）
	APIBase      string `yaml:"api_base"`       // LLM API base URL
	Model        string `yaml:"model"`          // 默认模型
	EmbedModel   string `yaml:"embed_model"`    // 嵌入模型
	MmdcPath     string `yaml:"mmdc_path"`      // mermaid-cli 路径
	PythonPath   string `yaml:"python_path"`    // python 路径
	ThemePath    string `yaml:"theme_path"`     // 主题文件路径
	LogLevel     string `yaml:"log_level"`
}

// DefaultRunConfig 返回默认运行时配置。
func DefaultRunConfig() *RunConfig {
	home, _ := os.UserHomeDir()
	return &RunConfig{
		DBPath:     filepath.Join(home, ".bidgen", "bidgen.db"),
		RouterURL:  "",                              // 默认不连 router-svc
		APIBase:    "https://api.openai.com/v1",     // 默认 OpenAI
		Model:      "gpt-4o",
		EmbedModel: "text-embedding-3-small",
		MmdcPath:   "mmdc",
		PythonPath: "python3",
		LogLevel:   "info",
	}
}
