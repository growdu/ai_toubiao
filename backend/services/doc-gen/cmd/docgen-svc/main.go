// docgen-svc 是 doc-gen 模块的服务化入口（Phase2）。
// 内核与 bidgen CLI 共享同一 Pipeline，仅入口壳不同。
// 提供 HTTP API 供 workflow-svc 和 api-gateway 调用。
//
// 端点：
//   POST /api/v1/docgen/generate   { material_dir, rfp_path, options } → { task_id }
//   GET  /api/v1/docgen/tasks/:id  → { status, progress, output_path, issues }
//   POST /api/v1/docgen/assemble   { bid_package_id, format: "word" | "pdf" } -> { download_url, format, output_path } → { download_url }
//   GET  /api/v1/docgen/download/:id  → serves the generated .docx file
//   POST /api/v1/docgen/render        { chapters } → { download_url }  (external integration)
//   POST /api/v1/docgen/learn         { chapters, industry } → { pattern_id, quality_score }
//   GET  /api/v1/docgen/patterns       ?industry=X&rfp_type=Y → { patterns[] }
//   GET  /healthz                  → { status: ok }
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/analyzer"
	"github.com/bidwriter/services/doc-gen/internal/assembler"
	"github.com/bidwriter/services/doc-gen/internal/auditor"
	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/generator"
	"github.com/bidwriter/services/doc-gen/internal/illustrator"
	"github.com/bidwriter/services/doc-gen/internal/ingest"
	"github.com/bidwriter/services/doc-gen/internal/learner"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/bidwriter/services/doc-gen/internal/pdfexport"
	"github.com/bidwriter/services/doc-gen/internal/planner"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
)

// Task 是一个异步生成任务。
type Task struct {
	ID         string             `json:"id"`
	Status     string             `json:"status"` // pending / running / done / failed
	OutputPath string             `json:"output_path,omitempty"`
	Issues     int                `json:"issues,omitempty"`
	Error      string             `json:"error,omitempty"`
	StartedAt  time.Time          `json:"started_at"`
	FinishedAt *time.Time         `json:"finished_at,omitempty"`
	Result     *core.PipelineResult `json:"-"`
}

// TaskManager 管理异步任务。
type TaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*Task
}

func NewTaskManager() *TaskManager {
	return &TaskManager{tasks: make(map[string]*Task)}
}

func (tm *TaskManager) Create() *Task {
	t := &Task{
		ID:        uuid.New().String(),
		Status:    "pending",
		StartedAt: time.Now(),
	}
	tm.mu.Lock()
	tm.tasks[t.ID] = t
	tm.mu.Unlock()
	return t
}

func (tm *TaskManager) Get(id string) (*Task, bool) {
	tm.mu.RLock()
	t, ok := tm.tasks[id]
	tm.mu.RUnlock()
	return t, ok
}

func (tm *TaskManager) Update(t *Task) {
	tm.mu.Lock()
	tm.tasks[t.ID] = t
	tm.mu.Unlock()
}

// ---- 服务 ----

type Server struct {
	store        *store.SQLiteStore
	llm          llm.Client
	tasks        *TaskManager
	log          *slog.Logger
	learner      *learner.Learner
	pdfConverter pdfexport.Converter
}

