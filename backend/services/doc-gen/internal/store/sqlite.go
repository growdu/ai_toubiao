// Package store 的 SQLite 实现（Phase1）。
// 使用 modernc.org/sqlite 纯 Go 驱动，零 CGO 依赖。
// 向量检索用内存 cosine similarity（CLI 数据量小，够用）。
package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// SQLiteStore 是 Store 的 SQLite 实现。
type SQLiteStore struct {
	db   *sql.DB
	path string
}

// NewSQLite 创建并打开 SQLite 数据库。
func NewSQLite(path string) (*SQLiteStore, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite 单写
	return &SQLiteStore{db: db, path: path}, nil
}

// Init 创建数据库 schema。
func (s *SQLiteStore) Init(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, schemaSQL)
	if err != nil {
		return fmt.Errorf("init schema: %w", err)
	}
	return nil
}

// Close 关闭数据库。
func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

const schemaSQL = `
CREATE TABLE IF NOT EXISTS chunks (
    id TEXT PRIMARY KEY,
    file_path TEXT NOT NULL,
    category TEXT NOT NULL,
    chunk_offset INTEGER NOT NULL,
    text TEXT NOT NULL,
    embedding TEXT,
    file_mtime TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_chunks_category ON chunks(category);
CREATE INDEX IF NOT EXISTS idx_chunks_file_path ON chunks(file_path);

CREATE TABLE IF NOT EXISTS rfp_profiles (
    id TEXT PRIMARY KEY,
    project_name TEXT,
    industry TEXT,
    issuer TEXT,
    bid_deadline TEXT,
    scoring_tree TEXT,
    star_clauses TEXT,
    dark_rules TEXT,
    qualifications TEXT,
    rfp_type TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS outlines (
    id TEXT PRIMARY KEY,
    rfp_id TEXT NOT NULL,
    pattern_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS chapter_specs (
    id TEXT PRIMARY KEY,
    outline_id TEXT NOT NULL,
    title TEXT NOT NULL,
    level INTEGER NOT NULL DEFAULT 1,
    chap_order INTEGER NOT NULL,
    target_words INTEGER NOT NULL,
    scoring_item_ids TEXT,
    figure_requirements TEXT
);
CREATE INDEX IF NOT EXISTS idx_chapter_specs_outline ON chapter_specs(outline_id);

CREATE TABLE IF NOT EXISTS figure_specs (
    id TEXT PRIMARY KEY,
    chapter_id TEXT NOT NULL,
    type TEXT NOT NULL,
    source TEXT,
    caption TEXT
);

CREATE TABLE IF NOT EXISTS chapter_contents (
    id TEXT PRIMARY KEY,
    chapter_id TEXT NOT NULL,
    markdown TEXT NOT NULL,
    word_count INTEGER NOT NULL,
    evidence_refs TEXT,
    prompt_variant TEXT,
    revision INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS illustrations (
    id TEXT PRIMARY KEY,
    spec_id TEXT NOT NULL,
    png_bytes BLOB,
    render_engine TEXT,
    ooxml TEXT,
    width_px INTEGER,
    fallback_chain TEXT,
    status TEXT NOT NULL DEFAULT 'ok'
);

CREATE TABLE IF NOT EXISTS bid_packages (
    id TEXT PRIMARY KEY,
    rfp_id TEXT NOT NULL,
    outline_id TEXT NOT NULL,
    quality_score REAL NOT NULL DEFAULT 0,
    label TEXT NOT NULL DEFAULT 'draft',
    pattern_id TEXT,
    output_path TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS bid_patterns (
    id TEXT PRIMARY KEY,
    industry TEXT,
    rfp_type TEXT,
    outline_template TEXT,
    chart_distribution TEXT,
    section_word_ratio TEXT,
    scoring_coverage REAL,
    quality_score REAL,
    label TEXT NOT NULL DEFAULT 'draft',
    source_bid_id TEXT,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_patterns_industry ON bid_patterns(industry, rfp_type);

CREATE TABLE IF NOT EXISTS prompt_variants (
    id TEXT PRIMARY KEY,
    task TEXT NOT NULL,
    name TEXT NOT NULL,
    template TEXT NOT NULL,
    alpha INTEGER NOT NULL DEFAULT 1,
    beta INTEGER NOT NULL DEFAULT 1,
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX IF NOT EXISTS idx_prompt_variants_task ON prompt_variants(task);

CREATE TABLE IF NOT EXISTS audit_issues (
    id TEXT PRIMARY KEY,
    bid_id TEXT NOT NULL,
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
    mod_time TEXT NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
`

