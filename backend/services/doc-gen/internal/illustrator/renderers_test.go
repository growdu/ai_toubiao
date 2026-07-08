package illustrator

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/bidwriter/services/doc-gen/internal/core"
	"github.com/google/uuid"
)

func TestTableRenderer(t *testing.T) {
	r := &TableRenderer{}
	theme := core.DefaultTheme()

	spec := core.FigureSpec{
		ID:     uuid.New(),
		Type:   core.FigureTable,
		Source: "| 项目 | 金额 |\n|---|---|\n| 硬件 | 100万 |\n| 软件 | 200万 |",
		Caption: "报价表",
	}

	ill, err := r.Render(context.Background(), spec, theme)
	if err != nil {
		t.Fatalf("TableRenderer.Render: %v", err)
	}
	if ill.OOXML == "" {
		t.Fatal("expected non-empty OOXML")
	}
	if ill.Status != "ok" {
		t.Fatalf("expected status 'ok', got %q", ill.Status)
	}
	// 验证 OOXML 包含表格标签
	if !contains(ill.OOXML, "<w:tbl>") {
		t.Fatal("expected OOXML to contain <w:tbl>")
	}
}

func TestTableRenderer_JSON(t *testing.T) {
	r := &TableRenderer{}
	theme := core.DefaultTheme()

	spec := core.FigureSpec{
		ID:     uuid.New(),
		Type:   core.FigureTable,
		Source: `[["项目","金额"],["硬件","100万"],["软件","200万"]]`,
		Caption: "报价表",
	}

	ill, err := r.Render(context.Background(), spec, theme)
	if err != nil {
		t.Fatalf("TableRenderer.Render JSON: %v", err)
	}
	if ill.OOXML == "" {
		t.Fatal("expected non-empty OOXML for JSON table")
	}
}

func TestMermaidRenderer(t *testing.T) {
	// 跳过如果没有 mmdc
	mmdcPath := "mmdc"
	if _, err := exec.LookPath(mmdcPath); err != nil {
		t.Skip("mmdc not installed, skipping")
	}

	// 创建 puppeteer 配置
	ppConfig := "/tmp/test_puppeteer.json"
	os.WriteFile(ppConfig, []byte(`{"args":["--no-sandbox","--disable-setuid-sandbox"]}`), 0644)

	r := &MermaidRenderer{
		MmdcPath:        mmdcPath,
		PuppeteerConfig: ppConfig,
	}
	theme := core.DefaultTheme()

	spec := core.FigureSpec{
		ID:     uuid.New(),
		Type:   core.FigureMermaid,
		Source: "graph TD\n    A[开始] --> B[处理]\n    B --> C[结束]",
		Caption: "流程图",
	}

	ill, err := r.Render(context.Background(), spec, theme)
	if err != nil {
		t.Fatalf("MermaidRenderer.Render: %v", err)
	}
	if len(ill.PNGBytes) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}
	if ill.RenderEngine != "mmdc" {
		t.Fatalf("expected engine 'mmdc', got %q", ill.RenderEngine)
	}
}

func TestDataChartRenderer(t *testing.T) {
	// 跳过如果没有 python3
	pythonPath := "python3"
	if _, err := exec.LookPath(pythonPath); err != nil {
		t.Skip("python3 not installed, skipping")
	}

	r := &DataChartRenderer{
		PythonPath: pythonPath,
	}
	theme := core.DefaultTheme()

	spec := core.FigureSpec{
		ID:     uuid.New(),
		Type:   core.FigureDataChart,
		Source: `{"labels":["Q1","Q2","Q3","Q4"],"values":[100,200,150,300]}`,
		Caption: "季度业绩",
	}

	ill, err := r.Render(context.Background(), spec, theme)
	if err != nil {
		t.Fatalf("DataChartRenderer.Render: %v", err)
	}
	if len(ill.PNGBytes) == 0 {
		t.Fatal("expected non-empty PNG bytes")
	}
	if ill.RenderEngine != "matplotlib" {
		t.Fatalf("expected engine 'matplotlib', got %q", ill.RenderEngine)
	}
}

func TestBeautifier(t *testing.T) {
	b := &Beautifier{}
	theme := core.DefaultTheme()

	// 创建一个简单的 PNG 图片（1x1 红色像素）
	pngData := createTestPNG()
	ill := &core.Illustration{
		PNGBytes: pngData,
	}

	b.Beautify(ill, theme)
	// 美化后应该仍有 PNG 数据
	if len(ill.PNGBytes) == 0 {
		t.Fatal("expected non-empty PNG after beautify")
	}
	if ill.WidthPx <= 0 {
		t.Fatal("expected positive width after beautify")
	}
}

func TestIllustrator_FallbackChain(t *testing.T) {
	// 测试：没有渲染器时应降级为占位图
	il := New(nil, &Beautifier{})
	chapters := []core.Chapter{
		{Spec: core.ChapterSpec{
			FigureSpecs: []core.FigureSpec{
				{ID: uuid.New(), Type: core.FigureMermaid, Caption: "测试图"},
			},
		}},
	}
	figs, err := il.Illustrate(context.Background(), chapters, core.DefaultTheme())
	if err != nil {
		t.Fatalf("Illustrate: %v", err)
	}
	if len(figs) != 1 {
		t.Fatalf("expected 1 figure, got %d", len(figs))
	}
	if figs[0].Status != "placeholder" {
		t.Fatalf("expected status 'placeholder', got %q", figs[0].Status)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
