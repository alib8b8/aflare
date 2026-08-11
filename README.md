<div align="center">
  <h1>aflare</h1>
  <p>
    <strong>中文</strong> ·
    <a href="README.en.md">English</a>
  </p>
  <p><strong>让 AI 告别聊天，开始执行</strong></p>
  <p><em>ReAct 推理循环 · 300+ 技能模板 · 确定性工作流执行 · 10 类可插拔能力</em></p>

  <p>
    <a href="https://github.com/alib8b8/aflare/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/aflare/ci.yml?branch=main&style=flat-square&label=CI" alt="CI 状态" />
    </a>
    <a href="https://github.com/alib8b8/aflare/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/aflare?display_name=tag&include_prereleases&style=flat-square" alt="发布版本" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-AGPL%20v3.0-blue.svg?style=flat-square" alt="许可证" />
    </a>
  </p>
</div>

---

## 快速开始

```bash
# 安装
brew install alib8b8/tap/aflare
# 或: curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash

# 可选：安装 bubblewrap 以获得完整沙箱隔离（code_interpreter 节点需要）
# Ubuntu/Debian: sudo apt install bubblewrap
# macOS:        brew install bubblewrap
# Fedora:       sudo dnf install bubblewrap
```

```bash
# 关键词生成工作流
aflare create "每 10 分钟检查 BTC 价格，超过 70000 发 Telegram 通知"
# 输出: 工作流已生成 → btc-monitor.yaml

# 运行工作流
aflare run btc-monitor.yaml

# 交互式 AI Agent 对话（ReAct Agent + 300+ 技能）
aflare chat
# 或者: aflare chat -p deepseek -m deepseek-chat

# 守护进程式 Agent（融合 stdin + 定时任务） + 可插拔能力
aflare agent -c reflection,bdi,utility
```

---

## 项目状态

aflare 目前处于 **v0.7 早期阶段**。核心 Runtime 能力（DAG 调度、WAL 崩溃恢复、Saga 事务补偿、幂等、重试/熔断）已实现并通过 CI 验证。部分高阶特性（信创芯片适配、宇树机器人）为实验性支持，欢迎试用和反馈。

---

## 这是什么？

aflare 是一个**本地优先的自动化 Agent**，也是**确定性工作流执行引擎**。两种模式共用同一核心：

```
对话式 Agent                    声明式工作流
─────────────────              ─────────────────
aflare chat                    aflare create
  ↓                              ↓
ReAct Agent 思考              关键词匹配生成
  ↓                              ↓
调用 300+ 技能模板               YAML 工作流
  ↓                              ↓
工具执行 → 反思 → 优化           DAG 调度执行
```

**Agent 模式**：通过 `aflare chat` 或 `aflare agent` 启动。内置 ReAct 推理循环，拥有 300+ 预置技能模板（16 个领域），支持 10 类可插拔能力（反思、人机协同、BDI 目标管理、效用驱动优化等）。

**工作流模式**：`aflare create` 通过关键词匹配将描述转为 YAML 工作流。YAML 确定了每一步做什么、依赖谁、失败怎么办。Runtime 负责 DAG 调度、WAL 崩溃恢复、Saga 事务补偿、熔断、审计——所有操作可追溯、可回放、可验证。

---

## 三层模型

```
L0: Agent        —  "帮我监控 BTC，跌 5% 通知我"
                    ├── ReAct 推理循环（思考 → 调工具 → 观察 → 回答）
                    ├── 300+ 技能模板（16 个领域）
                    └── 10 类可插拔能力（反思/人机协同/BDI/效用驱动等）
                       ↓
L1: Workflow     —  YAML 确定性工作流（schedule → get_price → condition → telegram）
                       ↓
L2: Runtime      —  确定性执行层
                    ├── DAG 并行调度
                    ├── Checkpoint / Resume（WAL 崩溃恢复）
                    ├── Session 持久化（跨轮次上下文保持）
                    ├── Saga 事务补偿
                    ├── Idempotency（幂等）
                    ├── Retry / Rate Limit / Circuit Breaker
                    ├── HMAC 审计链
                    └── Secret 脱敏
```