// ---- 材料索引 ----

func (s *SQLiteStore) SaveChunk(ctx context.Context, c *core.Chunk) error {
	emb, _ := json.Marshal(c.Embedding)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO chunks (id, file_path, category, chunk_offset, text, embedding) VALUES (?,?,?,?,?,?)`,
		c.ID.String(), c.FilePath, c.Category, c.Offset, c.Text, string(emb))
	return err
}

func (s *SQLiteStore) SaveChunks(ctx context.Context, chunks []core.Chunk) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO chunks (id, file_path, category, chunk_offset, text, embedding) VALUES (?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, c := range chunks {
		emb, _ := json.Marshal(c.Embedding)
		if _, err := stmt.ExecContext(ctx, c.ID.String(), c.FilePath, c.Category, c.Offset, c.Text, string(emb)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) DeleteChunksByFile(ctx context.Context, filePath string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM chunks WHERE file_path = ?`, filePath)
	return err
}

// ---- 增量索引 ----

// GetFileMeta 查询文件元信息，用于增量跳过。
func (s *SQLiteStore) GetFileMeta(ctx context.Context, filePath string) (*core.FileMeta, error) {
	var meta core.FileMeta
	var modTimeStr string
	err := s.db.QueryRowContext(ctx,
		`SELECT file_path, content_hash, mod_time FROM file_meta WHERE file_path = ?`, filePath).
		Scan(&meta.FilePath, &meta.Hash, &modTimeStr)
	if err != nil {
		return nil, err
	}
	meta.ModTime, _ = time.Parse(time.RFC3339Nano, modTimeStr)
	return &meta, nil
}

// SaveFileMeta 写入或更新文件元信息。
func (s *SQLiteStore) SaveFileMeta(ctx context.Context, meta *core.FileMeta) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO file_meta (file_path, content_hash, mod_time) VALUES (?,?,?)`,
		meta.FilePath, meta.Hash, meta.ModTime.Format(time.RFC3339Nano))
	return err
}

func (s *SQLiteStore) ListChunks(ctx context.Context, category string) ([]core.Chunk, error) {
	var rows *sql.Rows
	var err error
	if category == "" {
		rows, err = s.db.QueryContext(ctx, `SELECT id, file_path, category, chunk_offset, text, embedding FROM chunks`)
	} else {
		rows, err = s.db.QueryContext(ctx, `SELECT id, file_path, category, chunk_offset, text, embedding FROM chunks WHERE category = ?`, category)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (s *SQLiteStore) ListChunksByFile(ctx context.Context, filePath string) ([]core.Chunk, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, file_path, category, chunk_offset, text, embedding FROM chunks WHERE file_path = ?`, filePath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanChunks(rows)
}

func (s *SQLiteStore) SearchChunks(ctx context.Context, queryVec []float32, topK int) ([]core.Chunk, error) {
	// Phase1：全表加载到内存做 cosine similarity。
	// 数据量小（CLI 模式）时性能足够；Phase2 换 pgvector。
	all, err := s.ListChunks(ctx, "")
	if err != nil {
		return nil, err
	}
	type scored struct {
		chunk core.Chunk
		score float32
	}
	results := make([]scored, 0, len(all))
	for _, c := range all {
		if len(c.Embedding) == 0 || len(queryVec) == 0 {
			continue
		}
		results = append(results, scored{chunk: c, score: cosineSim(queryVec, c.Embedding)})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].score > results[j].score })
	if len(results) > topK {
		results = results[:topK]
	}
	out := make([]core.Chunk, len(results))
	for i, r := range results {
		out[i] = r.chunk
	}
	return out, nil
}

