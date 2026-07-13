# 用户手册

> 面向 **投标专员 / 项目经理 / 商务经理** 的端到端使用指南。本文档基于当前代码（`main` 分支，2026-07）撰写，所有功能均可在 `http://localhost:8081` 验证。

---

## 0. 阅读对象与范围

本文档覆盖你能在浏览器里看到并操作的 **全部功能**：

- 账号注册、登录、退出
- 创建 / 管理标书
- 在工作区里审稿、拒稿、改稿、保存
- 上传 / 检索知识库素材
- 配置账户偏好、套餐升级
- 命令面板（⌘K）与快捷键

**不**覆盖：自托管部署（见 `deployment.md`）、API 集成（见 `api-spec.md`）、二次开发（见 `high-level-design.md`）。

---

## 1. 快速上手（5 分钟）

### 1.1 登录

打开 `http://localhost:8081`，跳转到营销首页。点击 **「登录」**：

| 字段 | 示例 |
|---|---|
| 租户标识 | `demo-a` |
| 邮箱 | `admin@demo-a.test` |
| 密码 | `password123` |

> ⚠️ 演示账号密码基于 bcrypt 哈希；如果登录失败且后端日志显示 `failed SASL auth`，说明 PG 数据库被外部修改过，需按本文档 **8. 故障排查** 处理。

成功后会跳到 `/bids`（标书列表）。

### 1.2 创建第一份标书

1. 点击右上角 **「+ 新建标书」**
2. 输入项目名（如 `XX 医院信息化项目`），确定
3. 自动跳到工作区，标书进入 `pending` 状态，几秒后开始异步生成大纲

### 1.3 审稿并导出

- 在 **大纲 Tab**：逐章审阅，看到 `✓` 通过 / `⚠ 待审核` 标记
- 点击章节进入 **编辑 Tab**：左侧 Markdown 编辑，右侧字数警告与章节元数据
- 全部章节审完 → 顶部 **「导出」** 按钮 → 选择 `.docx`（默认）或 `.pdf`（需部署时安装 LibreOffice）→ 下载

---

## 2. 账号与会话

### 2.1 注册新租户

如果还没有租户，登录页点 **「注册」**：

| 字段 | 规则 | 示例 |
|---|---|---|
| 工作区名称 | 必填，中英文均可 | `建工集团` |
| 工作区标识 | 3-32 字符，小写字母数字连字符 | `jiangong` |
| 邮箱 | 标准邮箱格式 | `admin@jiangong.cn` |
| 密码 | ≥8 字符 | （自行设置） |
| 显示名 | 可选 | `张经理` |

注册成功后 **自动登录**，跳到 `/bids`。

### 2.2 退出登录

侧边栏底部头像 → **「退出登录」**，或命令面板 ⌘K → 输入 `退出`。

### 2.3 Token 失效处理

访问令牌有效期 24 小时。过期后任意接口返回 401：

- **客户端表现**：顶部出现红色错误条
- **自动行为**：清空本地 token，跳转到 `/login`
- **手动恢复**：再次登录即可（无需操作服务端）

---

## 3. 标书管理（BidsPage）

### 3.1 列表视图

| 列 / 卡片 | 含义 |
|---|---|
| 项目名 | `projects.name`，创建时输入 |
| 行业 | 从 RFP 解析结果提取 |
| 状态 | 见下表 |
| 进度 | 已生成章节 / 总章节 |
| 更新时间 | 排序依据（默认最新在前） |

#### 标书状态机

```
pending → parsing → outlining → facts → generating → auditing → exporting → done
                          ↓         ↓         ↓           ↓
                        paused (HIL 暂停点，可恢复或取消)
```

