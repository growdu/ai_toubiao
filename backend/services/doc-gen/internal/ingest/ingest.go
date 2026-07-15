// Package ingest 实现材料目录摄取：递归遍历 -> 归一化(zip 解包/去重)
// -> 分类 -> python 子进程提取 -> 清洗分块 -> 向量化 -> 入库。
// 详见 docs/architecture/doc-gen-ingest.md 与 ADR-0012。
package ingest

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
)

//go:embed extract.py
var extractPyScript string

// Ingester 实现 core.Ingestor 接口。
type Ingester struct {
	Store          store.Store
	LLM            llm.Client
	Log            *slog.Logger
	EmbedBatchSize int
	PythonPath     string
	ExtractTimeout time.Duration
	extractScript  string
}

// New 创建 Ingester。
func New(s store.Store, client llm.Client, log *slog.Logger) *Ingester {
	return &Ingester{
		Store:          s,
		LLM:            client,
		Log:            log,
		EmbedBatchSize: 32,
		PythonPath:     "python3",
		ExtractTimeout: 120 * time.Second,
	}
}

// extractResult 是 extract.py 的 JSON 输出契约。
type extractResult struct {
	Text     string   `json:"text"`
	Tables   []string `json:"tables"`
	Warnings []string `json:"warnings"`
	NeedsOCR bool     `json:"needs_ocr"`
}

// Ingest 遍历材料目录，归一化、提取、分块、向量化，存入索引。
func (ing *Ingester) Ingest(ctx context.Context, dir string, rfpPath string) ([]core.Chunk, error) {
	log := ing.Log
	if log == nil {
		log = slog.Default()
	}
	if _, err := ing.ensureExtractScript(); err != nil {
		return nil, fmt.Errorf("prepare extract script: %w", err)
	}

	rfpAbs := ""
	if rfpPath != "" {
		rfpAbs, _ = filepath.Abs(rfpPath)
	}

	var files []fileEntry
	seenHash := make(map[string]bool)
	skipped := 0
	if err := ing.collectFiles(ctx, dir, rfpAbs, &files, seenHash, &skipped, 0); err != nil {
		return nil, fmt.Errorf("walk dir %s: %w", dir, err)
	}
	if rfpAbs != "" {
		found := false
		for _, f := range files {
			if f.Path == rfpAbs {
				found = true
				break
			}
		}
		if !found {
			if h, err := fileSha256(rfpAbs); err == nil {
				seenHash[h] = true
			}
			files = append(files, fileEntry{
				Path: rfpAbs, Name: filepath.Base(rfpPath),
				Category: "rfp", Format: normalizeExt(filepath.Base(rfpPath)),
			})
		}
	}

	log.Info("ingest: 发现文件", "count", len(files), "dedup_skipped", skipped)

	var allChunks []core.Chunk
	for _, f := range files {
		chunks, err := ing.ingestFile(ctx, f)
		if err != nil {
			log.Warn("ingest: 解析文件失败，跳过", "file", f.Name, "err", err)
			continue
		}
		if len(chunks) > 0 {
			if err := ing.Store.SaveChunks(ctx, chunks); err != nil {
				log.Warn("ingest: 保存分块失败", "file", f.Name, "err", err)
			}
			allChunks = append(allChunks, chunks...)
			log.Info("ingest: 文件完成", "file", f.Name, "category", f.Category, "chunks", len(chunks))
		}
	}

	log.Info("ingest: 全部完成", "total_chunks", len(allChunks))
	return allChunks, nil
}

type fileEntry struct {
	Path        string
	Name        string
	Category    string
	Format      string
	ContentHash string
}