func newServer() (*Server, error) {
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	dbPath := os.Getenv("DOCGEN_DB_PATH")
	if dbPath == "" {
		dbPath = "docgen.db"
	}

	st, err := store.NewSQLite(dbPath)
	if err != nil {
		return nil, fmt.Errorf("store: %w", err)
	}
	if err := st.Init(context.Background()); err != nil {
		return nil, fmt.Errorf("store init: %w", err)
	}

	// LLM 客户端
	var client llm.Client
	if key := os.Getenv("ANTHROPIC_AUTH_TOKEN"); key != "" {
		base := os.Getenv("ANTHROPIC_BASE_URL")
		if base == "" {
			base = "https://api.anthropic.com"
		}
		model := os.Getenv("ANTHROPIC_MODEL")
		if model == "" {
			model = "claude-sonnet-4-20250514"
		}
		client = llm.NewAnthropicClient(key, base, model)
		log.Info("LLM: Anthropic 兼容", "base", base, "model", model)
	} else if key := os.Getenv("LLM_API_KEY"); key != "" {
		base := os.Getenv("LLM_API_BASE")
		if base == "" {
			base = "https://api.openai.com/v1"
		}
		model := os.Getenv("LLM_MODEL")
		if model == "" {
			model = "gpt-4o"
		}
		client = llm.NewDirectClient(key, base, model, "text-embedding-3-small")
		log.Info("LLM: OpenAI 直连", "base", base, "model", model)
	} else {
		client = llm.NoopClient{}
		log.Warn("LLM: noop（未配置）")
	}
	client = llm.NewRetryClient(client, 3, log)

	// PDF converter: prefer explicit PDF_SOFFICE_BIN, fall back to PATH
	// lookup. When neither is available we still start — the assemble
	// handler will return 503 on format=pdf, but word-only callers
	// are unaffected.
	pdfBin := os.Getenv("PDF_SOFFICE_BIN")
	conv := pdfexport.New(pdfBin)
	if conv.Available() {
		log.Info("PDF converter: enabled")
	} else {
		log.Warn("PDF converter: disabled (libreoffice not found); /assemble format=pdf will return 503")
	}

	return &Server{
		store:        st,
		llm:          client,
		tasks:        NewTaskManager(),
		log:          log,
		learner:      learner.New(st, log),
		pdfConverter: conv,
	}, nil
}

func (s *Server) buildPipeline() *core.Pipeline {
	st := s.store
	client := s.llm

	ing := ingest.New(st, client, s.log)
	ana := analyzer.New(client, s.log)
	pln := planner.New(client, s.log)
	gen := generator.New(client, st, s.log)

	mmdcPath := os.Getenv("MMDC_PATH")
	if mmdcPath == "" {
		mmdcPath = "mmdc"
	}
	pythonPath := os.Getenv("PYTHON_PATH")
	if pythonPath == "" {
		pythonPath = "python3"
	}
	ppConfig := os.Getenv("PUPPETEER_CONFIG")

	var renderers []illustrator.Renderer
	renderers = append(renderers, &illustrator.MermaidRenderer{MmdcPath: mmdcPath, PuppeteerConfig: ppConfig, LLM: client})
	renderers = append(renderers, &illustrator.DataChartRenderer{PythonPath: pythonPath, LLM: client})
	renderers = append(renderers, &illustrator.AIImageRenderer{LLM: client})
	renderers = append(renderers, &illustrator.TableRenderer{})

	il := illustrator.New(renderers, &illustrator.Beautifier{})
	aud := auditor.New(s.log)
	asm := assembler.New(s.log)
	lrn := learner.New(st, s.log)
	pln.Learner = lrn

	return &core.Pipeline{
		Ingestor:    ing,
		Analyzer:    ana,
		Planner:     pln,
		Generator:   gen,
		Illustrator: il,
		Auditor:     aud,
		Assembler:   asm,
		Learner:     lrn,
		Log:         s.log,
	}
}

// ---- HTTP 处理器 ----

