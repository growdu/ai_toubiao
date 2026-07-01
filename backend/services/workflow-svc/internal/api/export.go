// Package api implements the HTTP layer for workflow-svc.
package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
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
	Format    string        `json:"format"` // "word" or "pdf"
	Chapters  []ChapterData `json:"chapters"`
	Title     string        `json:"title"`
	ProjectName string      `json:"project_name"`
}

// exportWord generates a minimal DOCX and writes it to w.
func (h *Handlers) exportWord(w http.ResponseWriter, r *http.Request, title string, chapters []ChapterData) {
	rid := logger.RequestIDFrom(r.Context())

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
		"word/document.xml": buildDocumentXML(title, chapters),
		"word/styles.xml":   defaultStylesXML(),
		"word/settings.xml": `<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:defaultTabStop w:val="720"/></w:settings>`,
	}

	for name, content := range files {
		f, err := zw.Create(name)
		if err != nil {
			httperr.InternalError(w, rid)
			return
		}
		if _, err := f.Write([]byte(content)); err != nil {
			httperr.InternalError(w, rid)
			return
		}
	}
	if err := zw.Close(); err != nil {
		httperr.InternalError(w, rid)
		return
	}

	filename := fmt.Sprintf("%s.docx", strings.ReplaceAll(title, " ", "_"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	io.Copy(w, &buf)
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

// GET /api/v1/bids/{id}/export/word
func (h *Handlers) exportWordHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	wf, err := h.Store.Get(r.Context(), id)
	if err != nil {
		httperr.NotFound(w, rid, "workflow")
		return
	}

	var req ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If no body, use defaults
		req = ExportRequest{Format: "word"}
	}

	title := req.Title
	if title == "" {
		title = fmt.Sprintf("标书_%s", wf.ProjectID.String())
	}

	// Parse chapters from request or use defaults
	chapters := req.Chapters
	if len(chapters) == 0 {
		chapters = []ChapterData{
			{Title: "第一章 投标函", Level: 1, SortOrder: 1, Content: "(内容待生成)"},
			{Title: "第二章 项目理解与总体思路", Level: 1, SortOrder: 2, Content: "(内容待生成)"},
			{Title: "第三章 技术方案", Level: 1, SortOrder: 3, Content: "(内容待生成)"},
			{Title: "第四章 项目实施计划", Level: 1, SortOrder: 4, Content: "(内容待生成)"},
			{Title: "第五章 质量保证措施", Level: 1, SortOrder: 5, Content: "(内容待生成)"},
			{Title: "第六章 售后服务", Level: 1, SortOrder: 6, Content: "(内容待生成)"},
		}
	}

	h.exportWord(w, r, title, chapters)
}

// GET /api/v1/bids/{id}/export/pdf
func (h *Handlers) exportPDFHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	// For MVP, PDF export falls back to Word since LibreOffice may not be available
	// The frontend will show PDF as a future enhancement
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	wf, err := h.Store.Get(r.Context(), id)
	if err != nil {
		httperr.NotFound(w, rid, "workflow")
		return
	}

	title := fmt.Sprintf("标书_%s", wf.ProjectID.String())
	chapters := []ChapterData{
		{Title: "第一章 投标函", Level: 1, SortOrder: 1, Content: "(内容待生成)"},
		{Title: "第二章 项目理解与总体思路", Level: 1, SortOrder: 2, Content: "(内容待生成)"},
		{Title: "第三章 技术方案", Level: 1, SortOrder: 3, Content: "(内容待生成)"},
		{Title: "第四章 项目实施计划", Level: 1, SortOrder: 4, Content: "(内容待生成)"},
		{Title: "第五章 质量保证措施", Level: 1, SortOrder: 5, Content: "(内容待生成)"},
		{Title: "第六章 售后服务", Level: 1, SortOrder: 6, Content: "(内容待生成)"},
	}

	// For now, return Word format since PDF conversion requires LibreOffice
	// TODO: Implement proper PDF export via LibreOffice
	h.exportWord(w, r, title, chapters)
}

// POST /api/v1/bids/{id}/export - export with chapter data
func (h *Handlers) exportDocumentHandler(w http.ResponseWriter, r *http.Request) {
	rid := logger.RequestIDFrom(r.Context())
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		httperr.InvalidInput(w, rid, "invalid id", nil)
		return
	}

	_, err = h.Store.Get(r.Context(), id)
	if err != nil {
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
		chapters = []ChapterData{
			{Title: "第一章 投标函", Level: 1, SortOrder: 1, Content: "(内容待生成)"},
			{Title: "第二章 项目理解与总体思路", Level: 1, SortOrder: 2, Content: "(内容待生成)"},
			{Title: "第三章 技术方案", Level: 1, SortOrder: 3, Content: "(内容待生成)"},
			{Title: "第四章 项目实施计划", Level: 1, SortOrder: 4, Content: "(内容待生成)"},
			{Title: "第五章 质量保证措施", Level: 1, SortOrder: 5, Content: "(内容待生成)"},
			{Title: "第六章 售后服务", Level: 1, SortOrder: 6, Content: "(内容待生成)"},
		}
	}

	if req.Format == "pdf" {
		// TODO: PDF export via LibreOffice
		h.exportWord(w, r, title, chapters)
		return
	}

	h.exportWord(w, r, title, chapters)
}