// Package pdfexport converts DOCX bid documents to PDF.
//
// docgen-svc is a stateless HTTP service that ships the assembled .docx
// straight back to the caller. PDF was previously hard-failed with 501
// because nothing in this module knew how to spawn LibreOffice. This
// package is the seam: the HTTP layer injects a Converter (real or
// stub), and the assembler handler can call `ConvertFile(ctx, docxPath,
// outPath)` whenever a caller asks for format="pdf".
//
// The default production implementation shells out to `soffice` /
// `libreoffice` exactly the way workflow-svc does — we keep the
// dependencies flat instead of pulling in a shared helper package
// because doc-gen is intentionally self-contained (it has its own go.mod
// chain) and the converter is only ~80 lines.
package pdfexport

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ErrUnavailable signals that no usable LibreOffice binary is on the
// host. The HTTP layer should translate this to 503 with an actionable
// hint, not a generic 500.
var ErrUnavailable = errors.New("pdf converter not available: install libreoffice (soffice)")

// Converter turns DOCX bytes into PDF bytes.
type Converter interface {
	// Available reports whether the converter can run on this host.
	// Returns false when LibreOffice is not on PATH and no override
	// was configured — the handler uses this to decide whether to
	// accept a `format=pdf` request at all.
	Available() bool
	// ConvertFile reads a DOCX from disk, converts it to PDF, and
	// writes the PDF to outPath (which must not already exist). Returns
	// the path of the written file (== outPath) on success.
	ConvertFile(ctx context.Context, docxPath, outPath string) error
	// ConvertStream is a streaming variant used when the input lives in
	// memory (e.g. a freshly assembled DOCX the handler does not want
	// to spill to a temp file just for the conversion).
	ConvertStream(ctx context.Context, docx io.Reader, out io.Writer) error
}

// LibreOfficeConverter shells out to `soffice --headless --convert-to pdf`.
type LibreOfficeConverter struct {
	// Binary is the path to `soffice` / `libreoffice`. Empty means
	// auto-detect via PATH.
	Binary string
}

// New returns a LibreOffice-backed converter. binary="" auto-detects.
func New(binary string) *LibreOfficeConverter {
	return &LibreOfficeConverter{Binary: binary}
}

func (l *LibreOfficeConverter) resolve() string {
	if l.Binary != "" {
		return l.Binary
	}
	for _, name := range []string{"soffice", "libreoffice"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// Available returns true when the resolved binary is on disk and
// executable. exec.LookPath handles both PATH lookup and absolute-path
// liveness, so a misconfigured /opt/libreoffice/program/soffice that
// doesn't exist also returns false.
func (l *LibreOfficeConverter) Available() bool {
	_, err := exec.LookPath(l.resolve())
	return err == nil
}

// ConvertFile writes the PDF to outPath. The caller is responsible for
// choosing a writable location; we do not silently create temp files
// because the handler wants the path to persist for the download
// endpoint.
func (l *LibreOfficeConverter) ConvertFile(ctx context.Context, docxPath, outPath string) error {
	if !l.Available() {
		return ErrUnavailable
	}
	if docxPath == "" {
		return fmt.Errorf("docx path is empty")
	}
	if outPath == "" {
		return fmt.Errorf("out path is empty")
	}
	// Ensure the parent directory exists (it almost always does; this
	// mainly guards against typos in config).
	if dir := filepath.Dir(outPath); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir out dir: %w", err)
		}
	}
	outDir := filepath.Dir(outPath)
	cmd := exec.CommandContext(ctx, l.resolve(),
		"--headless", "--norestore", "--nologo", "--nofirststartwizard",
		"--convert-to", "pdf", "--outdir", outDir, docxPath)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("libreoffice convert: %w (%s)", err, strings.TrimSpace(string(outBytes)))
	}
	// LibreOffice writes <basename>.pdf next to the source file. Move
	// (rename) it to the requested outPath so the caller can rely on
	// the path.
	produced := strings.TrimSuffix(docxPath, filepath.Ext(docxPath)) + ".pdf"
	if produced == outPath {
		return nil
	}
	if err := os.Rename(produced, outPath); err != nil {
		return fmt.Errorf("rename %s → %s: %w", produced, outPath, err)
	}
	return nil
}

// ConvertStream is the streaming variant. It temp-files the DOCX,
// converts, and streams the PDF out. The temp files are always cleaned
// up regardless of error path.
func (l *LibreOfficeConverter) ConvertStream(ctx context.Context, docx io.Reader, out io.Writer) error {
	if !l.Available() {
		return ErrUnavailable
	}
	tmpDocx, err := os.CreateTemp("", "docgen-*.docx")
	if err != nil {
		return fmt.Errorf("create temp docx: %w", err)
	}
	defer os.Remove(tmpDocx.Name())
	if _, err := io.Copy(tmpDocx, docx); err != nil {
		tmpDocx.Close()
		return fmt.Errorf("write temp docx: %w", err)
	}
	if err := tmpDocx.Close(); err != nil {
		return fmt.Errorf("close temp docx: %w", err)
	}
	tmpPDF, err := os.CreateTemp("", "docgen-*.pdf")
	if err != nil {
		return fmt.Errorf("create temp pdf: %w", err)
	}
	tmpPDF.Close()
	defer os.Remove(tmpPDF.Name())

	if err := l.ConvertFile(ctx, tmpDocx.Name(), tmpPDF.Name()); err != nil {
		return err
	}
	f, err := os.Open(tmpPDF.Name())
	if err != nil {
		return fmt.Errorf("open produced pdf: %w", err)
	}
	defer f.Close()
	if _, err := io.Copy(out, f); err != nil {
		return fmt.Errorf("stream pdf: %w", err)
	}
	return nil
}
