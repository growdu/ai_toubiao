# CI 与本地开发

## 持续集成

工作流在 `.github/workflows/` 下：

| Workflow | 触发 | 用途 | 严格度 |
|---|---|---|---|
| `ci.yml` | push to main / PR | 文件结构 + markdownlint + mermaid 渲染 + 链接检查 | 严格（CI 红） |
| `pages.yml` | push to main | mkdocs 构建 + 部署到 GitHub Pages | 严格（CI 红） |

### CI 检查项

| 检查 | 工具 | 失败时 |
|---|---|---|
| 必需文件存在且非空 | bash | CI 红 |
| Markdown 风格 | markdownlint-cli2 | CI 红 |
| Mermaid 块渲染 | mermaid.js + puppeteer-core + Chrome | CI 红 |
| 链接检查 | lychee | 仅 Job Summary（不阻塞） |
| MkDocs 严格构建 | mkdocs `strict: true` | CI 红（pages job） |

## 本地开发

### 校验 Mermaid 图

```bash
npm install                    # 首次：安装 mermaid + puppeteer-core
npm run lint:mermaid           # 校验默认 docs/**/*.md + README.md
MERMAID_LINT_VERBOSE=1 npm run lint:mermaid   # 打印每个块的行号
node tools/mermaid-lint.mjs README.md         # 只校验某个文件
```

需本机已安装 Chrome / Chromium；非默认路径可用 `MERMAID_LINT_CHROME` 环境变量指定。

### 本地预览 GitHub Pages

```bash
# 首次：创建 Python 虚拟环境并安装依赖
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt

# 本地预览（默认 http://127.0.0.1:8000）
mkdocs serve

# 严格模式构建
mkdocs build --strict
```

### GitHub Pages 部署

- 推送到 `main` → `pages.yml` 自动构建
- 构建产物部署到 `gh-pages` 分支
- GitHub 仓库 Settings → Pages → Source 选 `gh-pages` 分支
- 默认 URL：`https://growdu.github.io/ai_toubiao/`

## 文件组织

```
.
├── README.md                 # 仓库首页（GitHub 列表展示）
├── docs/                     # MkDocs 文档源
│   ├── index.md              # MkDocs 首页
│   ├── requirements-spec.md  # 需求规格说明书
│   ├── framework.md          # 设计纲要
│   ├── tech-selection.md     # 技术选型
│   ├── high-level-design.md  # 概要设计
│   ├── diaoyan.md            # 调研报告
│   └── ci-and-local-dev.md   # CI 与本地开发（本文件）
├── tools/
│   └── mermaid-lint.mjs      # Mermaid 块渲染校验
├── .github/workflows/
│   ├── ci.yml                # 文档校验
│   └── pages.yml             # GitHub Pages 部署
├── mkdocs.yml                # MkDocs 配置（Material 主题）
├── requirements.txt          # Python 依赖
├── package.json              # Node 依赖（mermaid-lint）
├── .markdownlint.json        # markdownlint 规则
└── .lychee.toml              # 链接检查配置
```

## License

Private · 仅供内部使用
