// Package workers implements Asynq task workers for the bid pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/services/workflow-svc/internal/state"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PlannerPayload is the task payload for outline generation.
type PlannerPayload struct {
	WorkflowID uuid.UUID `json:"workflow_id"`
	BidJobID   uuid.UUID `json:"bid_job_id"`
	TenantID   uuid.UUID `json:"tenant_id"`
	DocumentID uuid.UUID `json:"document_id"`
}

// PlannerWorker processes outline generation tasks.
type PlannerWorker struct {
	log  *slog.Logger
	pool *pgxpool.Pool
	cfg  Config
}

// NewPlannerWorker creates a new planner worker.
func NewPlannerWorker(log *slog.Logger, pool *pgxpool.Pool, cfg Config) *PlannerWorker {
	return &PlannerWorker{log: log, pool: pool, cfg: cfg}
}

// Process handles the outline generation task.
func (w *PlannerWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload PlannerPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("planner: processing",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.String("bid_job_id", payload.BidJobID.String()))

	// 1. Load material + parsed result. Prefer the bid_job's parse_result
	//    (written by the 4-step wizard parse endpoint) so the user's
	//    edited material flows into outline generation; fall back to
	//    document-svc for legacy flows that still carry a document_id.
	parseResult := map[string]any{}
	var bidParse []byte
	_ = w.pool.QueryRow(ctx,
		`SELECT parse_result FROM bid_jobs WHERE id = $1`, payload.BidJobID).Scan(&bidParse)
	if len(bidParse) > 0 {
		_ = json.Unmarshal(bidParse, &parseResult)
	}
	// Flatten parse_result.parsed (project_name, industry, ...) to top level
	// so downstream code reading parseResult["project_name"] keeps working.
	if parsed, ok := parseResult["parsed"].(map[string]any); ok {
		for k, v := range parsed {
			if _, exists := parseResult[k]; !exists {
				parseResult[k] = v
			}
		}
	}
	if _, ok := parseResult["project_name"]; !ok && payload.DocumentID != uuid.Nil {
		docClient := NewDocumentClient(w.cfg.DocumentURL)
		if pr, err := docClient.GetParseResult(ctx, payload.DocumentID); err == nil {
			parseResult = pr
		}
	}

	// 2. Retrieve relevant evidence from knowledge base.
	kbClient := NewKnowledgeClient(w.cfg.KnowledgeURL)
	projectName := ""
	if v, ok := parseResult["project_name"].(string); ok {
		projectName = v
	}
	evidence, _ := kbClient.Search(ctx, payload.TenantID, projectName+" 招标要求", 5)

	// Build evidence context for the LLM prompt.
	evidenceCtx := ""
	for _, e := range evidence {
		evidenceCtx += fmt.Sprintf("- [%s]: %s\n", e.MaterialTitle, e.Content[:min(200, len(e.Content))])
	}
	if evidenceCtx == "" {
		evidenceCtx = "(暂无相关证据)"
	}

	// 2.5. Query historical patterns from docgen-svc (if available).
	patternCtx := ""
	if w.cfg.DocgenURL != "" {
		patternsClient := NewDocgenPatternsClient(w.cfg.DocgenURL)
		industry, _ := parseResult["industry"].(string)
		rfpType, _ := parseResult["rfp_type"].(string)
		patterns, perr := patternsClient.GetPatterns(ctx, industry, rfpType, 3)
		if perr == nil && len(patterns) > 0 {
			best := patterns[0]
			patternCtx = fmt.Sprintf("\n参考历史大纲（质量评分%.0f，结果%s）：\n%s\n",
				best.QualityScore, best.Label, best.OutlineTemplate)
		}
	}

	// 3. Call router-svc with TaskOutlineGen to generate chapter outline.
	routerClient := NewRouterClient(w.cfg.RouterURL)
	messages := []chatMessage{
		{Role: "system", Content: "你是一个专业的标书编写助手。请根据以下招标解析结果和证据，生成章节大纲。以JSON数组格式返回，每项包含：title(章节标题)、level(层级1-3)、sort_order(排序序号)。只返回JSON数组，不要其他文字。"},
		{Role: "user", Content: fmt.Sprintf("项目名称：%s\n招标解析：%v\n可用证据：\n%s%s\n请生成章节大纲：", projectName, parseResult, evidenceCtx, patternCtx)},
	}
	resp, err := routerClient.Chat(ctx, payload.TenantID, "outline_generate", messages, 2048)
	if err != nil || resp == nil {
		w.log.Warn("planner: router call failed, using default outline", slog.Any("error", err))
		resp = &chatResponse{Content: ""}
	}

	// Parse LLM response to extract chapter list.
	chapters := parseChapterOutline(resp.Content, parseResult)

	w.log.Info("planner: outline generated",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.Int("chapter_count", len(chapters)))

	// 4. Write chapter_specs to database.
	if err := w.persistChapters(ctx, payload, chapters); err != nil {
		return fmt.Errorf("persist chapters: %w", err)
	}

	// 5. Update bid_job progress count + advance workflow to 'facts' (review
	// state). The user reviews the outline here (HIL pause point) and then
	// transitions to 'generating' which triggers chapter generation via the
	// dispatch handler.
	if err := w.advanceWorkflow(ctx, payload, len(chapters)); err != nil {
		w.log.Warn("planner: failed to advance workflow state", slog.Any("error", err))
	}

	w.log.Info("planner: outline generated successfully",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.Int("chapter_count", len(chapters)))

	return nil
}

