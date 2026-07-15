// Package ingest 的分块与噪声清洗辅助函数。
package ingest

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// cleanNoise 清洗提取文本中的目录域与页码噪声。
// 去除 Word 目录的 PAGEREF/TOC 域指令，压缩多余空行。
func cleanNoise(text string) string {
	text = rePAGEREF.ReplaceAllString(text, "")
	text = reTOCField.ReplaceAllString(text, "")
	text = reBlankLines.ReplaceAllString(text, "\n\n")
	return text
}

var (
	rePAGEREF    = regexp.MustCompile(`PAGEREF\s+_Toc\d+\s+\\h`)
	reTOCField   = regexp.MustCompile(`TOC\s+\\o.*?\\u`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// semanticChunk 将文本按语义切分为不超过 targetTokens 的块。
// 策略：优先按标题/空行切段；超长段落按字符滑窗；短段合并。
// 中文按 1 token ≈ 1.6 字估算。
func semanticChunk(text string, targetTokens, overlap int) []string {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	if targetTokens <= 0 {
		targetTokens = 512
	}
	if overlap < 0 {
		overlap = 0
	}

	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) <= 1 {
		paragraphs = strings.Split(text, "\n")
	}

	var chunks []string
	var current strings.Builder
	currentTokens := 0

	flush := func() {
		if current.Len() > 0 {
			t := strings.TrimSpace(current.String())
			if t != "" {
				chunks = append(chunks, t)
			}
			current.Reset()
			currentTokens = 0
		}
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		paraTokens := estimateTokens(para)

		if paraTokens > targetTokens {
			flush()
			windowChunks := slideWindow(para, targetTokens, overlap)
			chunks = append(chunks, windowChunks...)
			continue
		}

		if currentTokens+paraTokens > targetTokens && currentTokens > 0 {
			flush()
		}

		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(para)
		currentTokens += paraTokens
	}
	flush()

	merged := make([]string, 0, len(chunks))
	for _, c := range chunks {
		if len(merged) > 0 && estimateTokens(merged[len(merged)-1]) < 128 {
			merged[len(merged)-1] = merged[len(merged)-1] + "\n\n" + c
		} else {
			merged = append(merged, c)
		}
	}

	return merged
}

// slideWindow 对超长文本做字符级滑窗切分。
func slideWindow(text string, targetTokens, overlap int) []string {
	targetChars := targetTokens * 2
	overlapChars := overlap * 2
	if targetChars <= 0 {
		targetChars = 1024
	}
	// 防止 overlap >= target 导致步长 ≤0 死循环
	if overlapChars >= targetChars {
		overlapChars = targetChars / 2
	}

	runes := []rune(text)
	var chunks []string
	for i := 0; i < len(runes); i += targetChars - overlapChars {
		end := i + targetChars
		if end > len(runes) {
			end = len(runes)
		}
		chunk := strings.TrimSpace(string(runes[i:end]))
		if chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
	}
	return chunks
}

// estimateTokens 粗略估算文本的 token 数。
// 中文：1 token ≈ 1.6 字；英文：1 token ≈ 4 字符。
func estimateTokens(text string) int {
	charCount := utf8.RuneCountInString(text)
	cjk := 0
	for _, r := range text {
		if r >= 0x4E00 && r <= 0x9FFF {
			cjk++
		}
	}
	if charCount == 0 {
		return 0
	}
	cjkRatio := float64(cjk) / float64(charCount)
	return int(float64(charCount) * (cjkRatio/1.6 + (1-cjkRatio)/4))
}