func (s *Server) handleGenerate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MaterialDir string `json:"material_dir"`
		RFPPath     string `json:"rfp_path"`
		OutPath     string `json:"out_path"`
		NoIllustrate bool  `json:"no_illustrate"`
		NoAudit      bool  `json:"no_audit"`
		Concurrency  int   `json:"concurrency"`
		Budget       int   `json:"budget"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	if req.MaterialDir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "material_dir is required"})
		return
	}

	task := s.tasks.Create()
	go s.runTask(task, req)

	writeJSON(w, http.StatusAccepted, map[string]any{
		"task_id": task.ID,
		"status":  task.Status,
	})
}

func (s *Server) runTask(task *Task, req struct {
	MaterialDir  string `json:"material_dir"`
	RFPPath      string `json:"rfp_path"`
	OutPath      string `json:"out_path"`
	NoIllustrate bool   `json:"no_illustrate"`
	NoAudit      bool   `json:"no_audit"`
	Concurrency  int    `json:"concurrency"`
	Budget       int    `json:"budget"`
}) {
	task.Status = "running"
	s.tasks.Update(task)

	pipe := s.buildPipeline()
	opts := &core.GenerateOptions{
		MaterialDir:  req.MaterialDir,
		RFPPath:      req.RFPPath,
		OutPath:      req.OutPath,
		Theme:        core.DefaultTheme(),
		Concurrency:  req.Concurrency,
		TotalBudget:  req.Budget,
		NoIllustrate: req.NoIllustrate,
		NoAudit:      req.NoAudit,
	}
	if opts.Concurrency == 0 {
		opts.Concurrency = 10
	}
	if opts.TotalBudget == 0 {
		opts.TotalBudget = 60000
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	result, err := pipe.Run(ctx, opts)
	now := time.Now()
	task.FinishedAt = &now

	if err != nil {
		task.Status = "failed"
		task.Error = err.Error()
		s.log.Error("task failed", "task_id", task.ID, "err", err)
	} else {
		task.Status = "done"
		task.OutputPath = result.OutputPath
		task.Issues = len(result.Issues)
		task.Result = result
		s.log.Info("task done", "task_id", task.ID, "output", result.OutputPath)
	}
	s.tasks.Update(task)
}

func (s *Server) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/docgen/tasks/")
	if taskID == "" {
		// fallback for older Go
	}
	task, ok := s.tasks.Get(taskID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found"})
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// handleLearn accepts a completed bid's result (chapters + metadata)
// and feeds it to the learner, which extracts a reusable BidPattern
// and updates the Prompt Bandit posterior. This is the feedback loop
// that lets the system improve over time.
//
// POST /api/v1/docgen/learn
//   { "chapters": [{"title","content","word_count"}],
//     "industry": "IT", "rfp_type": "货物", "label": "won" }
func (s *Server) handleLearn(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Chapters []struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			WordCount int    `json:"word_count"`
		} `json:"chapters"`
		Industry string `json:"industry"`
		RFPType  string `json:"rfp_type"`
		Label    string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	if len(req.Chapters) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "chapters is required"})
		return
	}

	// Build a minimal BidPackage from the external data.
	pkg := &core.BidPackage{
		ID:        uuid.New(),
		RFPID:     uuid.New(),
		OutlineID: uuid.New(),
		Label:     req.Label,
		CreatedAt: time.Now(),
	}
	for _, ch := range req.Chapters {
		wc := ch.WordCount
		if wc == 0 {
			wc = len([]rune(ch.Content))
		}
		chapterID := uuid.New()
		pkg.Chapters = append(pkg.Chapters, core.Chapter{
			Spec: core.ChapterSpec{ID: chapterID, Title: ch.Title},
			Content: core.ChapterContent{
				ID:        uuid.New(),
				ChapterID: chapterID,
				Markdown:  ch.Content,
				WordCount: wc,
			},
		})
	}

	// Build a minimal RFPProfile.
	profile := &core.RFPProfile{
		ID:       uuid.New(),
		Industry: req.Industry,
		RFPType:  req.RFPType,
	}

	// Run the learner.
	if err := s.learner.Learn(r.Context(), pkg, profile); err != nil {
		s.log.Error("learn: failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "learn failed: " + err.Error()})
		return
	}

	s.log.Info("learn: pattern saved", "chapters", len(req.Chapters), "quality", pkg.QualityScore)
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "learned",
		"quality_score": pkg.QualityScore,
		"pattern_id":    pkg.PatternID,
	})
}

// handlePatterns retrieves historical bid patterns matching the given
// industry/RFP type. workflow-svc's planner can use these as reference
// when generating a new outline.
//
// GET /api/v1/docgen/patterns?industry=X&rfp_type=Y&top_k=5
func (s *Server) handlePatterns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	industry := q.Get("industry")
	rfpType := q.Get("rfp_type")
	topK := 5
	if v := q.Get("top_k"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			topK = n
		}
	}

	patterns, err := s.learner.RetrievePatterns(r.Context(), industry, rfpType, topK)
	if err != nil {
		s.log.Warn("patterns: retrieve failed", "err", err)
		patterns = nil // graceful: return empty list
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patterns": patterns,
		"count":    len(patterns),
	})
}

// handleRender accepts chapter data from an external source (e.g.
// workflow-svc), renders any mermaid code blocks into images, and
// assembles a richly-formatted .docx. This is the integration point
// between the workflow-svc pipeline and docgen-svc's assembler.
//
// POST /api/v1/docgen/render
//   { "title": "投标文件", "format": "word",
//     "chapters": [{"title","content","level","sort_order"}] }
func (s *Server) handleRender(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title    string `json:"title"`
		Format   string `json:"format"`
		Chapters []struct {
			Title     string `json:"title"`
			Content   string `json:"content"`
			Level     int    `json:"level"`
			SortOrder int    `json:"sort_order"`
		} `json:"chapters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}
	if len(req.Chapters) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "chapters is required"})
		return
	}
	format := strings.ToLower(req.Format)
	if format == "" {
		format = "word"
	}
	if format != "word" {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "format not supported: " + format})
		return
	}
	if req.Title == "" {
		req.Title = "投标文件"
	}

	// Build a BidPackage from the external chapter data.
	theme := core.DefaultTheme()
	pkg := &core.BidPackage{
		ID:        uuid.New(),
		RFPID:     uuid.New(),
		OutlineID: uuid.New(),
		CreatedAt: time.Now(),
	}

	// Set up renderers for code-block rendering.
	mmdcPath := os.Getenv("MMDC_PATH")
	if mmdcPath == "" {
		mmdcPath = "mmdc"
	}
	pythonPath := os.Getenv("PYTHON_PATH")
	if pythonPath == "" {
		pythonPath = "python3"
	}
	mermaidRenderer := &illustrator.MermaidRenderer{MmdcPath: mmdcPath, LLM: s.llm}
	chartRenderer := &illustrator.DataChartRenderer{PythonPath: pythonPath, LLM: s.llm}

	for idx, ch := range req.Chapters {
		level := ch.Level
		if level == 0 {
			level = 1
		}
		content := ch.Content

		// Extract and render mermaid code blocks.
		content, figs := s.renderFigureBlocks(r.Context(), content, idx, mermaidRenderer, chartRenderer, theme)
		pkg.Figures = append(pkg.Figures, figs...)

		chapterID := uuid.New()
		pkg.Chapters = append(pkg.Chapters, core.Chapter{
			Spec: core.ChapterSpec{
				ID:    chapterID,
				Title: ch.Title,
				Level: level,
				Order: ch.SortOrder,
			},
			Content: core.ChapterContent{
				ID:        uuid.New(),
				ChapterID: chapterID,
				Markdown:  content,
			},
		})
	}

	// Assemble the document.
	outPath := fmt.Sprintf("标书_render_%s.docx", time.Now().Format("20060102_150405"))
	pkg.OutputPath = outPath
	asm := assembler.New(s.log)
	path, err := asm.Assemble(r.Context(), pkg, theme)
	if err != nil {
		s.log.Error("render: assemble failed", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "assemble failed: " + err.Error()})
		return
	}

	// Track via task manager so the download endpoint can serve it.
	task := s.tasks.Create()
	task.Status = "done"
	task.OutputPath = path
	now := time.Now()
	task.FinishedAt = &now
	s.tasks.Update(task)

	s.log.Info("render: done", "task_id", task.ID, "chapters", len(req.Chapters), "figures", len(pkg.Figures))
	writeJSON(w, http.StatusOK, map[string]any{
		"download_url": "/api/v1/docgen/download/" + task.ID,
		"task_id":      task.ID,
		"chapters":     len(req.Chapters),
		"figures":      len(pkg.Figures),
	})
}

