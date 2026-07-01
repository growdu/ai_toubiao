// Package api implements the HTTP layer for workflow-svc.
package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"os/exec"
	"os"
	"strings"

	"github.com/bidwriter/shared/pkg/httperr"
	"github.com/bidwriter/shared/pkg/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ChapterData represents a chapter for export.
type ChapterData struct {
	SpecID    uuid.UUID `json:"spec_id"`
	Title     string    `json:"title"`
	Level     int       `json:"level"`
	Content   string    `json:"content"`
	SortOrder int       `json:"sort_order"`
}

// ExportRequest is the request body for document export.
type ExportRequest struct {
	Format      string        `json:"format"` // "word" or "pdf"
	Chapters    []ChapterData `json:"chapters"`
	Title       string        `json:"title"`
	ProjectName string        `json:"project_name"`
}

// DocBuilder produces a DOCX file from the given chapters. Defined as an
// interface so the handler can be unit-tested with an in-memory builder and
// the default OOXML implementation can be swapped for unioffice / gooxml
// without changing HTTP code.
type DocBuilder interface {
	Build(w io.Writer, title string, chapters []ChapterData) error
}

// PDFConverter turns a DOCX stream into a PDF stream. Implementations typically
// shell out to LibreOffice (`soffice --headless --convert-to pdf`). When the
// converter is not configured (PDFConverter == nil) or unavailable, the
// handler falls back to returning the DOCX with a 200 + warning header.
type PDFConverter interface {
	// Available reports whether the converter runtime is usable right now
	// (e.g. the LibreOffice binary is on PATH). Handlers can use this to
	// surface a 503 instead of failing on every request.
	Available() bool
	// ConvertToPDF streams the DOCX from `in` to a PDF written to `out`.
	// The implementation is responsible for closing/cleaning up the
	// subprocess and any temp files.
	ConvertToPDF(ctx context.Context, in io.Reader, out io.Writer) error
}

// ErrPDFConverterUnavailable is returned by exportPDFHandler when no
// PDFConverter is wired (e.g. soffice not installed in the container).
var ErrPDFConverterUnavailable = errors.New("pdf converter not available")

// ooxmlBuilder is the default DocBuilder: it writes a minimal but valid
// Office Open XML (.docx) zip directly via encoding/xml strings. This is
// intentionally simple — full styles and rich content can be added later
// without touching the handler layer.
type ooxmlBuilder struct{}

func (ooxmlBuilder) Build(w io.Writer, title string, chapters []ChapterData) error {
	zw := zip.NewWriter(w)
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
		"word/document.xml": buildDocumentXML(title, chapters),
		"word/styles.xml":   defaultStylesXML(),
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:defaultTabStop w:val="720"/></w:settings>`,
	}

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			return fmt.Errorf("create zip entry %s: %w", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			return fmt.Errorf("write zip entry %s: %w", name, err)
		}
	}
	return zw.Close()
}

// buildDocumentXML creates the word/document.xml content.
func buildDocumentXML(title string, chapters []ChapterData) string {
	var buf strings.Builder
	buf.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:pPr><w:pStyle w:val="Title"/></w:pPr><w:r><w:t>`)
	buf.WriteString(html.EscapeString(title))
	buf.WriteString(`</w:t></w:r></w:p>`)

	for _, ch := range chapters {
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

		content := strings.TrimSpace(ch.Content)
		if content == "" {
			content = "(内容待生成)"
		}
		paragraphs := strings.Split(content, "\n")
		for _, para := range paragraphs {
			para = strings.TrimSpace(para)
			if para == "" {
				continue
			}
			buf.WriteString(`<w:p><w:r><w:t>`)
			buf.WriteString(html.EscapeString(para))
			buf.WriteString(`</w:t></w:r></w:p>`)
		}
	}
	buf.WriteString("</w:body></w:document>")
	return buf.String()
}

// defaultStylesXML returns minimal OOXML styles.
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

