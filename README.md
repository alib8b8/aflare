<div align="center">
  <h1>aflare</h1>
  <p>🌍
    <strong>中文</strong> ·
    <a href="README.en.md">English</a>
  </p>
  <p><strong>AI 不应该只是回答你，它应该执行你的意图。</strong></p>
  <p><em>aflare — Deterministic Execution Runtime for AI</em></p>
  <p>自然语言描述意图 → YAML 工作流 → 确定性执行。AI 做「脑子」，Workflow 做「骨架」，Runtime 做「手」。</p>

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

## 这是什么？

aflare 是一个 **AI 确定性执行运行时**。它把 AI 的「理解能力」和「执行能力」分开：

```
你说的话  →  AI 翻译成意图  →  YAML 工作流  →  Runtime 执行
(自然语言)    (LLM)            (确定性)       (DAG / WAL / Saga / Retry / Audit)
```

传统 AI Agent 的问题是：LLM 既负责理解，又负责决策执行——不稳定、不可预测、难审计。

aflare 的思路是：**AI 只负责翻译，执行由 Runtime 保证**。YAML 工作流确定了每一步做什么、依赖谁、失败怎么办，Runtime 负责 DAG 调度、检查点恢复、Saga 事务补偿、熔断、审计——所有操作可追溯、可回放、可验证。

---

## 三层模型

```
L1: AI Intent    —  "帮我监控 BTC，跌 5% 通知我"
                        ↓
L2: Workflow     —  YAML 确定性工作流（schedule → get_price → condition → telegram）
                        ↓
L3: Harness + Runtime  —  确定性执行层
                    ├── DAG 并行调度
                    ├── Checkpoint / Resume（WAL 崩溃恢复）
                    ├── Session 持久化（跨轮次上下文保持）
                    ├── Saga 事务补偿
                    ├── Idempotency（幂等）
                    ├── Retry / Rate Limit / Circuit Breaker
                    ├── HMAC 审计链
                    └── Secret 脱敏
```

**aflare 的护城河不是 LLM，是 Harness + Runtime。**

> Harness 工程是 AI 时代的「操作系统」——黄仁勋与 LangChain 对谈中定义的下一代软件范式：确定性执行与状态管理是 Agent 可靠性的基石。aflare 的 Checkpoint/Session 持久化机制正是 Harness 层的核心实现，确保 AI 工作流跨轮次、跨崩溃保持上下文一致。

---

## 🚀 快速开始

```bash
# 安装（macOS / Linux / Windows）
brew install alib8b8/tap/aflare
curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

```bash
# 一句话生成工作流
aflare create "monitor BTC price every 10 minutes and send telegram alert when > 70000"

# 运行
aflare run btc-monitor.yaml
```

📖 [完整入门 →](docs/getting-started.md)

---

## 💡 和别的工具有什么区别？

| 工具 | 问题 | aflare |
|------|------|--------|
| **AI Agent** | LLM 决定执行，不可预测，难审计 | AI 只翻译意图，YAML 保证执行 |
| **n8n** | 可视化工作流，但太重（Docker），不含 AI 层 | 单二进制，终端原生，AI 生成工作流 |
| **Bash** | 难写难维护，无错误恢复 | 自然语言生成，内置重试/熔断/检查点 |

**aflare 不是 AI 助手，也不是工作流工具——它是在 AI 和操作系统之间的一层：AI Execution Runtime。**

---

## ✨ 核心能力

### Runtime 保障（确定性执行）
- **DAG 并行调度** — 拓扑排序依赖调度，无依赖步骤并发执行
- **Harness 层：WAL 崩溃恢复 + Session 持久化** — append-only 持久化 + CRC32 校验，`--resume` 从中断处恢复；Session 跨轮次保持上下文，实现 Continual Harness
- **Saga 事务补偿** — 多步骤写入失败自动反向回滚
- **Idempotency** — Idempotency-Key + 原子占位 + 跨进程锁，防重复执行
- **Retry / Rate Limit / Circuit Breaker** — 指数退避 + 令牌桶 + 熔断器状态机

### 安全与合规
- HMAC 哈希链审计日志（防篡改）
- AES-GCM 加密 + PBKDF2（600K 迭代）
- Secret 自动脱敏（10+ 种模式：AWS/GitHub/JWT/私钥）
- SSRF 防护 / Path Traversal / Command Injection 白名单
- 出站数据量异常监控 + 熔断器自动隔离

### AI 集成
- 自然语言 → YAML 工作流生成
- 22+ 模型支持（OpenAI / DeepSeek / Qwen / GLM / Kimi / 昇腾 / 寒武纪 / 海光）
- 完全离线运行（Ollama 本地 LLM）
- LLM 智能路由（EWMA 延迟预测 + 帕累托成本排序）
- 100+ 内置模板，开箱即用

### 工程深度
- 表达式引擎：字节码 IR + 向量化批量求值
- TLA+ 形式化验证 DAG 调度器
- Prometheus 指标端点
- 单二进制部署，零运行时依赖
- CI 双架构验证（x86-64 + ARM64 鲲鹏）

📖 完整能力见 [文档](docs/) · [节点参考](docs/custom-nodes.md)

---

## 🏗️ 架构

```
┌──────────────────────────────────────────────────────┐
│                    aflare Runtime                     │
│                                                       │
│  ┌──────────┐   ┌──────────┐   ┌──────────────────┐  │
│  │ AI Intent │──▶│ Workflow │──▶│ Harness +         │  │
│  │ (LLM)    │   │ (YAML)   │   │ Deterministic     │  │
│  └──────────┘   └──────────┘   │ Executor          │  │
│                                 │                    │  │
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

## 🗺️ 路线图

| 版本 | 状态 | 重点 |
|------|------|------|
| v0.6 | ✅ | Agent 记忆基础设施、语音 AI 工具链、WAL 持久化、TLA+ 验证 |
| **v0.7** | **当前** | 金融场景增强（Saga / 幂等 / 审计链）、信创三芯适配（昇腾 / 寒武纪 / 海光）、宇树机器人 |
| v1.0 | 📅 | 稳定 API、LTS |

📖 [完整路线图 →](ROADMAP.md)

---

## 🔒 安全

aflare 内置多层安全防护：SSRF 防护、Path Traversal 防御、Command Injection 白名单、AES-GCM 加密、Secret 脱敏、HMAC 审计链、熔断器、出站监控。CI 自动运行 `gofmt` / `go vet` / `gosec` / `govulncheck`。

📖 [安全指南 →](SECURITY.md)

---

## 📚 文档

- [入门指南](docs/getting-started.md) · [YAML 语法](docs/getting-started.md#workflow-configuration)
- [数据流](docs/dataflow.md) · [调度](docs/scheduling.md) · [MCP](docs/mcp.md) · [插件](docs/plugins.md)
- [Web UI](docs/webui.md) · [可视化](docs/visualizer.md) · [自定义节点](docs/custom-nodes.md)

---

## 🤝 贡献

欢迎社区贡献！Fork → 创建分支 → 修改 → `go test ./...` → PR。

📖 [贡献指南 →](CONTRIBUTING.md)

---

## 📄 许可证

GNU Affero General Public License v3.0 — [LICENSE](LICENSE)

---

<div align="center">
  <p>Built with ❤️ for developers who want AI to actually execute.</p>
  <p>
    <a href="https://github.com/alib8b8/aflare">GitHub</a>
    ·
    <a href="https://github.com/alib8b8/aflare/issues">Issues</a>
    ·
    <a href="https://github.com/alib8b8/aflare/discussions">Discussions</a>
  </p>
</div>