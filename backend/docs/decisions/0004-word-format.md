# 0004. Word 导出格式

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 架构组
- 前端组

## 背景

标书最终交付格式是 Word（.docx）。需要决策 v1 支持哪些格式。

**约束条件**：
- 国内投标市场事实标准（90%+ 招标文件要求 .docx）
- .docx 生态成熟（docx 库 + Word/WPS 都能打开）
- ODF 主要欧洲政府用，国内几乎无
- WPS 是 .docx 超集（自家格式基本是 .docx 改名）
- 不同客户的格式要求细节差异大（页边距、字体、字号）

**需要决策**：v1 支持哪些 Word 格式。

## 决策

**v1 仅支持 .docx（Office Open XML）；ODF/WPS 看 M3 反馈。**

借鉴 yibiao 的 exportService 设计：Markdown → AST → docx blocks。

## 理由

- ✅ .docx 是国内投标市场事实标准（90%+）
- ✅ 生态成熟（库 + 客户端）
- ✅ Mermaid 渲染、图片嵌入都有成熟方案
- ✅ 借鉴 yibiao 已验证的设计

## 考虑的替代方案

### 方案 A：支持 .docx + .pdf + .odf

- ❌ v1 工作量大
- ❌ 多种格式一致性维护成本高
- ❌ .pdf 排版固定（不利于微调）

### 方案 B：仅 .docx（**选择**）

- ✅ 实施简单
- ✅ 满足 90%+ 客户
- ✅ 借鉴 yibiao 经验

### 方案 C：仅 .pdf

- ❌ 国内不接受（标书要求可编辑）
- ❌ 排版固定（客户没法微调）

## 后果

### 正面

- v1 实施简单
- 排版问题少（.docx 可调整）
- 客户能继续在 Word/WPS 编辑

### 负面

- 不支持 .pdf 直接交付（部分客户需要）
- 不支持 ODF（极少数政府/事业单位）

### 中性（需要承担的工作）

- 用 Go 的 docx 库（github.com/nguyenthenguyen/docx 或类似）
- Markdown → AST → docx blocks
- Mermaid 渲染（在线/离线）
- 图片嵌入

## 实施细节

### 导出流程

```mermaid
flowchart LR
    A[项目数据] --> B[组装 Markdown]
    B --> C[解析 AST]
    C --> D[生成 docx blocks]
    D --> E[嵌入图片/Mermaid]
    E --> F[上传 S3]
    F --> G[返回下载链接]
```

### 库选择

- 主：[github.com/nguyenthenguyen/docx](https://github.com/nguyenthenguyen/docx)
- 备：[github.com/unidoc/unioffice](https://github.com/unidoc/unioffice)（商业许可更友好）

### AST → docx 映射

```go
type DocBlock interface {
    ToDocx(doc *docx.Document) error
}

type HeadingBlock struct {
    Level int
    Text  string
}

type ParagraphBlock struct {
    Text  string
    Style string
}

type ImageBlock struct {
    URL   string
    Alt   string
    Width int
}

type TableBlock struct {
    Headers []string
    Rows    [][]string
}

type MermaidBlock struct {
    Code string
}
```

### Mermaid 渲染

```go
func RenderMermaid(code string) ([]byte, error) {
    // 1. 优先用 mermaid.ink 在线
    encoded := base64.URLEncoding.EncodeToString([]byte(code))
    url := fmt.Sprintf("https://mermaid.ink/img/%s", encoded)

    // 2. 下载图片
    resp, err := http.Get(url)
    if err == nil && resp.StatusCode == 200 {
        return io.ReadAll(resp.Body)
    }

    // 3. 降级：本地 mmdc CLI（需要 Node.js）
    return renderMermaidLocal(code)
}
```

### 图片处理

- 小图（< 1MB）：嵌入 docx（base64）
- 大图（≥ 1MB）：外链（docx 里只存 URL）

### 模板配置

```yaml
# configs/export_templates.yaml
templates:
  default:
    page:
      margin_top_cm: 2.54
      margin_bottom_cm: 2.54
      margin_left_cm: 3.18
      margin_right_cm: 3.18
    font:
      body: 宋体
      heading: 黑体
      monospace: Courier New
    heading:
      h1: { font: 黑体, size: 16, bold: true }
      h2: { font: 黑体, size: 14, bold: true }
      h3: { font: 黑体, size: 12, bold: true }

  wps_compat:
    # WPS 兼容预设（如果客户反馈有兼容问题）
    page:
      margin_top_cm: 2.5
      # ... 调整
```

## 退出条件

需要重新评估的触发条件：

- 🔴 > 10% 客户主动要求 ODF
- 🔴 WPS 兼容问题反复出现
- 🔴 政府/事业单位强制要求 .pdf

## 后续动作

- M2 监控导出失败率（特别是 WPS 打开问题）
- M3 评估是否需要 .pdf（看客户反馈）
- M3+ 看是否需要 ODF（看政企客户比例）

## 参考

- [架构 / 模块设计 - document-svc](../architecture/modules.md#document-svc)
- [Plan / v1 设计 第 8 节](../plan/v1-design.md)
- yibiao exportService 设计（参考）