// Package store 的 PostgreSQL 实现（Phase2）。
// 使用 pgx 驱动 + pgvector 扩展，与 SQLite 实现共享同一 Store 接口。
// 切换时仅改 store 实例，内核代码不变。
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresStore 是 Store 的 PostgreSQL 实现。
type PostgresStore struct {
	db *sql.DB
}

// NewPostgres 创建并打开 PostgreSQL 数据库。
func NewPostgres(dsn string) (*PostgresStore, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	return &PostgresStore{db: db}, nil
}

// Init 创建数据库 schema（PostgreSQL + pgvector）。
func (s *PostgresStore) Init(ctx context.Context) error {
	// 先确保 pgvector 扩展存在
	_, err := s.db.ExecContext(ctx, `CREATE EXTENSION IF NOT EXISTS vector`)
	if err != nil {
		return fmt.Errorf("create pgvector extension: %w", err)
	}
	_, err = s.db.ExecContext(ctx, pgSchemaSQL)
	return err
}

func (s *PostgresStore) Close() error { return s.db.Close() }

const pgSchemaSQL = `
CREATE TABLE IF NOT EXISTS chunks (
    id UUID PRIMARY KEY,
    file_path TEXT NOT NULL,
    category TEXT NOT NULL,
    chunk_offset INTEGER NOT NULL,
    text TEXT NOT NULL,
    embedding vector(1536),
    file_mtime TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_chunks_category ON chunks(category);
CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);

CREATE TABLE IF NOT EXISTS rfp_profiles (
    id UUID PRIMARY KEY,
    project_name TEXT,
    industry TEXT,
    issuer TEXT,
    bid_deadline TIMESTAMPTZ,
    scoring_tree JSONB,
    star_clauses JSONB,
    dark_rules JSONB,
    qualifications JSONB,
    rfp_type TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS outlines (
    id UUID PRIMARY KEY,
    rfp_id UUID NOT NULL,
    pattern_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS chapter_specs (
    id UUID PRIMARY KEY,
    outline_id UUID NOT NULL,
    title TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    chap_order INTEGER NOT NULL,
    target_words INTEGER NOT NULL,
    scoring_item_ids JSONB,
    figure_requirements TEXT
);
CREATE INDEX IF NOT EXISTS idx_chapter_specs_outline ON chapter_specs(outline_id);

CREATE TABLE IF NOT EXISTS figure_specs (
    id UUID PRIMARY KEY,
    chapter_id UUID NOT NULL,
    type TEXT NOT NULL,
    source TEXT,
    caption TEXT
);

CREATE TABLE IF NOT EXISTS chapter_contents (
    id UUID PRIMARY KEY,
    chapter_id UUID NOT NULL,
    markdown TEXT NOT NULL,
    word_count INTEGER NOT NULL,
    evidence_refs JSONB,
    prompt_variant TEXT,
    revision INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS illustrations (
    id UUID PRIMARY KEY,
    spec_id UUID NOT NULL,
    png_bytes BYTEA,
    render_engine TEXT,
    ooxml TEXT,
    width_px INTEGER,
    fallback_chain TEXT,
    status TEXT NOT NULL DEFAULT 'ok'
);

CREATE TABLE IF NOT EXISTS bid_packages (
    id UUID PRIMARY KEY,
    rfp_id UUID NOT NULL,
    outline_id UUID NOT NULL,
    quality_score REAL NOT NULL DEFAULT 0,
    label TEXT NOT NULL DEFAULT 'draft',
    pattern_id UUID,
    output_path TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS bid_patterns (
    id UUID PRIMARY KEY,
    industry TEXT,
    rfp_type TEXT,
    outline_template TEXT,
    chart_distribution TEXT,
    section_word_ratio TEXT,
    scoring_coverage REAL,
    quality_score REAL,
    label TEXT NOT NULL DEFAULT 'draft',
    source_bid_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_patterns_industry ON bid_patterns(industry, rfp_type);

CREATE TABLE IF NOT EXISTS prompt_variants (
    id UUID PRIMARY KEY,
    task TEXT NOT NULL,
    name TEXT NOT NULL,
    template TEXT NOT NULL,
    alpha INTEGER NOT NULL DEFAULT 1,
    beta INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_prompt_variants_task ON prompt_variants(task);

CREATE TABLE IF NOT EXISTS audit_issues (
    id UUID PRIMARY KEY,
    bid_id UUID NOT NULL,
    chapter_title TEXT,
    severity TEXT,
    dimension TEXT,
    issue TEXT,
    suggestion TEXT,
    evidence TEXT
);
CREATE INDEX IF NOT EXISTS idx_audit_issues_bid ON audit_issues(bid_id);

CREATE TABLE IF NOT EXISTS file_meta (
    file_path TEXT PRIMARY KEY,
    content_hash TEXT NOT NULL,
    mod_time TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
`

