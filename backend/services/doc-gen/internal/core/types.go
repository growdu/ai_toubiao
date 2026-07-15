// Package core 定义 doc-gen 内核的纯数据模型与接口契约。
// 所有组件通过这些类型交互，不直接持有 IO 句柄。
// 详见 docs/doc-gen/architecture.md 第五节"核心数据模型"。
package core

import (
	"time"

	"github.com/google/uuid"
)

// ---- 招标文件画像 ----

// RFPProfile 是从招标文件抽取的结构化画像。
type RFPProfile struct {
	ID             uuid.UUID     `json:"id"          db:"id"`
	ProjectName    string        `json:"project_name" db:"project_name"`
	Industry       string        `json:"industry"    db:"industry"`
	Issuer         string        `json:"issuer"      db:"issuer"`
	BidDeadline    *time.Time    `json:"bid_deadline,omitempty" db:"bid_deadline"`
	ScoringTree    []ScoringItem `json:"scoring_tree" db:"scoring_tree"`
	StarClauses    []StarClause  `json:"star_clauses" db:"star_clauses"`
	DarkRules      []string      `json:"dark_rules"   db:"dark_rules"`
	Qualifications []string      `json:"qualifications" db:"qualifications"`
	RFPType        string        `json:"rfp_type"     db:"rfp_type"`
	RawText        string        `json:"-"            db:"-"` // 原始全文，不入库
	CreatedAt      time.Time     `json:"created_at"   db:"created_at"`
}

// ScoringItem 是评分项树的一个节点。
type ScoringItem struct {
	ID             string        `json:"id"`
	Category       string        `json:"category"` // 形式/资格/商务/技术
	Name           string        `json:"name"`
	Weight         float64       `json:"weight"`          // 百分制权重
	ChapterMapping []string      `json:"chapter_mapping"` // 建议落入的章节
	Children       []ScoringItem `json:"children,omitempty"`
}

// StarClause 是★号废标条款。
type StarClause struct {
	ID       string `json:"id"`
	Clause   string `json:"clause"`   // 条款原文
	Section  string `json:"section"`  // 所在章节
	Severity string `json:"severity"` // critical / major
}

// ---- 大纲与章节 ----

// Outline 是规划产出的章节大纲。
type Outline struct {
	ID        uuid.UUID     `json:"id"        db:"id"`
	RFPID     uuid.UUID     `json:"rfp_id"    db:"rfp_id"`
	Chapters  []ChapterSpec `json:"chapters"  db:"-"`
	PatternID *uuid.UUID    `json:"pattern_id,omitempty" db:"pattern_id"`
	CreatedAt time.Time     `json:"created_at" db:"created_at"`
}

// ChapterSpec 是一个章节的规格说明。
type ChapterSpec struct {
	ID                 uuid.UUID    `json:"id"          db:"id"`
	OutlineID          uuid.UUID    `json:"outline_id"  db:"outline_id"`
	Title              string       `json:"title"       db:"title"`
	Level              int          `json:"level"       db:"level"`
	Order              int          `json:"order"       db:"chap_order"`
	TargetWords        int          `json:"target_words" db:"target_words"`
	ScoringItemIDs     []string     `json:"scoring_item_ids" db:"-"` // 响应的评分项
	FigureRequirements string       `json:"figure_requirements" db:"figure_requirements"`
	FigureSpecs        []FigureSpec `json:"figure_specs" db:"-"` // 规划产出的图表需求
}

// ---- 图表 ----

// FigureType 枚举图表类型。
type FigureType string

const (
	FigureMermaid   FigureType = "mermaid"
	FigureDataChart FigureType = "data_chart"
	FigureAIImage   FigureType = "ai_image"
	FigureTable     FigureType = "table"
)

// FigureSpec 是图表规格（要什么图）。
type FigureSpec struct {
	ID            uuid.UUID      `json:"id"            db:"id"`
	ChapterID     uuid.UUID      `json:"chapter_id"    db:"chapter_id"`
	Type          FigureType     `json:"type"          db:"type"`
	Source        string         `json:"source"        db:"source"` // mermaid源码/数据JSON/prompt
	Caption       string         `json:"caption"       db:"caption"`
	ThemeOverride map[string]any `json:"theme_override,omitempty" db:"-"`
}

// Illustration 是图表渲染产物（渲染结果）。
type Illustration struct {
	ID            uuid.UUID `json:"id"             db:"id"`
	SpecID        uuid.UUID `json:"spec_id"        db:"spec_id"`
	PNGBytes      []byte    `json:"-"              db:"png_bytes"` // 二进制 PNG
	RenderEngine  string    `json:"render_engine"  db:"render_engine"`
	OOXML         string    `json:"ooxml,omitempty" db:"ooxml"` // 表格类型存 OOXML XML
	WidthPx       int       `json:"width_px"       db:"width_px"`
	FallbackChain string    `json:"fallback_chain" db:"fallback_chain"`
	Status        string    `json:"status"         db:"status"` // ok / placeholder / failed
}

// ---- 章节内容 ----

// ChapterContent 是生成产出的章节正文。
type ChapterContent struct {
	ID            uuid.UUID `json:"id"           db:"id"`
	ChapterID     uuid.UUID `json:"chapter_id"   db:"chapter_id"`
	Markdown      string    `json:"markdown"     db:"markdown"`
	WordCount     int       `json:"word_count"   db:"word_count"`
	EvidenceRefs  []string  `json:"evidence_refs" db:"-"`
	PromptVariant string    `json:"prompt_variant" db:"prompt_variant"`
	Revision      int       `json:"revision"     db:"revision"`
	CreatedAt     time.Time `json:"created_at"   db:"created_at"`
}

