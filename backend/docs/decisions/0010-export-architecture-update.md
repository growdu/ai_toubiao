# 0010. 标书 Word / PDF 导出架构(更新 0004)

## 状态

Accepted (supersedes 部分 0004 库选择段落;不改变 .docx 为 v1 主交付物)

## 日期

2026-06-30

## 参与者

- 架构组
- 后端组

## 背景

0004 (2026-06-27) 选定了 v1 主交付物是 .docx,并列出主备库候选:`nguyenthenguyen/docx` 与 `unidoc/unioffice`。当前 MVP 实现 (`workflow-svc/internal/api/export.go`) 放弃了库,直接用 `archive/zip + encoding/xml` 手写 OOXML,理由:无依赖 + 约 200 行代码可覆盖最小章节列表。但偏离了 0004 的库选,需要补一个 ADR 写清楚:

1. 为什么 MVP 暂时手写
2. 何时升级到库
3. PDF 走 LibreOffice headless 还是其它
4. 整体架构如何让 Phase 2 替换不撕裂调用方

## 决策

### Word (.docx) 路径 — 阶段性

| 阶段 | 实现 | 触发条件 |
|---|---|---|
| **Phase 1 (现在)** | 手写 zip + OOXML,封装在 `DocBuilder` 接口后 | 章节 ≤ 10,无表格/图片,无章节样式控制 |
| **Phase 2 (M3+)** | 引入 [`unidoc/unioffice`](https://github.com/unidoc/unioffice) (商用 license);或自维护的开源 fork | 客户反馈需要表格/页眉页脚/字体嵌入 |

`DocBuilder` 接口(新增)让两个阶段调用方一致:

```go
type DocBuilder interface {
    AddHeading(level int, text string) error
    AddParagraph(text string) error
    AddTable(headers []string, rows [][]string) error
    AddImage(bytes []byte, format string) error
    Bytes() ([]byte, error)
}
```

Phase 1 实现名 `StdlibOOXMLBuilder`,Phase 2 实现名 `UniofficeBuilder`。`exportWordHandler` 接收一个由配置切换的 builder (默认 stdlib)。

### PDF 路径

走 **LibreOffice headless** (`soffice --headless --convert-to pdf`),通过 `os/exec` + `exec.CommandContext` 启动,带 60 秒超时与临时文件清理。失败的"LibreOffice 不在"或"转 pdf 失败"降级回到 .docx 字节流并打 slog warning。

```go
func convertToPDF(ctx context.Context, docx []byte) ([]byte, error) {
    dir, _ := os.MkdirTemp("", "lo-")
    defer os.RemoveAll(dir)
    in := filepath.Join(dir, "in.docx")
    out := filepath.Join(dir, "in.pdf")
    os.WriteFile(in, docx, 0644)

    ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, "soffice", "--headless", "--convert-to", "pdf",
        "--outdir", dir, in)
    if err := cmd.Run(); err != nil { return nil, err }
    return os.ReadFile(out)
}
```

### 调用方契约

`GET /api/v1/bids/{id}/export/{word,pdf}` 与 `POST /api/v1/bids/{id}/export` 的 HTTP 形态不变。Word/PDF 走 `DocBuilder` + (PDF only) `convertToPDF`。前端 ExportPage 维持现状 (axios blob)。

## 理由

- ✅ DocBuilder 接口让 Phase 2 替换是 0 调用方改动
- ✅ LibreOffice 是开源 + 工业级 PDF 输出,比 pandoc + weasyprint 都更接近 Word 排版
- ✅ 失败降级到 .docx 保证用户体验不会因为 PDF 转换失败而整页报错
- ❌ 引入 LibreOffice binary 依赖,使部署 docker image 多 ~ 600 MB;但 as a worker 部署又是隔离的,所以可接受

## 考虑的替代方案

### 方案 A：直接上 unioffice,PDF 用 unioffice 自带转换

- ❌ unioffice 的 PDF 转换 license 是单独 **commercial** 的,独立收费
- ❌ 当前 phase 项目还没拿到商业 license
- ❌ 不写 DocBuilder 接口,后期替换很疼

### 方案 B：pandoc (Markdown→PDF) + docx-template (Go) (选择 DOCX 路径, 备 LibreOffice)

- ❌ 中文 word 字体配置繁琐,样式能力薄弱
- ❌ 维护成本 高于手写 zip

### 方案 C：手写 zip+XML + LibreOffice headless(选择)

- ✅ 当前可行,接口稳定
- ✅ PDF 输出与 Word 排版完全一致

### 方案 D：Chromedp / Puppeteer (HTML→PDF)

- ❌ 引入浏览器 / Node,镜像庞大
- ❌ 排版一致性差

## 后果

### 正面

- Phase 1 落地:LibreOffice 在/不在都不影响主路径
- 与 0004 不冲突,扩展而非替换
- DocBuilder 解耦 Phase 2 升级成本

### 负面

- LibreOffice 需要服务器预装;docker image 体积增大
- 手写 zip+XML 没有图片/表格支持 — 标书若需要这两样,必升 Phase 2
- Phase 2 路径需要 license 采购

### 中性

- `export.go` 重构:抽出 DocBuilder + `ConvertToPDF` helper
- 单测:测 stdlib builder 输出是合法 zip + document.xml 解析无错 + convertToPDF 在 soffice 缺席时返回特定 error
- 集成测:docker-compose 加 `soffice` 服务,gitHub Actions matrix 跑

## 实施细节

```go
// workflow-svc/internal/export/builder.go
type DocBuilder interface { /* 见上文 */ }
func NewBuilder() DocBuilder { return &StdlibOOXMLBuilder{} }

// workflow-svc/internal/export/pdf.go
func ConvertToPDF(ctx context.Context, docx []byte) ([]byte, error) { /* 见上文 */ }
```

## 退出条件

- 🔴 客户反馈需要表格/页眉/页脚 → 立即启动 Phase 2 unioffice 选型
- 🔴 LibreOffice 安全补丁跟不上 → 转 Chromium / weasyprint 评估
- 🔴 unioffice license 谈定 → 立即替换 builder 默认实现

## 参考

- [0004. Word 导出格式](./0004-word-format.md)
- [architecture / modules](../architecture/modules.md#workflow-svc)