// ---- 材料索引 ----

func (s *PostgresStore) SaveChunk(ctx context.Context, c *core.Chunk) error {
	emb := pgVectorFormat(c.Embedding)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chunks (id, file_path, category, chunk_offset, text, embedding) VALUES ($1,$2,$3,$4,$5,$6::vector)
		 ON CONFLICT (id) DO UPDATE SET text=EXCLUDED.text, embedding=EXCLUDED.embedding`,
		c.ID, c.FilePath, c.Category, c.Offset, c.Text, emb)
	return err
}

func (s *PostgresStore) SaveChunks(ctx context.Context, chunks []core.Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO chunks (id, file_path, category, chunk_offset, text, embedding) VALUES ($1,$2,$3,$4,$5,$6::vector)
		 ON CONFLICT (id) DO UPDATE SET text=EXCLUDED.text, embedding=EXCLUDED.embedding`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range chunks {
		emb := pgVectorFormat(c.Embedding)
		if _, err := stmt.ExecContext(ctx, c.ID, c.FilePath, c.Category, c.Offset, c.Text, emb); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *PostgresStore) DeleteChunksByFile(ctx context.Context, filePath string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE file_path = $1`, filePath)
	return err
}

// ---- 增量索引 ----

// GetFileMeta 查询文件元信息，用于增量跳过。
func (s *PostgresStore) GetFileMeta(ctx context.Context, filePath string) (*core.FileMeta, error) {
	var meta core.FileMeta
	err := s.db.QueryRowContext(ctx,
		`SELECT file_path, content_hash, mod_time FROM file_meta WHERE file_path = $1`, filePath).
		Scan(&meta.FilePath, &meta.Hash, &meta.ModTime)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

// SaveFileMeta 写入或更新文件元信息。
func (s *PostgresStore) SaveFileMeta(ctx context.Context, meta *core.FileMeta) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO file_meta (file_path, content_hash, mod_time) VALUES ($1,$2,$3)
		 ON CONFLICT (file_path) DO UPDATE SET content_hash=EXCLUDED.content_hash, mod_time=EXCLUDED.mod_time`,
		meta.FilePath, meta.Hash, meta.ModTime)
	return err
}

func (s *PostgresStore) ListChunks(ctx context.Context, category string) ([]core.Chunk, error) {
	if category == "" {
		rows, err := s.db.QueryContext(ctx, `SELECT id, file_path, category, chunk_offset, text FROM chunks`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		return pgScanChunks(rows)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, file_path, category, chunk_offset, text FROM chunks WHERE category = $1`, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgScanChunks(rows)
}

func (s *PostgresStore) ListChunksByFile(ctx context.Context, filePath string) ([]core.Chunk, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file_path, category, chunk_offset, text FROM chunks WHERE file_path = $1`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgScanChunks(rows)
}

func (s *PostgresStore) SearchChunks(ctx context.Context, queryVec []float32, topK int) ([]core.Chunk, error) {
	emb := pgVectorFormat(queryVec)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file_path, category, chunk_offset, text FROM chunks
		 WHERE embedding IS NOT NULL
		 ORDER BY embedding <-> $1::vector LIMIT $2`, emb, topK)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgScanChunks(rows)
}

func pgScanChunks(rows *sql.Rows) ([]core.Chunk, error) {
	var chunks []core.Chunk
	for rows.Next() {
		var c core.Chunk
		if err := rows.Scan(&c.ID, &c.FilePath, &c.Category, &c.Offset, &c.Text); err != nil {
			return nil, err
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// ---- RFP 画像 ----

func (s *PostgresStore) SaveRFPProfile(ctx context.Context, p *core.RFPProfile) error {
	scoring, _ := json.Marshal(p.ScoringTree)
	stars, _ := json.Marshal(p.StarClauses)
	dark, _ := json.Marshal(p.DarkRules)
	quals, _ := json.Marshal(p.Qualifications)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO rfp_profiles (id, project_name, industry, issuer, bid_deadline, scoring_tree, star_clauses, dark_rules, qualifications, rfp_type)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		 ON CONFLICT (id) DO UPDATE SET project_name=EXCLUDED.project_name, scoring_tree=EXCLUDED.scoring_tree, star_clauses=EXCLUDED.star_clauses`,
		p.ID, p.ProjectName, p.Industry, p.Issuer, p.BidDeadline, scoring, stars, dark, quals, p.RFPType)
	return err
}

func (s *PostgresStore) GetRFPProfile(ctx context.Context, id uuid.UUID) (*core.RFPProfile, error) {
	var p core.RFPProfile
	var scoring, stars, dark, quals []byte
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_name, industry, issuer, bid_deadline, scoring_tree, star_clauses, dark_rules, qualifications, rfp_type
		 FROM rfp_profiles WHERE id = $1`, id).
		Scan(&p.ID, &p.ProjectName, &p.Industry, &p.Issuer, &p.BidDeadline, &scoring, &stars, &dark, &quals, &p.RFPType)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(scoring, &p.ScoringTree)
	json.Unmarshal(stars, &p.StarClauses)
	json.Unmarshal(dark, &p.DarkRules)
	json.Unmarshal(quals, &p.Qualifications)
	return &p, nil
}

// ---- 大纲 ----

func (s *PostgresStore) SaveOutline(ctx context.Context, o *core.Outline) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO outlines (id, rfp_id, pattern_id) VALUES ($1,$2,$3) ON CONFLICT (id) DO NOTHING`,
		o.ID, o.RFPID, o.PatternID)
	return err
}