// defaultChapters returns the canonical 6-chapter bid outline used when the
// caller doesn't supply chapter data. Exported so tests and other handlers
// can share the same fallback.
func defaultChapters(projectID string) []ChapterData {
	return []ChapterData{
		{Title: "第一章 投标函", Level: 1, SortOrder: 1, Content: "(内容待生成)"},
		{Title: "第二章 项目理解与总体思路", Level: 1, SortOrder: 2, Content: "(内容待生成)"},
		{Title: "第三章 技术方案", Level: 1, SortOrder: 3, Content: "(内容待生成)"},
		{Title: "第四章 项目实施计划", Level: 1, SortOrder: 4, Content: "(内容待生成)"},
		{Title: "第五章 质量保证措施", Level: 1, SortOrder: 5, Content: "(内容待生成)"},
		{Title: "第六章 售后服务", Level: 1, SortOrder: 6, Content: "(内容待生成)"},
	}
}

// writeDocx writes a DOCX to w using the configured builder. Defaults to
// the OOXML builder if h.DocBuilder is nil.
func (h *Handlers) writeDocx(w io.Writer, title string, chapters []ChapterData) error {
	b := h.DocBuilder
	if b == nil {
		b = ooxmlBuilder{}
	}
	return b.Build(w, title, chapters)
}

// writeDocxBuffer returns a fully buffered DOCX. Used by the PDF path which
// needs to feed bytes into LibreOffice.
func (h *Handlers) writeDocxBuffer(title string, chapters []ChapterData) (*bytes.Buffer, error) {
	var buf bytes.Buffer
	if err := h.writeDocx(&buf, title, chapters); err != nil {
		return nil, err
	}
	return &buf, nil
}

// libreOfficeConverter implements PDFConverter by shelling out to
// `soffice --headless --convert-to pdf --outdir <tmp> <docx>` and copying
// the produced PDF to the caller's writer. The binary path can be overridden
// via NewLibreOfficeConverter.
type libreOfficeConverter struct {
	binary string
}

// NewLibreOfficeConverter returns a PDFConverter that uses the given
// `soffice` (or `libreoffice`) binary. Pass an empty string to auto-detect.
func NewLibreOfficeConverter(binary string) PDFConverter {
	return &libreOfficeConverter{binary: binary}
}

func (l *libreOfficeConverter) resolve() string {
	if l.binary != "" {
		return l.binary
	}
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

func (l *libreOfficeConverter) Available() bool {
	// resolve() may return a literal configured path that doesn't actually
	// exist (e.g. a misconfigured /opt/libreoffice/program/soffice).
	// exec.LookPath handles both PATH lookup and absolute-path liveness
	// in one call.
	_, err := exec.LookPath(l.resolve())
	return err == nil
}

func (l *libreOfficeConverter) ConvertToPDF(ctx context.Context, in io.Reader, out io.Writer) error {
	if !l.Available() {
		return ErrPDFConverterUnavailable
	}
	binary := l.resolve()
	// Stream the DOCX into a temp file (LibreOffice wants a file path, not
	// stdin), then run the conversion, then copy the resulting PDF out.
	docx, err := os.CreateTemp("", "bid-*.docx")
	if err != nil {
		return fmt.Errorf("create temp docx: %w", err)
	}
	defer os.Remove(docx.Name())
	if _, err := io.Copy(docx, in); err != nil {
		docx.Close()
		return fmt.Errorf("write temp docx: %w", err)
	}
	if err := docx.Close(); err != nil {
		return fmt.Errorf("close temp docx: %w", err)
	}

	cmd := exec.CommandContext(ctx, binary,
		"--headless", "--norestore", "--nologo", "--nofirststartwizard",
		"--convert-to", "pdf", "--outdir", filepathDir(docx.Name()), docx.Name())
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("libreoffice convert: %w (%s)", err, string(outBytes))
	}
	pdfPath := strings.TrimSuffix(docx.Name(), ".docx") + ".pdf"
	defer os.Remove(pdfPath)
	return copyFile(pdfPath, out)
}

// filepathDir is a tiny shim so we don't pull in path/filepath just for one
// call. It returns the directory portion of p.
func filepathDir(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[:i]
	}
	return "."
}

// copyFile streams a file at src to dst, then closes the source.
func copyFile(src string, dst io.Writer) error {
	f, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open converted pdf: %w", err)
	}
	defer f.Close()
	_, err = io.Copy(dst, f)
	return err
}

