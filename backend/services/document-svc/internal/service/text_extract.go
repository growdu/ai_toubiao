package service

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// extractPDFTextRobust extracts text from PDF bytes using multiple strategies:
//  1. pdftotext (poppler-utils) — the gold standard, available on most servers.
//  2. Fallback to the regex-based extractor for environments without pdftotext.
//
// The fallback is deliberately simple; for production with scanned PDFs an
// OCR layer (tesseract / cloud OCR) would be added as a third strategy.
func extractPDFTextRobust(ctx context.Context, data []byte) string {
	// Strategy 1: shell out to pdftotext.
	if text := extractViaPdftotext(ctx, data); text != "" {
		return text
	}
	// Strategy 2: regex-based fallback.
	return extractPDFText(data)
}

// extractViaPdftotext writes the PDF to a temp file, runs `pdftotext`, and
// returns the extracted text. Returns "" if pdftotext is unavailable or fails.
func extractViaPdftotext(ctx context.Context, data []byte) string {
	bin := os.Getenv("PDFTOTEXT_BIN")
	if bin == "" {
		bin = "pdftotext"
	}
	if _, err := exec.LookPath(bin); err != nil {
		return "" // pdftotext not installed
	}

	tmpDir := os.TempDir()
	inPath := fmt.Sprintf("%s/%s.pdf", tmpDir, uuid.New().String())
	outPath := strings.TrimSuffix(inPath, ".pdf") + ".txt"
	defer os.Remove(inPath)
	defer os.Remove(outPath)

	if err := os.WriteFile(inPath, data, 0644); err != nil {
		return ""
	}

	execCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	// -layout preserves reading order; -enc UTF-8 ensures proper Chinese output.
	cmd := exec.CommandContext(execCtx, bin, "-layout", "-enc", "UTF-8", inPath, outPath)
	if err := cmd.Run(); err != nil {
		return ""
	}

	text, err := os.ReadFile(outPath)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(text))
}

// extractDOCXText extracts text from .docx bytes with paragraph-awareness.
// It parses word/document.xml and preserves paragraph breaks, producing
// more structured text than the naive tag-stripper.
func extractDOCXText(data []byte) string {
	zipReader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		// Not a valid ZIP — fall back to raw string.
		return string(data)
	}

	var buf strings.Builder
	for _, f := range zipReader.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			continue
		}
		xmlData, _ := io.ReadAll(rc)
		rc.Close()
		buf.WriteString(extractTextFromDocxXML(string(xmlData)))
		break
	}
	if buf.Len() == 0 {
		return string(data)
	}
	return strings.TrimSpace(buf.String())
}

// extractTextFromDocxXML parses OOXML paragraph elements and extracts text
// from <w:t> runs, preserving paragraph boundaries with newlines.
// This is much more accurate than stripping all tags because it understands
// the document structure.
func extractTextFromDocxXML(xml string) string {
	var buf strings.Builder

	// Split on paragraph boundaries (</w:p>).
	paragraphs := strings.Split(xml, "</w:p>")
	for _, para := range paragraphs {
		// Extract all <w:t...>...</w:t> content within this paragraph.
		textRe := regexp.MustCompile(`<w:t[^>]*>([^<]*)</w:t>`)
		matches := textRe.FindAllStringSubmatch(para, -1)
		var paraText strings.Builder
		for _, m := range matches {
			paraText.WriteString(m[1])
		}
		text := strings.TrimSpace(paraText.String())
		if text != "" {
			buf.WriteString(text)
			buf.WriteString("\n")
		}
	}
	return strings.TrimSpace(buf.String())
}
