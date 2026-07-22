package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChapterHandlers handles chapter-spec and chapter-content endpoints.
type ChapterHandlers struct {
	Pool      *pgxpool.Pool
	Log       *slog.Logger
	Enqueuer  Enqueuer // optional; enables single-chapter generation
	RouterURL string   // optional; enables material parsing via router-svc
}

// ChapterSpecOut is the JSON shape returned to the frontend.
type ChapterSpecOut struct {
	ID              string `json:"id"`
	BidJobID        string `json:"bid_job_id"`
	ParentID        string `json:"parent_id,omitempty"`
	Title           string `json:"title"`
	Level           int    `json:"level"`
	OrderIndex      int    `json:"order_index"`
	ChapterType     string `json:"chapter_type"`
	TargetWordCount int    `json:"target_word_count"`
	MinWordCount    int    `json:"min_word_count"`
	WritingStyle    string `json:"writing_style"`
	Priority        string `json:"priority"`
	Status          string `json:"status"`
	ApprovedAt      string `json:"approved_at,omitempty"`
	ApprovedBy      string `json:"approved_by,omitempty"`
	RejectionReason string `json:"rejection_reason,omitempty"`
}

// ChapterContentOut is the JSON shape for chapter content.
type ChapterContentOut struct {
	ChapterSpecID string `json:"chapter_spec_id"`
	Version       int    `json:"version"`
	ContentText   string `json:"content_text"`
	WordCount     int    `json:"word_count"`
	MinWordMet    bool   `json:"min_word_met"`
	GeneratedBy   string `json:"generated_by"`
	LLMModel      string `json:"llm_model,omitempty"`
	LLMTask       string `json:"llm_task,omitempty"`
	GenerationMs  int64  `json:"generation_duration_ms,omitempty"`
	Status        string `json:"status,omitempty"`
}

// ChapterRoutes registers chapter endpoints under /api/v1/bids/{id}.
func (h *ChapterHandlers) ChapterRoutes(r chi.Router) {
	r.Get("/outline", h.listOutline)
	r.Post("/outline", h.addChapter)
	r.Post("/outline/reorder", h.reorderOutline)
	r.Put("/material", h.saveMaterial)
	r.Post("/parse", h.parseMaterial)
	r.Get("/parse", h.getParse)
	r.Put("/parse", h.updateParse)
	r.Route("/chapters/{chapterId}", func(r chi.Router) {
		r.Put("/", h.updateChapter)
		r.Delete("/", h.deleteChapter)
		r.Get("/content", h.getChapterContent)
		r.Put("/content", h.saveChapterContent)
		r.Post("/generate", h.generateChapter)
		r.Post("/approve", h.approveChapter)
		r.Post("/reject", h.rejectChapter)
	})
}

// bidJobIDFromWorkflow looks up the bid_job_id associated with a workflow.
func (h *ChapterHandlers) bidJobIDFromWorkflow(ctx context.Context, workflowID uuid.UUID) (uuid.UUID, error) {
	var bidJobID uuid.UUID
	err := h.Pool.QueryRow(ctx,
		`SELECT id FROM bid_jobs WHERE workflow_id = $1 LIMIT 1`, workflowID).Scan(&bidJobID)
	return bidJobID, err
}

