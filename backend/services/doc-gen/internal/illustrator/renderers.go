// Package illustrator 的四类渲染器实现 + 美化层。
package illustrator

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/bidwriter/services/doc-gen/internal/llm"
	"github.com/google/uuid"
)

func newUUID() uuid.UUID { return uuid.New() }

// ---- Mermaid 渲染器 ----

// MermaidRenderer 用 mmdc (mermaid-cli) 渲染流程图。
type MermaidRenderer struct {
	MmdcPath string // mmdc 命令路径，默认 "mmdc"
	PuppeteerConfig string // puppeteer 配置路径（禁用 sandbox 等）
	LLM      llm.Client
}

func (r *MermaidRenderer) Type() core.FigureType { return core.FigureMermaid }

func (r *MermaidRenderer) Render(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error) {
	mmdcPath := r.MmdcPath
	if mmdcPath == "" {
		mmdcPath = "mmdc"
	}

	// 如果没有 mermaid 源码，用 LLM 生成
	src := spec.Source
	if src == "" && r.LLM != nil {
		var err error
		src, err = r.generateMermaidSource(ctx, spec.Caption, theme)
		if err != nil {
			return nil, fmt.Errorf("mermaid generate: %w", err)
		}
	}
	if src == "" {
		return nil, fmt.Errorf("mermaid: no source and LLM unavailable")
	}

	// 应用主题
	src = applyMermaidTheme(src, theme)

	// 写临时文件
	tmpDir := os.TempDir()
	mmdFile := filepath.Join(tmpDir, fmt.Sprintf("bidgen_%s.mmd", uuid.New().String()))
	pngFile := filepath.Join(tmpDir, strings.Replace(filepath.Base(mmdFile), ".mmd", ".png", 1))

	if err := os.WriteFile(mmdFile, []byte(src), 0644); err != nil {
		return nil, fmt.Errorf("mermaid: write mmd: %w", err)
	}
	defer os.Remove(mmdFile)
	defer os.Remove(pngFile)

	// shell-out mmdc
	args := []string{"-i", mmdFile, "-o", pngFile,
			"-t", theme.Mermaid.Theme, "-b", "white", "-w", fmt.Sprintf("%d", theme.Chart.WidthPx)}
		if r.PuppeteerConfig != "" {
			args = append(args, "-p", r.PuppeteerConfig)
		}
		cmd := exec.CommandContext(ctx, mmdcPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("mermaid: mmdc failed: %w (output: %s)", err, string(output))
	}

	pngBytes, err := os.ReadFile(pngFile)
	if err != nil {
		return nil, fmt.Errorf("mermaid: read png: %w", err)
	}

	return &core.Illustration{
		ID:           uuid.New(),
		SpecID:       spec.ID,
		PNGBytes:     pngBytes,
		RenderEngine: "mmdc",
		WidthPx:      theme.Chart.WidthPx,
		Status:       "ok",
	}, nil
}