| 状态 | 含义 | 你能做什么 |
|---|---|---|
| `pending` | 已创建，等待解析 RFP | 编辑项目名 |
| `parsing` / `outlining` / `facts` | 后端处理中 | 等待 |
| `generating` | 章节内容生成 | 等待（最快 10s/章） |
| `auditing` | 合规审查 | 等待 |
| `exporting` | 组装 Word/PDF | 等待 |
| `done` | 完成 | 导出、查看、学习反馈 |
| `failed` | 失败 | 查看错误日志、删除重建 |
| `paused` | HIL 暂停 | 恢复或取消 |
| `cancelled` | 已取消 | 仅查看 |

### 3.2 过滤与搜索

- 顶部 **搜索框**：按项目名模糊匹配
- **状态 Tab**（全部 / 进行中 / 已完成 / 失败）：单选过滤
- **排序**：按更新时间（默认）/ 按项目名

### 3.3 创建标书

弹窗里只需填写 **项目名**：

- 后端自动创建一个 `project`（如不存在同租户同名）+ 一个 `bid_job`
- 进入工作区后系统自动调用 workflow-svc 启动状态机

### 3.4 删除标书

鼠标悬停卡片 → 右上角 **⋮** 菜单 → **删除**（仅 owner 可见）。删除会同时清理：
- 标书的所有 chapter_specs / chapter_contents / evidence
- 已上传的 RFP 文档及其解析结果
- 已导出的 docx/pdf 文件（MinIO）

---

## 4. 标书工作区（BidWorkspace）

进入工作区后，左侧是章节树，中间是编辑器，右侧是检查器。顶部是 7 步进度条。

### 4.1 顶部进度条

```
1. RFP 解析 → 2. 大纲生成 → 3. 事实抽取 → 4. 章节生成 → 5. 合规审查 → 6. 导出 Word/PDF
   parsing     outlining       facts        generating     auditing      exporting
```

当前步骤高亮蓝色，已完成步骤打勾。

### 4.2 章节树（左侧）

- 按层级缩进展示（1 级章节 → 子章节）
- 图标：
  - `✓` 已审核
  - `⚠` 待审核
  - `○` 未开始
  - `✗` 被拒稿
- 点击节点 → 中央加载该章节

### 4.3 章节编辑器（中间）

上方 3 个 Tab：

| Tab | 内容 |
|---|---|
| **内容** | Markdown 编辑器，支持实时字数统计 |
| **大纲** | 只读视图，看生成时的原始大纲 |
| **审稿意见** | 该章节的所有反馈历史 |

#### 字数警告

| 阈值 | 颜色 |
|---|---|
| 达到 `min_word_count` | 绿色 ✓ |
| 低于 `min_word_count` | 红色 ⚠ |
| 超过 `target_word_count` | 橙色 ⚠ |

#### 保存

- **手动**：⌘S（Mac）/ Ctrl+S（Win）
- **自动**：离开编辑器前自动保存到 draft

### 4.4 章节检查器（右侧）

5 个分区：

1. **基本信息**：标题、层级、状态、优先级
2. **字数与风格**：min / target 字数、写作风格（叙述 / 列表 / 表格）
3. **Prompt 快照**：生成该章节时使用的系统 prompt（方便调试与复现）
4. **配置**：模型、temperature、max_tokens
5. **事件日志**：时间线视图，最近 50 条状态变更、拒稿原因、保存版本

### 4.5 拒稿流程

1. 在 **审稿意见** Tab 点 **「拒稿」**
2. 选择拒稿模板（或自定义原因）：
   - 内容不实
   - 与证据矛盾
   - 偏离大纲
   - 字数不足
   - 触犯暗标规则
   - 自定义输入
3. 提交后章节状态变 `rejected`，状态机自动重新调度 `generating`（生成会复用事实和证据，仅替换章节文本）

### 4.6 学习反馈（标书 `done` 后）

顶部 **「导出」** 旁边有 **「学习反馈」** 按钮：

- 标状态：**已中标 / 未中标 / 草稿**
- 选择后提交到 docgen-svc 的 `learn` 端点
- 后端抽取本次标书的"模式"（行业 + 章节结构 + 风格）写入学习库
- 下次同类标书会优先推荐高质量模板

