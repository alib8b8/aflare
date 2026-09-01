<div align="center">
  <h1>aflare</h1>
  <p>
    <a href="README.md">English</a> ·
    <strong>简体中文</strong>
  </p>
  <p><strong>让 AI 告别聊天，开始执行</strong></p>
  <p><em>本地优先 · 数据不出本地 · 连接你自己的 LLM / 文件 / 笔记 / 数据库</em></p>

  <p>
    <a href="https://github.com/alib8b8/aflare/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/aflare/ci.yml?branch=main&style=flat-square&label=CI" alt="CI 状态" />
    </a>
    <a href="https://github.com/alib8b8/aflare/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/aflare?display_name=tag&include_prereleases&style=flat-square" alt="发布版本" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-AGPL%20v3.0-blue.svg?style=flat-square" alt="许可证" />
    </a>
  </p>
</div>

---

## 快速开始

**macOS / Linux：**

```bash
curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

**Windows（PowerShell）：**

```powershell
irm https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 | iex
```

<details>
<summary><b>其他安装方式</b>（手动下载 / deb · rpm / GitHub Action）</summary>

```bash
# 手动下载二进制
#   GitHub:  https://github.com/alib8b8/aflare/releases
#   国内加速: https://ghproxy.com/https://github.com/alib8b8/aflare/releases
```

- `deb` / `rpm` 包见每个 Release 的附件。
- 国内网络下安装脚本会自动切换到镜像加速下载。

把 aflare 工作流作为 CI 步骤运行（预编译二进制，无需 Docker 构建）：

```yaml
- uses: alib8b8/aflare/action@v0.12.0
  with:
    workflow: .aflare/pr-review.yaml
```

详见 [action/README.md](action/README.md)。

</details>

**60 秒上手：**

```bash
aflare doctor                      # 环境自检（零配置）
aflare run examples/content-processor.yaml   # 读取 post.md → 转 HTML → 写 post.html
aflare init                        # 配置 LLM（本地 Ollama 或云厂商）

aflare create "每 10 分钟检查贵州茅台股价，超过 1400 发飞书通知"
aflare run stock-monitor.yaml      # 关键词生成工作流（加 --ai 用 LLM 生成）

aflare chat                        # 交互式 ReAct Agent 对话
```

> 可选：安装 bubblewrap 获得完整沙箱隔离（`code_interpreter` 节点需要）——`sudo apt install bubblewrap` / `brew install bubblewrap`。
>
> 生成的行情监控工作流数据来自公开行情接口，可能有延迟，仅供个人研究参考，不构成投资建议。

---

## aflare 是什么？

一个**本地优先的自动化 Agent**，同时也是一个**确定性工作流引擎**——单个二进制。你显式授权 AI 访问自己的数据（目录、笔记库、本地数据库），AI 在你划定的权限上限内确定性地工作。

```
aflare chat / agent          aflare create
  ReAct Agent                  → YAML 工作流
  （对话式）                     ↓
       ↓                    DAG 调度执行
  节点工具                  （WAL 恢复 · Saga · 重试 · 审计）
```

当前版本 **v0.12.0**，目标用户先做本地——本地数据留在本机，aflare 是 AI 与这些数据之间「确定且安全」的控制层。

---

## 核心特性

- **本地优先，数据不出本地** —— 单二进制、零运行时依赖、内存约 10–30MB；工作流、执行历史、记忆和密钥全部留在本地磁盘；完全支持离线；无使用遥测。
- **接入你自己的 LLM** —— Ollama / vLLM / LM Studio / 任何 OpenAI 兼容接口；回环地址无需 API Key；没有 LLM 时关键词匹配兜底，离线照样能用。多厂商路由降成本、防锁定：OpenRouter 一个端点通吃各家模型，或用原生 `llm_router` 节点按成本/延迟路由并自动降级。见 [LLM 路由](docs/openrouter.md)。
- **本地数据与 API 连接器** —— 命名、显式授权的目录、数据库与 HTTP API 连接器（`files` / `notes` / `sqlite` / `mysql` / `postgres` / `http`）；凭据只存于加密密钥库，权限上限只能收紧、不能放宽。见 [Connector API](docs/connector-api.md)。
- **确定性运行时** —— DAG 并行调度（TLA+ 形式化验证）、WAL 崩溃恢复 + `--resume` 断点续跑、Saga 事务补偿、幂等、重试/限流/熔断。每个操作可追溯、可重放、可验证。
- **Agent + 工作流双模式** —— 对话式 ReAct Agent（`aflare chat`）与守护进程 Agent（`aflare agent`）共用一个内核；6 种可插拔能力（反思 / 人工介入 / 效用驱动 / 记忆 / 规划 / 工作流）。
- **Agent 互联与指挥** —— aflare 指挥和监督其他 Agent：CLI 通道（`codex` / `claude` / `gemini` 或任意通用 CLI）与 A2A 协议通道，`supervisor` 节点真实委派，单 Agent 失败不拖垮整批。
- **安全内建** —— HMAC 防篡改审计链、AES-GCM 加密密钥库、自动密钥脱敏、SSRF / 路径穿越 / 命令注入防御、出站异常监控 + 自动熔断隔离，四个安全等级（L0–L3）。
- **可扩展生态** —— MCP Server / Client、Go 自定义节点、社区插件、CI 用的 [GitHub Action](action/README.md)、30+ 内置 LLM 供应商、面向 OpenClaw 生态的 [OpenClaw 插件](contrib/openclaw/README.md)。
- **开箱即用的示例** —— [`examples/real-world/`](examples/real-world/) 真实场景工作流包：工业监控（OpenFOAM 发散看门狗、相似案例 RAG 分诊）、DevOps CI 流水线、研究、批量处理，以及多智能体角色流水线（分析师→研究员→交易员→风控的交易团队、数字公司市场部与销售部）。

---

## 安全

四个安全等级（`--security-level`）：**L0** 宽松 → **L3** 最高（L2 拒绝无沙箱的 `code_interpreter`，L3 直接禁用）。CI 在每个 PR 上运行 `gofmt` / `go vet` / `gosec` / `govulncheck`。

[安全指南 →](SECURITY.md)

---

## 文档

- [快速上手](docs/getting-started.md) · [教程](docs/tutorial.md) · [YAML 语法](docs/getting-started.md#step-2-create-your-first-workflow)
- [数据流](docs/dataflow.md) · [调度](docs/scheduling.md) · [MCP](docs/mcp.md) · [插件](docs/plugins.md) · [连接器](docs/connector-api.md) · [LLM 路由](docs/openrouter.md)
- [Web UI](docs/webui.md) · [可视化](docs/visualizer.md) · [自定义节点](docs/custom-nodes.md)
- [API 参考](docs/api.md) · [节点参考](docs/nodes-reference.md)
- [部署](docs/deployment.md) · [Docker](docs/docker.md) · [多租户](docs/tenants.md)
- [故障排查](docs/troubleshooting.md) · [更新日志](CHANGELOG.md)

---

## 参与贡献

欢迎贡献！[贡献指南 →](CONTRIBUTING.md)

---

## 许可证

GNU Affero General Public License v3.0 —— [LICENSE](LICENSE)

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
