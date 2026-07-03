package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChapterHandlers handles chapter-spec and chapter-content endpoints.
type ChapterHandlers struct {
	Pool     *pgxpool.Pool
	Log      *slog.Logger
	Enqueuer Enqueuer // optional; enables single-chapter generation
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
}

// ChapterContentOut is the JSON shape for chapter content.
type ChapterContentOut struct {
	ChapterSpecID    string `json:"chapter_spec_id"`
	Version          int    `json:"version"`
	ContentText      string `json:"content_text"`
	WordCount        int    `json:"word_count"`
	MinWordMet       bool   `json:"min_word_met"`
	GeneratedBy      string `json:"generated_by"`
	LLMModel         string `json:"llm_model,omitempty"`
	LLMTask          string `json:"llm_task,omitempty"`
	GenerationMs     int64  `json:"generation_duration_ms,omitempty"`
	Status           string `json:"status,omitempty"`
}

// ChapterRoutes registers chapter endpoints under /api/v1/bids/{id}.
func (h *ChapterHandlers) ChapterRoutes(r chi.Router) {
	r.Get("/outline", h.listOutline)
	r.Post("/outline", h.addChapter)
	r.Put("/material", h.saveMaterial)
	r.Route("/chapters/{chapterId}", func(r chi.Router) {
		r.Put("/", h.updateChapter)
		r.Delete("/", h.deleteChapter)
		r.Get("/content", h.getChapterContent)
		r.Put("/content", h.saveChapterContent)
		r.Post("/generate", h.generateChapter)
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
		       writing_style, priority, status
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
			&c.WritingStyle, &c.Priority, &c.Status); err != nil {
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
	if req.Title != nil { sets = append(sets, "title = $"+strconv.Itoa(idx)); args = append(args, *req.Title); idx++ }
	if req.Level != nil { sets = append(sets, "level = $"+strconv.Itoa(idx)); args = append(args, *req.Level); idx++ }
	if req.OrderIndex != nil { sets = append(sets, "order_index = $"+strconv.Itoa(idx)); args = append(args, *req.OrderIndex); idx++ }
	if req.TargetWordCount != nil { sets = append(sets, "target_word_count = $"+strconv.Itoa(idx)); args = append(args, *req.TargetWordCount); idx++ }
	if req.MinWordCount != nil { sets = append(sets, "min_word_count = $"+strconv.Itoa(idx)); args = append(args, *req.MinWordCount); idx++ }
	if req.Priority != nil { sets = append(sets, "priority = $"+strconv.Itoa(idx)); args = append(args, *req.Priority); idx++ }
	if req.Status != nil { sets = append(sets, "status = $"+strconv.Itoa(idx)); args = append(args, *req.Status); idx++ }
	if len(sets) == 0 { httperr.InvalidInput(w, rid, "no fields to update", nil); return }
	sets = append(sets, "updated_at = NOW()")
	args = append(args, chapterID)
	query := "UPDATE chapter_specs SET " + joinStrings(sets, ", ") + " WHERE id = $" + strconv.Itoa(idx)
	_, err = h.Pool.Exec(r.Context(), query, args...)
	if err != nil { httperr.InternalError(w, rid); return }
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": chapterID.String(), "status": "updated"}})
}

func (h *ChapterHandlers) deleteChapter(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil { httperr.InvalidInput(w, rid, "invalid chapter id", nil); return }
	_, err = h.Pool.Exec(r.Context(), `DELETE FROM chapter_specs WHERE id = $1`, chapterID)
	if err != nil { httperr.InternalError(w, rid); return }
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"status": "deleted"}})
}

func (h *ChapterHandlers) getChapterContent(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	chapterID, err := uuid.Parse(chi.URLParam(r, "chapterId"))
	if err != nil { httperr.InvalidInput(w, rid, "invalid chapter id", nil); return }
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
	if err != nil { httperr.InvalidInput(w, rid, "invalid chapter id", nil); return }

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
		if err := h.Enqueuer.EnqueueChapter(r.Context(), wfID, bidJobID, tenantID, chapterID, title); err != nil {
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
	if len(parts) == 0 { return "" }
	out := parts[0]
	for _, p := range parts[1:] { out += sep + p }
	return out
}
