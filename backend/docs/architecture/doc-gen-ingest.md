# doc-gen 输入流程（Ingest）优化设计

> 让 bidgen 能正确摄取真实招标材料包（docx / pdf / xls / zip / 图纸混合），修复提取失败、分类误判、无去重增量等问题。
> 关联实现：`services/doc-gen/internal/ingest/`

*最后更新：2026-07-14*

## 1. 目标

- 正确提取 docx / pdf / xls / xlsx / zip 五类格式的文本与结构
- 修正分类逻辑，避免附件被误判为 RFP
- 引入内容去重与增量索引，避免重复摄取
- 保留表格结构、清洗目录噪声，提升下游生成质量
- 保持 CLI 本地优先（零外部服务依赖），工具缺失时优雅降级

## 2. 背景

`bidgen` 的 `Ingest`（`services/doc-gen/internal/ingest/ingest.go:46`）递归遍历材料目录，逐文件提取文本、分块、向量化入库。以真实材料包 `examples/suite`（华能光伏 EPC 招标）实测，当前实现存在多处阻断性问题。

### 2.1 当前流程

```mermaid
flowchart LR
    A[材料目录] --> B[WalkDir 遍历]
    B --> C[detectCategory 文件名关键词]
    C --> D[extractText 按扩展名]
    D --> E[semanticChunk 512/64]
    E --> F[EmbedBatch 向量化]
    F --> G[SaveChunks]
```

### 2.2 实测问题（examples/suite）

| 文件 | 提取 | 分类 | 问题 |
|---|---|---|---|
| 招标文件.docx | OK | rfp | 表格丢结构、未读页眉页脚 |
| 商务标.pdf.pdf | 失败 | rfp 误判 | pdftotext 缺失；含"招标"误判 RFP |
| 可研报告.pdf(28MB) | 失败 | rfp 误判 | 同上，核心技术材料丢失 |
| 合同条款.docx | OK | rfp 误判 | 含"招标"误判 |
| 工程量清单.xls | 失败 | rfp 误判 | OLE2 非 zip，extractXLSX 失败 |
| 总平图.pdf | 失败 | rfp 误判 | 图纸无文本层，需 OCR |
| 技术规范.zip | 失败 | rfp 误判 | 当文本读出垃圾；与目录重复 |
| 技术规范/5×docx | OK | other 误判 | 中文"技术规范"不匹配 technical |

环境工具：`pdftotext` / `soffice` / `antiword` / `catdoc` 全缺失，仅 `unzip`、`python3` 可用。

## 3. 设计

### 3.1 整体方案

在现有"遍历->提取->分块->向量化"前增加**归一化去重层**，重写**提取层**与**分类层**，并补齐**结构化**、**增量**、**OCR** 能力。

```mermaid
flowchart LR
    A[材料目录/zip] --> B[归一化: 解包zip/去重/规范名]
    B --> C[分类: 显式RFP优先 + 附件类型词]
    C --> D[提取: 格式策略表]
    D --> E[结构化清洗: 表格/去TOC噪声]
    E --> F[分块: 条款感知]
    F --> G[增量: hash跳过]
    G --> H[向量化入库]
```

### 3.2 归一化与去重层

- **递归解包**：遇到 `.zip` 解压到临时目录（`os.MkdirTemp`），递归 ingest，原 zip 不当文本读。层数上限 3 防套娃。
- **内容去重**：对每个文件算 `sha256`（前 64KB + 大小预筛，命中再全文），重复文件跳过。解决"顶层 zip ⊃ 技术规范 zip ⊃ 5 docx"与目录的三重重复。
- **文件名规范化**：剥离重复扩展名（`.pdf.pdf` -> `.pdf`），原名记录到 `Chunk.SourceName`。

### 3.3 文本提取层

按真实格式分流，工具缺失时降级并记录告警：

| 格式 | 主策略 | 降级策略 |
|---|---|---|
| .docx | python `python-docx`（保留表格行列） | Go `archive/zip` 读 document.xml |
| .pdf | python `pdfplumber` / `PyMuPDF` | `pdftotext`；都缺则标记待 OCR |
| .xlsx | python `openpyxl` | Go 读 sharedStrings.xml |
| .xls | python `xlrd` 或 `soffice --convert` | 缺失则跳过并告警 |
| .doc | `soffice` 转换 | `antiword` / `catdoc` |
| 图像/扫描件 | `tesseract` OCR（可选） | 标记 `needs_ocr`，不阻断 |

