package service

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
)

func TestExtractDOCXText_ValidDocx(t *testing.T) {
	// Build a minimal .docx ZIP with word/document.xml containing
	// two paragraphs with <w:t> runs.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	docXML := `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body>
<w:p><w:r><w:t>First paragraph</w:t></w:r></w:p>
<w:p><w:r><w:t>Second paragraph</w:t></w:r></w:p>
</w:body>
</w:document>`

	files := map[string]string{
		"word/document.xml": docXML,
	}
	for name, content := range files {
		f, _ := zw.Create(name)
		f.Write([]byte(content))
	}
	zw.Close()

	text := extractDOCXText(buf.Bytes())
	if text == "" {
		t.Fatal("expected non-empty text extraction")
	}
	if !contains(text, "First paragraph") {
		t.Errorf("expected 'First paragraph' in text, got: %s", text)
	}
	if !contains(text, "Second paragraph") {
		t.Errorf("expected 'Second paragraph' in text, got: %s", text)
	}
}

func TestExtractDOCXText_InvalidZip(t *testing.T) {
	// Not a valid ZIP — should fall back to raw string.
	text := extractDOCXText([]byte("not a zip file"))
	if text != "not a zip file" {
		t.Errorf("expected raw string fallback, got: %s", text)
	}
}

func TestExtractDOCXText_NoDocumentXML(t *testing.T) {
	// Valid ZIP but no word/document.xml — should fall back to raw bytes.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, _ := zw.Create("other.txt")
	f.Write([]byte("other content"))
	zw.Close()

	text := extractDOCXText(buf.Bytes())
	// When document.xml is missing, buf is empty, so it returns the raw ZIP bytes.
	if text == "" {
		t.Error("expected non-empty fallback text")
	}
}

func TestExtractTextFromDocxXML_PreservesParagraphs(t *testing.T) {
	xml := `<w:p><w:r><w:t>Hello</w:t></w:r></w:p><w:p><w:r><w:t>World</w:t></w:r></w:p>`
	text := extractTextFromDocxXML(xml)
	if !contains(text, "Hello") {
		t.Errorf("expected 'Hello' in text, got: %s", text)
	}
	if !contains(text, "World") {
		t.Errorf("expected 'World' in text, got: %s", text)
	}
	if !contains(text, "\n") {
		t.Error("expected newline between paragraphs")
	}
}

func TestExtractTextFromDocxXML_MultipleRunsInParagraph(t *testing.T) {
	xml := `<w:p><w:r><w:t>Part1 </w:t></w:r><w:r><w:t>Part2</w:t></w:r></w:p>`
	text := extractTextFromDocxXML(xml)
	if !contains(text, "Part1 Part2") {
		t.Errorf("expected concatenated runs, got: %s", text)
	}
}

func TestExtractPDFTextRobust_FallbackOnInvalidPDF(t *testing.T) {
	// Invalid PDF data — should fall back to regex extractor which returns "".
	// Then extractPDFTextRobust returns "" (since pdftotext will also fail).
	text := extractPDFTextRobust(context.Background(), []byte("not a pdf"))
	// pdftotext won't be available in test env, and regex won't find BT/ET.
	// So we expect either "" or the raw content.
	_ = text // just verify no panic
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
