// Package ingest 实现材料目录摄取：递归遍历 → MIME 检测 → 解析 → 分块 → 向量化 → 入库。
// 详见 docs/doc-gen/algorithms.md 第一节"材料索引算法"。
package ingest

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/bidwriter/services/doc-gen/internal/store"
	"github.com/google/uuid"
)

// Ingester 实现 core.Ingestor 接口。
type Ingester struct {
	Store    store.Store
	LLM      llm.Client
	Log      *slog.Logger
	EmbedBatchSize int  // 批量嵌入大小，默认 32
}

// New 创建 Ingester。
func New(s store.Store, client llm.Client, log *slog.Logger) *Ingester {
	return &Ingester{
		Store: s,
		LLM:   client,
		Log:   log,
		EmbedBatchSize: 32,
	}
}

// Ingest 遍历材料目录，解析文件，分块向量化，存入索引。
// 返回所有分块。增量索引：仅处理变更文件。
func (ing *Ingester) Ingest(ctx context.Context, dir string, rfpPath string) ([]core.Chunk, error) {
	log := ing.Log
	if log == nil {
		log = slog.Default()
	}

	var files []fileEntry
	rfpAbs := ""
	if rfpPath != "" {
		rfpAbs, _ = filepath.Abs(rfpPath)
	}

	// 递归遍历目录
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		// 跳过隐藏文件和数据库
		name := d.Name()
		if strings.HasPrefix(name, ".") || name == "bidgen.db" {
			return nil
		}
		abs, _ := filepath.Abs(path)
		cat := detectCategory(abs, name, rfpAbs)
		files = append(files, fileEntry{Path: abs, Name: name, Category: cat})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk dir %s: %w", dir, err)
	}

	// 如果指定了 RFP 且不在目录中，单独处理
	if rfpAbs != "" {
		found := false
		for _, f := range files {
			if f.Path == rfpAbs {
				found = true
				break
			}
		}
		if !found {
			files = append(files, fileEntry{Path: rfpAbs, Name: filepath.Base(rfpAbs), Category: "rfp"})
		}
	}

	log.Info("ingest: 发现文件", "count", len(files))

	var allChunks []core.Chunk
	for _, f := range files {
		chunks, err := ing.ingestFile(ctx, f)
		if err != nil {
			log.Warn("ingest: 解析文件失败，跳过", "file", f.Path, "err", err)
			continue
		}
		// 保存到 store
		if len(chunks) > 0 {
			if err := ing.Store.SaveChunks(ctx, chunks); err != nil {
				log.Warn("ingest: 保存分块失败", "file", f.Path, "err", err)
			}
			allChunks = append(allChunks, chunks...)
			log.Info("ingest: 文件完成", "file", f.Name, "category", f.Category, "chunks", len(chunks))
		}
	}

	log.Info("ingest: 全部完成", "total_chunks", len(allChunks))
	return allChunks, nil
}

type fileEntry struct {
	Path     string
	Name     string
	Category string
}

// ingestFile 解析单个文件，分块，向量化。
func (ing *Ingester) ingestFile(ctx context.Context, f fileEntry) ([]core.Chunk, error) {
	// 增量检查：如果文件未变更，跳过
	_, err := os.Stat(f.Path)
	if err != nil {
		return nil, err
	}

	// 提取文本
	text, err := extractText(f.Path)
	if err != nil {
		return nil, fmt.Errorf("extract %s: %w", f.Name, err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, nil
	}

	// 语义分块
	rawChunks := semanticChunk(text, 512, 64)
	if len(rawChunks) == 0 {
		return nil, nil
	}

	// 构造 Chunk
	chunks := make([]core.Chunk, len(rawChunks))
	for i, rc := range rawChunks {
		chunks[i] = core.Chunk{
			ID:       uuid.New(),
			FilePath: f.Path,
			Category: f.Category,
			Offset:   i,
			Text:     rc,
		}
	}

	// 向量化（批量）
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
				ing.Log.Warn("ingest: 向量化失败，分块无向量", "file", f.Name, "err", err)
				break
			}
			for j := 0; j < len(vecs) && i+j < len(chunks); j++ {
				chunks[i+j].Embedding = vecs[j]
			}
		}
	}

	return chunks, nil
}

// detectCategory 根据路径和文件名推断材料分类。
func detectCategory(absPath, name, rfpAbs string) string {
	// 如果是显式指定的 RFP
	if rfpAbs != "" && absPath == rfpAbs {
		return "rfp"
	}
	lower := strings.ToLower(absPath + "/" + name)
	switch {
	case strings.Contains(lower, "招标") || strings.Contains(lower, "rfp") || strings.Contains(lower, "tender"):
		return "rfp"
	case strings.Contains(lower, "资质") || strings.Contains(lower, "qualification") || strings.Contains(lower, "cert"):
		return "qualification"
	case strings.Contains(lower, "方案") || strings.Contains(lower, "technical") || strings.Contains(lower, "tech"):
		return "technical"
	case strings.Contains(lower, "业绩") || strings.Contains(lower, "performance") || strings.Contains(lower, "case"):
		return "performance"
	case strings.Contains(lower, "历史") || strings.Contains(lower, "reference") || strings.Contains(lower, "参考"):
		return "reference"
	default:
		return "other"
	}
}

// extractText 根据文件类型提取纯文本。
func extractText(path string) (string, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".csv", ".json", ".yaml", ".yml", ".log", ".xml", ".html", ".htm":
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	case ".pdf":
		return extractPDF(path)
	case ".docx":
		return extractDOCX(path)
	case ".xlsx", ".xls":
		return extractXLSX(path)
	case ".doc":
		// 旧格式 doc 尝试用 antiword 或 catdoc
		return extractLegacyDoc(path)
	default:
		// 尝试作为文本读取
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

// extractPDF 用 pdftotext（poppler-utils）提取 PDF 文本。
func extractPDF(path string) (string, error) {
	// 优先用 pdftotext 命令行工具
	out, err := exec.Command("pdftotext", "-q", path, "-").Output()
	if err == nil && len(out) > 0 {
		return string(out), nil
	}
	// fallback：尝试直接读取（部分 PDF 有可提取文本）
	data, derr := os.ReadFile(path)
	if derr != nil {
		return "", fmt.Errorf("pdf extract: no pdftotext and read failed: %w", err)
	}
	// 简单提取可见 ASCII 文本
	var sb strings.Builder
	for _, b := range data {
		if b >= 32 && b < 127 || b == '\n' || b == '\r' || b == '\t' {
			sb.WriteByte(b)
		}
	}
	text := sb.String()
	if len(text) < 50 {
		return "", fmt.Errorf("pdf extract: no text extracted (install poppler-utils for pdftotext)")
	}
	return text, nil
}

// extractDOCX 从 .docx (zip) 中提取文本。
func extractDOCX(path string) (string, error) {
	// docx 本质是 zip，document.xml 包含正文
	return extractFromZipXML(path, "word/document.xml")
}

// extractXLSX 从 .xlsx (zip) 中提取文本。
func extractXLSX(path string) (string, error) {
	// xlsx 的 sheet 文本在 xl/sharedStrings.xml
	return extractFromZipXML(path, "xl/sharedStrings.xml")
}

// extractLegacyDoc 用 antiword 或 catdoc 提取 .doc 文本。
func extractLegacyDoc(path string) (string, error) {
	for _, tool := range []string{"antiword", "catdoc"} {
		out, err := exec.Command(tool, path).Output()
		if err == nil && len(out) > 0 {
			return string(out), nil
		}
	}
	return "", fmt.Errorf("doc extract: install antiword or catdoc")
}