// persistChapters inserts all generated chapter specs into chapter_specs.
func (w *PlannerWorker) persistChapters(ctx context.Context, payload PlannerPayload, chapters []map[string]any) error {
	tx, err := w.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Clear any previously generated specs for this bid job (re-runs).
	if _, err := tx.Exec(ctx, `DELETE FROM chapter_specs WHERE bid_job_id = $1`, payload.BidJobID); err != nil {
		return fmt.Errorf("clear old specs: %w", err)
	}

	for _, ch := range chapters {
		title, _ := ch["title"].(string)
		if title == "" {
			continue
		}
		level := intFromAny(ch["level"], 1)
		if level < 1 {
			level = 1
		}
		if level > 3 {
			level = 3
		}
		sortOrder := intFromAny(ch["sort_order"], 0)
		if sortOrder == 0 {
			sortOrder = intFromAny(ch["sort_order"], 0)
		}

		specID := uuid.New()
		_, err := tx.Exec(ctx, `
			INSERT INTO chapter_specs
			    (id, bid_job_id, title, level, order_index,
			     chapter_type, target_word_count, min_word_count,
			     writing_style, priority, status, version)
			VALUES ($1, $2, $3, $4, $5, 'normal', 1500, 800, 'formal', 'normal', 'planned', 1)`,
			specID, payload.BidJobID, title, level, sortOrder)
		if err != nil {
			return fmt.Errorf("insert chapter_spec %q: %w", title, err)
		}
	}

	// Update bid_job total_chapters count.
	if _, err := tx.Exec(ctx, `
		UPDATE bid_jobs SET total_chapters = $2, updated_at = NOW()
		WHERE id = $1`, payload.BidJobID, len(chapters)); err != nil {
		return fmt.Errorf("update bid_job: %w", err)
	}

	return tx.Commit(ctx)
}

// advanceWorkflow transitions the workflow from outlining to facts (the
// next happy-path state), so the pipeline can proceed to chapter generation.
func (w *PlannerWorker) advanceWorkflow(ctx context.Context, payload PlannerPayload, chapterCount int) error {
	if w.pool == nil {
		return nil
	}
	// Transition workflow state: outlining -> facts (next in linear plan).
	nextState, ok := nextAfterOutlining()
	if !ok {
		return nil
	}
	_, err := w.pool.Exec(ctx, `
		UPDATE workflows SET status = $2, current_step = $3, updated_at = NOW()
		WHERE id = $1 AND status = 'outlining'`,
		payload.WorkflowID, nextState, "facts")
	if err != nil {
		return fmt.Errorf("update workflow state: %w", err)
	}

	// Record the transition event.
	_, _ = w.pool.Exec(ctx, `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, $2, 'outlining', $3, $4, $5)`,
		payload.WorkflowID, payload.TenantID, nextState, payload.TenantID,
		fmt.Sprintf("outline generated: %d chapters", chapterCount))
	return nil
}

// nextAfterOutlining returns the state that follows outlining in the
// linear plan. Kept here to avoid importing state in every worker.
func nextAfterOutlining() (model.State, bool) {
	return state.NextState(model.StateOutlining)
}

// parseChapterOutline extracts chapter specs from LLM JSON response.
func parseChapterOutline(content string, parseResult map[string]any) []map[string]any {
	if content == "" {
		return defaultChapterOutline(parseResult)
	}
	// Try to extract JSON array from response.
	start := -1
	end := -1
	for i := 0; i < len(content); i++ {
		if content[i] == '[' {
			start = i
		}
		if content[i] == ']' && start >= 0 {
			end = i + 1
			break
		}
	}
	if start < 0 || end <= start {
		return defaultChapterOutline(parseResult)
	}
	var chapters []map[string]any
	if err := json.Unmarshal([]byte(content[start:end]), &chapters); err != nil {
		return defaultChapterOutline(parseResult)
	}
	return chapters
}

// defaultChapterOutline returns a fallback outline when LLM fails.
func defaultChapterOutline(parseResult map[string]any) []map[string]any {
	_ = parseResult // project name may be used in future versions
	return []map[string]any{
		{"title": "第一章 投标函", "level": 1, "sort_order": 1},
		{"title": "第二章 项目理解与总体思路", "level": 1, "sort_order": 2},
		{"title": "第三章 技术方案", "level": 1, "sort_order": 3},
		{"title": "第四章 项目实施计划", "level": 1, "sort_order": 4},
		{"title": "第五章 质量保证措施", "level": 1, "sort_order": 5},
		{"title": "第六章 售后服务", "level": 1, "sort_order": 6},
	}
}

// EnqueueOutline enqueues an outline generation task.
func EnqueueOutline(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID, documentID uuid.UUID) error {
	payload := PlannerPayload{
		WorkflowID: workflowID,
		BidJobID:   bidJobID,
		TenantID:   tenantID,
		DocumentID: documentID,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskOutlineGenerate, data)
	_, err = client.EnqueueContext(ctx, task,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
		asynq.Queue(QueuePlanner))
	return err
}

// intFromAny safely extracts an int from an any (JSON number or float).
func intFromAny(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return def
	}
}
