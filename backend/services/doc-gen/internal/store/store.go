// Package store 定义 doc-gen 的存储抽象。
// Phase1 用 SQLite 实现，Phase2 替换为 PostgreSQL，内核代码不变。
package store

import (
	"context"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

// Store 是所有持久化操作的统一接口。
// 组件通过此接口访问数据，不直接持有 DB 句柄。
type Store interface {
	// ---- 初始化 ----
	Init(ctx context.Context) error
	Close() error

	// ---- 材料索引 ----
	SaveChunk(ctx context.Context, c *core.Chunk) error
	SaveChunks(ctx context.Context, chunks []core.Chunk) error
	DeleteChunksByFile(ctx context.Context, filePath string) error
	ListChunks(ctx context.Context, category string) ([]core.Chunk, error)
	SearchChunks(ctx context.Context, queryVec []float32, topK int) ([]core.Chunk, error)

	// ---- RFP 画像 ----
	SaveRFPProfile(ctx context.Context, p *core.RFPProfile) error
	GetRFPProfile(ctx context.Context, id uuid.UUID) (*core.RFPProfile, error)

	// ---- 大纲 ----
	SaveOutline(ctx context.Context, o *core.Outline) error
	GetOutline(ctx context.Context, id uuid.UUID) (*core.Outline, error)
	SaveChapterSpec(ctx context.Context, spec *core.ChapterSpec) error
	SaveFigureSpec(ctx context.Context, spec *core.FigureSpec) error

	// ---- 章节内容 ----
	SaveChapterContent(ctx context.Context, c *core.ChapterContent) error

	// ---- 图表 ----
	SaveIllustration(ctx context.Context, ill *core.Illustration) error

	// ---- 标书包 ----
	SaveBidPackage(ctx context.Context, pkg *core.BidPackage) error
	GetBidPackage(ctx context.Context, id uuid.UUID) (*core.BidPackage, error)
	ListBidPackages(ctx context.Context) ([]core.BidPackage, error)

	// ---- 学习：模式库 ----
	SavePattern(ctx context.Context, p *core.BidPattern) error
	SearchPatterns(ctx context.Context, industry, rfpType string, topK int) ([]core.BidPattern, error)
	ListPatterns(ctx context.Context) ([]core.BidPattern, error)

	// ---- 学习：Prompt 变体 ----
	SavePromptVariant(ctx context.Context, v *PromptVariant) error
	ListPromptVariants(ctx context.Context, task string) ([]PromptVariant, error)
	UpdatePromptVariant(ctx context.Context, v *PromptVariant) error

	// ---- 审计 ----
	SaveAuditIssues(ctx context.Context, issues []core.AuditIssue) error
}

// PromptVariant 是多臂老虎机的一个臂（Prompt 变体）。
type PromptVariant struct {
	ID        uuid.UUID `json:"id"        db:"id"`
	Task      string    `json:"task"      db:"task"`       // outline/content/mermaid
	Name      string    `json:"name"      db:"name"`       // 变体名
	Template  string    `json:"template"  db:"template"`   // prompt 模板
	Alpha     int       `json:"alpha"     db:"alpha"`      // 成功次数（Beta 分布）
	Beta      int       `json:"beta"      db:"beta"`       // 失败次数
	CreatedAt string    `json:"created_at" db:"created_at"`
}