func scanChunks(rows *sql.Rows) ([]core.Chunk, error) {
	var chunks []core.Chunk
	for rows.Next() {
		var c core.Chunk
		var idStr, embStr string
		if err := rows.Scan(&idStr, &c.FilePath, &c.Category, &c.Offset, &c.Text, &embStr); err != nil {
			return nil, err
		}
		c.ID = uuid.MustParse(idStr)
		if embStr != "" {
			_ = json.Unmarshal([]byte(embStr), &c.Embedding)
		}
		chunks = append(chunks, c)
	}
	return chunks, rows.Err()
}

// ---- RFP 画像 ----

func (s *SQLiteStore) SaveRFPProfile(ctx context.Context, p *core.RFPProfile) error {
	scoringTree, _ := json.Marshal(p.ScoringTree)
	starClauses, _ := json.Marshal(p.StarClauses)
	darkRules, _ := json.Marshal(p.DarkRules)
	quals, _ := json.Marshal(p.Qualifications)
	var deadline any
	if p.BidDeadline != nil {
		deadline = p.BidDeadline.Format(time.RFC3339)
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO rfp_profiles (id, project_name, industry, issuer, bid_deadline, scoring_tree, star_clauses, dark_rules, qualifications, rfp_type) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.ID.String(), p.ProjectName, p.Industry, p.Issuer, deadline, string(scoringTree), string(starClauses), string(darkRules), string(quals), p.RFPType)
	return err
}

func (s *SQLiteStore) GetRFPProfile(ctx context.Context, id uuid.UUID) (*core.RFPProfile, error) {
	var p core.RFPProfile
	var idStr, scoringTree, starClauses, darkRules, quals string
	var deadline sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, project_name, industry, issuer, bid_deadline, scoring_tree, star_clauses, dark_rules, qualifications, rfp_type FROM rfp_profiles WHERE id = ?`,
		id.String()).Scan(&idStr, &p.ProjectName, &p.Industry, &p.Issuer, &deadline, &scoringTree, &starClauses, &darkRules, &quals, &p.RFPType)
	if err != nil {
		return nil, err
	}
	p.ID = uuid.MustParse(idStr)
	_ = json.Unmarshal([]byte(scoringTree), &p.ScoringTree)
	_ = json.Unmarshal([]byte(starClauses), &p.StarClauses)
	_ = json.Unmarshal([]byte(darkRules), &p.DarkRules)
	_ = json.Unmarshal([]byte(quals), &p.Qualifications)
	if deadline.Valid {
		t, _ := time.Parse(time.RFC3339, deadline.String)
		p.BidDeadline = &t
	}
	return &p, nil
}

// ---- 大纲 ----

func (s *SQLiteStore) SaveOutline(ctx context.Context, o *core.Outline) error {
	var patternID any
	if o.PatternID != nil {
		patternID = o.PatternID.String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO outlines (id, rfp_id, pattern_id) VALUES (?,?,?)`,
		o.ID.String(), o.RFPID.String(), patternID)
	return err
}

func (s *SQLiteStore) GetOutline(ctx context.Context, id uuid.UUID) (*core.Outline, error) {
	var o core.Outline
	var idStr, rfpID, patternID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, rfp_id, pattern_id FROM outlines WHERE id = ?`, id.String()).
		Scan(&idStr, &rfpID, &patternID)
	if err != nil {
		return nil, err
	}
	o.ID = uuid.MustParse(idStr.String)
	o.RFPID = uuid.MustParse(rfpID.String)
	if patternID.Valid && patternID.String != "" {
		pid := uuid.MustParse(patternID.String)
		o.PatternID = &pid
	}
	return &o, nil
}

func (s *SQLiteStore) SaveChapterSpec(ctx context.Context, spec *core.ChapterSpec) error {
	ids, _ := json.Marshal(spec.ScoringItemIDs)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO chapter_specs (id, outline_id, title, level, chap_order, target_words, scoring_item_ids, figure_requirements) VALUES (?,?,?,?,?,?,?,?)`,
		spec.ID.String(), spec.OutlineID.String(), spec.Title, spec.Level, spec.Order, spec.TargetWords, string(ids), spec.FigureRequirements)
	return err
}