// exportWord generates a minimal DOCX and writes it to w via the configured
// DocBuilder. Exposed for the unit tests.
func (h *Handlers) exportWord(w http.ResponseWriter, r *http.Request, title string, chapters []ChapterData) {
	rid := logger.RequestIDFrom(r.Context())

	var buf bytes.Buffer
	if err := h.writeDocx(&buf, title, chapters); err != nil {
		h.Log.Error("build docx", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}

	filename := fmt.Sprintf("%s.docx", strings.ReplaceAll(title, " ", "_"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	io.Copy(w, &buf)
}

// GET /api/v1/bids/{id}/export/word
func (h *Handlers) exportWordHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	if _, err := h.Store.Get(r.Context(), id); err != nil {
		httperr.NotFound(w, rid, "workflow")
		return
	}

	var req ExportRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httperr.InvalidInput(w, rid, "invalid JSON: "+err.Error(), nil)
			return
		}
	}
	if req.Format == "" {
		req.Format = "word"
	}

	title := req.Title
	if title == "" {
		title = fmt.Sprintf("标书_%s", id.String())
	}
	chapters := req.Chapters
	if len(chapters) == 0 {
		chapters = defaultChapters(id.String())
	}
	h.exportWord(w, r, title, chapters)
}

// GET /api/v1/bids/{id}/export/pdf
func (h *Handlers) exportPDFHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	if _, err := h.Store.Get(r.Context(), id); err != nil {
		httperr.NotFound(w, rid, "workflow")
		return
	}

	title := fmt.Sprintf("标书_%s", id.String())
	chapters := defaultChapters(id.String())

	if h.PDFConverter == nil || !h.PDFConverter.Available() {
		// Fall back to DOCX with a warning header — keeps the API usable
		// in environments without LibreOffice installed.
		w.Header().Set("X-Export-Fallback", "docx; pdf converter unavailable")
		h.exportWord(w, r, title, chapters)
		return
	}

	docx, err := h.writeDocxBuffer(title, chapters)
	if err != nil {
		h.Log.Error("build docx for pdf", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	var pdfBuf bytes.Buffer
	if err := h.PDFConverter.ConvertToPDF(r.Context(), bytes.NewReader(docx.Bytes()), &pdfBuf); err != nil {
		h.Log.Error("convert pdf", slog.String("err", err.Error()))
		httperr.InternalError(w, rid)
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, strings.ReplaceAll(title, " ", "_")))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", pdfBuf.Len()))
	io.Copy(w, &pdfBuf)
}

// POST /api/v1/bids/{id}/export — export with chapter data
func (h *Handlers) exportDocumentHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}
	if _, err := h.Store.Get(r.Context(), id); err != nil {
		httperr.NotFound(w, rid, "workflow")
		return
	}

	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httperr.InvalidInput(w, rid, "invalid JSON: "+err.Error(), nil)
		return
	}

	title := req.Title
	if title == "" {
		title = fmt.Sprintf("标书_%s", id.String())
	}
	chapters := req.Chapters
	if len(chapters) == 0 {
		chapters = defaultChapters(id.String())
	}

	if req.Format == "pdf" {
		if h.PDFConverter == nil || !h.PDFConverter.Available() {
			w.Header().Set("X-Export-Fallback", "docx; pdf converter unavailable")
			h.exportWord(w, r, title, chapters)
			return
		}
		docx, err := h.writeDocxBuffer(title, chapters)
		if err != nil {
			h.Log.Error("build docx for pdf", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}
		var pdfBuf bytes.Buffer
		if err := h.PDFConverter.ConvertToPDF(r.Context(), bytes.NewReader(docx.Bytes()), &pdfBuf); err != nil {
			h.Log.Error("convert pdf", slog.String("err", err.Error()))
			httperr.InternalError(w, rid)
			return
		}
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.pdf"`, strings.ReplaceAll(title, " ", "_")))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", pdfBuf.Len()))
		io.Copy(w, &pdfBuf)
		return
	}
	h.exportWord(w, r, title, chapters)
}

// exportWordHandler is the HTTP entry point referenced by Routes().

