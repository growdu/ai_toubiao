package ingest

import (
	"strings"
	"testing"
)

func TestSemanticChunk_BasicSplit(t *testing.T) {
	text := "第一段内容。\n\n第二段内容。\n\n第三段内容。"
	chunks := semanticChunk(text, 512, 64)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk (short paragraphs merge), got %d", len(chunks))
	}
}

func TestSemanticChunk_LongParagraph(t *testing.T) {
	// 构造超长段落（超过 targetTokens）
	var sb strings.Builder
	for i := 0; i < 5000; i++ {
		sb.WriteString("这是一段很长的中文文本内容用于测试分块算法。")
	}
	chunks := semanticChunk(sb.String(), 512, 64)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks for long text, got %d", len(chunks))
	}
}

func TestSemanticChunk_EmptyText(t *testing.T) {
	chunks := semanticChunk("", 512, 64)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", len(chunks))
	}
	chunks = semanticChunk("   \n\n  \n  ", 512, 64)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for whitespace, got %d", len(chunks))
	}
}

func TestSemanticChunk_MergeShort(t *testing.T) {
	// 短段落应被合并
	text := "短句1。\n\n短句2。\n\n短句3。"
	chunks := semanticChunk(text, 512, 64)
	// 短段落应合并为 1 个块
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d", len(chunks))
	}
}

func TestEstimateTokens_CJK(t *testing.T) {
	tokens := estimateTokens("中文测试文本")
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
	// 中文 1 token ≈ 1.6 字，5 个字大约 3 token
	if tokens < 2 || tokens > 5 {
		t.Fatalf("expected 2-5 tokens for 5 CJK chars, got %d", tokens)
	}
}

func TestEstimateTokens_English(t *testing.T) {
	tokens := estimateTokens("hello world this is a test")
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
}

func TestDetectCategory(t *testing.T) {
	tests := []struct {
		path, name, rfpAbs string
		want               string
	}{
		{"/tmp/招标文件.pdf", "招标文件.pdf", "", "rfp"},
		{"/tmp/资质/证书.pdf", "证书.pdf", "", "qualification"},
		{"/tmp/技术方案/方案.txt", "方案.txt", "", "technical"},
		{"/tmp/业绩案例/case.txt", "case.txt", "", "performance"},
		{"/tmp/历史标书/ref.txt", "ref.txt", "", "reference"},
		{"/tmp/其他.txt", "其他.txt", "", "other"},
	}
	for _, tt := range tests {
		got := detectCategory(tt.path, tt.name, tt.rfpAbs)
		if got != tt.want {
			t.Errorf("detectCategory(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestExtractXMLText(t *testing.T) {
	xml := `<?xml version="1.0"?>
<root>
<w:p><w:t>第一段</w:t></w:p>
<w:p><w:t>第二段</w:t></w:p>
</root>`
	text := extractXMLText(xml)
	if !strings.Contains(text, "第一段") {
		t.Fatalf("expected text to contain '第一段', got %q", text)
	}
	if !strings.Contains(text, "第二段") {
		t.Fatalf("expected text to contain '第二段', got %q", text)
	}
}
