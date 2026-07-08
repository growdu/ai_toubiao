// Package assembler 实现文档组装：Markdown→OOXML + 图表嵌入 + 主题样式 → .docx。
// 详见 docs/doc-gen/algorithms.md 第七节"文档组装算法"。
package assembler

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

// Assembler 实现 core.Assembler 接口。
type Assembler struct {
	Log *slog.Logger
}

// New 创建 Assembler。
func New(log *slog.Logger) *Assembler {
	return &Assembler{Log: log}
}

// Assemble 将标书包组装为 .docx 文件。
func (a *Assembler) Assemble(ctx context.Context, pkg *core.BidPackage, theme *core.Theme) (string, error) {
	log := a.Log
	if log == nil {
		log = slog.Default()
	}

	outPath := pkg.OutputPath
	if outPath == "" {
		outPath = fmt.Sprintf("标书_%s.docx", time.Now().Format("20060102_150405"))
	}
	// 确保目录存在
	dir := filepath.Dir(outPath)
	if dir != "" && dir != "." {
		os.MkdirAll(dir, 0755)
	}

	// 构建 OOXML 文档内容
	docXML := a.buildDocumentXML(pkg, theme)

	// 构建图片映射
	images := a.collectImages(pkg)

	// 生成 .docx (ZIP)
	if err := writeDOCX(outPath, docXML, images); err != nil {
		return "", fmt.Errorf("assemble: write docx: %w", err)
	}

	log.Info("assembler: 完成", "output", outPath, "chapters", len(pkg.Chapters), "figures", len(images))
	return outPath, nil
}

// collectImages 收集所有渲染成功的图片。
func (a *Assembler) collectImages(pkg *core.BidPackage) map[string][]byte {
	images := make(map[string][]byte)
	for _, fig := range pkg.Figures {
		if fig.Status == "ok" && len(fig.PNGBytes) > 0 {
			name := fmt.Sprintf("image_%s.png", fig.SpecID.String())
			images[name] = fig.PNGBytes
		}
	}
	return images
}

