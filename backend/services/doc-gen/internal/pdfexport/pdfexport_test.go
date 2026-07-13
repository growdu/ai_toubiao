package pdfexport

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestLibreOfficeConverter_Available_EmptyBinary(t *testing.T) {
	// When Binary is empty we rely on PATH lookup. The test must not
	// assert true/false — only that Available() returns whatever
	// exec.LookPath decides — so we accept both.
	c := New("")
	_ = c.Available() // must not panic
}

func TestLibreOfficeConverter_Available_MissingBinary(t *testing.T) {
	// Pointing at a nonexistent path always returns false.
	c := New("/nonexistent/path/to/soffice")
	if c.Available() {
		t.Fatal("expected Available()=false for missing binary")
	}
}

func TestLibreOfficeConverter_Available_BinaryExists(t *testing.T) {
	// When a real binary is given (we use /bin/sh which is always on
	// Linux test images) resolve() short-circuits and Available()
	// should return true. The actual conversion would fail because
	// sh is not soffice, but that's covered by ConvertFile tests.
	c := New("/bin/sh")
	if !c.Available() {
		t.Fatal("expected Available()=true for /bin/sh")
	}
}

func TestLibreOfficeConverter_ResolveFromPath(t *testing.T) {
	c := New("")
	resolved := c.resolve()
	// On the CI host there may or may not be soffice. Either way the
	// function must not panic and must return either a real path or "".
	if resolved != "" {
		if _, err := os.Stat(resolved); err != nil {
			t.Fatalf("resolved path %q does not exist: %v", resolved, err)
		}
	}
}

func TestLibreOfficeConverter_ConvertFile_NotAvailable(t *testing.T) {
	c := New("/nonexistent/soffice")
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "in.docx")
	if err := os.WriteFile(docxPath, []byte("not a real docx"), 0o644); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	outPath := filepath.Join(tmpDir, "out.pdf")
	err := c.ConvertFile(context.Background(), docxPath, outPath)
	if err != ErrUnavailable {
		t.Fatalf("expected ErrUnavailable, got %v", err)
	}
}

func TestLibreOfficeConverter_ConvertFile_EmptyPaths(t *testing.T) {
	// Even when Available is true, empty paths must fail validation
	// rather than letting soffice write to "" (which would silently
	// succeed and produce zero-byte outputs in the working directory).
	c := New("/bin/sh")
	if err := c.ConvertFile(context.Background(), "", "/tmp/out.pdf"); err == nil {
		t.Fatal("expected error for empty docx path")
	}
	if err := c.ConvertFile(context.Background(), "/tmp/in.docx", ""); err == nil {
		t.Fatal("expected error for empty out path")
	}
}

func TestLibreOfficeConverter_ConvertFile_RealConversion_SkipsIfAbsent(t *testing.T) {
	// Real conversion test is skipped when soffice/libreoffice is not
	// installed so this stays green on dev machines and CI without
	// LibreOffice. Production deployments run with soffice present.
	c := New("")
	if !c.Available() {
		t.Skip("libreoffice/soffice not installed; skipping real conversion e2e")
	}
	tmpDir := t.TempDir()
	docxPath := filepath.Join(tmpDir, "in.docx")
	// Minimal .docx is a zip; we just write some bytes — soffice will
	// fail to render but ConvertFile itself returns an error which is
	// what we want to assert here (it must NOT silently succeed).
	if err := os.WriteFile(docxPath, []byte("garbage"), 0o644); err != nil {
		t.Fatalf("write docx: %v", err)
	}
	outPath := filepath.Join(tmpDir, "out.pdf")
	err := c.ConvertFile(context.Background(), docxPath, outPath)
	if err == nil {
		t.Fatal("expected error converting garbage docx")
	}
}
