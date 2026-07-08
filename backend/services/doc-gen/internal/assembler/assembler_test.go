package assembler

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

func TestEscapeXML(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain", "plain"},
		{"a<b>c", "a&lt;b&gt;c"},
		{`"quote"`, "&quot;quote&quot;"},
		{"a&b", "a&amp;b"},
	}
	for _, tt := range tests {
		got := escapeXML(tt.input)
		if got != tt.want {
			t.Errorf("escapeXML(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestWriteDOCX(t *testing.T) {
	docXML := `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>测试内容</w:t></w:r></w:p></w:body></w:document>`
	images := map[string][]byte{
		"image_test.png": {0x89, 0x50, 0x4E, 0x47}, // PNG header
	}
	path := "/tmp/test_write_docx.docx"
	if err := writeDOCX(path, docXML, images); err != nil {
		t.Fatalf("writeDOCX: %v", err)
	}
	// 验证文件存在
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("docx file not created: %v", err)
	}
	// 验证是有效 ZIP
	data, _ := os.ReadFile(path)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}
	// 验证必需文件
	hasContent := false
	hasDoc := false
	for _, f := range zr.File {
		if f.Name == "[Content_Types].xml" {
			hasContent = true
		}
		if f.Name == "word/document.xml" {
			hasDoc = true
		}
	}
	if !hasContent {
		t.Error("missing [Content_Types].xml")
	}
	if !hasDoc {
		t.Error("missing word/document.xml")
	}
}

func TestAssembler_GenerateDOCX(t *testing.T) {
	a := &Assembler{}
	theme := core.DefaultTheme()

	pkg := &core.BidPackage{
		ID:        uuid.New(),
		RFPID:     uuid.New(),
		OutlineID: uuid.New(),
		Chapters: []core.Chapter{
			{
				Spec: core.ChapterSpec{ID: uuid.New(), Title: "技术方案", Level: 1, TargetWords: 1000},
				Content: core.ChapterContent{
					ID:        uuid.New(),
					ChapterID: uuid.New(),
					Markdown:  "## 系统架构\n\n本系统采用微服务架构。\n\n- 高可用\n- 可扩展\n\n## 功能设计\n\n核心功能包括用户管理和权限控制。",
				},
			},
			{
				Spec: core.ChapterSpec{ID: uuid.New(), Title: "商务方案", Level: 1, TargetWords: 800},
				Content: core.ChapterContent{
					ID:        uuid.New(),
					ChapterID: uuid.New(),
					Markdown:  "## 报价\n\n总报价 500 万元。",
				},
			},
		},
	}

	outPath := "/tmp/test_bidgen_assemble.docx"
	pkg.OutputPath = outPath

	path, err := a.Assemble(context.Background(), pkg, theme)
	if err != nil {
		t.Fatalf("Assemble: %v", err)
	}
	if path != outPath {
		t.Fatalf("expected path %q, got %q", outPath, path)
	}

	// 验证 .docx 是有效 ZIP
	data, err := readFile(path)
	if err != nil {
		t.Fatalf("read docx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("invalid docx zip: %v", err)
	}

	// 验证必需文件存在
	required := map[string]bool{
		"[Content_Types].xml":          false,
		"_rels/.rels":                  false,
		"word/document.xml":            false,
		"word/styles.xml":              false,
		"word/_rels/document.xml.rels": false,
	}
	for _, f := range zr.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
	}
	for name, found := range required {
		if !found {
			t.Errorf("missing required file in docx: %s", name)
		}
	}

	// 验证 document.xml 包含章节内容
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			rc, _ := f.Open()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			rc.Close()
			content := buf.String()
			if !strings.Contains(content, "技术方案") {
				t.Error("expected '技术方案' in document.xml")
			}
			if !strings.Contains(content, "商务方案") {
				t.Error("expected '商务方案' in document.xml")
			}
		}
	}
}

func TestFigurePlaceholderXML_Placeholder(t *testing.T) {
	a := &Assembler{}
	pkg := &core.BidPackage{ID: uuid.New()}
	imageIdx := 0

	// 测试无渲染结果的占位符
	xml := a.figurePlaceholderXML("[!figure:mermaid caption=流程图]", pkg, &imageIdx)
	if !strings.Contains(xml, "图表占位") {
		t.Fatal("expected placeholder text for unrendered figure")
	}
}

func TestFigurePlaceholderXML_Table(t *testing.T) {
	a := &Assembler{}
	specID := uuid.New()
	pkg := &core.BidPackage{
		ID: uuid.New(),
		Figures: []core.Illustration{
			{
				ID:     uuid.New(),
				SpecID: specID,
				OOXML:  "<w:tbl><w:tr><w:tc><w:p><w:r><w:t>测试</w:t></w:r></w:p></w:tc></w:tr></w:tbl>",
				Status: "ok",
			},
		},
	}
	imageIdx := 0
	xml := a.figurePlaceholderXML("[!figure:table caption=报价表]", pkg, &imageIdx)
	if !strings.Contains(xml, "<w:tbl>") {
		t.Fatal("expected table OOXML in figure placeholder")
	}
	if !strings.Contains(xml, "报价表") {
		t.Fatal("expected caption '报价表' in figure placeholder")
	}
}
