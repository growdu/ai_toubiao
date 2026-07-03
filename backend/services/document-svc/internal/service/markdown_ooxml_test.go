package service

import (
	"strings"
	"testing"
)

func TestMarkdownToOOXML_Heading(t *testing.T) {
	xml := markdownToOOXML("# Title\n## Subtitle\n### Section")
	if !strings.Contains(xml, `w:val="Heading1"`) {
		t.Error("expected Heading1 style for # heading")
	}
	if !strings.Contains(xml, `w:val="Heading2"`) {
		t.Error("expected Heading2 style for ## heading")
	}
	if !strings.Contains(xml, `w:val="Heading3"`) {
		t.Error("expected Heading3 style for ### heading")
	}
}

func TestMarkdownToOOXML_BulletList(t *testing.T) {
	xml := markdownToOOXML("- Item 1\n- Item 2\n* Item 3")
	if !strings.Contains(xml, "• Item 1") {
		t.Error("expected bullet character for - item")
	}
	if !strings.Contains(xml, "• Item 2") {
		t.Error("expected bullet character for second - item")
	}
	if !strings.Contains(xml, "• Item 3") {
		t.Error("expected bullet character for * item")
	}
}

func TestMarkdownToOOXML_NumberedList(t *testing.T) {
	xml := markdownToOOXML("1. First\n2. Second\n3. Third")
	if !strings.Contains(xml, "1. First") {
		t.Error("expected numbered item 1")
	}
	if !strings.Contains(xml, "2. Second") {
		t.Error("expected numbered item 2")
	}
}

func TestMarkdownToOOXML_BoldItalic(t *testing.T) {
	xml := markdownToOOXML("This is **bold** and *italic* text.")
	if !strings.Contains(xml, "<w:b/>") {
		t.Error("expected bold run for **text**")
	}
	if !strings.Contains(xml, "<w:i/>") {
		t.Error("expected italic run for *text*")
	}
}

func TestMarkdownToOOXML_CodeInline(t *testing.T) {
	xml := markdownToOOXML("Use `code` here.")
	if !strings.Contains(xml, "Courier New") {
		t.Error("expected monospace font for `code`")
	}
}

func TestMarkdownToOOXML_Table(t *testing.T) {
	xml := markdownToOOXML("| Col1 | Col2 |\n|---|---|\n| A | B |")
	if !strings.Contains(xml, "<w:tbl>") {
		t.Error("expected table element")
	}
	if !strings.Contains(xml, "D9E2F3") {
		t.Error("expected header shading")
	}
	if !strings.Contains(xml, ">A<") {
		t.Error("expected cell content A")
	}
}

func TestMarkdownToOOXML_FigurePlaceholder(t *testing.T) {
	xml := markdownToOOXML("[!figure:1 type=mermaid caption=流程图]")
	if !strings.Contains(xml, "图表占位") {
		t.Error("expected figure placeholder text")
	}
}

func TestMarkdownToOOXML_PlainParagraph(t *testing.T) {
	xml := markdownToOOXML("Just a plain paragraph.")
	if !strings.Contains(xml, "<w:p>") {
		t.Error("expected paragraph element")
	}
	if !strings.Contains(xml, "Just a plain paragraph.") {
		t.Error("expected plain text content")
	}
}

func TestMarkdownToOOXML_EmptyInput(t *testing.T) {
	xml := markdownToOOXML("")
	if xml != "" {
		t.Errorf("expected empty string for empty input, got %s", xml)
	}
}

func TestMarkdownToOOXML_MixedContent(t *testing.T) {
	md := `# 第一章 技术方案

## 1.1 系统架构

本系统采用微服务架构，具有以下特点：

- 高可用性
- 可扩展性
- 安全性

### 关键指标

| 指标 | 目标值 |
|---|---|
| 可用性 | 99.9% |
| 响应时间 | <100ms |

**注意**：以上指标为*最低要求*。`
	xml := markdownToOOXML(md)
	// Verify all elements present
	checks := []string{
		`w:val="Heading1"`,
		`w:val="Heading2"`,
		`w:val="Heading3"`,
		"• 高可用性",
		"<w:tbl>",
		"99.9%",
		"<w:b/>",
		"<w:i/>",
	}
	for _, check := range checks {
		if !strings.Contains(xml, check) {
			t.Errorf("expected %q in output", check)
		}
	}
}