// Chapter 是 Generator 产出的完整章节（spec + content）。
type Chapter struct {
	Spec    ChapterSpec    `json:"spec"`
	Content ChapterContent `json:"content"`
}

// ---- 标书包 ----

// BidPackage 是一次生成的完整标书包。
type BidPackage struct {
	ID           uuid.UUID      `json:"id"          db:"id"`
	RFPID        uuid.UUID      `json:"rfp_id"      db:"rfp_id"`
	OutlineID    uuid.UUID      `json:"outline_id"  db:"outline_id"`
	Chapters     []Chapter      `json:"chapters"    db:"-"`
	Figures      []Illustration `json:"figures"     db:"-"`
	QualityScore float64        `json:"quality_score" db:"quality_score"`
	Label        string         `json:"label"       db:"label"` // won/lost/draft
	PatternID    *uuid.UUID     `json:"pattern_id,omitempty" db:"pattern_id"`
	OutputPath   string         `json:"output_path" db:"output_path"`
	CreatedAt    time.Time      `json:"created_at"  db:"created_at"`
}

// ---- 学习 ----

// BidPattern 是从历史标书抽取的可复用模式。
type BidPattern struct {
	ID                uuid.UUID  `json:"id"                 db:"id"`
	Industry          string     `json:"industry"           db:"industry"`
	RFPType           string     `json:"rfp_type"           db:"rfp_type"`
	OutlineTemplate   string     `json:"outline_template"   db:"outline_template"` // JSON
	ChartDistribution string     `json:"chart_distribution" db:"chart_distribution"`
	SectionWordRatio  string     `json:"section_word_ratio" db:"section_word_ratio"`
	ScoringCoverage   float64    `json:"scoring_coverage"   db:"scoring_coverage"`
	QualityScore      float64    `json:"quality_score"      db:"quality_score"`
	Label             string     `json:"label"              db:"label"` // won/lost/draft
	SourceBidID       *uuid.UUID `json:"source_bid_id,omitempty" db:"source_bid_id"`
	CreatedAt         time.Time  `json:"created_at"         db:"created_at"`
}

// ---- 审计 ----

// AuditIssue 是审计发现的问题。
type AuditIssue struct {
	ID           uuid.UUID `json:"id"            db:"id"`
	BidID        uuid.UUID `json:"bid_id"        db:"bid_id"`
	ChapterTitle string    `json:"chapter_title" db:"chapter_title"`
	Severity     string    `json:"severity"      db:"severity"`  // critical/major/minor
	Dimension    string    `json:"dimension"     db:"dimension"` // compliance/consistency/...
	Issue        string    `json:"issue"         db:"issue"`
	Suggestion   string    `json:"suggestion"    db:"suggestion"`
	Evidence     string    `json:"evidence"      db:"evidence"`
}

// ---- 索引/分块 ----

// Chunk 是材料分块。
type Chunk struct {
	ID        uuid.UUID `json:"id"         db:"id"`
	FilePath  string    `json:"file_path"  db:"file_path"`
	Category  string    `json:"category"   db:"category"` // rfp/reference/technical/commercial/drawing/qualification/performance/other
	Offset    int       `json:"offset"     db:"chunk_offset"`
	Text      string    `json:"text"       db:"text"`
	Embedding []float32 `json:"-"          db:"-"` // 向量，内存中使用
	// 扩展字段（内存态）：去重与溯源
	ContentHash string `json:"content_hash,omitempty" db:"-"` // 源文件 sha256
	SourceName  string `json:"source_name,omitempty"  db:"-"` // 规范化前原始文件名
	NeedsOCR    bool   `json:"needs_ocr,omitempty"    db:"-"` // 无文本层标记
}

// ---- 质量评分 ----

// QualityReport 是质量评分报告。
type QualityReport struct {
	Total               float64 `json:"total"`
	ScoringItemCoverage float64 `json:"scoring_item_coverage"`
	WordCountCompliance float64 `json:"word_count_compliance"`
	FigureRichness      float64 `json:"figure_richness"`
	EvidenceGrounding   float64 `json:"evidence_grounding"`
	ConsistencyScore    float64 `json:"consistency_score"`
	AuditPassRate       float64 `json:"audit_pass_rate"`
	DarkBidCompliance   float64 `json:"dark_bid_compliance"`
}

// ---- LLM 消息 ----

// Message 是 LLM 对话的一条消息。
type Message struct {
	Role    string `json:"role"` // system/user/assistant
	Content string `json:"content"`
}

// LLMRequest 是 LLM 调用请求。
type LLMRequest struct {
	Task        string    `json:"task"` // rfp_parse/outline_generate/content_generate/...
	Messages    []Message `json:"messages"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
}

// LLMResponse 是 LLM 调用响应。
type LLMResponse struct {
	Content          string  `json:"content"`
	Model            string  `json:"model"`
	Provider         string  `json:"provider"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	CostUSD          float64 `json:"cost_usd"`
	CacheHit         bool    `json:"cache_hit"`
}

// ---- 增量索引 ----

// FileMeta 记录源文件的元信息，用于增量摄取跳过未变更文件（doc-gen-ingest.md §3.6）。
type FileMeta struct {
	FilePath string    `json:"file_path" db:"file_path"`
	Hash     string    `json:"hash"       db:"content_hash"`
	ModTime  time.Time `json:"mod_time"   db:"mod_time"`
}
