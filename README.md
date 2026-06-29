# ai_toubiao

投标工具箱（OpenBidKit / 易标）相关独立工作仓库。

## 目录结构

  configs/
    routes-smoke.yaml          # router-svc 的 mock 模式 routes 文件（不含真 provider key）
  scripts/
    router-svc-smoke.mjs       # 19 个测试（15 单元 + 4 live 端到端）
  docs/
    ROUTER_SVC_INTEGRATION.md  # 协议、mapping、启动、测试、配置示例
    router-integration.patch.diff  # 集成 OpenBidKit_Yibiao 客户端的 patch

## 这是什么

本仓库承载的是 **OpenBidKit_Yibiao Electron 客户端 ↔ bidwriter Go router-svc** 集成的独立可移植产物。完整的代码修改在 OpenBidKit_Yibiao 仓库的 commit `44f7f7d`，本仓库只放那些**可以脱离 OpenBidKit 仓库独立使用**的东西：

- `configs/routes-smoke.yaml` —— mock 模式路由表，注入到 router-svc 后可以做端到端联调（不消耗真 LLM 配额）
- `scripts/router-svc-smoke.mjs` —— 不依赖 Electron 环境，跑 `node` 就能验证客户端/服务端协议是否符合预期
- `docs/router-integration.patch.diff` —— 完整 patch（350+ 行），用来把集成改回 OpenBidKit_Yibiao 的一个干净 checkout
- `docs/ROUTER_SVC_INTEGRATION.md` —— 协议、配置、跑测试的完整指南

## 快速验证

  # 单元测试（不需要任何后端）
  node scripts/router-svc-smoke.mjs

  # 端到端（先启 router-svc，参考 docs/ROUTER_SVC_INTEGRATION.md）
  node scripts/router-svc-smoke.mjs --live http://127.0.0.1:8085

## 关联项目

- **OpenBidKit_Yibiao**（Electron 客户端）：`/work/ai/OpenBidKit_Yibiao`，commit `44f7f7d`
- **bidwriter**（Go 后端，含 router-svc）：`/work/ai/bidwriter`
