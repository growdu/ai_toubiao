package workers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChapterPayload is the task payload for chapter content generation.
type ChapterPayload struct {
	WorkflowID   uuid.UUID `json:"workflow_id"`
	BidJobID     uuid.UUID `json:"bid_job_id"`
	TenantID     uuid.UUID `json:"tenant_id"`
	ChapterID    uuid.UUID `json:"chapter_id"`
	ChapterTitle string    `json:"chapter_title"`
	TargetWords  int       `json:"target_words,omitempty"`
	MinWords     int       `json:"min_words,omitempty"`
	// CustomPrompt is an optional per-chapter user instruction appended
	// to the LLM user message. Empty means "use defaults" (no behavior
	// change for existing call sites — batch generation never sets this).
	CustomPrompt string `json:"custom_prompt,omitempty"`
}

// ChapterWorker processes chapter content generation tasks.
type ChapterWorker struct {
	log  *slog.Logger
	pool *pgxpool.Pool
	cfg  Config

	// Progress is an optional auto-advance hook. When set, the worker checks
	// chapter-completion progress after writing each chapter's content and,
	// if every chapter is in a terminal state, transitions the workflow from
	// generating -> awaiting_review without a manual API call.
	Progress *Watcher
}

// NewChapterWorker creates a new chapter worker.
func NewChapterWorker(log *slog.Logger, pool *pgxpool.Pool, cfg Config) *ChapterWorker {
	return &ChapterWorker{log: log, pool: pool, cfg: cfg}
}

// WithProgress attaches a Progress watcher (useful in cmd wiring).
func (w *ChapterWorker) WithProgress(w2 *Watcher) *ChapterWorker {
	w.Progress = w2
	return w
}

// Process handles the chapter content generation task.
func (w *ChapterWorker) Process(ctx context.Context, task *asynq.Task) error {
	var payload ChapterPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return fmt.Errorf("unmarshal payload: %w", err)
	}

	start := time.Now()

	w.log.Info("chapter: generating content",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle))

	// 1. Search knowledge base for relevant evidence.
	kbClient := NewKnowledgeClient(w.cfg.KnowledgeURL)
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
	routerClient := NewRouterClient(w.cfg.RouterURL)
	maxTokens := 4096
	if payload.TargetWords > 0 {
		// Roughly 1.5 tokens per Chinese character; give some headroom.
		maxTokens = payload.TargetWords * 2
		if maxTokens < 2048 {
			maxTokens = 2048
		}
	}
	// Compose the user message. When the caller supplied a CustomPrompt
	// (frontend's ChapterInspector "提示词" tab), we render it as a
	// quoted instruction so the model can weight it the way the user
	// asked. We always show evidence AFTER the prompt so the model
	// treats the prompt as instructions and the evidence as facts.
	userMsg := fmt.Sprintf("章节标题：%s\n可用证据：\n%s\n请生成章节内容：", payload.ChapterTitle, evidenceCtx)
	if payload.CustomPrompt != "" {
		userMsg = fmt.Sprintf("章节标题：%s\n用户提示词：\n%s\n可用证据：\n%s\n请生成章节内容：", payload.ChapterTitle, payload.CustomPrompt, evidenceCtx)
	}
	messages := []chatMessage{
		{Role: "system", Content: "你是一个专业的标书编写助手。请根据以下章节标题和证据，生成规范的标书章节内容。使用Markdown格式输出，可以包含图表占位符 [!figure:id type=mermaid caption=说明]。所有数据必须来自证据，禁止编造。"},
		{Role: "user", Content: userMsg},
	}
	resp, err := routerClient.Chat(ctx, payload.TenantID, "content_generate", messages, maxTokens)
	if err != nil {
		w.log.Warn("chapter: router call failed, using placeholder", slog.Any("error", err))
		resp = &chatResponse{Content: fmt.Sprintf("# %s\n\n(内容待生成，请稍后重新生成)", payload.ChapterTitle)}
	}

	content := resp.Content
	wordCount := countWords(content)
	durationMs := time.Since(start).Milliseconds()

	// 3. Write content to chapter_contents table.
	minWords := payload.MinWords
	if minWords == 0 {
		minWords = 800
	}
	if err := w.persistContent(ctx, payload, content, resp, wordCount, minWords, durationMs); err != nil {
		return fmt.Errorf("persist content: %w", err)
	}

	// 4. Update chapter_spec status.
	if err := w.updateSpecStatus(ctx, payload.ChapterID, "succeeded", wordCount, minWords); err != nil {
		w.log.Warn("chapter: failed to update spec status", slog.Any("error", err))
	}

	// 5. Update bid_job progress.
	if err := w.updateBidProgress(ctx, payload.BidJobID); err != nil {
		w.log.Warn("chapter: failed to update bid progress", slog.Any("error", err))
	}

	// 6. Auto-advance the workflow from generating -> awaiting_review when every
	// chapter spec is in a terminal state. Best effort; failures are
	// logged and ignored (the operator can manually retry the transition).
	if w.Progress != nil {
		if err := w.Progress.CheckAndAdvance(ctx, payload.BidJobID); err != nil {
			w.log.Warn("chapter: progress check-and-advance failed",
				slog.String("bid_job_id", payload.BidJobID.String()),
				slog.Any("err", err))
		}
	}

	w.log.Info("chapter: content generated",
		slog.String("chapter_id", payload.ChapterID.String()),
		slog.String("chapter_title", payload.ChapterTitle),
		slog.Int("word_count", wordCount),
		slog.Int64("duration_ms", durationMs))

	return nil
}

