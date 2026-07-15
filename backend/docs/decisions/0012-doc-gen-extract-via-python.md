# 0012. doc-gen 文本提取走 python 子进程

## 状态

Proposed

## 日期

2026-07-14

## 背景

doc-gen 的 ingest 需要提取 docx / pdf / xls / xlsx 等格式文本。当前实现用 Go 原生（`archive/zip` 读 XML）+ shell-out（`pdftotext` / `antiword`），存在明显短板：

- PDF 依赖 `pdftotext`，环境常缺失，fallback 的 ASCII 提取对压缩流无效
- `.xls`（OLE2）无法用 zip 解析，`extractXLSX` 必失败
- docx 只读 `document.xml` 文本节点，表格行列结构丢失

以真实材料包 `examples/suite` 实测，8 类文件中 5 类提取失败，核心的可研报告与工程量清单全部丢失。

## 决策

文本提取统一走 `python3` 子进程：Go 侧通过 `exec.Command` 调用 `extract.py`，传入文件路径与格式，输出 JSON `{text, tables, warnings}`。使用 `python-docx` / `pdfplumber` / `openpyxl` / `xlrd` 等库。

## 理由

- python 文档解析库生态最全、质量最高，覆盖 docx / pdf / xls / xlsx
- 环境已有 `python3`，零额外运行时
- 子进程隔离：python 崩溃或超时不影响 Go 主进程
- 统一 JSON 契约，便于扩展新格式与保留表格结构

## 替代方案

- **纯 Go 库**（unidoc / xuri / excelize）：PDF 与旧 xls 生态弱，unidoc 商用许可证复杂。否决。
- **libreoffice 统一转换**：质量高但依赖重（约 500MB）、启动慢，违背 CLI 本地优先。保留作 `.doc` / `.xls` 降级。
- **内嵌 python（go-python）**：增加构建与交叉编译复杂度。否决。

## 后果

- **正面**：覆盖 docx / pdf / xls / xlsx，保留表格结构，单库缺失可逐格式降级
- **负面**：引入 python 依赖（`pip install`），子进程有 IPC 开销（单文件级别，可接受）
- **成本**：维护 `extract.py` 脚本与 python 依赖声明（requirements.txt）

## 参考

- [doc-gen 输入流程优化设计](../architecture/doc-gen-ingest.md)
- [ADR-0011 doc-gen CLI 优先](0011-doc-gen-cli-first.md)
