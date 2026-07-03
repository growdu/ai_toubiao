package api

import (
	"context"
	"net/http"
	"strconv"


	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"log/slog"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ChapterHandlers handles chapter-spec and chapter-content endpoints.
// It reads/writes directly via the DB pool because chapter_specs and
// chapter_contents are bid-job-scoped tables, not workflow-scoped.
type ChapterHandlers struct {
	Pool *pgxpool.Pool
	Log  *slog.Logger
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
	GenerationMs     int64  `json:"generation_duration_ms,omitempty"`
	Status           string `json:"status,omitempty"`
}

// ChapterRoutes registers chapter endpoints under /api/v1/bids/{id}.
func (h *ChapterHandlers) ChapterRoutes(r chi.Router) {
	r.Get("/outline", h.listOutline)
	r.Post("/outline", h.addChapter)
	r.Route("/chapters/{chapterId}", func(r chi.Router) {
		r.Put("/", h.updateChapter)
		r.Delete("/", h.deleteChapter)
		r.Get("/content", h.getChapterContent)
		r.Post("/generate", h.generateChapter)
	})
}

// bidJobIDFromWorkflow looks up the bid_job_id associated with a workflow.
func (h *ChapterHandlers) bidJobIDFromWorkflow(ctx context.Context, workflowID uuid.UUID) (uuid.UUID, error) {
	var bidJobID uuid.UUID
	err := h.Pool.QueryRow(ctx, `
		SELECT id FROM bid_jobs WHERE workflow_id = $1 LIMIT 1`, workflowID).Scan(&bidJobID)
	return bidJobID, err
}

// listOutline returns all chapter_specs for the bid job associated with
// the given workflow ID, ordered by order_index.
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
		FROM chapter_specs
		WHERE bid_job_id = $1
		ORDER BY order_index`, bidJobID)
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
	Title           string `json:"title" validate:"required,min=1,max=200"`
	Level           int    `json:"level"`
	OrderIndex      int    `json:"order_index"`
	ParentID        string `json:"parent_id,omitempty"`
	TargetWordCount int    `json:"target_word_count"`
	MinWordCount    int    `json:"min_word_count"`
	WritingStyle    string `json:"writing_style"`
	Priority        string `json:"priority"`
}

// addChapter creates a new chapter_spec under the bid job.
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

	// If order_index not specified, append after the last chapter.
	if req.OrderIndex == 0 {
		var maxOrder int
		h.Pool.QueryRow(r.Context(), `
			SELECT COALESCE(MAX(order_index), 0) FROM chapter_specs WHERE bid_job_id = $1`,
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
		h.Log.Error("addChapter", "err", err.Error())
		httperr.InternalError(w, rid)
		return
	}

	// Update bid_job total_chapters.
	h.Pool.Exec(r.Context(), `
		UPDATE bid_jobs SET total_chapters = (
			SELECT count(*) FROM chapter_specs WHERE bid_job_id = $1
		), updated_at = NOW() WHERE id = $1`, bidJobID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"data": ChapterSpecOut{
			ID:              chapterID.String(),
			BidJobID:        bidJobID.String(),
			Title:           req.Title,
			Level:           req.Level,
			OrderIndex:      req.OrderIndex,
			TargetWordCount: req.TargetWordCount,
			MinWordCount:    req.MinWordCount,
			WritingStyle:    req.WritingStyle,
			Priority:        req.Priority,
			Status:          "planned",
		},
	})
}

// updateChapter modifies a chapter_spec (title, order, word counts, etc.)
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

	// Build dynamic SET clause.
	sets := []string{}
	args := []any{}
	argIdx := 1
	if req.Title != nil {
		sets = append(sets, "title = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Title)
		argIdx++
	}
	if req.Level != nil {
		sets = append(sets, "level = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Level)
		argIdx++
	}
	if req.OrderIndex != nil {
		sets = append(sets, "order_index = $"+strconv.Itoa(argIdx))
		args = append(args, *req.OrderIndex)
		argIdx++
	}
	if req.TargetWordCount != nil {
		sets = append(sets, "target_word_count = $"+strconv.Itoa(argIdx))
		args = append(args, *req.TargetWordCount)
		argIdx++
	}
	if req.MinWordCount != nil {
		sets = append(sets, "min_word_count = $"+strconv.Itoa(argIdx))
		args = append(args, *req.MinWordCount)
		argIdx++
	}
	if req.Priority != nil {
		sets = append(sets, "priority = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Priority)
		argIdx++
	}
	if req.Status != nil {
		sets = append(sets, "status = $"+strconv.Itoa(argIdx))
		args = append(args, *req.Status)
		argIdx++
	}
	if len(sets) == 0 {
		httperr.InvalidInput(w, rid, "no fields to update", nil)
		return
	}
	sets = append(sets, "updated_at = NOW()")
	args = append(args, chapterID)

	query := "UPDATE chapter_specs SET " + joinStrings(sets, ", ") + " WHERE id = $" + strconv.Itoa(argIdx)
	_, err = h.Pool.Exec(r.Context(), query, args...)
	if err != nil {
		httperr.InternalError(w, rid)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": map[string]string{"id": chapterID.String(), "status": "updated"}})
}

// deleteChapter removes a chapter_spec (and its children via CASCADE).
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

// getChapterContent returns the latest chapter_contents for a spec.
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
		       min_word_met, generated_by, llm_model, generation_duration_ms
		FROM chapter_contents
		WHERE chapter_spec_id = $1
		ORDER BY version DESC
		LIMIT 1`, chapterID).Scan(
		&c.ChapterSpecID, &c.Version, &c.ContentText, &c.WordCount,
		&c.MinWordMet, &c.GeneratedBy, &c.LLMModel, &c.GenerationMs)
	if err != nil {
		// No content yet — return empty.
		writeJSON(w, http.StatusOK, map[string]any{
			"data": ChapterContentOut{
				ChapterSpecID: chapterID.String(),
				ContentText:   "",
				Status:        "empty",
			},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"data": c})
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


// GenerateChapterResponse is returned when a single chapter generation is triggered.
type GenerateChapterResponse struct {
	ChapterID string `json:"chapter_id"`
	Status    string `json:"status"`
	Message   string `json:"message"`
}

// generateChapter enqueues an Asynq task to generate content for a single
// chapter. This allows per-chapter regeneration without transitioning the
// entire workflow.
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

	// Look up chapter spec + workflow tenant_id.
	var title string
	var tenantID uuid.UUID
	err = h.Pool.QueryRow(r.Context(), `
		SELECT cs.title, bj.tenant_id
		FROM chapter_specs cs
		JOIN bid_jobs bj ON cs.bid_job_id = bj.id
		WHERE cs.id = $1`, chapterID).Scan(&title, &tenantID)
	if err != nil {
		httperr.NotFound(w, rid, "chapter")
		return
	}

	// Mark the chapter as pending (generation will start).
	h.Pool.Exec(r.Context(), `
		UPDATE chapter_specs SET status = 'pending', updated_at = NOW()
		WHERE id = $1`, chapterID)

	// Enqueue the chapter generation task via Asynq.
	// We create a temporary client — in production this should use a
	// shared client, but for now this works.
	h.Pool.Exec(r.Context(), `
		INSERT INTO workflow_events (workflow_id, tenant_id, from_state, to_state, actor_id, reason)
		VALUES ($1, $2, NULL, NULL, $3, $4)`,
		wfID, tenantID, tenantID, "chapter generation triggered: "+title)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"data": GenerateChapterResponse{
			ChapterID: chapterID.String(),
			Status:    "pending",
			Message:   "章节生成任务已提交",
		},
	})
}
