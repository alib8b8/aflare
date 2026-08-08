<div align="center">
  <h1>aflare</h1>
  <p>🌍
    <a href="README.md">中文</a> ·
    <strong>English</strong>
  </p>
  <p><strong>AI shouldn't just answer your intent. It should execute it.</strong></p>
  <p><em>aflare — Deterministic Execution Runtime for AI</em></p>
  <p>Natural language describes intent → YAML workflow → Deterministic execution. AI is the brain, Workflow is the skeleton, Runtime is the hands.</p>

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

## What is this?

aflare is a **deterministic execution runtime for AI**. It separates AI's "understanding" from "execution":

```
Your words  →  AI translates intent  →  YAML Workflow  →  Runtime executes
(NL)           (LLM)                    (deterministic)    (DAG / WAL / Saga / Retry / Audit)
```

The problem with traditional AI Agents: the LLM both understands and decides execution — unstable, unpredictable, hard to audit.

aflare's approach: **AI only translates. The Runtime guarantees execution.** The YAML workflow defines exactly what each step does, what it depends on, and what happens on failure. The Runtime handles DAG scheduling, checkpoint recovery, Saga transaction compensation, circuit breaking, and auditing — every operation is traceable, replayable, and verifiable.

---

## Three-Layer Model

```
L1: AI Intent    —  "Monitor BTC, notify me if it drops 5%"
                        ↓
L2: Workflow     —  YAML deterministic workflow (schedule → get_price → condition → telegram)
                        ↓
L3: Runtime      —  Execution layer
                    ├── DAG parallel scheduling
                    ├── Checkpoint / Resume (WAL crash recovery)
                    ├── Saga transaction compensation
                    ├── Idempotency
                    ├── Retry / Rate Limit / Circuit Breaker
                    ├── HMAC audit chain
                    └── Secret redaction
```

**aflare's moat is not the LLM. It's the Runtime.**

---

## 🚀 Quick Start

```bash
# Install (macOS / Linux / Windows)
brew install alib8b8/tap/aflare
curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

```bash
# Generate a workflow in one line
aflare create "monitor BTC price every 10 minutes and send telegram alert when > 70000"

# Run it
aflare run btc-monitor.yaml
```

📖 [Full Getting Started →](docs/getting-started.md)

---

## 💡 How is this different?

| Tool | Problem | aflare |
|------|---------|--------|
| **AI Agent** | LLM decides execution — unpredictable, hard to audit | AI only translates intent, YAML guarantees execution |
| **n8n** | Visual workflow, but heavy (Docker), no AI layer | Single binary, terminal-native, AI generates workflows |
| **Bash** | Hard to write and maintain, no error recovery | NL generation, built-in retry/circuit-breaking/checkpoint |

**aflare is not an AI assistant, nor a workflow tool — it's a layer between AI and the OS: an AI Execution Runtime.**

---

## ✨ Core Capabilities

### Runtime Guarantees (Deterministic Execution)
- **DAG Parallel Scheduling** — topological sort dependency scheduling, independent steps run concurrently
- **WAL Crash Recovery** — append-only persistence + CRC32, `--resume` recovers from interruption
- **Saga Transaction Compensation** — multi-step write failures auto-rolled-back in reverse
- **Idempotency** — Idempotency-Key + atomic placeholder + cross-process lock, prevents duplicate execution
- **Retry / Rate Limit / Circuit Breaker** — exponential backoff + token bucket + breaker state machine

### Security & Compliance
- HMAC hash-chain audit log (tamper-proof)
- AES-GCM encryption + PBKDF2 (600K iterations)
- Auto secret redaction (10+ patterns: AWS/GitHub/JWT/private keys)
- SSRF protection / Path Traversal / Command Injection whitelist
- Outbound data volume anomaly monitor + circuit breaker auto-isolation

### AI Integration
- Natural language → YAML workflow generation
- 22+ model support (OpenAI / DeepSeek / Qwen / GLM / Kimi / Ascend / Cambricon / Hygon)
- Fully offline capable (Ollama local LLM)
- LLM smart routing (EWMA latency prediction + Pareto cost sorting)
- 100+ built-in templates, ready to use

### Engineering Depth
- Expression engine: bytecode IR + vectorized batch evaluation
- TLA+ formal verification of DAG scheduler
- Prometheus metrics endpoint
- Single binary, zero runtime dependencies
- CI validates both architectures (x86-64 + ARM64 Kunpeng)

📖 Full capabilities: [Docs](docs/) · [Node Reference](docs/custom-nodes.md)

---

## 🏗️ Architecture

```
┌──────────────────────────────────────────────────────┐
│                    aflare Runtime                     │
│                                                       │
│  ┌──────────┐   ┌──────────┐   ┌──────────────────┐  │
│  │ AI Intent │──▶│ Workflow │──▶│ Deterministic     │  │
│  │ (LLM)    │   │ (YAML)   │   │ Executor          │  │
│  └──────────┘   └──────────┘   │                    │  │
│                                 │ • DAG Scheduler   │  │
│                                 │ • WAL / Checkpoint│  │
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

## 🗺️ Roadmap

| Version | Status | Focus |
|---------|--------|-------|
| v0.6 | ✅ | Agent memory infrastructure, voice AI toolchain, WAL persistence, TLA+ verification |
| **v0.7** | **Current** | Financial scenario (Saga / Idempotency / Audit chain), triple-chip Xinchuang (Ascend / Cambricon / Hygon), Unitree robot |
| v1.0 | 📅 | Stable API, LTS |

📖 [Full Roadmap →](ROADMAP.md)

---

## 🔒 Security

aflare has built-in multi-layer security: SSRF protection, Path Traversal defense, Command Injection whitelist, AES-GCM encryption, Secret redaction, HMAC audit chain, circuit breakers, outbound monitoring. CI auto-runs `gofmt` / `go vet` / `gosec` / `govulncheck`.

📖 [Security Guide →](SECURITY.md)

---

## 📚 Documentation

- [Getting Started](docs/getting-started.md) · [YAML Syntax](docs/getting-started.md#workflow-configuration)
- [Dataflow](docs/dataflow.md) · [Scheduling](docs/scheduling.md) · [MCP](docs/mcp.md) · [Plugins](docs/plugins.md)
- [Web UI](docs/webui.md) · [Visualizer](docs/visualizer.md) · [Custom Nodes](docs/custom-nodes.md)

---

## 🤝 Contributing

We welcome contributions! Fork → branch → change → `go test ./...` → PR.

📖 [Contributing →](CONTRIBUTING.md)

---

## 📄 License

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