func (s *PostgresStore) GetOutline(ctx context.Context, id uuid.UUID) (*core.Outline, error) {
	var o core.Outline
	err := s.db.QueryRowContext(ctx, `SELECT id, rfp_id, pattern_id FROM outlines WHERE id = $1`, id).
		Scan(&o.ID, &o.RFPID, &o.PatternID)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *PostgresStore) SaveChapterSpec(ctx context.Context, spec *core.ChapterSpec) error {
	ids, _ := json.Marshal(spec.ScoringItemIDs)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chapter_specs (id, outline_id, title, level, chap_order, target_words, scoring_item_ids, figure_requirements)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO UPDATE SET title=EXCLUDED.title`,
		spec.ID, spec.OutlineID, spec.Title, spec.Level, spec.Order, spec.TargetWords, ids, spec.FigureRequirements)
	return err
}

func (s *PostgresStore) SaveFigureSpec(ctx context.Context, spec *core.FigureSpec) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO figure_specs (id, chapter_id, type, source, caption) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (id) DO NOTHING`,
		spec.ID, spec.ChapterID, spec.Type, spec.Source, spec.Caption)
	return err
}

// ---- 章节内容 ----

func (s *PostgresStore) SaveChapterContent(ctx context.Context, c *core.ChapterContent) error {
	refs, _ := json.Marshal(c.EvidenceRefs)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO chapter_contents (id, chapter_id, markdown, word_count, evidence_refs, prompt_variant, revision)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO UPDATE SET markdown=EXCLUDED.markdown`,
		c.ID, c.ChapterID, c.Markdown, c.WordCount, refs, c.PromptVariant, c.Revision)
	return err
}

// ---- 图表 ----

func (s *PostgresStore) SaveIllustration(ctx context.Context, ill *core.Illustration) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO illustrations (id, spec_id, png_bytes, render_engine, ooxml, width_px, fallback_chain, status)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO UPDATE SET png_bytes=EXCLUDED.png_bytes`,
		ill.ID, ill.SpecID, ill.PNGBytes, ill.RenderEngine, ill.OOXML, ill.WidthPx, ill.FallbackChain, ill.Status)
	return err
}

// ---- 标书包 ----

func (s *PostgresStore) SaveBidPackage(ctx context.Context, pkg *core.BidPackage) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bid_packages (id, rfp_id, outline_id, quality_score, label, pattern_id, output_path)
		 VALUES ($1,$2,$3,$4,$5,$6,$7) ON CONFLICT (id) DO UPDATE SET quality_score=EXCLUDED.quality_score`,
		pkg.ID, pkg.RFPID, pkg.OutlineID, pkg.QualityScore, pkg.Label, pkg.PatternID, pkg.OutputPath)
	return err
}

func (s *PostgresStore) GetBidPackage(ctx context.Context, id uuid.UUID) (*core.BidPackage, error) {
	var pkg core.BidPackage
	err := s.db.QueryRowContext(ctx,
		`SELECT id, rfp_id, outline_id, quality_score, label, pattern_id, output_path FROM bid_packages WHERE id = $1`, id).
		Scan(&pkg.ID, &pkg.RFPID, &pkg.OutlineID, &pkg.QualityScore, &pkg.Label, &pkg.PatternID, &pkg.OutputPath)
	if err != nil {
		return nil, err
	}
	return &pkg, nil
}

func (s *PostgresStore) ListBidPackages(ctx context.Context) ([]core.BidPackage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, rfp_id, outline_id, quality_score, label, output_path FROM bid_packages ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pkgs []core.BidPackage
	for rows.Next() {
		var pkg core.BidPackage
		if err := rows.Scan(&pkg.ID, &pkg.RFPID, &pkg.OutlineID, &pkg.QualityScore, &pkg.Label, &pkg.OutputPath); err != nil {
			return nil, err
		}
		pkgs = append(pkgs, pkg)
	}
	return pkgs, rows.Err()
}

// ---- 模式库 ----

func (s *PostgresStore) SavePattern(ctx context.Context, p *core.BidPattern) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO bid_patterns (id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) ON CONFLICT (id) DO UPDATE SET quality_score=EXCLUDED.quality_score, label=EXCLUDED.label`,
		p.ID, p.Industry, p.RFPType, p.OutlineTemplate, p.ChartDistribution, p.SectionWordRatio, p.ScoringCoverage, p.QualityScore, p.Label, p.SourceBidID)
	return err
}