// listOutline returns all chapter_specs for the bid job.
func (h *ChapterHandlers) listOutline(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{}})
		return
	}
	rows, err := h.Pool.Query(r.Context(), `
		SELECT id, bid_job_id, COALESCE(parent_id::text, ''), title, level,
		       order_index, chapter_type, target_word_count, min_word_count,
		       writing_style, priority, status,
		       COALESCE(TO_CHAR(approved_at, 'YYYY-MM-DD"T"HH24:MI:SSOF'), ''),
		       COALESCE(approved_by::text, ''),
		       COALESCE(rejection_reason, '')
		FROM chapter_specs WHERE bid_job_id = $1 ORDER BY order_index`, bidJobID)
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	defer rows.Close()
	var out []ChapterSpecOut
	for rows.Next() {
		var c ChapterSpecOut
		if err := rows.Scan(&c.ID, &c.BidJobID, &c.ParentID, &c.Title, &c.Level,
			&c.OrderIndex, &c.ChapterType, &c.TargetWordCount, &c.MinWordCount,
			&c.WritingStyle, &c.Priority, &c.Status,
			&c.ApprovedAt, &c.ApprovedBy, &c.RejectionReason); err != nil {
			httperr.InternalError(w, rid)
			return
		}
		out = append(out, c)
	}
	if out == nil {
		out = []ChapterSpecOut{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": out})
}

// addChapterRequest is the body for POST /outline.
type addChapterRequest struct {
	Title           string `json:"title"`
	Level           int    `json:"level"`
	OrderIndex      int    `json:"order_index"`
	ParentID        string `json:"parent_id,omitempty"`
	TargetWordCount int    `json:"target_word_count"`
	MinWordCount    int    `json:"min_word_count"`
	WritingStyle    string `json:"writing_style"`
	Priority        string `json:"priority"`
}

func (h *ChapterHandlers) addChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	var req addChapterRequest
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if req.Title == "" {
		httperr.InvalidInput(w, rid, "title is required", nil)
		return
	}
	if req.Level < 1 || req.Level > 3 {
		req.Level = 1
	}
	if req.TargetWordCount == 0 {
		req.TargetWordCount = 1500
	}
	if req.MinWordCount == 0 {
		req.MinWordCount = 800
	}
	if req.WritingStyle == "" {
		req.WritingStyle = "formal"
	}
	if req.Priority == "" {
		req.Priority = "normal"
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		httperr.NotFound(w, rid, "bid_job for workflow")
		return
	}
	if req.OrderIndex == 0 {
		var maxOrder int
		h.Pool.QueryRow(r.Context(),
			`SELECT COALESCE(MAX(order_index), 0) FROM chapter_specs WHERE bid_job_id = $1`,
			bidJobID).Scan(&maxOrder)
		req.OrderIndex = maxOrder + 1
	}
	chapterID := uuid.New()
	var parentID *uuid.UUID
	if req.ParentID != "" {
		if pid, err := uuid.Parse(req.ParentID); err == nil {
			parentID = &pid
		}
	}
	_, err = h.Pool.Exec(r.Context(), `
		INSERT INTO chapter_specs
			(id, bid_job_id, parent_id, title, level, order_index,
			 chapter_type, target_word_count, min_word_count,
			 writing_style, priority, status, version)
		VALUES ($1, $2, $3, $4, $5, $6, 'normal', $7, $8, $9, $10, 'planned', 1)`,
		chapterID, bidJobID, parentID, req.Title, req.Level, req.OrderIndex,
		req.TargetWordCount, req.MinWordCount, req.WritingStyle, req.Priority)
	if err != nil {
		h.Log.Error("addChapter", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	h.Pool.Exec(r.Context(), `
		UPDATE bid_jobs SET total_chapters = (
			SELECT count(*) FROM chapter_specs WHERE bid_job_id = $1
		), updated_at = NOW() WHERE id = $1`, bidJobID)
	writeJSON(w, http.StatusCreated, map[string]any{
		"data": ChapterSpecOut{
			ID: chapterID.String(), BidJobID: bidJobID.String(),
			Title: req.Title, Level: req.Level, OrderIndex: req.OrderIndex,
			TargetWordCount: req.TargetWordCount, MinWordCount: req.MinWordCount,
			WritingStyle: req.WritingStyle, Priority: req.Priority, Status: "planned",
		},
	})
}

func (h *ChapterHandlers) updateChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}
	var req struct {
		Title           *string `json:"title,omitempty"`
		Level           *int    `json:"level,omitempty"`
		OrderIndex      *int    `json:"order_index,omitempty"`
		TargetWordCount *int    `json:"target_word_count,omitempty"`
		MinWordCount    *int    `json:"min_word_count,omitempty"`
		Priority        *string `json:"priority,omitempty"`
		Status          *string `json:"status,omitempty"`
	}
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	sets := []string{}
	args := []any{}
	idx := 1
	if req.Title != nil {
		sets = append(sets, "title = $"+strconv.Itoa(idx))
		args = append(args, *req.Title)
		idx++
	}
	if req.Level != nil {
		sets = append(sets, "level = $"+strconv.Itoa(idx))
		args = append(args, *req.Level)
		idx++
	}
	if req.OrderIndex != nil {
		sets = append(sets, "order_index = $"+strconv.Itoa(idx))
		args = append(args, *req.OrderIndex)
		idx++
	}
	if req.TargetWordCount != nil {
		sets = append(sets, "target_word_count = $"+strconv.Itoa(idx))
		args = append(args, *req.TargetWordCount)
		idx++
	}
	if req.MinWordCount != nil {
		sets = append(sets, "min_word_count = $"+strconv.Itoa(idx))
		args = append(args, *req.MinWordCount)
		idx++
	}
	if req.Priority != nil {
		sets = append(sets, "priority = $"+strconv.Itoa(idx))
		args = append(args, *req.Priority)
		idx++
	}
	if req.Status != nil {
		sets = append(sets, "status = $"+strconv.Itoa(idx))
		args = append(args, *req.Status)
		idx++
	}
	if len(sets) == 0 {
		httperr.InvalidInput(w, rid, "no fields to update", nil)
		return
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, chapterID)
	query := "UPDATE chapter_specs SET " + joinStrings(sets, ", ") + " WHERE id = $" + strconv.Itoa(idx)
	_, err = h.Pool.Exec(r.Context(), query, args...)
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": chapterID.String(), "status": "updated"}})
}

