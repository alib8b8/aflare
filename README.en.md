<div align="center">
  <h1>aflare</h1>
  <p>
    <a href="README.md">中文</a> ·
    <strong>English</strong>
  </p>
  <p><strong>AI Beyond Chat — Get Things Done</strong></p>
  <p><em>ReAct Reasoning Loop · 300+ Skill Templates · Deterministic Workflow Execution · 10 Pluggable Capabilities</em></p>

  <p>
    <a href="https://github.com/alib8b8/aflare/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/aflare/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status" />
    </a>
    <a href="https://github.com/alib8b8/aflare/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/aflare?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-AGPL%20v3.0-blue.svg?style=flat-square" alt="license" />
    </a>
  </p>
</div>

---

## Quick Start

```bash
# Install
brew install alib8b8/tap/aflare
# or: curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash

# Optional: install bubblewrap for full sandbox isolation (required by code_interpreter node)
# Ubuntu/Debian: sudo apt install bubblewrap
# macOS:        brew install bubblewrap
# Fedora:       sudo dnf install bubblewrap
```

```bash
# Generate workflow from keywords
aflare create "monitor BTC price every 10 minutes, alert via Telegram when > 70000"
# Output: workflow generated → btc-monitor.yaml

# Run the workflow
aflare run btc-monitor.yaml

# Interactive AI Agent chat (ReAct Agent + 300+ skills)
aflare chat
# Or: aflare chat -p deepseek -m deepseek-chat

# Daemon-mode Agent (stdin + scheduler fusion) + pluggable capabilities
aflare agent -c reflection,bdi,utility
```

---

## Project Status

aflare is currently at **v0.7 early stage**. Core Runtime capabilities (DAG scheduling, WAL crash recovery, Saga transaction compensation, idempotency, retry/circuit-breaking) are implemented and verified by CI. Agent features (ReAct Agent chat, 300+ skill templates, 10 pluggable capabilities, unified event loop) are complete. Some advanced features (domestic chip support, Unitree robot) are experimental. Feedback and contributions welcome.

---

## What is this?

aflare is both a **local-first automation Agent** and a **deterministic workflow execution engine**. Two modes, one core:

```
Conversational Agent             Declarative Workflow
─────────────────              ─────────────────
aflare chat                    aflare create
  ↓                              ↓
ReAct Agent reasoning          Keyword matching generation
  ↓                              ↓
Invoke 300+ skill templates      YAML workflow
  ↓                              ↓
Tool execution → Reflect →      DAG scheduled execution
  Optimize
```

**Agent Mode**: Launch via `aflare chat` or `aflare agent`. Built-in ReAct reasoning loop, 300+ pre-built skill templates (16 domains), 10 pluggable capability types (reflection, human-in-the-loop, BDI goal management, utility-driven optimization, etc.).

**Workflow Mode**: `aflare create` converts descriptions into YAML workflows via keyword matching. The YAML defines exactly what each step does, its dependencies, and failure handling. The Runtime handles DAG scheduling, WAL crash recovery, Saga transaction compensation, circuit breaking, and auditing — every operation is traceable, replayable, and verifiable.

---

## Three-Layer Model

```
L0: Agent        —  "Monitor BTC, notify me if it drops 5%"
                    ├── ReAct reasoning loop (think → call tool → observe → answer)
                    ├── 300+ skill templates (16 domains)
                    └── 10 pluggable capabilities (reflection/HITL/BDI/utility etc.)
                       ↓
L1: Workflow     —  YAML deterministic workflow (schedule → get_price → condition → telegram)
                       ↓
L2: Runtime      —  Execution layer
                    ├── DAG parallel scheduling
                    ├── Checkpoint / Resume (WAL crash recovery)
                    ├── Session persistence (cross-turn context)
                    ├── Saga transaction compensation
                    ├── Idempotency
                    ├── Retry / Rate Limit / Circuit Breaker
                    ├── HMAC audit chain
                    └── Secret redaction
```

---

## How is this different?

| Tool | Problem | aflare |
|------|---------|--------|
| **AI Agent (general)** | LLM decides execution — unpredictable, hard to audit | Deterministic YAML workflows as execution backend, traceable and replayable |
| **n8n** | Visual workflow, but heavier (Docker), no built-in generation | Single binary, terminal-native, keyword-based workflow generation |
| **Bash** | Hard to write and maintain, no error recovery | Description-based generation, built-in retry/circuit-breaking/checkpoint |
| **LangChain/AutoGPT** | Pure Agent without deterministic execution guarantees | Agent + Runtime dual mode, Agent can degrade to deterministic workflow |
| **Claude Code/Cursor** | Cloud-dependent, code-editing focused | Local-first, general automation, 300+ skills, auditable execution |

---