// buildDocumentXML 构建 word/document.xml 的完整内容。
func (a *Assembler) buildDocumentXML(pkg *core.BidPackage, theme *core.Theme) string {
	var sb strings.Builder

	sb.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	sb.WriteString(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">`)
	sb.WriteString(`<w:body>`)

	// 封面
	sb.WriteString(a.titlePageXML(pkg))

	// 各章节
	imageIdx := 0
	for _, ch := range pkg.Chapters {
		sb.WriteString(a.chapterXML(ch, theme, pkg, &imageIdx))
	}

	// 文档结尾标记
	sb.WriteString(`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440"/></w:sectPr>`)
	sb.WriteString(`</w:body></w:document>`)

	return sb.String()
}

// titlePageXML 生成封面。
func (a *Assembler) titlePageXML(pkg *core.BidPackage) string {
	var sb strings.Builder
	title := "投标文件"
	sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="52"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, escapeXML(title)))
	sb.WriteString(`<w:p><w:pPr><w:jc w:val="center"/><w:spacing w:before="2400"/></w:pPr><w:r><w:rPr><w:sz w:val="28"/></w:rPr><w:t> </w:t></w:r></w:p>`)
	sb.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:br w:type="page"/></w:r></w:p>`)
	return sb.String()
}

// chapterXML 将一个章节的 Markdown 转为 OOXML。
func (a *Assembler) chapterXML(ch core.Chapter, theme *core.Theme, pkg *core.BidPackage, imageIdx *int) string {
	var sb strings.Builder
	md := ch.Content.Markdown

	// 章节标题
	level := ch.Spec.Level
	if level == 0 {
		level = 1
	}
	headingSize := 36 - (level-1)*4
	if headingSize < 24 {
		headingSize = 24
	}
	sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr><w:r><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, headingSize, escapeXML(ch.Spec.Title)))

	// 逐行处理 Markdown
	lines := strings.Split(md, "\n")
	// 跳过第一个标题行（如果与章节标题重复）
	startIdx := 0
	if len(lines) > 0 {
		firstLine := strings.TrimSpace(strings.TrimLeft(strings.TrimLeft(lines[0], "#"), " "))
		if firstLine == ch.Spec.Title || strings.HasPrefix(lines[0], "## "+ch.Spec.Title) {
			startIdx = 1
		}
	}

	for i := startIdx; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// 跳过空行
		if trimmed == "" {
			continue
		}

		// 图片占位符 [!figure:type caption=...]
		if strings.HasPrefix(trimmed, "[!figure:") {
			sb.WriteString(a.figurePlaceholderXML(trimmed, pkg, imageIdx))
			continue
		}

		// Markdown 标题
		if strings.HasPrefix(trimmed, "##") {
			hLevel := 2
			temp := trimmed
			for strings.HasPrefix(temp, "#") {
				hLevel++
				temp = temp[1:]
			}
			hText := strings.TrimSpace(temp)
			hSize := 36 - (hLevel-1)*4
			if hSize < 20 {
				hSize = 20
			}
			sb.WriteString(fmt.Sprintf(`<w:p><w:r><w:rPr><w:b/><w:sz w:val="%d"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, hSize, escapeXML(hText)))
			continue
		}

		// 列表项
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			text := strings.TrimSpace(trimmed[2:])
			sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:ind w:left="420"/></w:pPr><w:r><w:t>• %s</w:t></w:r></w:p>`, escapeXML(text)))
			continue
		}

		// 数字列表
		if matched, _ := regexp.MatchString(`^\d+\.\s`, trimmed); matched {
			text := regexp.MustCompile(`^\d+\.\s`).ReplaceAllString(trimmed, "")
			sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:ind w:left="420"/></w:pPr><w:r><w:t>%s</w:t></w:r></w:p>`, escapeXML(text)))
			continue
		}

		// 表格（简单的 Markdown 表格）
		if strings.Contains(trimmed, "|") && strings.Count(trimmed, "|") >= 2 {
			// 简单处理：作为段落输出
			sb.WriteString(fmt.Sprintf(`<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, escapeXML(trimmed)))
			continue
		}

		// 普通段落
		sb.WriteString(fmt.Sprintf(`<w:p><w:r><w:t xml:space="preserve">%s</w:t></w:r></w:p>`, escapeXML(trimmed)))
	}

	return sb.String()
}

// figurePlaceholderXML 处理图表占位符，嵌入图片或表格。
func (a *Assembler) figurePlaceholderXML(placeholder string, pkg *core.BidPackage, imageIdx *int) string {
	// 解析 [!figure:type caption=xxx]
	re := regexp.MustCompile(`\[!figure:(\w+)\s+caption=(.+?)\]`)
	matches := re.FindStringSubmatch(placeholder)
	if len(matches) < 3 {
		return fmt.Sprintf(`<w:p><w:r><w:t>%s</w:t></w:r></w:p>`, escapeXML(placeholder))
	}
	figType := matches[1]
	caption := matches[2]

	// 查找对应的渲染结果
	for _, fig := range pkg.Figures {
		if fig.Status != "ok" {
			continue
		}
		// 如果是表格类型，插入 OOXML 表格
		if fig.OOXML != "" && figType == "table" {
			var sb strings.Builder
			sb.WriteString(fig.OOXML)
			sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:sz w:val="18"/><w:i/></w:rPr><w:t>%s</w:t></w:r></w:p>`, escapeXML(caption)))
			return sb.String()
		}
		// 如果是图片，嵌入
		if len(fig.PNGBytes) > 0 {
			*imageIdx++
			imgName := fmt.Sprintf("image_%s.png", fig.SpecID.String())
			imgIdx := *imageIdx
			var sb strings.Builder
			sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="5274310" cy="2960820"/><wp:docPr id="%d" name="%s"/><a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"><wp:pic><wp:nvPicPr><wp:cNvPr id="%d" name="%s"/><wp:cNvPicPr/></wp:nvPicPr><wp:blipFill><a:blip r:embed="rId%d"/><a:stretch><a:fillRect/></a:stretch></wp:blipFill><a:srcRect/></wp:pic></a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>`, imgIdx, escapeXML(imgName), imgIdx, escapeXML(imgName), imgIdx+10))
			sb.WriteString(fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:sz w:val="18"/><w:i/></w:rPr><w:t>%s</w:t></w:r></w:p>`, escapeXML(caption)))
			return sb.String()
		}
	}

	// 未找到渲染结果，输出占位文字
	return fmt.Sprintf(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:rPr><w:color w:val="999999"/><w:i/></w:rPr><w:t>[图表占位: %s]</w:t></w:r></w:p>`, escapeXML(caption))
}

// writeDOCX 将 OOXML 内容和图片写入 .docx (ZIP) 文件。
func writeDOCX(path string, docXML string, images map[string][]byte) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

	// [Content_Types].xml
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Default Extension="png" ContentType="image/png"/>
<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>
</Types>`
	writeZipFile(w, "[Content_Types].xml", contentTypes)

	// _rels/.rels
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`
	writeZipFile(w, "_rels/.rels", rels)

	// word/_rels/document.xml.rels
	var relSB strings.Builder
	relSB.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n")
	relSB.WriteString(`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`)
	relSB.WriteString(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>`)
	idx := 10
	for imgName := range images {
		idx++
		relSB.WriteString(fmt.Sprintf(`<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/%s"/>`, idx, imgName))
	}
	relSB.WriteString(`</Relationships>`)
	writeZipFile(w, "word/_rels/document.xml.rels", relSB.String())

	// word/document.xml
	writeZipFile(w, "word/document.xml", docXML)

	// word/styles.xml
	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="微软雅黑" w:eastAsia="微软雅黑" w:hAnsi="微软雅黑"/><w:sz w:val="21"/></w:rPr></w:rPrDefault></w:docDefaults>
</w:styles>`
	writeZipFile(w, "word/styles.xml", styles)

	// 图片文件
	for imgName, data := range images {
		writeZipFileBinary(w, "word/media/"+imgName, data)
	}

	return nil
}

func writeZipFile(w *zip.Writer, name, content string) {
	f, err := w.Create(name)
	if err != nil {
		return
	}
	f.Write([]byte(content))
}

func writeZipFileBinary(w *zip.Writer, name string, data []byte) {
	f, err := w.Create(name)
	if err != nil {
		return
	}
	f.Write(data)
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// encodeBase64 工具函数。
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// newUUID 工具函数。
func newUUID() uuid.UUID { return uuid.New() }
