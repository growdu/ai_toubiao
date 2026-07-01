package api

import (
	"bytes"
	"strings"
	"testing"
)

// BenchmarkOoxmlBuilder_DefaultOutline measures DOCX build cost for the
// "small" bid (3 chapters) — typical user export shape.
//
// This is the hot path for /export/word and the /export (POST) endpoint.
// If the unioffice / gooxml migration happens, this benchmark becomes
// the regression gate.
func BenchmarkOoxmlBuilder_DefaultOutline(b *testing.B) {
	chapters := []ChapterData{
		{Title: "项目背景与目标", Content: strings.Repeat("这是项目背景描述。", 50)},
		{Title: "技术方案", Content: strings.Repeat("技术方案详细说明。", 80)},
		{Title: "实施计划", Content: strings.Repeat("实施计划时间表。", 40)},
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := (ooxmlBuilder{}).Build(&buf, "测试标书", chapters); err != nil {
			b.Fatalf("build: %v", err)
		}
	}
}

// BenchmarkOoxmlBuilder_LargeBid stresses the same builder with a more
// realistic large bid (10 chapters × ~500 chars). Helps catch quadratic
// behaviour in the docx/relationships code.
func BenchmarkOoxmlBuilder_LargeBid(b *testing.B) {
	chapters := make([]ChapterData, 10)
	for i := range chapters {
		chapters[i] = ChapterData{
			Title:   "第" + itoa(i+1) + "章",
			Content: strings.Repeat("内容段落。", 50),
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		if err := (ooxmlBuilder{}).Build(&buf, "大型标书", chapters); err != nil {
			b.Fatalf("build: %v", err)
		}
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}