<div align="center">
  <h1>aflare</h1>
  <p>
    <strong>中文</strong> ·
    <a href="README.en.md">English</a>
  </p>
  <p><strong>关键词描述意图 → YAML 工作流 → 确定性执行</strong></p>
  <p><em>确定性工作流执行运行时</em></p>

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
```

---

## 项目状态

aflare 目前处于 **v0.7 早期阶段**。核心 Runtime 能力（DAG 调度、WAL 崩溃恢复、Saga 事务补偿、幂等、重试/熔断）已实现并通过 CI 验证。部分高阶特性（信创芯片适配、宇树机器人）为实验性支持，欢迎试用和反馈。

---

## 这是什么？

aflare 把工作流的「描述」和「执行」分开：

```
你说的话  →  关键词匹配  →  YAML 工作流  →  Runtime 执行
(描述)      (正则+关键词)    (确定性)       (DAG / WAL / Saga / Retry / Audit)
```

`aflare create` 通过正则和关键词匹配将描述转为 YAML 工作流（**不是 LLM 生成**，见 [`generator.go`](internal/workflow/generator.go)）。YAML 工作流确定了每一步做什么、依赖谁、失败怎么办。Runtime 负责 DAG 调度、检查点恢复、Saga 事务补偿、熔断、审计——所有操作可追溯、可回放、可验证。

---

## 三层模型

```
L1: Intent       —  "帮我监控 BTC，跌 5% 通知我"
                       ↓
L2: Workflow     —  YAML 确定性工作流（schedule → get_price → condition → telegram）
                       ↓
L3: Runtime      —  确定性执行层
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
| **AI Agent** | LLM 决定执行，不可预测，难审计 | 确定性 YAML 工作流，执行可追溯、可回放 |
| **n8n** | 可视化工作流，但较重（Docker），无内置生成 | 单二进制，终端原生，关键词匹配生成工作流 |
| **Bash** | 难写难维护，无错误恢复 | 描述生成，内置重试/熔断/检查点 |

---

## 核心能力

### 功能矩阵

| 功能 | 状态 | 验证状态 |
|------|------|----------|
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
│                    aflare Runtime                     │
│                                                       │
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
| **v0.7** | **当前** | 金融场景增强（Saga / 幂等 / 审计链）、信创芯片实验性适配、宇树机器人实验性支持 |
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

欢迎社区贡献！Fork → 创建分支 → 修改 → `go test ./...` → PR。

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