func (h *ChapterHandlers) deleteChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}
	_, err = h.Pool.Exec(r.Context(), `DELETE FROM chapter_specs WHERE id = $1`, chapterID)
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "deleted"}})
}

func (h *ChapterHandlers) getChapterContent(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}
	var c ChapterContentOut
	err = h.Pool.QueryRow(r.Context(), `
		SELECT chapter_spec_id, version, content_text, word_count,
		       min_word_met, generated_by,
		       COALESCE(llm_model, ''), COALESCE(llm_task, ''),
		       COALESCE(generation_duration_ms, 0)
		FROM chapter_contents WHERE chapter_spec_id = $1
		ORDER BY version DESC LIMIT 1`, chapterID).Scan(
		&c.ChapterSpecID, &c.Version, &c.ContentText, &c.WordCount,
		&c.MinWordMet, &c.GeneratedBy, &c.LLMModel, &c.LLMTask, &c.GenerationMs)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": ChapterContentOut{
			ChapterSpecID: chapterID.String(), ContentText: "", Status: "empty",
		}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
}

// saveChapterContent saves user-edited content as a new version with
// generated_by='human'. This allows inline editing in the frontend.
func (h *ChapterHandlers) saveChapterContent(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}

	var req struct {
		ContentText string `json:"content_text"`
	}
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}

	// Get min_word_count for this chapter to check if met.
	var minWords int
	h.Pool.QueryRow(r.Context(),
		`SELECT min_word_count FROM chapter_specs WHERE id = $1`, chapterID).Scan(&minWords)

	wordCount := countChars(req.ContentText)
	hash := sha256.Sum256([]byte(req.ContentText))
	contentHash := hex.EncodeToString(hash[:])

	var version int
	h.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(MAX(version), 0) + 1 FROM chapter_contents WHERE chapter_spec_id = $1`,
		chapterID).Scan(&version)

	_, err = h.Pool.Exec(r.Context(), `
		INSERT INTO chapter_contents
			(chapter_spec_id, version, content_path, content_text, content_hash,
			 word_count, min_word_met, generated_by, llm_task)
		VALUES ($1, $2, '', $3, $4, $5, $6, 'human', 'manual_edit')`,
		chapterID, version, req.ContentText, contentHash,
		wordCount, wordCount >= minWords)
	if err != nil {
		h.Log.Error("saveChapterContent", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	// Update chapter spec status.
	h.Pool.Exec(r.Context(),
		`UPDATE chapter_specs SET status = 'succeeded', updated_at = NOW() WHERE id = $1`,
		chapterID)

	writeJSON(w, http.StatusOK, map[string]any{"data": ChapterContentOut{
		ChapterSpecID: chapterID.String(), Version: version,
		ContentText: req.ContentText, WordCount: wordCount,
		MinWordMet: wordCount >= minWords, GeneratedBy: "human",
	}})
}

// generateChapter enqueues an Asynq task to generate content for a single
// chapter. If the Enqueuer is not wired, it falls back to marking the
// chapter as pending.
func (h *ChapterHandlers) generateChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}

	// Optional request body — currently only `prompt` (the per-chapter
	// user instruction from ChapterInspector's "提示词" tab). The body
	// is read best-effort: a missing or malformed body simply means
	// "no custom prompt", which is the historical behavior.
	var req struct {
		Prompt string `json:"prompt"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = readJSON(r.Body, &req) // ignore — empty prompt is fine
	}
	customPrompt := strings.TrimSpace(req.Prompt)

	// Look up chapter spec + bid job info.
	var title string
	var tenantID uuid.UUID
	var bidJobID uuid.UUID
	err = h.Pool.QueryRow(r.Context(), `
		SELECT cs.title, bj.tenant_id, bj.id
		FROM chapter_specs cs
		JOIN bid_jobs bj ON cs.bid_job_id = bj.id
		WHERE cs.id = $1`, chapterID).Scan(&title, &tenantID, &bidJobID)
	if err != nil {
		httperr.NotFound(w, rid, "chapter")
		return
	}

	// Mark the chapter as pending.
	h.Pool.Exec(r.Context(),
		`UPDATE chapter_specs SET status = 'pending', updated_at = NOW() WHERE id = $1`,
		chapterID)

	// Enqueue the chapter generation task via Asynq if enqueuer is available.
	if h.Enqueuer != nil {
		if err := h.Enqueuer.EnqueueChapter(r.Context(), wfID, bidJobID, tenantID, chapterID, title, customPrompt); err != nil {
			h.Log.Warn("generateChapter: enqueue failed",
				slog.String("chapter_id", chapterID.String()),
				slog.String("err", err.Error()))
			// Revert status to planned on failure.
			h.Pool.Exec(r.Context(),
				`UPDATE chapter_specs SET status = 'planned' WHERE id = $1`, chapterID)
			httperr.Write(w, http.StatusServiceUnavailable, "ENQUEUE_FAILED",
				"生成任务入队失败: "+err.Error(), rid, nil)
			return
		}
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": map[string]any{
			"chapter_id": chapterID.String(),
			"status":     "pending",
			"message":    "章节生成任务已提交",
		},
	})
}

