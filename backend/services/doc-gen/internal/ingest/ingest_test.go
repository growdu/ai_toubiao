package ingest

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsTextFormat(t *testing.T) {
	tests := []struct {
		ext  string
		want bool
	}{
		{"txt", true}, {"md", true}, {"csv", true}, {"json", true},
		{"yaml", true}, {"yml", true}, {"log", true}, {"xml", true},
		{"html", true}, {"htm", true},
		{"pdf", false}, {"docx", false}, {"xls", false}, {"xlsx", false}, {"", false},
	}
	for _, tt := range tests {
		if got := isTextFormat(tt.ext); got != tt.want {
			t.Errorf("isTextFormat(%q) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestFileSha256(t *testing.T) {
	f, err := os.CreateTemp("", "sha256-*.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString("hello world")
	f.Close()

	h1, err := fileSha256(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	h2, err := fileSha256(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("hash inconsistent: %s != %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars", len(h1))
	}
}

func TestFileSha256_DifferentContent(t *testing.T) {
	f1, _ := os.CreateTemp("", "sha256-a-*.txt")
	defer os.Remove(f1.Name())
	f1.WriteString("content A")
	f1.Close()

	f2, _ := os.CreateTemp("", "sha256-b-*.txt")
	defer os.Remove(f2.Name())
	f2.WriteString("content B")
	f2.Close()

	h1, _ := fileSha256(f1.Name())
	h2, _ := fileSha256(f2.Name())
	if h1 == h2 {
		t.Error("different content should have different hashes")
	}
}

// TestUnzipToTemp_ZipSlipBlocked 验证路径穿越攻击被拦截。
func TestUnzipToTemp_ZipSlipBlocked(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 正常文件
	w, err := zw.Create("normal.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("ok"))

	// 路径穿越尝试
	w2, err := zw.Create("../escape.txt")
	if err != nil {
		t.Fatal(err)
	}
	w2.Write([]byte("escaped"))

	zw.Close()

	tmpZip, err := os.CreateTemp("", "zipslip-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpZip.Name())
	tmpZip.Write(buf.Bytes())
	tmpZip.Close()

	dir, err := unzipToTemp(tmpZip.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// 正常文件应在临时目录内
	if _, err := os.Stat(filepath.Join(dir, "normal.txt")); err != nil {
		t.Errorf("normal.txt not extracted: %v", err)
	}

	// 穿越文件不应出现在临时目录的上级
	escapePath := filepath.Join(filepath.Dir(dir), "escape.txt")
	if _, err := os.Stat(escapePath); err == nil {
		os.Remove(escapePath)
		t.Error("zip slip: escape.txt was written outside temp dir")
	}
}

// TestUnzipToTemp_NormalZip 验证正常 zip 解包。
func TestUnzipToTemp_NormalZip(t *testing.T) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, err := zw.Create("dir/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	w.Write([]byte("content"))
	zw.Close()

	tmpZip, err := os.CreateTemp("", "normal-*.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpZip.Name())
	tmpZip.Write(buf.Bytes())
	tmpZip.Close()

	dir, err := unzipToTemp(tmpZip.Name())
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	data, err := os.ReadFile(filepath.Join(dir, "dir", "file.txt"))
	if err != nil {
		t.Fatalf("nested file not found: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("expected 'content', got %q", string(data))
	}
}