func (s *SQLiteStore) SaveFigureSpec(ctx context.Context, spec *core.FigureSpec) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO figure_specs (id, chapter_id, type, source, caption) VALUES (?,?,?,?,?)`,
		spec.ID.String(), spec.ChapterID.String(), string(spec.Type), spec.Source, spec.Caption)
	return err
}

// ---- 章节内容 ----

func (s *SQLiteStore) SaveChapterContent(ctx context.Context, c *core.ChapterContent) error {
	refs, _ := json.Marshal(c.EvidenceRefs)
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO chapter_contents (id, chapter_id, markdown, word_count, evidence_refs, prompt_variant, revision) VALUES (?,?,?,?,?,?,?)`,
		c.ID.String(), c.ChapterID.String(), c.Markdown, c.WordCount, string(refs), c.PromptVariant, c.Revision)
	return err
}

// ---- 图表 ----

func (s *SQLiteStore) SaveIllustration(ctx context.Context, ill *core.Illustration) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO illustrations (id, spec_id, png_bytes, render_engine, ooxml, width_px, fallback_chain, status) VALUES (?,?,?,?,?,?,?,?)`,
		ill.ID.String(), ill.SpecID.String(), ill.PNGBytes, ill.RenderEngine, ill.OOXML, ill.WidthPx, ill.FallbackChain, ill.Status)
	return err
}

// ---- 标书包 ----

func (s *SQLiteStore) SaveBidPackage(ctx context.Context, pkg *core.BidPackage) error {
	var patternID any
	if pkg.PatternID != nil {
		patternID = pkg.PatternID.String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO bid_packages (id, rfp_id, outline_id, quality_score, label, pattern_id, output_path) VALUES (?,?,?,?,?,?,?)`,
		pkg.ID.String(), pkg.RFPID.String(), pkg.OutlineID.String(), pkg.QualityScore, pkg.Label, patternID, pkg.OutputPath)
	return err
}