// saveMaterial saves RFP material text to the bid_job's metadata.
func (h *ChapterHandlers) saveMaterial(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	var req struct {
		MaterialText string `json:"material_text"`
	}
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		httperr.NotFound(w, rid, "bid_job")
		return
	}
	// Store material text in bid_jobs.parse_result as a JSON field.
	_, err = h.Pool.Exec(r.Context(), `
		UPDATE bid_jobs
		SET parse_result = parse_result || jsonb_build_object('material_text', $2::text),
		    updated_at = NOW()
		WHERE id = $1`, bidJobID, req.MaterialText)
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "saved"}})
}

// countChars counts non-whitespace characters (suitable for Chinese text).
func countChars(s string) int {
	count := 0
	for _, r := range s {
		if r > 32 {
			count++
		}
	}
	return count
}

func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += sep + p
	}
	return out
}

// reorderOutline 更新章节顺序与父子关系。前端拖拽后下发一个有序列表，
// 每项含 id 与重排后的 parent_id（可为空）。按列表顺序写回 order_index。
func (h *ChapterHandlers) reorderOutline(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		httperr.NotFound(w, rid, "bid_job")
		return
	}
	var req struct {
		Ordered []struct {
			ID       string `json:"id"`
			ParentID string `json:"parent_id"`
		} `json:"ordered"`
	}
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if len(req.Ordered) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
		return
	}
	tx, err := h.Pool.Begin(r.Context())
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	defer tx.Rollback(r.Context())
	for i, item := range req.Ordered {
		var parentID any
		if item.ParentID != "" {
			parentID = item.ParentID
		}
		if _, err := tx.Exec(r.Context(), `
			UPDATE chapter_specs
			SET order_index = $2, parent_id = $3::uuid, updated_at = NOW()
			WHERE id = $1 AND bid_job_id = $4`,
			item.ID, i, parentID, bidJobID); err != nil {
			h.Log.Error("reorderOutline", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}
	}
	if err := tx.Commit(r.Context()); err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]bool{"ok": true}})
}

