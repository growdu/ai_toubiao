package api

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bidwriter/services/workflow-svc/internal/model"
	"github.com/bidwriter/shared/pkg/tenant"
	"github.com/google/uuid"
)

func TestOoxmlBuilder_ProducesValidZip(t *testing.T) {
	var buf bytes.Buffer
	if err := (ooxmlBuilder{}).Build(&buf, "My Bid", []ChapterData{
		{Title: "Ch 1", Level: 1, SortOrder: 1, Content: "Hello\nWorld"},
	}); err != nil {
		t.Fatalf("Build: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	want := map[string]bool{
		"[Content_Types].xml":         false,
		"_rels/.rels":                false,
		"word/_rels/document.xml.rels": false,
		"word/document.xml":           false,
		"word/styles.xml":             false,
		"word/settings.xml":           false,
	}
	for _, f := range zr.File {
		if _, ok := want[f.Name]; ok {
			want[f.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("missing zip entry: %s", name)
		}
	}
}

func TestBuildDocumentXML_TitleAndEscape(t *testing.T) {
	out := buildDocumentXML("Title & <son>", []ChapterData{{Title: "C1", Content: "line1\nline2"}})
	// The raw ampersand + angle brackets must be XML-escaped in the output.
	if !strings.Contains(out, "Title &amp; &lt;son&gt;") {
		t.Errorf("title not properly escaped; output snippet: %.200s", out)
	}
	if !strings.Contains(out, "Heading1") {
		t.Error("expected Heading1 style ref")
	}
}

func TestBuildDocumentXML_ClampsHeadingLevel(t *testing.T) {
	// Level 0 → 1, level 99 → 9.
	out := buildDocumentXML("T", []ChapterData{
		{Title: "A", Level: 0, Content: "x"},
		{Title: "B", Level: 99, Content: "y"},
	})
	if !strings.Contains(out, `w:val="Heading1"`) {
		t.Error("expected level 0 clamped to Heading1")
	}
	if !strings.Contains(out, `w:val="Heading9"`) {
		t.Error("expected level 99 clamped to Heading9")
	}
}

func TestDefaultChapters_HasSixEntries(t *testing.T) {
	ch := defaultChapters("project-1")
	if len(ch) != 6 {
		t.Fatalf("len=%d, want 6", len(ch))
	}
	for i, c := range ch {
		if c.SortOrder != i+1 {
			t.Errorf("ch[%d].SortOrder=%d, want %d", i, c.SortOrder, i+1)
		}
		if c.Level != 1 {
			t.Errorf("ch[%d].Level=%d, want 1", i, c.Level)
		}
	}
}

func TestExportWordHandler_ReturnsDocx(t *testing.T) {
	id := uuid.New()
	be := &fakeBackend{
		getFn: func(context.Context, uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: id, ProjectID: uuid.New(), Status: model.StateDone}, nil
		},
	}
	h := &Handlers{
		Store: be, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
		DocBuilder: ooxmlBuilder{},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/"+id.String()+"/export/word", nil).
		WithContext(tenant.WithTenant(context.Background(), uuid.NewString()))
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/vnd.openxmlformats-officedocument.wordprocessingml.document" {
		t.Errorf("Content-Type=%q", got)
	}
	// Sanity-check the body is a real zip.
	if _, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len())); err != nil {
		t.Errorf("body not a valid zip: %v", err)
	}
}

func TestExportPDFHandler_FallbackToDocxWhenNoConverter(t *testing.T) {
	id := uuid.New()
	be := &fakeBackend{
		getFn: func(context.Context, uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: id, ProjectID: uuid.New(), Status: model.StateDone}, nil
		},
	}
	h := &Handlers{
		Store:        be,
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		DocBuilder:   ooxmlBuilder{},
		PDFConverter: nil, // missing
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/"+id.String()+"/export/pdf", nil).
		WithContext(tenant.WithTenant(context.Background(), uuid.NewString()))
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("X-Export-Fallback"); got == "" {
		t.Error("expected X-Export-Fallback header to be set when converter missing")
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "officedocument.wordprocessingml") {
		t.Errorf("expected docx content type, got %q", w.Header().Get("Content-Type"))
	}
}

func TestExportPDFHandler_FallbackWhenConverterUnavailable(t *testing.T) {
	id := uuid.New()
	be := &fakeBackend{
		getFn: func(context.Context, uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: id, ProjectID: uuid.New(), Status: model.StateDone}, nil
		},
	}
	// Converter explicitly reports unavailable (binary not on PATH).
	c := &fakeConverter{available: false}
	h := &Handlers{
		Store:        be,
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		DocBuilder:   ooxmlBuilder{},
		PDFConverter: c,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/"+id.String()+"/export/pdf", nil).
		WithContext(tenant.WithTenant(context.Background(), uuid.NewString()))
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if w.Header().Get("X-Export-Fallback") == "" {
		t.Error("expected fallback header")
	}
	if c.convertCalled {
		t.Error("ConvertToPDF should not be called when Available()==false")
	}
}