func (s *PostgresStore) SearchPatterns(ctx context.Context, industry, rfpType string, topK int) ([]core.BidPattern, error) {
	query := `SELECT id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id FROM bid_patterns`
	var args []any
	argIdx := 1
	if industry != "" && rfpType != "" {
		query += fmt.Sprintf(` WHERE industry = $%d AND rfp_type = $%d`, argIdx, argIdx+1)
		args = append(args, industry, rfpType)
		argIdx += 2
	} else if industry != "" {
		query += fmt.Sprintf(` WHERE industry = $%d`, argIdx)
		args = append(args, industry)
		argIdx++
	}
	query += ` ORDER BY CASE label WHEN 'won' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, quality_score DESC`
	if topK > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, argIdx)
		args = append(args, topK)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgScanPatterns(rows)
}

func (s *PostgresStore) ListPatterns(ctx context.Context) ([]core.BidPattern, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id FROM bid_patterns ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return pgScanPatterns(rows)
}

func pgScanPatterns(rows *sql.Rows) ([]core.BidPattern, error) {
	var patterns []core.BidPattern
	for rows.Next() {
		var p core.BidPattern
		if err := rows.Scan(&p.ID, &p.Industry, &p.RFPType, &p.OutlineTemplate, &p.ChartDistribution, &p.SectionWordRatio, &p.ScoringCoverage, &p.QualityScore, &p.Label, &p.SourceBidID); err != nil {
			return nil, err
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

// ---- Prompt 变体 ----

func (s *PostgresStore) SavePromptVariant(ctx context.Context, v *PromptVariant) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO prompt_variants (id, task, name, template, alpha, beta) VALUES ($1,$2,$3,$4,$5,$6) ON CONFLICT (id) DO UPDATE SET alpha=EXCLUDED.alpha, beta=EXCLUDED.beta`,
		v.ID, v.Task, v.Name, v.Template, v.Alpha, v.Beta)
	return err
}

func (s *PostgresStore) ListPromptVariants(ctx context.Context, task string) ([]PromptVariant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, task, name, template, alpha, beta FROM prompt_variants WHERE task = $1`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var variants []PromptVariant
	for rows.Next() {
		var v PromptVariant
		if err := rows.Scan(&v.ID, &v.Task, &v.Name, &v.Template, &v.Alpha, &v.Beta); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (s *PostgresStore) UpdatePromptVariant(ctx context.Context, v *PromptVariant) error {
	_, err := s.db.ExecContext(ctx, `UPDATE prompt_variants SET alpha = $1, beta = $2 WHERE id = $3`, v.Alpha, v.Beta, v.ID)
	return err
}

// ---- 审计 ----

func (s *PostgresStore) SaveAuditIssues(ctx context.Context, issues []core.AuditIssue) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT INTO audit_issues (id, bid_id, chapter_title, severity, dimension, issue, suggestion, evidence) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, iss := range issues {
		if _, err := stmt.ExecContext(ctx, iss.ID, iss.BidID, iss.ChapterTitle, iss.Severity, iss.Dimension, iss.Issue, iss.Suggestion, iss.Evidence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 工具 ----

// pgVectorFormat 将 []float32 转为 pgvector 文本格式 "[1.0,2.0,3.0]"。
func pgVectorFormat(vec []float32) string {
	if len(vec) == 0 {
		return ""
	}
	out := "["
	for i, v := range vec {
		if i > 0 {
			out += ","
		}
		out += fmt.Sprintf("%g", v)
	}
	return out + "]"
}

// 确保 PostgresStore 实现了 Store 接口（编译时检查）
var _ Store = (*PostgresStore)(nil)