// approveChapter 标记章节为已审核：status='approved'，写入 approved_at/approved_by，
// 清空 rejection_reason。已审核章节才允许进入审计阶段。
func (h *ChapterHandlers) approveChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}
	actor := r.Header.Get("X-User-ID")
	var actorUUID any
	if actor != "" {
		if u, err := uuid.Parse(actor); err == nil {
			actorUUID = u
		}
	}
	var approvedAt time.Time
	err = h.Pool.QueryRow(r.Context(), `
		UPDATE chapter_specs
		SET status = 'approved', approved_at = NOW(), approved_by = $2,
		    rejection_reason = NULL, updated_at = NOW()
		WHERE id = $1
		RETURNING approved_at`, chapterID, actorUUID).Scan(&approvedAt)
	if err != nil {
		httperr.NotFound(w, rid, "chapter")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"id":          chapterID.String(),
		"status":      "approved",
		"approved_at": approvedAt.Format(time.RFC3339),
	}})
}

// rejectChapter 把章节送回生成队列：status 回退为 'planned'，记录
// rejection_reason 供生成 worker 与前端 tooltip 展示。
func (h *ChapterHandlers) rejectChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid chapter id", nil)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = readJSON(r.Body, &req)
	}
	reason := strings.TrimSpace(req.Reason)
	if _, err := h.Pool.Exec(r.Context(), `
		UPDATE chapter_specs
		SET status = 'planned', rejection_reason = NULLIF($2, ''),
		    approved_at = NULL, approved_by = NULL, updated_at = NOW()
		WHERE id = $1`, chapterID, reason); err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"id":               chapterID.String(),
		"status":           "planned",
		"rejection_reason": reason,
	}})
}

