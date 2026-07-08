// Package ingest 的文本提取与分块辅助函数。
package ingest

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"
)

// extractFromZipXML 从 zip 文件中提取指定 XML 条目，剥离标签返回纯文本。
// 用于 .docx 和 .xlsx 文本提取。
func extractFromZipXML(zipPath, entryName string) (string, error) {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", fmt.Errorf("open zip %s: %w", zipPath, err)
	}
	defer r.Close()

	var entry *zip.File
	for _, f := range r.File {
		if f.Name == entryName {
			entry = f
			break
		}
	}
	if entry == nil {
		return "", fmt.Errorf("entry %s not found in %s", entryName, zipPath)
	}

	rc, err := entry.Open()
	if err != nil {
		return "", fmt.Errorf("open entry %s: %w", entryName, err)
	}
	defer rc.Close()

	raw, err := io.ReadAll(rc)
	if err != nil {
		return "", fmt.Errorf("read entry %s: %w", entryName, err)
	}

	// 解析 XML，提取所有文本节点
	return extractXMLText(string(raw)), nil
}

// extractXMLText 从 XML 字符串中提取纯文本内容。
// 处理 <w:t>（docx 段落文本）和 <t>（xlsx 共享字符串）等标签。
func extractXMLText(xmlStr string) string {
	// 先用正则把 </w:p> 和 </row> 等替换为换行
	re := regexp.MustCompile(`</(?:w:p|row|tableRow|si)>`)
	xmlStr = re.ReplaceAllString(xmlStr, "\n")

	var sb strings.Builder
	decoder := xml.NewDecoder(strings.NewReader(xmlStr))
	for {
		tok, err := decoder.Token()
		if err != nil {
			break
		}
		switch v := tok.(type) {
		case xml.CharData:
			text := string(v)
			if strings.TrimSpace(text) != "" {
				sb.WriteString(text)
			}
		}
	}
	return sb.String()
}

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

	// 按双换行（段落）分割
	paragraphs := strings.Split(text, "\n\n")
	if len(paragraphs) <= 1 {
		// 尝试按单换行分割
		paragraphs = strings.Split(text, "\n")
	}

	var chunks []string
	var current strings.Builder
	currentTokens := 0

	flush := func() {
		if current.Len() > 0 {
			text := strings.TrimSpace(current.String())
			if text != "" {
				chunks = append(chunks, text)
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

		// 如果段落本身超长，按滑窗切
		if paraTokens > targetTokens {
			flush()
			windowChunks := slideWindow(para, targetTokens, overlap)
			chunks = append(chunks, windowChunks...)
			continue
		}

		// 如果加入后超限，先 flush
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

	// 合并过短的块（< 128 tokens 向后合并）
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
	// 按字符数估算（中文 1 token ≈ 1.6 字，英文 1 token ≈ 4 字符）
	// 取保守值：1 token ≈ 2 字符
	targetChars := targetTokens * 2
	overlapChars := overlap * 2
	if targetChars <= 0 {
		targetChars = 1024
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
	// 检测中文字符比例
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
	// 加权估算
	return int(float64(charCount) * (cjkRatio/1.6 + (1-cjkRatio)/4))
}
