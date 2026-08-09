<div align="center">
  <h1>aflare</h1>
  <p>
    <a href="README.md">中文</a> ·
    <strong>English</strong>
  </p>
  <p><strong>Keywords describe intent → YAML workflow → Deterministic execution</strong></p>
  <p><em>Deterministic Workflow Execution Runtime</em></p>

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
```

```bash
# Generate workflow from keywords and run
aflare create "monitor BTC price every 10 minutes, alert via Telegram when > 70000"
aflare run btc-monitor.yaml
```

---

## Project Status

aflare is currently in **early-stage v0.7**. Core Runtime capabilities (DAG scheduling, WAL crash recovery, Saga transaction compensation, idempotency, retry/circuit-breaking) are implemented and verified by CI. Some advanced features (domestic chip support, Unitree robot) are experimental. Feedback and contributions welcome.

---

## What is this?

aflare separates the "description" of a workflow from its "execution":

```
Your words  →  Keyword matching  →  YAML Workflow  →  Runtime executes
(description)  (regex + keywords)    (deterministic)    (DAG / WAL / Saga / Retry / Audit)
```

`aflare create` converts descriptions into YAML workflows via regex and keyword matching (**not LLM-generated**, see [`generator.go`](internal/workflow/generator.go)). The YAML workflow defines exactly what each step does, what it depends on, and what happens on failure. The Runtime handles DAG scheduling, checkpoint recovery, Saga transaction compensation, circuit breaking, and auditing — every operation is traceable, replayable, and verifiable.

---

## Three-Layer Model

```
L1: Intent       —  "Monitor BTC, notify me if it drops 5%"
                        ↓
L2: Workflow     —  YAML deterministic workflow (schedule → get_price → condition → telegram)
                        ↓
L3: Runtime      —  Execution layer
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
| **AI Agent** | LLM decides execution — unpredictable, hard to audit | Deterministic YAML workflows, execution is traceable and replayable |
| **n8n** | Visual workflow, but heavier (Docker), no built-in generation | Single binary, terminal-native, keyword-based workflow generation |
| **Bash** | Hard to write and maintain, no error recovery | Description-based generation, built-in retry/circuit-breaking/checkpoint |

---

## Core Capabilities

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

### Engineering
- Expression engine: bytecode IR + vectorized batch evaluation
- DAG scheduler formally verified with TLA+ (spec at [`docs/tla/dag_scheduler.tla`](docs/tla/dag_scheduler.tla), bounded model-checking via `dag_formal_test.go`)
- Prometheus metrics endpoint
- Single binary, zero runtime dependencies
- CI validates both architectures (x86-64 + ARM64)

#### Experimental
- Ascend / Cambricon / Hygon domestic chip support (basic functionality available, under active development)
- Unitree robot integration (simulate mode available, physical mode requires hardware)

---

## Architecture

```
┌──────────────────────────────────────────────────────┐
│                    aflare Runtime                     │
│                                                       │
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
| **v0.7** | **Current** | Financial scenario (Saga / Idempotency / Audit chain), experimental domestic chip support, experimental Unitree robot |
| v1.0 | Planned | Stable API, LTS |

[Full Roadmap →](ROADMAP.md)

---

## Security

aflare has built-in multi-layer security: SSRF protection, Path Traversal defense, Command Injection whitelist, AES-GCM encryption, Secret redaction, HMAC audit chain, circuit breakers, outbound monitoring. CI auto-runs `gofmt` / `go vet` / `gosec` / `govulncheck`.

[Security Guide →](SECURITY.md)

---

## Documentation

- [Getting Started](docs/getting-started.md) · [YAML Syntax](docs/getting-started.md#workflow-configuration)
- [Dataflow](docs/dataflow.md) · [Scheduling](docs/scheduling.md) · [MCP](docs/mcp.md) · [Plugins](docs/plugins.md)
- [Web UI](docs/webui.md) · [Visualizer](docs/visualizer.md) · [Custom Nodes](docs/custom-nodes.md)

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