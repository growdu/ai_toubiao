// docgen-svc 是 doc-gen 模块的服务化入口（Phase2）。
// 内核与 bidgen CLI 共享同一 Pipeline，仅入口壳不同。
// 提供 HTTP API 供 workflow-svc 和 api-gateway 调用。
//
// 端点：
//   POST /api/v1/docgen/generate   { material_dir, rfp_path, options } → { task_id }
//   GET  /api/v1/docgen/tasks/:id  → { status, progress, output_path, issues }
//   POST /api/v1/docgen/assemble   { bid_package_id, format } → { download_url }
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
	store  *store.SQLiteStore
	llm    llm.Client
	tasks  *TaskManager
	log    *slog.Logger
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

	return &Server{
		store: st,
		llm:   client,
		tasks: NewTaskManager(),
		log:   log,
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
	if format != "word" {
		writeJSON(w, http.StatusNotImplemented, map[string]any{"error": "format not supported yet: " + format + " (only 'word')"})
		return
	}

	outPath := req.OutPath
	if outPath == "" {
		outPath = fmt.Sprintf("标书_%s.docx", time.Now().Format("20060102_150405"))
	}
	pkg := task.Result.Package
	pkg.OutputPath = outPath

	asm := assembler.New(s.log)
	path, err := asm.Assemble(r.Context(), pkg, core.DefaultTheme())
	if err != nil {
		s.log.Error("assemble failed", "task_id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "assemble failed: " + err.Error()})
		return
	}
	s.log.Info("assemble done", "task_id", id, "output", path)
	writeJSON(w, http.StatusOK, map[string]any{
		"download_url": path,
		"format":       format,
		"task_id":      id,
	})
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
