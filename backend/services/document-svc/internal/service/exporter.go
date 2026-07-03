package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/bidwriter/services/document-svc/internal/storage"
	"github.com/bidwriter/services/document-svc/internal/store"
	"github.com/google/uuid"
)

// ExporterService handles document export to Word/PDF.
type ExporterService struct {
	store   *store.Store
	storage storage.Storage
	log     *slog.Logger
}

// NewExporterService creates an ExporterService.
func NewExporterService(s *store.Store, st storage.Storage, log *slog.Logger) *ExporterService {
	return &ExporterService{store: s, storage: st, log: log}
}

// ExportFormat represents the output format.
type ExportFormat string

const (
	FormatWord ExportFormat = "word"
	FormatPDF  ExportFormat = "pdf"
)

// ChapterData is a single chapter to include in the export.
type ChapterData struct {
	SpecID    uuid.UUID `json:"spec_id"`
	Title     string    `json:"title"`
	Level     int       `json:"level"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sort_order"`
}

// ExportRequest describes what to export.
type ExportRequest struct {
	BidJobID   uuid.UUID     `json:"bid_job_id"`
	Format     ExportFormat  `json:"format"` // "word" or "pdf"
	TemplateID *uuid.UUID    `json:"template_id,omitempty"`
	Chapters   []ChapterData `json:"chapters"`
	Title      string        `json:"title"` // document title
}

// ExportResult is the output of an export operation.
type ExportResult struct {
	DocumentID  uuid.UUID `json:"document_id"`
	StorageKey  string    `json:"storage_key"`
	DownloadURL string    `json:"download_url"`
	SizeBytes   int64     `json:"size_bytes"`
	Format      string    `json:"format"`
}

// Export assembles a Word document from chapter data and returns a download URL.
func (e *ExporterService) Export(ctx context.Context, req *ExportRequest) (*ExportResult, error) {
	e.log.Info("export starting",
		slog.String("bid_job_id", req.BidJobID.String()),
		slog.String("format", string(req.Format)),
		slog.Int("chapters", len(req.Chapters)))

	if len(req.Chapters) == 0 {
		return nil, fmt.Errorf("no chapters to export")
	}

	// Sort chapters by sort_order.
	sorted := make([]ChapterData, len(req.Chapters))
	copy(sorted, req.Chapters)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].SortOrder < sorted[i].SortOrder {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Assemble document XML.
	docXML := e.assembleDocumentXML(req.Title, sorted)

	// Create DOCX.
	ext := ".docx"
	storageKey, size, err := e.createDOCX(ctx, req.BidJobID, docXML, ext)
	if err != nil {
		return nil, fmt.Errorf("create docx: %w", err)
	}

	// PDF conversion if requested.
	if req.Format == FormatPDF {
		pdfKey, pdfSize, err := e.convertToPDF(ctx, storageKey)
		if err != nil {
			e.log.Warn("PDF conversion failed, returning Word", slog.Any("error", err))
		} else {
			storageKey = pdfKey
			size = pdfSize
			ext = ".pdf"
		}
	}

	docID := uuid.New()
	e.log.Info("export complete",
		slog.String("bid_job_id", req.BidJobID.String()),
		slog.String("storage_key", storageKey),
		slog.Int64("size", size))

	return &ExportResult{
		DocumentID:  docID,
		StorageKey:  storageKey,
		DownloadURL: "/api/v1/documents/content?key=" + storageKey,
		SizeBytes:   size,
		Format:      string(req.Format),
	}, nil
}