---

## 5. 知识库（KnowledgePage）

存放你的 **公司级素材**：资质证书、过往项目案例、专利、团队成员、设备清单等，生成章节时被自动检索引用。

### 5.1 七大分类

| 图标 | 分类 | 用途 |
|---|---|---|
| 📜 | 资质证书 | 投标资格证明 |
| 📂 | 项目案例 | 类似项目业绩 |
| 💡 | 专利 | 技术亮点 |
| 👥 | 团队成员 | 核心人员简历 |
| 🛠️ | 设备 | 硬件清单 |
| 🏆 | 资格认证 | ISO、CMMI 等 |
| 📄 | 其他 | 兜底 |

### 5.2 上传素材

两种方式：

1. **拖拽上传**：拖文件到右侧上传区
2. **手动登记**：点 **「+ 新增素材」**，填：
   - 标题
   - 分类
   - 内容（直接粘贴文本 或 上传 PDF/Word）

提交后素材进入 `pending` → 后台异步分块、向量化 → `ready`，**约 10 秒**。

### 5.3 状态

| 状态 | 含义 |
|---|---|
| `pending` | 等待处理 |
| `processing` | 分块 + 向量化中 |
| `ready` | 已索引，可被检索 |
| `failed` | 处理失败（可重试） |

### 5.4 检索

工作区生成章节时，knowledge-svc 自动混合检索（vector + BM25 + RRF），无需你手动操作。如果你想手动验证素材是否能被检索到，命令面板 ⌘K → 输入 `搜索素材`。

---

## 6. 设置（SettingsPage）

侧边栏 → **设置**，4 个 Tab：

### 6.1 账户

- 显示当前用户邮箱、角色（owner / member）
- 修改显示名（暂未实现，留待 1.x 版本）

### 6.2 计费 / 套餐

| 套餐 | 适用 | 月度预算 |
|---|---|---|
| Free | 试用 | $0 |
| Pro | 中小团队 | $50 |
| Enterprise | 大客户 | 定制 |

- 显示 **本月已用 / 总额度** 进度条
- 点击 **「升级 Pro」** 调起支付流（当前为 mock，立即生效）

### 6.3 通知偏好

- 邮件 / 钉钉 / 企微 三渠道开关
- 哪些事件需要通知：
  - 标书生成完成
  - 审稿拒稿
  - 导出完成
  - 月度预算耗尽

### 6.4 系统信息

- 服务版本（git commit hash）
- API 网关地址
- 数据中心位置
- 资源用量

---

## 7. 命令面板与快捷键

按 **⌘K**（Mac）/ **Ctrl+K**（Win）打开命令面板：

| 类别 | 命令 |
|---|---|
| 导航 | 标书管理 / 知识库 / 设置 |
| 外观 | 切换主题（浅色 / 深色 / 跟随系统） |
| 账户 | 退出登录 |
| 操作 | 搜索素材 / 重新生成当前章节 / 跳到下一章节 |

### 全部快捷键

| 快捷键 | 行为 |
|---|---|
| ⌘K / Ctrl+K | 打开命令面板 |
| ⌘S / Ctrl+S | 保存当前编辑 |
| ← / → | 上一章 / 下一章 |
| Esc | 关闭弹窗 / 命令面板 |
| `?` | 打开帮助 |

---

## 8. 故障排查

### 8.1 登录返回 `500 Internal Server Error`

查看后端日志：

```bash
docker exec bidwriter-stack tail -20 /logs/api-gateway.log
```

**典型原因**：

| 日志关键词 | 根因 | 修复 |
|---|---|---|
| `failed SASL auth for user "postgres"` | PG `postgres` role 被改成 `NOLOGIN` 或密码漂移 | 重置 PG：见 **8.2** |
| `connection refused on 5434` | PG 容器没起来 | `docker ps -a --filter name=bidwriter-pg-test` |
| `pq: relation "users" does not exist` | migration 没跑全 | 重跑 `apply-migrations.sh` |