// collectFiles 递归遍历目录，遇 zip 解包到临时目录再递归；按 sha256 去重。
func (ing *Ingester) collectFiles(ctx context.Context, dir string, rfpAbs string, files *[]fileEntry, seen map[string]bool, skipped *int, depth int) error {
	log := ing.Log
	if log == nil {
		log = slog.Default()
	}
	if depth > 3 {
		log.Warn("ingest: zip 嵌套超过 3 层，跳过", "dir", dir)
		return nil
	}
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := d.Name()
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".db") {
			return nil
		}
		abs, _ := filepath.Abs(path)
		ext := normalizeExt(name)

		if ext == "zip" {
			tmp, err := unzipToTemp(abs)
			if err != nil {
				log.Warn("ingest: zip 解包失败，跳过", "file", name, "err", err)
				return nil
			}
			defer os.RemoveAll(tmp)
			return ing.collectFiles(ctx, tmp, rfpAbs, files, seen, skipped, depth+1)
		}

		hash, err := fileSha256(abs)
		if err != nil {
			log.Warn("ingest: hash 失败，跳过去重", "file", name, "err", err)
		} else if seen[hash] {
			*skipped++
			log.Info("ingest: 重复文件跳过", "file", name)
			return nil
		} else {
			seen[hash] = true
		}

		*files = append(*files, fileEntry{
			Path: abs, Name: name, Category: detectCategory(abs, name, rfpAbs),
			Format: ext, ContentHash: hash,
		})
		return nil
	})
}

// ingestFile 提取单个文件，清洗分块，向量化。
func (ing *Ingester) ingestFile(ctx context.Context, f fileEntry) ([]core.Chunk, error) {
	log := ing.Log
	if log == nil {
		log = slog.Default()
	}

	text := ""
	needsOCR := false

	if isTextFormat(f.Format) {
		data, err := os.ReadFile(f.Path)
		if err != nil {
			return nil, err
		}
		text = string(data)
	} else {
		res, err := ing.runExtract(ctx, f.Path, f.Format)
		if err != nil {
			return nil, fmt.Errorf("extract %s: %w", f.Name, err)
		}
		text = res.Text
		needsOCR = res.NeedsOCR
		if len(res.Warnings) > 0 {
			log.Warn("ingest: 提取告警", "file", f.Name, "warnings", res.Warnings)
		}
	}

	text = cleanNoise(text)
	if strings.TrimSpace(text) == "" {
		if needsOCR {
			log.Warn("ingest: 无文本层，需 OCR", "file", f.Name)
		}
		return nil, nil
	}

	rawChunks := semanticChunk(text, 512, 64)
	if len(rawChunks) == 0 {
		return nil, nil
	}

	chunks := make([]core.Chunk, len(rawChunks))
	for i, rc := range rawChunks {
		chunks[i] = core.Chunk{
			ID:          uuid.New(),
			FilePath:    f.Path,
			Category:    f.Category,
			Offset:      i,
			Text:        rc,
			ContentHash: f.ContentHash,
			SourceName:  f.Name,
			NeedsOCR:    needsOCR,
		}
	}

	if ing.LLM != nil {
		batchSize := ing.EmbedBatchSize
		if batchSize <= 0 {
			batchSize = 32
		}
		for i := 0; i < len(chunks); i += batchSize {
			end := i + batchSize
			if end > len(chunks) {
				end = len(chunks)
			}
			batch := make([]string, end-i)
			for j := i; j < end; j++ {
				batch[j-i] = chunks[j].Text
			}
			vecs, err := ing.LLM.EmbedBatch(ctx, batch)
			if err != nil {
				log.Warn("ingest: 向量化失败，分块无向量", "file", f.Name, "err", err)
				break
			}
			for j := 0; j < len(vecs) && i+j < len(chunks); j++ {
				chunks[i+j].Embedding = vecs[j]
			}
		}
	}

	return chunks, nil
}

// runExtract 调用 extract.py 子进程提取文本。
func (ing *Ingester) runExtract(ctx context.Context, path, format string) (*extractResult, error) {
	timeout := ing.ExtractTimeout
	if timeout <= 0 {
		timeout = 120 * time.Second
	}
	tCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	py := ing.PythonPath
	if py == "" {
		py = "python3"
	}
	cmd := exec.CommandContext(tCtx, py, ing.extractScript, path, format)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("python extract: %w stderr=%s", err, stderr.String())
	}
	var res extractResult
	if err := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &res); err != nil {
		return nil, fmt.Errorf("parse extract json: %w out=%s", err, stdout.String())
	}
	return &res, nil
}