func TestExportPDFHandler_RealConverterInvoked(t *testing.T) {
	id := uuid.New()
	be := &fakeBackend{
		getFn: func(context.Context, uuid.UUID) (*model.Workflow, error) {
			return &model.Workflow{ID: id, ProjectID: uuid.New(), Status: model.StateDone}, nil
		},
	}
	c := &fakeConverter{available: true, pdfBytes: []byte("%PDF-1.4 fake")}
	h := &Handlers{
		Store:        be,
		Log:          slog.New(slog.NewTextHandler(io.Discard, nil)),
		DocBuilder:   ooxmlBuilder{},
		PDFConverter: c,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/bids/"+id.String()+"/export/pdf", nil).
		WithContext(tenant.WithTenant(context.Background(), uuid.NewString()))
	w := httptest.NewRecorder()
	h.Routes().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d, want 200; body=%s", w.Code, w.Body.String())
	}
	if got := w.Header().Get("Content-Type"); got != "application/pdf" {
		t.Errorf("Content-Type=%q, want application/pdf", got)
	}
	if !c.convertCalled {
		t.Error("expected ConvertToPDF to be called")
	}
	if !bytes.Equal(w.Body.Bytes(), c.pdfBytes) {
		t.Error("body bytes do not match converter output")
	}
}

func TestLibreOfficeConverter_Available(t *testing.T) {
	// With an obviously-missing path, Available must be false.
	c := NewLibreOfficeConverter("/nonexistent/path/to/soffice")
	if c.Available() {
		t.Error("expected Available()=false for nonexistent binary")
	}
	if err := c.ConvertToPDF(context.Background(), strings.NewReader("x"), io.Discard); !errors.Is(err, ErrPDFConverterUnavailable) {
		t.Errorf("expected ErrPDFConverterUnavailable, got %v", err)
	}
}

func TestLibreOfficeConverter_ResolveFromPath(t *testing.T) {
	// Auto-detect mode: if soffice / libreoffice is on PATH, Available=true;
	// otherwise false. Either is acceptable; just assert no panic.
	c := NewLibreOfficeConverter("")
	_ = c.Available()
}

// TestLibreOfficeConverter_RealConversion is an end-to-end check: it builds
// a real DOCX in memory, pipes it through the LibreOffice shell-out path,
// and verifies the output is an actual PDF (magic bytes, EOF marker, sane
// size). Skipped automatically when soffice is not installed, so this stays
// green in minimal containers while still asserting the full PDF pipeline
// wherever the binary is present.
func TestLibreOfficeConverter_RealConversion(t *testing.T) {
	c := NewLibreOfficeConverter("")
	if !c.Available() {
		t.Skip("libreoffice/soffice not installed; skipping real conversion e2e")
	}

	// Build a real DOCX with multiple chapters so LibreOffice has actual
	// content to paginate.
	var docxBuf bytes.Buffer
	if err := (ooxmlBuilder{}).Build(&docxBuf, "E2E 测试标书", []ChapterData{
		{Title: "第一章 项目概述", Level: 1, SortOrder: 1, Content: "本章介绍项目背景与目标。\n第二段说明范围。"},
		{Title: "1.1 背景", Level: 2, SortOrder: 2, Content: "项目由甲方发起,旨在……"},
		{Title: "第二章 技术方案", Level: 1, SortOrder: 3, Content: "技术方案分为三个层次。"},
	}); err != nil {
		t.Fatalf("build docx: %v", err)
	}
	if docxBuf.Len() < 200 {
		t.Fatalf("docx suspiciously small: %d bytes", docxBuf.Len())
	}

	// Convert with a generous timeout — first-run LibreOffice init can be slow.
	ctx, cancel := context.WithTimeout(context.Background(), 90*1_000_000_000) // 90s
	defer cancel()

	var pdfBuf bytes.Buffer
	if err := c.ConvertToPDF(ctx, bytes.NewReader(docxBuf.Bytes()), &pdfBuf); err != nil {
		t.Fatalf("ConvertToPDF: %v", err)
	}

	out := pdfBuf.Bytes()
	if len(out) < 500 {
		t.Fatalf("pdf suspiciously small: %d bytes", len(out))
	}

	// PDF files start with "%PDF-" and end with "%%EOF".
	if !bytes.HasPrefix(out, []byte("%PDF-")) {
		t.Fatalf("missing %%PDF- magic; first 16 bytes: %q", out[:min(16, len(out))])
	}
	// %%EOF may be followed by whitespace/newline; check suffix ignoring trailing whitespace.
	trimmed := bytes.TrimRight(out, " \t\r\n")
	if !bytes.HasSuffix(trimmed, []byte("%%EOF")) {
		t.Fatalf("missing %%EOF trailer; last 32 bytes: %q", out[max(0, len(out)-32):])
	}

	// Sanity: PDF version field after magic — should be 1.x through 2.x.
	if len(out) >= 8 {
		ver := string(out[5:8]) // e.g. "1.4", "1.7", "2.0"
		if !strings.HasPrefix(ver, "1.") && !strings.HasPrefix(ver, "2.") {
			t.Errorf("unexpected PDF version field %q", ver)
		}
	}

	t.Logf("converted %d bytes docx -> %d bytes pdf", docxBuf.Len(), pdfBuf.Len())
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// fakeConverter lets handler tests pin Available() and intercept the
// PDF conversion step.
type fakeConverter struct {
	available     bool
	convertCalled bool
	convertErr    error
	pdfBytes      []byte
}

func (f *fakeConverter) Available() bool { return f.available }
func (f *fakeConverter) ConvertToPDF(_ context.Context, _ io.Reader, out io.Writer) error {
	f.convertCalled = true
	if f.convertErr != nil {
		return f.convertErr
	}
	_, err := out.Write(f.pdfBytes)
	return err
}