---

## 和别的工具有什么区别？

| 工具 | 问题 | aflare |
|------|------|--------|
| **AI Agent (通用)** | LLM 决定执行，不可预测，难审计 | 确定性 YAML 工作流作为执行后端，可追溯、可回放 |
| **n8n** | 可视化工作流，但较重（Docker），无内置生成 | 单二进制，终端原生，关键词匹配生成工作流 |
| **Bash** | 难写难维护，无错误恢复 | 描述生成，内置重试/熔断/检查点 |
| **LangChain/AutoGPT** | 纯 Agent 无确定性执行保障 | Agent + Runtime 双重模式，Agent 可降级为确定性工作流 |
| **Claude Code/Cursor** | 云端依赖，代码编辑场景 | 本地优先，通用自动化，300+ 技能，执行可审计 |

---

## 核心能力

### 功能矩阵

| 功能 | 状态 | 验证状态 |
|------|------|----------|
| **ReAct Agent 对话** (`aflare chat`) | ✅ | 有测试 |
| **守护进程式 Agent** (`aflare agent`) | ✅ | 有测试 |
| **300+ 技能模板**（16 个领域） | ✅ | 有测试 |
| **10 类可插拔能力**（反思/人机协同/BDI/效用驱动等） | ✅ | 有测试 |
| **多源输入融合**（stdin + 定时任务 + 文件监听） | ✅ | 有测试 |
| DAG 并行调度 | ✅ | 有测试 + TLA+ 形式化验证 |
| WAL 崩溃恢复 + Session 持久化 | ✅ | 有测试 |
| Saga 事务补偿 | ✅ | 有测试 |
| Idempotency（幂等） | ✅ | 有测试 |
| Retry / Rate Limit / Circuit Breaker | ✅ | 有测试 |
| HMAC 审计链 | ✅ | 有测试 |
| Secret 脱敏 | ✅ | 有测试 |
| 表达式引擎（字节码 IR + 向量化） | ✅ | 有测试 |
| 关键词匹配生成工作流 | ✅ | 有测试 |
| MCP 协议支持（Server/Client） | ✅ | 有测试 |
| LLM 节点（22+ 模型） | ✅ | 有测试 |
| 安全等级（L0-L3） | ✅ | 有测试 |