## Core Capabilities

### Feature Matrix

| Feature | Status | Verification |
|---------|--------|-------------|
| **ReAct Agent Chat** (`aflare chat`) | ✅ | Tested |
| **Daemon-mode Agent** (`aflare agent`) | ✅ | Tested |
| **300+ Skill Templates** (16 domains) | ✅ | Tested |
| **10 Pluggable Capabilities** (reflection/HITL/BDI/utility etc.) | ✅ | Tested |
| **Multi-source Input Fusion** (stdin + scheduler + filewatch) | ✅ | Tested |
| DAG Parallel Scheduling | ✅ | Tested + TLA+ formal verification |
| WAL Crash Recovery + Session Persistence | ✅ | Tested |
| Saga Transaction Compensation | ✅ | Tested |
| Idempotency | ✅ | Tested |
| Retry / Rate Limit / Circuit Breaker | ✅ | Tested |
| HMAC Audit Chain | ✅ | Tested |
| Secret Redaction | ✅ | Tested |
| Expression Engine (bytecode IR + vectorized) | ✅ | Tested |
| Keyword-based Workflow Generation | ✅ | Tested |
| MCP Protocol Support (Server/Client) | ✅ | Tested |
| LLM Nodes (22+ models) | ✅ | Tested |
| Security Levels (L0-L3) | ✅ | Tested |