### 8.2 重置 PG 数据库

```bash
# 1. 停 PG
docker rm -f bidwriter-pg-test

# 2. 重建（保持数据卷 /home/ubuntu/bidwriter-pg-data 完整时跳过 rm -rf）
docker run -d --name bidwriter-pg-test \
  --restart unless-stopped \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=bidwriter \
  -p 5434:5432 \
  -v /home/ubuntu/bidwriter-pg-data:/var/lib/postgresql/data \
  -v /tmp/bidwriter-initdb:/docker-entrypoint-initdb.d:ro \
  pgvector/pgvector:pg16

# 3. 重启 stack
cd backend && ./scripts/start-stack.sh restart

# 4. 重新设置 demo 用户密码（首次）
docker exec bidwriter-pg-test psql -U postgres -d bidwriter -c "
UPDATE users SET password_hash = '\$2a\$10\$<新生成的 hash>' WHERE email LIKE '%@demo-%';
"
```

新 bcrypt hash 生成：

```bash
docker run --rm -v /tmp:/work -w /work golang:1.25-alpine sh -c '
cat > main.go <<EOF
package main
import ("fmt"; "golang.org/x/crypto/bcrypt")
func main() { h,_:=bcrypt.GenerateFromPassword([]byte("password123"),10); fmt.Println(string(h)) }
EOF
go mod init tmp && go get golang.org/x/crypto/bcrypt && go run main.go
'
```

### 8.3 页面白屏 / 路由 404

按 **⌘Shift+R** 强制刷新浏览器。dist 由 nginx 容器挂载在 `/home/ubuntu/ai_toubiao/web/dist`，build 后立即生效。

### 8.4 PDF 导出返回 503

服务器没装 LibreOffice：

```bash
# 在跑 stack 的机器上
apt-get install -y libreoffice

# 或在 Dockerfile 里预设
```

docgen-svc 启动日志会显示 `PDF converter: enabled` 或 `disabled`。

### 8.5 章节生成卡住

检查 Asynq 队列：

```bash
docker exec bidwriter-stack tail -20 /logs/workflow-svc.log
# 看 "Starting processing" / "chapter: ..." 日志
```

如果一直 no progress，可能是 LLM Provider 限流或故障。切到 **设置 → 系统信息** 看当前 provider 状态。

---

## 9. 进阶用法

### 9.1 团队协作

- **owner** 角色：可创建 / 删除标书、邀请成员、修改套餐
- **member** 角色：仅可编辑分配的章节、提交反馈、导出

邀请成员：当前为手动分配（`users.tenant_id` + `users.role`）。UI 上的邀请流留待 v1.x。

### 9.2 批量操作

- 在 BidsPage 多选（暂未实现，留待 v1.x）
- 命令面板里输入 `rebuild` 一键重建当前标书（保留事实和证据，仅重跑章节）

### 9.3 模板复用

如果你的租户积累了高质量"中标"模式，下次新建标书时：

1. 选择 **行业**（如 `医疗信息化`）
2. 大纲生成会优先复用类似模式的章节结构
3. 章节生成会参考高质量示例的写作风格

学习反馈越多，推荐质量越高（Bandit 算法 + 质量评分）。

---

## 10. 反馈与支持

- **Bug Report**：[GitHub Issues](https://github.com/growdu/ai_toubiao/issues/new?template=bug_report.md)
- **功能请求**：[Feature Request](https://github.com/growdu/ai_toubiao/issues/new?template=feature_request.md)
- **架构 / API 问题**：先看 `docs/high-level-design.md` 和 `docs/api-spec.md`，再在 Issue 引用具体章节
- **紧急**：联系项目 owner（见 `OWNERS` 文件）

---

**文档版本**：基于 commit `e73c712`（2026-07-13）
