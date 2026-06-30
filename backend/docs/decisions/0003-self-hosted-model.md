# 0003. 私有化部署是否提供模型

## 状态

Accepted

## 日期

2026-06-27

## 参与者

- 架构组
- 运维组
- 销售组

## 背景

BidWriter Enterprise 客户可以选择私有化部署。问题是：私有化时是否提供 AI 模型？

**约束条件**：
- 模型包 50-200GB，分发成本高、版本管理复杂
- 客户硬件差异大（有 8xA100 也有 24GB 显存单卡）
- 合规：客户数据出域是大忌
- 灵活性：客户可用任何 OpenAI 协议的服务
- "开箱即用"是 Enterprise 客户的关键需求

**需要决策**：私有化部署包是否包含模型。

## 决策

**默认不提供模型，客户自备；提供 Ollama 一键部署脚本 + 模型推荐清单。**

可选项：客户机器配置低于最低要求时，可付费购买"模型包 + 上门部署"服务。

## 理由

- ✅ 模型包分发成本高、版本管理复杂
- ✅ 客户硬件差异大，无法一刀切
- ✅ 合规：客户数据不能出域
- ✅ 灵活性：客户可用任何 OpenAI 兼容服务
- ✅ Ollama 脚本让"开箱即用"成为可能

## 考虑的替代方案

### 方案 A：私有化必带模型包

- ❌ 模型分发成本高（50-200GB/包）
- ❌ 客户硬件不够就跑不起来
- ❌ 模型版本管理复杂
- ❌ 合规风险（模型权重 = 知识产权）
- ⚠️ 仅适合硬件充足 + 必须自包含的客户

### 方案 B：完全不提供模型（**选择**）

- ✅ 灵活
- ✅ 合规友好
- ✅ 客户硬件自适应
- ❌ "开箱即用"差

### 方案 C：默认不提供 + 可选付费包

- ✅ 灵活 + 合规友好
- ✅ 满足"开箱即用"需求（付费）
- ✅ 商业模式更清晰
- ⚠️ 实施复杂度中等

## 后果

### 正面

- 模型分发、版本管理成本低
- 客户数据完全本地化（合规友好）
- 客户可用任何 OpenAI 兼容服务（灵活）
- Ollama 脚本大幅降低部署门槛

### 负面

- "开箱即用"体验不如 SaaS
- 客户需要一定技术能力配置模型
- 需要维护 Ollama 部署脚本

### 中性（需要承担的工作）

- 提供 Helm Chart + Ollama StatefulSet
- 提供离线模型加载脚本
- 提供模型推荐清单
- 提供"模型包"附加服务（如需要）

## 实施细节

### 私有化部署包

```
bidwriter-enterprise-v1.0/
├── helm/                          # Kubernetes Helm Chart
│   ├── bidwriter/
│   │   ├── api-gateway/
│   │   ├── workflow-svc/
│   │   ├── ... (其他服务)
│   │   └── ollama/                # 可选 Ollama
│   └── README.md
├── scripts/
│   ├── install.sh                 # 一键安装
│   ├── load-models.sh             # 离线模型加载
│   └── backup.sh                  # 备份脚本
├── docs/
│   ├── deployment.md
│   ├── model-recommendations.md
│   └── troubleshooting.md
└── LICENSE                        # AGPL-3.0
```

### Ollama 一键部署

```bash
# 安装 Ollama + 推荐模型
./scripts/install-ollama.sh

# 验证
curl http://localhost:11434/api/tags
# 期望输出：qwen2.5:72b-instruct, deepseek-v3:32b
```

### 模型推荐清单

| 显存 | 推荐模型 | 量化 | 用途 |
|---|---|---|---|
| 24 GB | Qwen2.5-32B-Instruct | AWQ 4-bit | 兜底模型 |
| 48 GB | Qwen2.5-72B-Instruct | AWQ 4-bit | 主推荐 |
| 80 GB+ | DeepSeek-V3 | FP16 | 高质量场景 |
| 8 GB | Qwen2.5-7B-Instruct | AWQ 4-bit | 极限低配 |

### 离线加载

```bash
# 在线下载
./scripts/download-models.sh --output /mnt/usb/

# 离线导入
./scripts/load-models.sh --input /mnt/usb/models/
```

### vLLM / TGI 备选

高级客户可用 vLLM / TGI 部署：

```yaml
# configs/router.yaml
providers:
  - name: local-vllm
    type: openai_compatible
    base_url: http://vllm-server:8000/v1
    model: Qwen/Qwen2.5-72B-Instruct
```

### "模型包"附加服务（可选）

- 客户机器 < 24GB 显存
- 客户要求"开箱即用"
- 销售可推荐付费包：
  - 工程师上门部署
  - 预加载模型（合规允许的模型）
  - 远程运维 3 个月

## 退出条件

需要重新评估的触发条件：

- 🔴 客户机器普遍配置高（> 48GB 显存）
- 🔴 私有化市场规模大到值得自建模型分发
- 🔴 客户普遍要求"开箱即用"
- 🔴 出现硬件成本更低的方案（如 Apple Silicon）

## 参考

- [架构 / AI 路由](../architecture/ai-router.md)
- [架构 / 模块设计](../architecture/modules.md)
- [运维 / 部署](../operations/deployment.md)
- [ADR-0002 模型路由](0002-ai-router-quality.md)