> See [Experimental](#experimental) below for experimental features.

### Agent Capabilities (Conversational + Daemon)

- **ReAct Reasoning Loop** — Think → Call Tool → Observe → Answer, with native function calling and JSON fallback
- **300+ Pre-built Skill Templates** — Covering 16 domains (Finance, Healthcare, Supply Chain, DevOps, etc.), auto-matched and executed by Agent
- **Unified Event Loop** — Conversational (`aflare chat`) and daemon (`aflare agent`) share the same `AgentLoop` core, supporting stdin / scheduler / filewatch multi-source input fusion
- **10 Pluggable Capabilities** — Enable on demand, mapping the complete Agent type taxonomy:

| Capability | Type | Description |
|------------|------|-------------|
| `reflection` | Self-Critique | Auto-evaluate output quality after each turn, trigger self-correction |
| `human-in-loop` | Human-in-the-Loop | Pause at critical decisions, request human confirmation |
| `bdi` | Belief-Desire-Intention | Maintain goal tracking, belief extraction, periodic goal context injection |
| `utility` | Utility-Driven | 6-dimension scoring (correctness/completeness/efficiency/safety/clarity/actionability), optimize decisions |
| `adaptive` | Learning/Adaptive | Learn from feedback, improve across turns |
| `memory` | Stateful | Cross-session long-term memory, remember user preferences |
| `planning` | Planning | Generate plans before acting, execute step by step |
| `multi-agent` | Multi-Agent Collaboration | Decompose complex tasks, multi-role coordination |
| `workflow` | Workflow/Pipeline | Prioritize existing templates, stable and predictable |
| `simulation` | Simulation/Generative | Human-like behavior modeling, scenario generation |

### Runtime Guarantees (Deterministic Execution)
- **DAG Parallel Scheduling** — topological sort dependency scheduling, independent steps run concurrently
- **WAL Crash Recovery + Session Persistence** — append-only persistence + CRC32, `--resume` recovers from interruption; Session preserves context across turns
- **Saga Transaction Compensation** — multi-step write failures auto-rolled-back in reverse
- **Idempotency** — Idempotency-Key + atomic placeholder + cross-process lock, prevents duplicate execution
- **Retry / Rate Limit / Circuit Breaker** — exponential backoff + token bucket + breaker state machine

### Security & Compliance
- HMAC hash-chain audit log (tamper-proof)
- AES-GCM encryption + PBKDF2 (600K iterations)
- Auto secret redaction (10+ patterns: AWS/GitHub/JWT/private keys)
- SSRF protection / Path Traversal / Command Injection whitelist
- Outbound data volume anomaly monitor + circuit breaker auto-isolation

### Workflow Generation
- Keyword-based YAML workflow generation (`aflare create`, see [`generator.go`](internal/workflow/generator.go))
- 100+ built-in templates

### LLM Nodes (call LLM APIs within workflows)
- 22+ model support (OpenAI / DeepSeek / Qwen / GLM / Kimi, etc.)
- Fully offline capable (Ollama local LLM)
- LLM smart routing (EWMA latency prediction + Pareto cost sorting)

### MCP Protocol Support
- Built-in MCP Server, connectable by any MCP client (Claude, VS Code, Cursor, etc.)
- Provides workflow execution, validation, node query, code graph, and other tools
- Built-in MCP Client, workflows can call external MCP services directly

### Engineering
- Expression engine: bytecode IR + vectorized batch evaluation
- DAG scheduler formally verified with TLA+ (spec at [`docs/tla/dag_scheduler.tla`](docs/tla/dag_scheduler.tla), bounded model-checking via `dag_formal_test.go`)
- Prometheus metrics endpoint
- Single binary, zero runtime dependencies
- CI validates both architectures (x86-64 + ARM64)

### Experimental
- Ascend / Cambricon / Hygon domestic chip support (basic functionality available, under active development)
- Unitree robot integration (simulate mode available, physical mode requires hardware)

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    aflare                             │
│                                                       │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Agent Layer (L0)                                  │ │
│  │                                                    │ │
│  │  aflare chat / aflare agent                       │ │
│  │  ┌──────────┐  ┌──────────┐  ┌────────────────┐  │ │
│  │  │ ReAct    │  │ 300+     │  │ 10 Pluggable   │  │ │
│  │  │ Reasoning│  │ Skills   │  │ Capabilities   │  │ │
│  │  └──────────┘  └──────────┘  └────────────────┘  │ │
│  │                                                    │ │
│  │  ┌──────────────────────────────────────────────┐ │ │
│  │  │ AgentLoop Unified Event Loop                   │ │ │
│  │  │ stdin · scheduler · filewatch · MCP · HTTP   │ │ │
│  │  └──────────────────────────────────────────────┘ │ │
│  └──────────────────────────────────────────────────┘ │
│                        ↓                               │
│  ┌──────────┐   ┌──────────┐   ┌──────────────────┐  │
│  │ Intent   │──▶│ Workflow │──▶│ Deterministic     │  │
│  │ (desc.)  │   │ (YAML)   │   │ Executor          │  │
│  └──────────┘   └──────────┘   │                    │  │
│                                 │ • DAG Scheduler   │  │
│                                 │ • WAL / Checkpoint│  │
│                                 │ • Session Persist │  │
│                                 │ • Saga / Retry    │  │
│                                 │ • Circuit Breaker │  │
│                                 │ • Audit / HMAC    │  │
│                                 └──────────────────┘  │
│                                                       │
│  ┌──────────────────────────────────────────────────┐ │
│  │ Execution Targets                                 │ │
│  │ Software (API/Web/DB) • Devices (Phone/HarmonyOS) │ │
│  │ Robots (Unitree/Drone/Arm) • IoT                  │ │
│  └──────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

---

## Roadmap

| Version | Status | Focus |
|---------|--------|-------|
| v0.6 | Done | Agent memory infrastructure, voice AI toolchain, WAL persistence, TLA+ verification |
| v0.7 | Done | Financial scenario enhancement (Saga / Idempotency / Audit chain), ReAct Agent chat, 300+ skill templates, 10 pluggable capabilities, Agent unified event loop |
| **v0.8** | **Current** | Domestic chip support refinement, Unitree robot physical support, Agent capability deepening |
| v1.0 | Planned | Stable API, LTS |

See [CHANGELOG.md](CHANGELOG.md) for details.

---

## Security

aflare has built-in multi-layer security with four security levels (`--security-level`):

| Level | Description |
|-------|-------------|
| **L0** | Relaxed: all nodes allowed, sandbox degradation only warns |
| **L1** | Standard: warn on sandbox degradation, heuristic blocking |
| **L2** | Strict: refuse code_interpreter execution without bwrap sandbox, command whitelist validation |
| **L3** | Maximum: disable code_interpreter nodes, strictest security policy |

Additional protections: SSRF protection, Path Traversal defense, Command Injection whitelist, AES-GCM encryption, Secret redaction, HMAC audit chain, circuit breakers, outbound monitoring. CI auto-runs `gofmt` / `go vet` / `gosec` / `govulncheck`.

[Security Guide →](SECURITY.md)

---

## Documentation

- [Getting Started](docs/getting-started.md) · [Tutorial](docs/tutorial.md) · [YAML Syntax](docs/getting-started.md#workflow-configuration)
- [Dataflow](docs/dataflow.md) · [Scheduling](docs/scheduling.md) · [MCP](docs/mcp.md) · [Plugins](docs/plugins.md)
- [Web UI](docs/webui.md) · [Visualizer](docs/visualizer.md) · [Custom Nodes](docs/custom-nodes.md)
- [API Reference](docs/api.md) · [Nodes Reference](docs/nodes-reference.md)
- [Deployment](docs/deployment.md) · [Docker](docs/docker.md) · [Distributed](docs/distributed.md) · [Multi-Tenancy](docs/tenants.md)
- [Troubleshooting](docs/troubleshooting.md)

---

## Contributing

We welcome contributions! Fork → branch → change → `go test ./...` → PR.

[Contributing →](CONTRIBUTING.md)

---

## License

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