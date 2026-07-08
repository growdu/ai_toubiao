// Package illustrator 定义图表渲染接口与实现。
// 详见 docs/doc-gen/algorithms.md 第五节"图表生成与美化算法"。
package illustrator

import (
	"context"
	"fmt"

	"github.com/bidwriter/services/doc-gen/internal/core"
)

// Renderer 是所有图表渲染器的统一接口。
type Renderer interface {
	// Type 返回渲染器类型标识。
	Type() core.FigureType
	// Render 渲染图表规格为 Illustration。
	Render(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error)
}

// Illustrator 实现 core.Illustrator 接口，编排类型选择→渲染→美化→校验。
type Illustrator struct {
	renderers  map[core.FigureType]Renderer
	beautifier *Beautifier
	log        func(format string, args ...any)
}

// New 创建 Illustrator。
func New(renderers []Renderer, beautifier *Beautifier) *Illustrator {
	m := make(map[core.FigureType]Renderer)
	for _, r := range renderers {
		m[r.Type()] = r
	}
	if beautifier == nil {
		beautifier = &Beautifier{}
	}
	return &Illustrator{
		renderers:  m,
		beautifier: beautifier,
		log:        func(string, ...any) {},
	}
}

// SetLog 设置日志函数。
func (il *Illustrator) SetLog(fn func(format string, args ...any)) {
	il.log = fn
}

// Illustrate 渲染所有章节的图表。
func (il *Illustrator) Illustrate(ctx context.Context, chapters []core.Chapter, theme *core.Theme) ([]core.Illustration, error) {
	if theme == nil {
		theme = core.DefaultTheme()
	}

	var illustrations []core.Illustration

	for _, ch := range chapters {
		for _, spec := range ch.Spec.FigureSpecs {
			ill, err := il.renderOne(ctx, spec, theme)
			if err != nil {
				il.log("illustrator: 渲染失败，降级为占位图: %s (err: %v)", spec.Caption, err)
				ill = &core.Illustration{
					ID:           newUUID(),
					SpecID:       spec.ID,
					Status:       "placeholder",
					FallbackChain: "all_failed→placeholder",
				}
			}
			illustrations = append(illustrations, *ill)
		}
	}

	return illustrations, nil
}

// renderOne 渲染单个图表，含 fallback 链。
func (il *Illustrator) renderOne(ctx context.Context, spec core.FigureSpec, theme *core.Theme) (*core.Illustration, error) {
	renderer, ok := il.renderers[spec.Type]
	if !ok {
		return nil, fmt.Errorf("no renderer for type %s", spec.Type)
	}

	ill, err := renderer.Render(ctx, spec, theme)
	if err != nil {
		return nil, err
	}

	// 美化后处理（表格类型跳过位图美化）
	if spec.Type != core.FigureTable && ill.PNGBytes != nil && len(ill.PNGBytes) > 0 {
		il.beautifier.Beautify(ill, theme)
	}

	if ill.Status == "" {
		ill.Status = "ok"
	}

	return ill, nil
}