func (s *SQLiteStore) GetBidPackage(ctx context.Context, id uuid.UUID) (*core.BidPackage, error) {
	var pkg core.BidPackage
	var idStr, rfpID, outlineID, patternID sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT id, rfp_id, outline_id, quality_score, label, pattern_id, output_path FROM bid_packages WHERE id = ?`,
		id.String()).Scan(&idStr, &rfpID, &outlineID, &pkg.QualityScore, &pkg.Label, &patternID, &pkg.OutputPath)
	if err != nil {
		return nil, err
	}
	pkg.ID = uuid.MustParse(idStr.String)
	pkg.RFPID = uuid.MustParse(rfpID.String)
	pkg.OutlineID = uuid.MustParse(outlineID.String)
	if patternID.Valid && patternID.String != "" {
		pid := uuid.MustParse(patternID.String)
		pkg.PatternID = &pid
	}
	return &pkg, nil
}

func (s *SQLiteStore) ListBidPackages(ctx context.Context) ([]core.BidPackage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, rfp_id, outline_id, quality_score, label, output_path FROM bid_packages ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var pkgs []core.BidPackage
	for rows.Next() {
		var pkg core.BidPackage
		var idStr, rfpID, outlineID string
		if err := rows.Scan(&idStr, &rfpID, &outlineID, &pkg.QualityScore, &pkg.Label, &pkg.OutputPath); err != nil {
			return nil, err
		}
		pkg.ID = uuid.MustParse(idStr)
		pkg.RFPID = uuid.MustParse(rfpID)
		pkg.OutlineID = uuid.MustParse(outlineID)
		pkgs = append(pkgs, pkg)
	}
	return pkgs, rows.Err()
}

// ---- 学习：模式库 ----

func (s *SQLiteStore) SavePattern(ctx context.Context, p *core.BidPattern) error {
	var sourceID any
	if p.SourceBidID != nil {
		sourceID = p.SourceBidID.String()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO bid_patterns (id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		p.ID.String(), p.Industry, p.RFPType, p.OutlineTemplate, p.ChartDistribution, p.SectionWordRatio, p.ScoringCoverage, p.QualityScore, p.Label, sourceID)
	return err
}

func (s *SQLiteStore) SearchPatterns(ctx context.Context, industry, rfpType string, topK int) ([]core.BidPattern, error) {
	query := `SELECT id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id FROM bid_patterns`
	var args []any
	if industry != "" && rfpType != "" {
		query += ` WHERE industry = ? AND rfp_type = ?`
		args = append(args, industry, rfpType)
	} else if industry != "" {
		query += ` WHERE industry = ?`
		args = append(args, industry)
	}
	// 按 quality_score 降序，won 优先
	query += ` ORDER BY CASE label WHEN 'won' THEN 0 WHEN 'draft' THEN 1 ELSE 2 END, quality_score DESC`
	if topK > 0 {
		query += fmt.Sprintf(` LIMIT %d`, topK)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPatterns(rows)
}

func (s *SQLiteStore) ListPatterns(ctx context.Context) ([]core.BidPattern, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, industry, rfp_type, outline_template, chart_distribution, section_word_ratio, scoring_coverage, quality_score, label, source_bid_id FROM bid_patterns ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPatterns(rows)
}

func scanPatterns(rows *sql.Rows) ([]core.BidPattern, error) {
	var patterns []core.BidPattern
	for rows.Next() {
		var p core.BidPattern
		var idStr, sourceID sql.NullString
		if err := rows.Scan(&idStr, &p.Industry, &p.RFPType, &p.OutlineTemplate, &p.ChartDistribution, &p.SectionWordRatio, &p.ScoringCoverage, &p.QualityScore, &p.Label, &sourceID); err != nil {
			return nil, err
		}
		p.ID = uuid.MustParse(idStr.String)
		if sourceID.Valid && sourceID.String != "" {
			sid := uuid.MustParse(sourceID.String)
			p.SourceBidID = &sid
		}
		patterns = append(patterns, p)
	}
	return patterns, rows.Err()
}

// ---- 学习：Prompt 变体 ----

func (s *SQLiteStore) SavePromptVariant(ctx context.Context, v *PromptVariant) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR REPLACE INTO prompt_variants (id, task, name, template, alpha, beta) VALUES (?,?,?,?,?,?)`,
		v.ID.String(), v.Task, v.Name, v.Template, v.Alpha, v.Beta)
	return err
}

func (s *SQLiteStore) ListPromptVariants(ctx context.Context, task string) ([]PromptVariant, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, task, name, template, alpha, beta FROM prompt_variants WHERE task = ?`, task)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var variants []PromptVariant
	for rows.Next() {
		var v PromptVariant
		var idStr string
		if err := rows.Scan(&idStr, &v.Task, &v.Name, &v.Template, &v.Alpha, &v.Beta); err != nil {
			return nil, err
		}
		v.ID = uuid.MustParse(idStr)
		variants = append(variants, v)
	}
	return variants, rows.Err()
}

func (s *SQLiteStore) UpdatePromptVariant(ctx context.Context, v *PromptVariant) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE prompt_variants SET alpha = ?, beta = ? WHERE id = ?`, v.Alpha, v.Beta, v.ID.String())
	return err
}

// ---- 审计 ----

func (s *SQLiteStore) SaveAuditIssues(ctx context.Context, issues []core.AuditIssue) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx,
		`INSERT OR REPLACE INTO audit_issues (id, bid_id, chapter_title, severity, dimension, issue, suggestion, evidence) VALUES (?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, iss := range issues {
		if _, err := stmt.ExecContext(ctx, iss.ID.String(), iss.BidID.String(), iss.ChapterTitle, iss.Severity, iss.Dimension, iss.Issue, iss.Suggestion, iss.Evidence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ---- 工具函数 ----

// cosineSim 计算两个向量的余弦相似度。
func cosineSim(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / float32(math.Sqrt(float64(na))*math.Sqrt(float64(nb)))
}