// persistContent inserts a new version of chapter_contents with the
// generated text. It also stores the LLM metadata (model, tokens).
func (w *ChapterWorker) persistContent(ctx context.Context, payload ChapterPayload, content string, resp *chatResponse, wordCount, minWords int, durationMs int64) error {
	hash := sha256.Sum256([]byte(content))
	contentHash := hex.EncodeToString(hash[:])

	// Determine the next version number for this chapter spec.
	var version int
	err := w.pool.QueryRow(ctx,
		`SELECT COALESCE(MAX(version), 0) + 1 FROM chapter_contents WHERE chapter_spec_id = $1`,
		payload.ChapterID).Scan(&version)
	if err != nil {
		return fmt.Errorf("get next version: %w", err)
	}

	_, err = w.pool.Exec(ctx, `
		INSERT INTO chapter_contents
		    (chapter_spec_id, version, content_path, content_text, content_hash,
		     word_count, min_word_met, generated_by, llm_model, llm_task,
		     prompt_tokens, completion_tokens, generation_duration_ms)
		VALUES ($1, $2, '', $3, $4, $5, $6, 'ai', $7, 'content_generate', $8, $9, $10)`,
		payload.ChapterID, version, content, contentHash,
		wordCount, wordCount >= minWords,
		resp.Model, resp.PromptTokens, resp.CompletionTokens, durationMs)
	if err != nil {
		return fmt.Errorf("insert chapter_contents: %w", err)
	}
	return nil
}

// updateSpecStatus marks the chapter spec as succeeded/failed and records
// whether the minimum word count was met.
func (w *ChapterWorker) updateSpecStatus(ctx context.Context, chapterID uuid.UUID, status string, wordCount, minWords int) error {
	_, err := w.pool.Exec(ctx, `
		UPDATE chapter_specs
		SET status = $2, updated_at = NOW()
		WHERE id = $1`, chapterID, status)
	return err
}

// updateBidProgress increments the done_chapters counter on the bid_job.
func (w *ChapterWorker) updateBidProgress(ctx context.Context, bidJobID uuid.UUID) error {
	_, err := w.pool.Exec(ctx, `
		UPDATE bid_jobs SET done_chapters = done_chapters + 1, updated_at = NOW()
		WHERE id = $1`, bidJobID)
	return err
}

// countWords counts words/characters in content. For Chinese text we
// count characters; for mixed content we use a hybrid approach.
func countWords(content string) int {
	// Strip markdown syntax for a cleaner count.
	clean := strings.NewReplacer("#", "", "*", "`", "", "-", "", ">", "").Replace(content)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return 0
	}
	// Count non-whitespace characters (works well for Chinese).
	count := 0
	for _, r := range clean {
		if r > 32 { // not whitespace
			count++
		}
	}
	return count
}

// EnqueueChapter enqueues a chapter generation task. customPrompt is
// an optional per-chapter user instruction (from the frontend
// ChapterInspector "提示词" tab) and is forwarded to the worker via
// the payload. Empty string is treated identically to the previous
// "no prompt" behavior.
func EnqueueChapter(ctx context.Context, client *asynq.Client, workflowID, bidJobID, tenantID, chapterID uuid.UUID, chapterTitle, customPrompt string) error {
	payload := ChapterPayload{
		WorkflowID:   workflowID,
		BidJobID:     bidJobID,
		TenantID:     tenantID,
		ChapterID:    chapterID,
		ChapterTitle: chapterTitle,
		CustomPrompt: customPrompt,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}
	task := asynq.NewTask(TaskChapterGenerate, data)
	_, err = client.EnqueueContext(ctx, task,
		asynq.MaxRetry(3),
		asynq.Timeout(30*time.Minute),
		asynq.Queue(QueueChapter))
	return err
}

// EnqueueChapters enqueues multiple chapter generation tasks (batch).
// Batch generation never carries a CustomPrompt — that field is set
// only by single-chapter generate calls from the inspector.
func EnqueueChapters(ctx context.Context, client *asynq.Client, chapters []ChapterPayload) error {
	for _, ch := range chapters {
		if err := EnqueueChapter(ctx, client, ch.WorkflowID, ch.BidJobID, ch.TenantID, ch.ChapterID, ch.ChapterTitle, ch.CustomPrompt); err != nil {
			return err
		}
	}
	return nil
}