// ensureExtractScript 把嵌入的 extract.py 写到临时文件，复用。
func (ing *Ingester) ensureExtractScript() (string, error) {
	if ing.extractScript != "" {
		if _, err := os.Stat(ing.extractScript); err == nil {
			return ing.extractScript, nil
		}
	}
	f, err := os.CreateTemp("", "bidgen-extract-*.py")
	if err != nil {
		return "", err
	}
	if _, err := f.WriteString(extractPyScript); err != nil {
		f.Close()
		return "", err
	}
	f.Close()
	ing.extractScript = f.Name()
	return ing.extractScript, nil
}

// detectCategory 根据文件名推断材料分类。
// 仅显式 --rfp 或"招标文件"为 RFP；附件按类型词分类，避免附件路径
// 含"招标"被误判（详见 doc-gen-ingest.md 3.4）。
func detectCategory(absPath, name, rfpAbs string) string {
	if rfpAbs != "" && absPath == rfpAbs {
		return "rfp"
	}
	low := strings.ToLower(name)
	switch {
	case strings.Contains(low, "招标文件") || strings.Contains(low, "rfp") || strings.Contains(low, "tender"):
		return "rfp"
	case strings.Contains(low, "投标文件") || strings.Contains(low, "商务标"):
		return "reference"
	case strings.Contains(low, "可研") || strings.Contains(low, "可行性研究"):
		return "technical"
	case strings.Contains(low, "技术规范") || strings.Contains(low, "规范书") || strings.Contains(low, "technical") || strings.Contains(low, "tech") || strings.Contains(low, "方案"):
		return "technical"
	case strings.Contains(low, "合同"):
		return "commercial"
	case strings.Contains(low, "清单") || strings.Contains(low, "报价"):
		return "commercial"
	case strings.Contains(low, "图纸") || strings.Contains(low, "总平图"):
		return "drawing"
	case strings.Contains(low, "资质") || strings.Contains(low, "qualification") || strings.Contains(low, "cert"):
		return "qualification"
	case strings.Contains(low, "业绩") || strings.Contains(low, "performance"):
		return "performance"
	case strings.Contains(low, "历史") || strings.Contains(low, "reference") || strings.Contains(low, "参考"):
		return "reference"
	default:
		return "other"
	}
}

// normalizeExt 返回规范化的扩展名（无点，小写），处理 .pdf.pdf 双扩展名。
func normalizeExt(name string) string {
	ext := strings.ToLower(filepath.Ext(name))
	ext = strings.TrimPrefix(ext, ".")
	dotExt := "." + ext
	if ext != "" && strings.HasSuffix(strings.ToLower(name), dotExt+dotExt) {
		return ext
	}
	return ext
}

// isTextFormat 判断是否为可直接读取的纯文本格式。
func isTextFormat(f string) bool {
	switch f {
	case "txt", "md", "csv", "json", "yaml", "yml", "log", "xml", "html", "htm":
		return true
	}
	return false
}

// fileSha256 计算文件内容的 sha256。
func fileSha256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// unzipToTemp 把 zip 解包到临时目录，返回目录路径（防 zip slip）。
func unzipToTemp(zipPath string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer r.Close()
	tmp, err := os.MkdirTemp("", "bidgen-zip-*")
	if err != nil {
		return "", err
	}
	cleanTmp := filepath.Clean(tmp)
	for _, f := range r.File {
		target := filepath.Join(tmp, f.Name)
		cleanTarget := filepath.Clean(target)
		if cleanTarget != cleanTmp && !strings.HasPrefix(cleanTarget, cleanTmp+string(os.PathSeparator)) {
			continue
		}
		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(target, 0755)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return tmp, err
		}
		out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return tmp, err
		}
		rc, err := f.Open()
		if err != nil {
			out.Close()
			return tmp, err
		}
		_, copyErr := io.Copy(out, rc)
		rc.Close()
		out.Close()
		if copyErr != nil {
			return tmp, copyErr
		}
	}
	return tmp, nil
}