// generateMermaidSource 用 LLM 生成 mermaid 源码。
func (r *MermaidRenderer) generateMermaidSource(ctx context.Context, caption string, theme *core.Theme) (string, error) {
	resp, err := r.LLM.Chat(ctx, &core.LLMRequest{
		Task: "mermaid_generate",
		Messages: []core.Message{
			{Role: "system", Content: "你是图表专家，只返回合法的 Mermaid 语法源码，不要 markdown 代码块标记。"},
			{Role: "user", Content: fmt.Sprintf("请生成一个 mermaid 流程图，主题：%s。只返回 mermaid 源码。", caption)},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return "", err
	}
	src := strings.TrimSpace(resp.Content)
	// 去除可能的 markdown 代码块标记
	src = strings.TrimPrefix(src, "```mermaid")
	src = strings.TrimPrefix(src, "```")
	src = strings.TrimSuffix(src, "```")
	return strings.TrimSpace(src), nil
}

// applyMermaidTheme 注入主题变量到 mermaid 源码。
func applyMermaidTheme(src string, theme *core.Theme) string {
	// 在源码开头注入 themeVariables（如果有的话）
	// mermaid 的 themeVariables 通过 frontmatter 注入
	if len(theme.Mermaid.ThemeVariables) == 0 {
		return src
	}
	var sb strings.Builder
	sb.WriteString("---\n")
	for k, v := range theme.Mermaid.ThemeVariables {
		sb.WriteString(fmt.Sprintf("%s: %s\n", k, v))
	}
	sb.WriteString("---\n")
	sb.WriteString(src)
	return sb.String()
}

// ---- 数据图表渲染器 ----

// DataChartRenderer 用 python matplotlib 渲染数据图表。
type DataChartRenderer struct {
	PythonPath string
	LLM        llm.Client
}

func (r *DataChartRenderer) Type() core.FigureType { return core.FigureDataChart }

func (r *DataChartRenderer) Render(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error) {
	pythonPath := r.PythonPath
	if pythonPath == "" {
		pythonPath = "python3"
	}

	// 如果有数据 JSON 作为 source，生成 matplotlib 脚本
	data := spec.Source
	if data == "" {
		return nil, fmt.Errorf("data_chart: no data source")
	}

	// 生成 Python 脚本
	palette, _ := json.Marshal(theme.Palette)
	script := fmt.Sprintf(`
import sys, json
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt

data = json.loads(%q)
palette = %s
plt.rcParams['font.sans-serif'] = ['%s', 'DejaVu Sans']
plt.rcParams['axes.unicode_minus'] = False
plt.figure(figsize=(%g, %g), dpi=%d)
colors = palette
if 'series' in data:
    for i, s in enumerate(data['series']):
        plt.plot(s.get('x', range(len(s.get('y',[])))), s['y'], color=colors[i%%len(colors)], label=s.get('name',''))
    if 'labels' in data:
        plt.xticks(range(len(data['labels'])), data['labels'])
    plt.legend()
elif 'labels' in data and 'values' in data:
    plt.bar(data['labels'], data['values'], color=colors[:len(data['values'])])
plt.title(%q, fontsize=14)
plt.grid(linestyle='%s', alpha=0.3)
plt.tight_layout()
buf = sys.stdout.buffer
plt.savefig(buf, format='png', dpi=%d)
plt.close()
`, data, string(palette), theme.Font.Family, theme.Chart.FigureSize[0], theme.Chart.FigureSize[1], theme.Chart.DPI,
		spec.Caption, theme.Chart.GridStyle, theme.Chart.DPI)

	cmd := exec.CommandContext(ctx, pythonPath, "-c", script)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("data_chart: python failed: %w (output: %s)", err, out.String())
	}

	if out.Len() == 0 {
		return nil, fmt.Errorf("data_chart: no output from python")
	}

	return &core.Illustration{
		ID:           uuid.New(),
		SpecID:       spec.ID,
		PNGBytes:     out.Bytes(),
		RenderEngine: "matplotlib",
		WidthPx:      theme.Chart.WidthPx,
		Status:       "ok",
	}, nil
}

// ---- AI 配图渲染器 ----

// AIImageRenderer 通过 LLM 生成 AI 配图（DALL·E 等）。
type AIImageRenderer struct {
	LLM llm.Client
}

func (r *AIImageRenderer) Type() core.FigureType { return core.FigureAIImage }

func (r *AIImageRenderer) Render(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error) {
	if r.LLM == nil {
		return nil, fmt.Errorf("ai_image: no LLM client")
	}

	// 生成图片 prompt
	prompt := spec.Source
	if prompt == "" {
		prompt = spec.Caption
	}

	// 用 LLM 生成图片（通过 chat 接口返回 base64）
	resp, err := r.LLM.Chat(ctx, &core.LLMRequest{
		Task: "image_generate",
		Messages: []core.Message{
			{Role: "system", Content: "你是一个图片生成助手。请生成与描述匹配的配图。"},
			{Role: "user", Content: prompt},
		},
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("ai_image: llm failed: %w", err)
	}

	// 尝试解析 base64 图片
	content := strings.TrimSpace(resp.Content)
	// 去除可能的 data URI 前缀
	if idx := strings.Index(content, "base64,"); idx >= 0 {
		content = content[idx+7:]
	}
	// 去除 markdown 标记
	content = strings.TrimPrefix(content, "![")
	if idx := strings.Index(content, "]("); idx >= 0 {
		content = content[idx+2:]
		content = strings.TrimSuffix(content, ")")
	}

	pngBytes, err := base64.StdEncoding.DecodeString(content)
	if err != nil || len(pngBytes) < 100 {
		// 不是 base64 图片，生成占位图
		return &core.Illustration{
			ID:           uuid.New(),
			SpecID:       spec.ID,
			RenderEngine: "ai_image",
			Status:       "placeholder",
			FallbackChain: "no_image_data→placeholder",
		}, nil
	}

	return &core.Illustration{
		ID:           uuid.New(),
		SpecID:       spec.ID,
		PNGBytes:     pngBytes,
		RenderEngine: "ai_image",
		WidthPx:      theme.Chart.WidthPx,
		Status:       "ok",
	}, nil
}

// ---- 表格渲染器 ----

// TableRenderer 生成原生 OOXML 表格（矢量、可编辑）。
type TableRenderer struct{}

func (r *TableRenderer) Type() core.FigureType { return core.FigureTable }

func (r *TableRenderer) Render(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error) {
	// 解析表格数据（Markdown 表格或 JSON）
	rows := parseTableData(spec.Source)
	if len(rows) == 0 {
		return nil, fmt.Errorf("table: no data")
	}

	// 生成 OOXML 表格 XML
	xml := buildOOXMLTable(rows, theme)

	return &core.Illustration{
		ID:           uuid.New(),
		SpecID:       spec.ID,
		OOXML:        xml,
		RenderEngine: "ooxml_table",
		Status:       "ok",
	}, nil
}

// parseTableData 从 Markdown 表格或 JSON 解析行数据。
func parseTableData(source string) [][]string {
	source = strings.TrimSpace(source)
	if source == "" {
		return nil
	}

	// 尝试 JSON
	if strings.HasPrefix(source, "[") {
		var rows [][]string
		if err := json.Unmarshal([]byte(source), &rows); err == nil {
			return rows
		}
		var objs []map[string]any
		if err := json.Unmarshal([]byte(source), &objs); err == nil && len(objs) > 0 {
			// 提取表头
			headers := make([]string, 0, len(objs[0]))
			for k := range objs[0] {
				headers = append(headers, k)
			}
			rows = append(rows, headers)
			for _, obj := range objs {
				row := make([]string, len(headers))
				for i, h := range headers {
					if v, ok := obj[h]; ok {
						row[i] = fmt.Sprintf("%v", v)
					}
				}
				rows = append(rows, row)
			}
			return rows
		}
	}

	// 解析 Markdown 表格
	lines := strings.Split(source, "\n")
	var rows [][]string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "|--") || strings.HasPrefix(line, "| -") {
			continue
		}
		if !strings.Contains(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		rows = append(rows, cells)
	}
	return rows
}

// buildOOXMLTable 构建 OOXML 表格 XML。
func buildOOXMLTable(rows [][]string, theme *core.Theme) string {
	var sb strings.Builder
	headerFill := theme.Table.HeaderFill
	if headerFill == "" {
		headerFill = "1F4E79"
	}
	headerColor := theme.Table.HeaderFontColor
	if headerColor == "" {
		headerColor = "FFFFFF"
	}

	sb.WriteString(`<w:tbl>`)
	sb.WriteString(`<w:tblPr><w:tblW w:w="5000" w:type="pct"/>`)
	if theme.Table.Zebra {
		sb.WriteString(`<w:tblLook w:val="04A0" w:firstRow="1" w:lastRow="0" w:firstColumn="1" w:lastColumn="0" w:noHBand="0" w:vBand="1"/>`)
	}
	sb.WriteString(`</w:tblPr>`)

	for i, row := range rows {
		sb.WriteString(`<w:tr>`)
		for _, cell := range row {
			sb.WriteString(`<w:tc><w:tcPr>`)
			if i == 0 {
				// 表头样式
				sb.WriteString(fmt.Sprintf(`<w:shd w:val="clear" w:color="auto" w:fill="%s"/>`, headerFill))
			} else if theme.Table.Zebra && i%2 == 0 {
				sb.WriteString(`<w:shd w:val="clear" w:color="auto" w:fill="F2F2F2"/>`)
			}
			sb.WriteString(`</w:tcPr>`)
			sb.WriteString(fmt.Sprintf(`<w:p><w:r><w:rPr>`, ))
			if i == 0 {
				sb.WriteString(fmt.Sprintf(`<w:color w:val="%s"/><w:b/>`, headerColor))
			}
			sb.WriteString(fmt.Sprintf(`<w:sz w:val="20"/></w:rPr><w:t>%s</w:t></w:r></w:p>`, escapeXML(cell)))
			sb.WriteString(`</w:tc>`)
		}
		sb.WriteString(`</w:tr>`)
	}
	sb.WriteString(`</w:tbl>`)
	return sb.String()
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}

// ---- 美化层 ----

// ---- 美化层 ----

// Beautifier 是引擎无关的主题后处理器。
type Beautifier struct{}

// Beautify 对渲染产物做主题后处理：尺寸归一 + 白底合成。
// 保证图片在 Word 中不出现透明黑底问题。
func (b *Beautifier) Beautify(ill *core.Illustration, theme *core.Theme) {
	if ill == nil || len(ill.PNGBytes) == 0 {
		return
	}

	// 解码图片
	img, err := png.Decode(bytes.NewReader(ill.PNGBytes))
	if err != nil {
		return // 解码失败跳过美化
	}

	bounds := img.Bounds()
	maxWidth := 1600
	if theme != nil && theme.Chart.WidthPx > 0 {
		maxWidth = theme.Chart.WidthPx
	}

	// 白底合成：创建白色背景，将原图绘制上去
	rgba := image.NewRGBA(bounds)
	// 先填充白色
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			rgba.Set(x, y, color.White)
		}
	}
	// 再叠加原图（保留 alpha 混合）
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			c := img.At(x, y)
			_, _, _, a := c.RGBA()
			if a > 0 {
				rgba.Set(x, y, c)
			}
		}
	}

	// 记录宽度
	if bounds.Dx() > maxWidth {
		ill.WidthPx = maxWidth
	} else {
		ill.WidthPx = bounds.Dx()
	}

	// 重新编码
	var buf bytes.Buffer
	if err := png.Encode(&buf, rgba); err == nil {
		ill.PNGBytes = buf.Bytes()
	}
}