// renderFigureBlocks scans markdown for ```mermaid and ```chart fenced
// code blocks, renders each to PNG via the appropriate renderer, and
// replaces the block with a [!figure:ID caption=...] placeholder that
// the assembler recognises. Rendered illustrations are returned so the
// caller can attach them to the BidPackage.
func (s *Server) renderFigureBlocks(ctx context.Context, md string, chapterIdx int, mr *illustrator.MermaidRenderer, cr *illustrator.DataChartRenderer, theme *core.Theme) (string, []core.Illustration) {
	var figs []core.Illustration
	lines := strings.Split(md, "\n")
	var out strings.Builder
	i := 0
	figCount := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		isMermaid := trimmed == "```mermaid" || strings.HasPrefix(trimmed, "```mermaid")
		isChart := trimmed == "```chart" || strings.HasPrefix(trimmed, "```chart")
		if isMermaid || isChart {
			// Collect code block content.
			var codeLines []string
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != "```" {
				codeLines = append(codeLines, lines[i])
				i++
			}
			i++ // skip closing ```

			source := strings.Join(codeLines, "\n")
			figCount++
			caption := fmt.Sprintf("图%d-%d", chapterIdx+1, figCount)
			specID := uuid.New()

			var spec core.FigureSpec
			var ill *core.Illustration
			var err error
			if isMermaid {
				spec = core.FigureSpec{ID: specID, Type: core.FigureMermaid, Source: source, Caption: caption}
				ill, err = mr.Render(ctx, spec, theme)
			} else {
				spec = core.FigureSpec{ID: specID, Type: core.FigureDataChart, Source: source, Caption: caption}
				ill, err = cr.Render(ctx, spec, theme)
			}
			if err != nil {
				s.log.Warn("render: figure failed, using placeholder", "type", spec.Type, "err", err)
				ill = &core.Illustration{
					ID:            uuid.New(),
					SpecID:        specID,
					Status:        "placeholder",
					FallbackChain: "render_failed->placeholder",
				}
			} else {
				ill.SpecID = specID
			}
			figs = append(figs, *ill)
			// Replace code block with figure placeholder.
			out.WriteString(fmt.Sprintf("[!figure:%s caption=%s]\n", specID.String(), caption))
			continue
		}
		out.WriteString(line)
		if i < len(lines)-1 {
			out.WriteString("\n")
		}
		i++
	}
	return out.String(), figs
}

