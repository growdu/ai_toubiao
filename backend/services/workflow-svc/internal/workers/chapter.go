package workers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// ChapterPayload is the task payload for chapter content generation.
type ChapterPayload struct {
	WorkflowID   uuid.UUID `json:"workflow_id"`
	BidJobID    uuid.UUID `json:"bid_job_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	ChapterID   uuid.UUID `json:"chapter_id"`
	ChapterTitle string   `json:"chapter_title"`
}

// ChapterWorker processes chapter content generation tasks.
type ChapterWorker struct {
	log *slog.Logger
}

// NewChapterWorker creates a new chapter worker.
func NewChapterWorker(log *slog.Logger) *ChapterWorker {
	return &ChapterWorker{log: log}
}

// Process handles the chapter content generation task.
func (w *ChapterWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload ChapterPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	w.log.Info("chapter: generating content",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle))

	// 1. Search knowledge base for relevant evidence.
	kbClient := NewKnowledgeClient("http://localhost:8086")
	evidence, _ := kbClient.Search(ctx, payload.TenantID, payload.ChapterTitle+" 招标要求 技术方案", 3)

	// Build evidence context.
	evidenceCtx := ""
	for _, e := range evidence {
		evidenceCtx += fmt.Sprintf("- [%s]: %s\n\n", e.MaterialTitle, e.Content[:min(300, len(e.Content))])
	}
	if evidenceCtx == "" {
		evidenceCtx = "(暂无相关证据，请基于招标要求编写)"
	}

	// 2. Call router-svc with TaskContentGen to generate chapter content.
	routerClient := NewRouterClient("http://localhost:8083")
	messages := []chatMessage{
		{Role: "system", Content: "你是一个专业的标书编写助手。请根据以下章节标题和证据，生成规范的标书章节内容。使用Markdown格式输出，可以包含图表占位符 [!figure:id type=mermaid caption=说明]。所有数据必须来自证据，禁止编造。"},
		{Role: "user", Content: fmt.Sprintf("章节标题：%s\n可用证据：\n%s\n请生成章节内容：", payload.ChapterTitle, evidenceCtx)},
	}
	resp, err := routerClient.Chat(ctx, payload.TenantID, "content_generate", messages, 4096)
	if err != nil {
		w.log.Warn("chapter: router call failed, using placeholder", slog.Any("error", err))
		resp = &chatResponse{Content: fmt.Sprintf("# %s\n\n(内容待生成，请稍后重新生成)", payload.ChapterTitle)}
	}

	// 3. Parse for chart placeholders.
	content := resp.Content

	// 4. Write content to chapter_contents table.
	// TODO: Implement actual DB insert via w.pool.
	_ = content

	w.log.Info("chapter: content generated",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle))

	return nil
}

// EnqueueChapter enqueues a chapter generation task.
func EnqueueChapter(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID, chapterID uuid.UUID, chapterTitle string) error {
	payload := ChapterPayload{
		WorkflowID:   workflowID,
		BidJobID:    bidJobID,
		TenantID:    tenantID,
		ChapterID:   chapterID,
		ChapterTitle: chapterTitle,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskChapterGenerate, data)
	_, err = client.EnqueueContext(ctx, task, asynq.MaxRetry(3), asynq.Timeout(30*60*1000))
	return err
}

// EnqueueChapters enqueues multiple chapter generation tasks (batch).
func EnqueueChapters(ctx context.Context, client *asynq.Client, chapters []ChapterPayload) error {
	for _, ch := range chapters {
		if err := EnqueueChapter(ctx, client, ch.WorkflowID, ch.BidJobID, ch.TenantID, ch.ChapterID, ch.ChapterTitle); err != nil {
			return err
		}
	}
	return nil
}
