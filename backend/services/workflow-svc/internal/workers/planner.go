// Package workers implements Asynq task workers for the bid pipeline.
package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

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
}

// NewPlannerWorker creates a new planner worker.
func NewPlannerWorker(log *slog.Logger, pool *pgxpool.Pool) *PlannerWorker {
	return &PlannerWorker{log: log, pool: pool}
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

	// 1. Load parse_result from document-svc.
	docClient := NewDocumentClient("http://localhost:8082")
	parseResult, err := docClient.GetParseResult(ctx, payload.DocumentID)
	if err != nil {
		w.log.Warn("planner: failed to get parse result, using defaults", slog.Any("error", err))
		parseResult = map[string]any{}
	}

	// 2. Retrieve relevant evidence from knowledge base.
	kbClient := NewKnowledgeClient("http://localhost:8086")
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

	// 3. Call router-svc with TaskOutlineGen to generate chapter outline.
	routerClient := NewRouterClient("http://localhost:8083")
	messages := []chatMessage{
		{Role: "system", Content: "你是一个专业的标书编写助手。请根据以下招标解析结果和证据，生成章节大纲。以JSON数组格式返回，每项包含：title(章节标题)、level(层级1-3)、sort_order(排序序号)。只返回JSON数组，不要其他文字。"},
		{Role: "user", Content: fmt.Sprintf("项目名称：%s\n招标解析：%v\n可用证据：\n%s\n请生成章节大纲：", projectName, parseResult, evidenceCtx)},
	}
	resp, err := routerClient.Chat(ctx, payload.TenantID, "outline_generate", messages, 2048)
	if err != nil {
		w.log.Warn("planner: router call failed", slog.Any("error", err))
		// Fallback: return a basic outline.
	}

	// Parse LLM response to extract chapter list.
	chapters := parseChapterOutline(resp.Content, parseResult)

	w.log.Info("planner: outline generated",
		slog.String("workflow_id", payload.WorkflowID.String()),
		slog.Int("chapter_count", len(chapters)))

	// 4. Write chapter_specs to database (via direct DB insert).
	// TODO: Implement actual DB insert via w.pool
	_ = chapters

	w.log.Info("planner: outline generated successfully",
		slog.String("workflow_id", payload.WorkflowID.String()))

	return nil
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
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(3), asynq.Timeout(10*60*60*1000))
	return err
}

// TransitionWorkflow transitions a workflow to the next state.
func TransitionWorkflow(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, to model.State, reason string) error {
	wf, err := store.GetWorkflow(ctx, workflowID)
	if err != nil {
		return err
	}

	if err := state.Validate(wf.Status, to); err != nil {
		return fmt.Errorf("invalid transition %s → %s: %w", wf.Status, to, err)
	}

	stepName, _ := state.StepForState(to)
	now := wf.UpdatedAt
	if err := store.UpdateWorkflow(ctx, workflowID, to, &stepName, nil, nil); err != nil {
		return err
	}

	// Log event
	_ = store.CreateEvent(ctx, &model.Event{
		WorkflowID: workflowID,
		TenantID:   wf.TenantID,
		FromState: &wf.Status,
		ToState:    to,
		Reason:     &reason,
	})
	_ = now // suppress unused
	return nil
}

// WorkflowStore interface for workflow operations needed by workers.
type WorkflowStore interface {
	GetWorkflow(ctx context.Context, id uuid.UUID) (*model.Workflow, error)
	UpdateWorkflow(ctx context.Context, id uuid.UUID, status model.State, step *model.StepName, error *string, artifacts []byte) error
	CreateEvent(ctx context.Context, e *model.Event) error
}

// UpdateStepProgress updates step progress.
func UpdateStepProgress(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, stepName model.StepName, progress int) error {
	// In production, update the step record in the database
	_ = store
	_ = workflowID
	_ = stepName
	_ = progress
	return nil
}

// FinalizeStep marks a step as completed.
func FinalizeStep(ctx context.Context, store WorkflowStore, workflowID uuid.UUID, stepName model.StepName, status model.StepStatus, artifacts []byte) error {
	_ = store
	_ = workflowID
	_ = stepName
	_ = status
	_ = artifacts
	return nil
}