> 实验性功能见下方 [实验性支持](#实验性支持) 章节。

### Agent 能力（对话式 + 守护进程式）

- **ReAct 推理循环** — 思考 → 调用工具 → 观察结果 → 回答，支持 native function calling 和 JSON fallback
- **300+ 预置技能模板** — 覆盖 16 个领域（金融、医疗、供应链、DevOps 等），Agent 自动匹配并执行
- **统一事件循环** — 对话式（`aflare chat`）和守护进程式（`aflare agent`）共用同一 `AgentLoop` 核心，支持 stdin / 定时任务 / 文件监听多源输入融合
- **10 类可插拔能力** — 按需启用，映射完整 Agent 类型分类学：

| 能力 | 类型 | 说明 |
|------|------|------|
| `reflection` | 反思/自我批评 | 每轮执行后自动评估输出质量，触发自我修正 |
| `human-in-loop` | 人机协同 | 关键操作暂停，请求人类确认后继续 |
| `bdi` | 信念-愿望-意图 | 维护目标追踪、信念提取、定期目标上下文注入 |
| `utility` | 效用驱动 | 6 维度评分（正确性/完整性/效率/安全/清晰/可操作），优化决策 |
| `adaptive` | 学习型/自适应 | 从反馈中学习，跨轮次改进表现 |
| `memory` | 有状态 | 跨会话长期记忆，记住用户偏好 |
| `planning` | 规划式 | 行动前生成计划，逐步执行 |
| `multi-agent` | 多 Agent 协作 | 复杂任务分解，多角色协调 |
| `workflow` | 工作流/管道式 | 优先使用已有模板，稳定可预测 |
| `simulation` | 模拟/生成式 | 类人行为建模，场景生成 |

### Runtime 保障（确定性执行）
- **DAG 并行调度** — 拓扑排序依赖调度，无依赖步骤并发执行
- **WAL 崩溃恢复 + Session 持久化** — append-only 持久化 + CRC32 校验，`--resume` 从中断处恢复；Session 跨轮次保持上下文
- **Saga 事务补偿** — 多步骤写入失败自动反向回滚
- **Idempotency** — Idempotency-Key + 原子占位 + 跨进程锁，防重复执行
- **Retry / Rate Limit / Circuit Breaker** — 指数退避 + 令牌桶 + 熔断器状态机

### 安全与合规
- HMAC 哈希链审计日志（防篡改）
- AES-GCM 加密 + PBKDF2（600K 迭代）
- Secret 自动脱敏（10+ 种模式：AWS/GitHub/JWT/私钥）
- SSRF 防护 / Path Traversal / Command Injection 白名单
- 出站数据量异常监控 + 熔断器自动隔离

### 工作流生成
- 关键词匹配生成 YAML 工作流（`aflare create`，见 [`generator.go`](internal/workflow/generator.go)）
- 100+ 内置模板

### LLM 节点（工作流中调用 LLM API）
- 22+ 模型支持（OpenAI / DeepSeek / Qwen / GLM / Kimi 等）
- 完全离线运行（Ollama 本地 LLM）
- LLM 智能路由（EWMA 延迟预测 + 帕累托成本排序）

### MCP 协议支持
- 内置 MCP Server，可被任何 MCP 客户端（Claude、VS Code、Cursor 等）连接
- 提供工作流运行、验证、节点查询、代码图谱等工具
- 内置 MCP Client，工作流中可直接调用外部 MCP 服务

### 工程能力
- 表达式引擎：字节码 IR + 向量化批量求值
- DAG 调度器经 TLA+ 形式化验证（spec 见 [`docs/tla/dag_scheduler.tla`](docs/tla/dag_scheduler.tla)，Go 测试 `dag_formal_test.go` 可执行有界模型检查）
- Prometheus 指标端点
- 单二进制部署，零运行时依赖
- CI 双架构验证（x86-64 + ARM64）

### 实验性支持
- 昇腾 / 寒武纪 / 海光国产芯片适配（基础功能可用，持续完善中）
- 宇树机器人集成（simulate 模式可用，实机模式需硬件）

---

## 架构

```
┌──────────────────────────────────────────────────────┐
│                    aflare                             │
│                                                       │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Agent Layer (L0)                                  │ │
│  │                                                    │ │
│  │  aflare chat / aflare agent                       │ │
│  │  ┌──────────┐  ┌──────────┐  ┌────────────────┐  │ │
│  │  │ ReAct    │  │ 300+     │  │ 10 类可插拔     │  │ │
│  │  │ 推理循环  │  │ 技能模板  │  │ 能力            │  │ │
│  │  └──────────┘  └──────────┘  └────────────────┘  │ │
│  │                                                    │ │
│  │  ┌──────────────────────────────────────────────┐ │ │
│  │  │ AgentLoop 统一事件循环                         │ │ │
│  │  │ stdin · scheduler · filewatch · MCP · HTTP   │ │ │
│  │  └──────────────────────────────────────────────┘ │ │
│  └──────────────────────────────────────────────────┘ │
│                        ↓                               │
│  ┌──────────┐   ┌──────────┐   ┌──────────────────┐  │
│  │ Intent   │──▶│ Workflow │──▶│ Deterministic     │  │
│  │ (描述)   │   │ (YAML)   │   │ Executor          │  │
│  └──────────┘   └──────────┘   │                    │  │
│                                 │ • DAG Scheduler   │  │
│                                 │ • WAL / Checkpoint│  │
│                                 │ • Session 持久化   │  │
│                                 │ • Saga / Retry    │  │
│                                 │ • Circuit Breaker │  │
│                                 │ • Audit / HMAC    │  │
│                                 └──────────────────┘  │
│                                                       │
│  ┌──────────────────────────────────────────────────┐ │
│  │ 执行目标                                          │ │
│  │ Software (API/Web/DB) • Devices (Phone/HarmonyOS) │ │
│  │ Robots (Unitree/Drone/Arm) • IoT                  │ │
│  └──────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

---

## 路线图

| 版本 | 状态 | 重点 |
|------|------|------|
| v0.6 | 已完成 | Agent 记忆基础设施、语音 AI 工具链、WAL 持久化、TLA+ 验证 |
| v0.7 | 已完成 | 金融场景增强（Saga / 幂等 / 审计链）、ReAct Agent 对话、300+ 技能模板、10 类可插拔能力、Agent 统一事件循环 |
| **v0.8** | **当前** | 信创芯片适配完善、宇树机器人实机支持、Agent 能力深化 |
| v1.0 | 计划中 | 稳定 API、LTS |

详情见 [CHANGELOG.md](CHANGELOG.md)

---

## 安全

aflare 内置多层安全防护，支持四级安全等级（`--security-level`）：

| 等级 | 说明 |
|------|------|
| **L0** | 宽松：允许所有节点，沙箱降级时仅警告 |
| **L1** | 标准：沙箱降级时警告，启发式拦截 |
| **L2** | 严格：无 bwrap 沙箱时拒绝执行 code_interpreter，命令白名单校验 |
| **L3** | 极严：禁用 code_interpreter 节点，最大安全策略 |

其他防护：SSRF 防护、Path Traversal 防御、Command Injection 白名单、AES-GCM 加密、Secret 脱敏、HMAC 审计链、熔断器、出站监控。CI 自动运行 `gofmt` / `go vet` / `gosec` / `govulncheck`。

[安全指南 →](SECURITY.md)

---

## 文档

- [入门指南](docs/getting-started.md) · [教程](docs/tutorial.md) · [YAML 语法](docs/getting-started.md#workflow-configuration)
- [数据流](docs/dataflow.md) · [调度](docs/scheduling.md) · [MCP](docs/mcp.md) · [插件](docs/plugins.md)
- [Web UI](docs/webui.md) · [可视化](docs/visualizer.md) · [自定义节点](docs/custom-nodes.md)
- [API 文档](docs/api.md) · [节点参考](docs/nodes-reference.md)
- [部署指南](docs/deployment.md) · [Docker](docs/docker.md) · [分布式](docs/distributed.md) · [多租户](docs/tenants.md)
- [故障排除](docs/troubleshooting.md)

---

## 贡献

欢迎社区贡献！除了代码，你还可以**提交 Skill 模板**：

1. **Fork** 本仓库
2. 在 `templates/` 对应领域目录下创建 YAML 模板（参考 [YAML 语法](docs/getting-started.md#workflow-configuration)）
3. 运行 `go test ./...` 验证
4. 提交 PR，附上模板用途说明

已有 300+ Skill 覆盖 16 个领域，你的模板可以补上缺失的一环。

[贡献指南 →](CONTRIBUTING.md)

---

## 许可证

GNU Affero General Public License v3.0 — [LICENSE](LICENSE)

---

<div align="center">
  <p>
    <a href="https://github.com/alib8b8/aflare">GitHub</a>
    ·
    <a href="https://github.com/alib8b8/aflare/issues">Issues</a>
    ·
    <a href="https://github.com/alib8b8/aflare/discussions">Discussions</a>
  </p>
</div>