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
	text := "短句1。\n\n短句2。\n\n短句3。"
	chunks := semanticChunk(text, 512, 64)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 merged chunk, got %d", len(chunks))
	}
}

func TestEstimateTokens_CJK(t *testing.T) {
	tokens := estimateTokens("中文测试文本")
	if tokens <= 0 {
		t.Fatalf("expected positive token count, got %d", tokens)
	}
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

// TestDetectCategory 验证按文件名分类，附件路径含"招标"不误判为 RFP。
func TestDetectCategory(t *testing.T) {
	tests := []struct {
		name, rfpAbs string
		want         string
	}{
		{"招标文件.pdf", "", "rfp"},
		{"附件-招标-商务标投标文件.pdf", "", "reference"},
		{"附件-招标-可行性研究报告.pdf", "", "technical"},
		{"低压开关柜技术规范书.docx", "", "technical"},
		{"附件-招标-合同条款.docx", "", "commercial"},
		{"附件-招标-工程量清单.xls", "", "commercial"},
		{"附件-招标-总平图.pdf", "", "drawing"},
		{"资质证书.pdf", "", "qualification"},
		{"业绩案例.txt", "", "performance"},
		{"其他.txt", "", "other"},
	}
	for _, tt := range tests {
		got := detectCategory("/tmp/"+tt.name, tt.name, tt.rfpAbs)
		if got != tt.want {
			t.Errorf("detectCategory(name=%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

// TestDetectCategory_ExplicitRFP 显式 --rfp 指定的一定是 RFP。
func TestDetectCategory_ExplicitRFP(t *testing.T) {
	got := detectCategory("/abs/任意名.pdf", "任意名.pdf", "/abs/任意名.pdf")
	if got != "rfp" {
		t.Fatalf("explicit rfpAbs should be rfp, got %q", got)
	}
}

func TestCleanNoise(t *testing.T) {
	text := "目录\nTOC \\o \"1-2\" \\h \\z \\u\n投标人注意事项 PAGEREF _Toc230702378 \\h IV\n正文内容"
	got := cleanNoise(text)
	if strings.Contains(got, "PAGEREF") {
		t.Fatalf("PAGEREF not removed: %q", got)
	}
	if strings.Contains(got, "TOC") {
		t.Fatalf("TOC field not removed: %q", got)
	}
	if !strings.Contains(got, "正文内容") {
		t.Fatalf("正文内容 lost: %q", got)
	}
}

func TestNormalizeExt(t *testing.T) {
	tests := []struct {
		name, want string
	}{
		{"文件.pdf", "pdf"},
		{"文件.pdf.pdf", "pdf"},
		{"文件.docx", "docx"},
		{"文件.xls", "xls"},
		{"无扩展名", ""},
		{"file.PDF", "pdf"},
	}
	for _, tt := range tests {
		got := normalizeExt(tt.name)
		if got != tt.want {
			t.Errorf("normalizeExt(%q) = %q, want %q", tt.name, got, tt.want)
		}
	}
}

func TestSlideWindow_Basic(t *testing.T) {
	text := strings.Repeat("a", 2000)
	chunks := slideWindow(text, 512, 64)
	if len(chunks) < 2 {
		t.Fatalf("expected >=2 chunks for 2000-char text, got %d", len(chunks))
	}
}

func TestSlideWindow_ShortText(t *testing.T) {
	chunks := slideWindow("short", 512, 64)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk for short text, got %d", len(chunks))
	}
}

func TestSlideWindow_Empty(t *testing.T) {
	chunks := slideWindow("", 512, 64)
	if len(chunks) != 0 {
		t.Fatalf("expected 0 chunks for empty text, got %d", len(chunks))
	}
}

// TestSlideWindow_OverlapExceedsTarget 验证 overlap >= target 不会死循环。
func TestSlideWindow_OverlapExceedsTarget(t *testing.T) {
	text := strings.Repeat("x", 1000)
	chunks := slideWindow(text, 100, 200)
	if len(chunks) == 0 {
		t.Fatal("expected non-zero chunks, got 0 (possible infinite loop)")
	}
}

func TestEstimateTokens_Mixed(t *testing.T) {
	tokens := estimateTokens("中文english混合text")
	if tokens <= 0 {
		t.Fatalf("expected positive token count for mixed text, got %d", tokens)
	}
	// 15 chars: 4 CJK + 11 ASCII，估算 ≈ 4/1.6 + 11/4 ≈ 5
	if tokens < 3 || tokens > 8 {
		t.Fatalf("expected 3-8 tokens for 15 mixed chars, got %d", tokens)
	}
}