// handleDownload serves the output file of a completed task.
// The path parameter is a task ID (UUID), not a filesystem path, so
// there is no path-traversal risk - we look up the task and serve
// whatever OutputPath it recorded.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	taskID := strings.TrimPrefix(r.URL.Path, "/api/v1/docgen/download/")
	if taskID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task id required"})
		return
	}
	task, ok := s.tasks.Get(taskID)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found: " + taskID})
		return
	}
	if task.OutputPath == "" {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "task has no output file", "status": task.Status})
		return
	}
	s.log.Info("download", "task_id", taskID, "path", task.OutputPath)
	// Pick the right Content-Type so curl / browsers / the web client
	// open the file with the correct handler. http.ServeFile does its
	// own MIME detection but only from the file's first 512 bytes; for
	// .pdf / .docx that is usually enough, but explicitly setting it
	// here keeps the contract obvious for new clients.
	if strings.HasSuffix(strings.ToLower(task.OutputPath), ".pdf") {
		w.Header().Set("Content-Type", "application/pdf")
	} else if strings.HasSuffix(strings.ToLower(task.OutputPath), ".docx") {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	}
	http.ServeFile(w, r, task.OutputPath)
}

// handleAssemble 重新组装一个已完成任务的标书包为指定格式文档。
// 目前支持 word(.docx)；pdf 需 LibreOffice headless，暂未实现(501)。
func (s *Server) handleAssemble(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID       string `json:"task_id"`
		BidPackageID string `json:"bid_package_id"`
		Format       string `json:"format"`  // word (default) | pdf
		OutPath      string `json:"out_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "invalid JSON: " + err.Error()})
		return
	}

	// 解析任务：优先 task_id，回退 bid_package_id。
	id := req.TaskID
	if id == "" {
		id = req.BidPackageID
	}
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "task_id or bid_package_id is required"})
		return
	}
	task, ok := s.tasks.Get(id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]any{"error": "task not found: " + id})
		return
	}
	if task.Status != "done" || task.Result == nil || task.Result.Package == nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "task not ready for assemble", "status": task.Status})
		return
	}

	format := strings.ToLower(req.Format)
	if format == "" {
		format = "word"
	}
	if format != "word" && format != "pdf" {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "format not supported: " + format + " (only 'word' or 'pdf')"})
		return
	}

	// DOCX path: assemble directly into the target file. We always go
	// through Word first because the source BidPackage is rich with
	// ooxml-only structures (numbering, custom styles) that would
	// be lossy to skip.
	docxPath := req.OutPath
	if docxPath == "" {
		docxPath = fmt.Sprintf("标书_%s.docx", time.Now().Format("20060102_150405"))
	}
	if !strings.HasSuffix(strings.ToLower(docxPath), ".docx") {
		docxPath += ".docx"
	}
	pkg := task.Result.Package
	pkg.OutputPath = docxPath

	asm := assembler.New(s.log)
	if _, err := asm.Assemble(r.Context(), pkg, core.DefaultTheme()); err != nil {
		s.log.Error("assemble failed", "task_id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "assemble failed: " + err.Error()})
		return
	}
	s.log.Info("assemble done", "task_id", id, "output", docxPath)

	finalPath := docxPath
	if format == "pdf" {
		if s.pdfConverter == nil || !s.pdfConverter.Available() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{
				"error":     "pdf conversion not available on this host; install libreoffice (soffice) or set PDF_SOFFICE_BIN",
				"hint":      "apt-get install -y libreoffice",
				"word_path": docxPath,
				"task_id":   id,
			})
			return
		}
		pdfPath := strings.TrimSuffix(docxPath, filepathExt(docxPath)) + ".pdf"
		if err := s.pdfConverter.ConvertFile(r.Context(), docxPath, pdfPath); err != nil {
			s.log.Error("pdf convert failed", "task_id", id, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "pdf conversion failed: " + err.Error(), "word_path": docxPath, "task_id": id})
			return
		}
		finalPath = pdfPath
		s.log.Info("pdf converted", "task_id", id, "output", pdfPath)
	}

	// Store the assembled (or converted) path so the download
	// endpoint serves the right file regardless of format.
	task.OutputPath = finalPath
	s.tasks.Update(task)
	writeJSON(w, http.StatusOK, map[string]any{
		"download_url": "/api/v1/docgen/download/" + id,
		"format":       format,
		"task_id":      id,
		"output_path":  finalPath,
	})
}

// filepathExt returns the lowercase file extension (including the
// leading dot), or empty string when there is none. Tiny shim so we
// don't pull in path/filepath for one call.
func filepathExt(p string) string {
	i := strings.LastIndex(p, ".")
	if i < 0 || i == len(p)-1 {
		return ""
	}
	return strings.ToLower(p[i:])
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
}

// ---- main ----

func main() {
	srv, err := newServer()
	if err != nil {
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/docgen/generate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handleGenerate(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/tasks/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handleTaskStatus(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/learn", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handleLearn(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/patterns", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handlePatterns(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/render", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handleRender(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/download/", func(w http.ResponseWriter, r *http.Request) {
		srv.handleDownload(w, r)
	})
	mux.HandleFunc("/api/v1/docgen/assemble", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		srv.handleAssemble(w, r)
	})
	mux.HandleFunc("/healthz", srv.handleHealthz)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	httpSrv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	serverErr := make(chan error, 1)
	go func() {
		srv.log.Info("docgen-svc listening", "addr", addr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		srv.log.Error("server error", "err", err)
		os.Exit(1)
	case <-ctx.Done():
		srv.log.Info("shutdown signal received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	httpSrv.Shutdown(shutdownCtx)
	srv.store.Close()
	srv.log.Info("shutdown complete")
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