// parseMaterial 调用 router-svc 把原始材料解析成结构化字段，写入
// bid_jobs.parse_result 并返回。4 步向导"步骤1：解析材料"的后端入口。
func (h *ChapterHandlers) parseMaterial(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		httperr.NotFound(w, rid, "bid_job")
		return
	}
	tenantID, _ := uuid.Parse(r.Header.Get("X-Tenant-ID"))
	var req struct {
		MaterialText string `json:"material_text"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		_ = readJSON(r.Body, &req)
	}
	material := req.MaterialText
	if material == "" {
		_ = h.Pool.QueryRow(r.Context(),
			`SELECT COALESCE(parse_result->>'material_text','') FROM bid_jobs WHERE id = $1`,
			bidJobID).Scan(&material)
	}
	if strings.TrimSpace(material) == "" {
		httperr.InvalidInput(w, rid, "材料为空，请先在步骤1粘贴或上传招标材料", nil)
		return
	}
	parsed := h.callParser(r.Context(), tenantID, material)
	if _, err := h.Pool.Exec(r.Context(), `
		UPDATE bid_jobs
		SET parse_result = parse_result || jsonb_build_object(
		        'material_text', $2::text,
		        'parsed', $3::jsonb,
		        'parsed_at', to_char(NOW(), 'YYYY-MM-DD"T"HH24:MI:SSOF')),
		    updated_at = NOW()
		WHERE id = $1`, bidJobID, material, []byte(parsed)); err != nil {
		h.Log.Error("parseMaterial persist", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"material_text": material,
		"parsed":        json.RawMessage(parsed),
	}})
}

// getParse 读取已保存的解析结果与原始材料，供前端步骤2展示与编辑。
func (h *ChapterHandlers) getParse(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
			"material_text": "", "parsed": map[string]any{},
		}})
		return
	}
	var material, parsed []byte
	_ = h.Pool.QueryRow(r.Context(),
		`SELECT COALESCE(parse_result->>'material_text',''),
		        COALESCE(parse_result->'parsed','{}'::jsonb)
		 FROM bid_jobs WHERE id = $1`, bidJobID).Scan(&material, &parsed)
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{
		"material_text": string(material),
		"parsed":        json.RawMessage(parsed),
	}})
}

// updateParse 用用户编辑后的解析结果覆盖 parse_result.parsed，并同步
// material_text。4 步向导"步骤2：审核编辑"的保存入口。
func (h *ChapterHandlers) updateParse(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	wfID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid workflow id", nil)
		return
	}
	bidJobID, err := h.bidJobIDFromWorkflow(r.Context(), wfID)
	if err != nil {
		httperr.NotFound(w, rid, "bid_job")
		return
	}
	var req struct {
		MaterialText string          `json:"material_text"`
		Parsed       json.RawMessage `json:"parsed"`
	}
	if err := readJSON(r.Body, &req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON", nil)
		return
	}
	if _, err := h.Pool.Exec(r.Context(), `
		UPDATE bid_jobs
		SET parse_result = parse_result || jsonb_build_object(
		        'material_text', $2::text,
		        'parsed', $3::jsonb),
		    updated_at = NOW()
		WHERE id = $1`, bidJobID, req.MaterialText, []byte(req.Parsed)); err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"status": "saved"}})
}

// callParser 调用 router-svc /chat，用结构化解析 prompt 把材料文本转成 JSON。
// router-svc 不可用时回退为空对象，保证流程不中断。
func (h *ChapterHandlers) callParser(ctx context.Context, tenantID uuid.UUID, material string) []byte {
	if h.RouterURL == "" {
		return []byte(`{}`)
	}
	prompt := `请从以下招标材料中提取结构化信息，严格只返回 JSON（不要任何解释文字），字段如下：
{"project_name":"项目名称","bid_no":"招标编号","industry":"行业","rfp_type":"招标类型","issuer":"招标人","deadline":"投标截止时间","budget":"预算金额","overview":"项目概述(100字内)","requirements":["采购需求要点"],"technical_specs":["技术参数"],"scoring_criteria":["评分标准要点"],"qualifications":["资质要求"]}
材料：
` + material
	body := map[string]any{
		"tenant_id":  tenantID,
		"task":       "rfp_parse",
		"messages":   []map[string]string{{"role": "system", "content": "你是招标文件解析助手，只输出 JSON。"}, {"role": "user", "content": prompt}},
		"max_tokens": 2048,
	}
	buf, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, h.RouterURL+"/api/v1/router/chat", bytes.NewReader(buf))
	httpReq.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		h.Log.Warn("parseMaterial: router call failed", slog.String("err", err.Error()))
		return []byte(`{}`)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return []byte(`{}`)
	}
	var wrapper struct {
		Data struct {
			Content string `json:"content"`
		} `json:"data"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&wrapper)
	if strings.TrimSpace(wrapper.Data.Content) == "" {
		return []byte(`{}`)
	}
	return []byte(extractJSON(wrapper.Data.Content))
}

// extractJSON 从 LLM 响应中提取最外层 JSON 对象（兼容 markdown 代码围栏）。
func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```json")
		s = strings.TrimPrefix(s, "```")
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return "{}"
	}
	return s[start : end+1]
}