// assembleDocumentXML builds the word/document.xml content for the DOCX.
func (e *ExporterService) assembleDocumentXML(title string, chapters []ChapterData) string {
	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>`)
	buf.WriteString(html.EscapeString(title))
	buf.WriteString(`</w:t></w:r></w:p>`)

	for _, ch := range chapters {
		// Heading.
		hLevel := ch.Level
		if hLevel <= 0 {
			hLevel = 1
		}
		if hLevel > 9 {
			hLevel = 9
		}
		buf.WriteString(`<w:p><w:pPr><w:pStyle w:val="Heading`)
		buf.WriteString(fmt.Sprintf("%d", hLevel))
		buf.WriteString(`"/></w:pPr><w:r><w:t>`)
		buf.WriteString(html.EscapeString(ch.Title))
		buf.WriteString(`</w:t></w:r></w:p>`)

		// Body content — convert Markdown to OOXML (headings, lists, tables,
		// bold/italic, figure placeholders).
		content := strings.TrimSpace(ch.Content)
		if content == "" {
			content = "(内容待生成)"
		}
		buf.WriteString(markdownToOOXML(content))
	}

	buf.WriteString("</w:body></w:document>")
	return buf.String()
}

// createDOCX builds a minimal .docx ZIP file and stores it.
func (e *ExporterService) createDOCX(ctx context.Context, bidJobID uuid.UUID, docXML string, ext string) (string, int64, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`,
		"word/document.xml": docXML,
		"word/styles.xml":   defaultStylesXML(),
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:defaultTabStop w:val="720"/></w:settings>`,
	}

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			return "", 0, err
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return "", 0, err
		}
	}
	if err := zw.Close(); err != nil {
		return "", 0, err
	}

	key := fmt.Sprintf("exports/%s/%d%s", bidJobID.String(), time.Now().Unix(), ext)
	_, _, size, err := e.storage.Put(ctx, key, &buf)
	if err != nil {
		return "", 0, fmt.Errorf("storage put: %w", err)
	}
	return key, size, nil
}

// defaultStylesXML returns a minimal OOXML styles definition.
func defaultStylesXML() string {
	return `<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:style w:type="paragraph" w:default="1" w:styleId="Normal">
<w:name w:val="Normal"/>
<w:rPr><w:sz w:val="24"/><w:szCs w:val="24"/></w:rPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Title">
<w:name w:val="Title"/>
<w:basedOn w:val="Normal"/>
<w:pPr><w:jc w:val="center"/></w:pPr>
<w:rPr><w:b/><w:sz w:val="56"/><w:szCs w:val="56"/></w:rPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Heading1">
<w:name w:val="heading 1"/>
<w:basedOn w:val="Normal"/>
<w:rPr><w:b/><w:sz w:val="36"/><w:szCs w:val="36"/></w:rPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Heading2">
<w:name w:val="heading 2"/>
<w:basedOn w:val="Normal"/>
<w:rPr><w:b/><w:sz w:val="32"/><w:szCs w:val="32"/></w:rPr>
</w:style>
<w:style w:type="paragraph" w:styleId="Heading3">
<w:name w:val="heading 3"/>
<w:basedOn w:val="Normal"/>
<w:rPr><w:b/><w:sz w:val="28"/><w:szCs w:val="28"/></w:rPr>
</w:style>
</w:styles>`
}

// convertToPDF converts a stored DOCX to PDF via LibreOffice headless.
func (e *ExporterService) convertToPDF(ctx context.Context, docxKey string) (string, int64, error) {
	tmpDir := os.TempDir()
	docxPath := filepath.Join(tmpDir, uuid.New().String()+".docx")
	defer os.Remove(docxPath)

	rc, err := e.storage.Get(ctx, docxKey)
	if err != nil {
		return "", 0, err
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return "", 0, err
	}
	if err := os.WriteFile(docxPath, data, 0644); err != nil {
		return "", 0, err
	}

	pdfPath := strings.TrimSuffix(docxPath, ".docx") + ".pdf"
	// Use context with timeout instead of cmd.Timeout.
	execCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(execCtx, "libreoffice", "--headless", "--convert-to", "pdf", "--outdir", tmpDir, docxPath)
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("libreoffice: %w", err)
	}

	pdfData, err := os.ReadFile(pdfPath)
	if err != nil {
		return "", 0, err
	}
	defer os.Remove(pdfPath)

	pdfKey := strings.TrimSuffix(docxKey, ".docx") + ".pdf"
	_, _, size, err := e.storage.Put(ctx, pdfKey, bytes.NewReader(pdfData))
	if err != nil {
		return "", 0, fmt.Errorf("storage put pdf: %w", err)
	}
	return pdfKey, size, nil
}