> **决策**：提取统一走 `python3` 子进程（环境已有 python3，库生态最全）。详见 [ADR-0012](../decisions/0012-doc-gen-extract-via-python.md)。

子进程契约：`python3 extract.py <path> --format <ext>` 输出 JSON `{text, tables, warnings}`，Go 侧 `exec.Command` 调用，超时 120s。

### 3.4 分类层重构

核心原则：**只有显式 `--rfp` 指定的是 RFP**，附件按类型词分类，去掉过宽的"招标"匹配。

```go
// 伪代码：分类优先级
func detectCategory(path, name, rfpAbs string) string {
    if path == rfpAbs {
        return "rfp"
    }
    switch {
    case contains(name, "招标文件"):
        return "rfp"
    case contains(name, "投标文件", "商务标"):
        return "reference" // 历史投标
    case contains(name, "可研", "可行性研究"):
        return "technical"
    case contains(name, "技术规范", "规范书"):
        return "technical"
    case contains(name, "合同"):
        return "commercial"
    case contains(name, "清单", "报价"):
        return "commercial"
    case contains(name, "图纸", "总平图"):
        return "drawing"
    case contains(name, "资质"):
        return "qualification"
    case contains(name, "业绩"):
        return "performance"
    default:
        return "other"
    }
}
```

类别枚举扩展：`rfp / reference / technical / commercial / drawing / qualification / performance / other`。

### 3.5 结构化与分块

- **表格保留**：docx / xlsx 表格转为 Markdown 表格存入 `Chunk.Text`，保留行列。
- **噪声清洗**：移除 `PAGEREF`、`TOC \o`、连续页码等目录噪声（正则）。
- **条款感知分块**：优先按"第X章 / 条 / 款"标题切段，再按 512 token 上限合并；表格不切断。

### 3.6 增量索引

`ingestFile` 增加 `mtime + sha256` 记录，命中则跳过提取与向量化。`Store` 新增 `FileMeta(path) (hash, mtime, error)` 查询与写入。修复当前注释称增量、实际全量的问题。

### 3.7 OCR（可选，Phase 2）

对提取文本 < 阈值的 PDF 标记 `needs_ocr`；若 `tesseract` 可用则补提取，否则记录告警不阻断。

## 4. 数据模型

`core.Chunk` 扩展字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| ContentHash | string | 源文件 sha256（去重 / 增量键） |
| SourceName | string | 规范化前原始文件名 |
| NeedsOCR | bool | 无文本层标记 |

`store.Store` 新增 `GetFileMeta(path)` / `SaveFileMeta(path, hash, mtime)`。

## 5. 风险与边界

- **python 依赖**：需 `pip install python-docx pdfplumber openpyxl xlrd`。缺失时逐格式降级，不整体失败。
- **大文件**：28MB 可研 PDF，提取走流式 + 子进程超时 120s，内存峰值可控。
- **OCR 准确率**：图纸 / 扫描件 OCR 质量有限，仅作辅助，不作为唯一来源。
- **套娃 zip**：递归层数上限 3 + 去重，防膨胀。

## 6. 验收标准

- [ ] `bidgen index examples/suite --rfp 招标文件.docx` 成功提取全部 8 类材料，无文件因格式失败
- [ ] 商务标 / 可研 / 合同 / 清单归类正确，不再误判 rfp
- [ ] 技术规范 5 份归 technical
- [ ] 重复 zip / 目录只摄取一次
- [ ] 二次 ingest 命中增量，跳过未变更文件
- [ ] docx 表格以 Markdown 保留
- [ ] 单元测试覆盖 detectCategory、去重、提取降级

## 7. 替代方案

- **纯 Go 库**（unidoc / xuri / excelize）：无子进程开销，但 PDF 与旧 xls 生态弱、许可证复杂。否决。
- **libreoffice 统一转换**：质量高但依赖重（约 500MB）、启动慢，不适合 CLI 本地优先。保留作 `.doc` / `.xls` 降级。
- **多模态 LLM 视觉提取**：质量高但成本高、慢，留作 Phase 2 图纸增强。

## 8. 相关文档

- [doc-gen 模块架构](doc-gen.md)
- [docgen-svc 服务化](docgen-svc.md)
- [ADR-0011 doc-gen CLI 优先](../decisions/0011-doc-gen-cli-first.md)
- [ADR-0012 提取走 python 子进程](../decisions/0012-doc-gen-extract-via-python.md)
- [文档规范](../development/documentation-style.md)
