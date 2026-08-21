<div align="center">
  <h1>aflare</h1>
  <p>
    <a href="README.md">中文</a> ·
    <strong>English</strong>
  </p>
  <p><strong>AI Beyond Chat — Get Things Done</strong></p>
  <p><em>Local-first · Data Stays Local · Connect Your Own LLM / Database / Knowledge Base · ReAct Reasoning · 330+ Skill Templates · Deterministic Workflow Execution</em></p>

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

### Install

**macOS / Linux** — one-line install (auto-detects OS & arch, verifies checksum):

```bash
curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh | bash
```

**Windows** — PowerShell one-line install (auto-detects arch, adds to user PATH):

```powershell
irm https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 | iex
```

<details>
<summary><b>Other install methods</b> (Homebrew / manual download / deb · rpm)</summary>

```bash
# Homebrew (macOS / Linuxbrew)
brew install alib8b8/tap/aflare

# Manual binary download
#   GitHub:  https://github.com/alib8b8/aflare/releases
#   CN accelerated: https://ghproxy.com/https://github.com/alib8b8/aflare/releases
```

- `deb` / `rpm` packages are attached to each Release.
- The install script auto-switches to mirror-accelerated downloads on CN networks.

</details>

> **Optional**: install bubblewrap for full sandbox isolation (required by `code_interpreter` node)
> - Ubuntu/Debian: `sudo apt install bubblewrap`
> - macOS:        `brew install bubblewrap`
> - Fedora:       `sudo dnf install bubblewrap`

```bash
# 1. Environment self-check (zero-config, runs immediately)
aflare doctor

# 2. Zero-config example: read post.md → convert to HTML → write post.html
aflare run examples/content-processor.yaml

# 3. Configure an LLM (interactive wizard: local Ollama or cloud provider)
aflare init

# 4. Generate a workflow from keywords (no LLM needed, pure template match; add --ai for LLM-generated complex ones)
aflare create "monitor BTC price every 10 minutes, alert via Telegram when > 70000"
# Output: workflow generated → btc-monitor.yaml
aflare run btc-monitor.yaml

# Crypto (CoinGecko), US stocks, HK stocks and A-shares all work — pick your market:
aflare create "check usAAPL stock price every 10 minutes, alert via Telegram when > 320"    # US: us<TICKER>
aflare create "check hk00700 stock price every 10 minutes, alert via Telegram when > 440"   # HK: hk<5-digit code>
aflare create "每 10 分钟检查贵州茅台 600519 股价，超过 1400 发 Telegram 通知"                    # A-share: 6-digit code

# 5. Interactive AI Agent chat (ReAct Agent + 330+ skills)
aflare chat
# Or: aflare chat -p deepseek -m deepseek-chat

# Daemon-mode Agent (stdin + scheduler fusion) + pluggable capabilities
aflare agent -c reflection,planning,utility
```

---

## Project Status

aflare is currently at **v0.9.0 stage** (v0.10 in development). Core Runtime capabilities (DAG scheduling, WAL crash recovery, Saga transaction compensation, idempotency, retry/circuit-breaking) are implemented and verified by CI. v0.9.0 delivers Chinese national cryptography support (SM3 audit chain / SM4 secrets, opt-in), audit-chain security hardening (per-install random HMAC key, cross-process log lock, bundle truncation-forgery defense), one-command MCP server install (`aflare mcp install`), and byte-identical 0.8.x upgrade compatibility. The current dev build adds: **Agent Plugins 1.0.0 host support** (bidirectional plugin ecosystem with VS Code / Cursor / Copilot and other clients), **MemHarness memory critique-reconstruction mode** (memory is a cue to reconstruct, not a fact of the current task), **step-level typed output contracts and bounded preview inputs**, **watermark deployment tracing**, plus a security self-audit round that fixed plugin path traversal, symlink bypass and memory data races (see [CHANGELOG](CHANGELOG.md)). Local inference services running on domestic chips (Ascend/Cambricon/Hygon) are accessed through OpenAI-compatible endpoints (no native SDK integration), and support keeps improving. Hardware device control (robots etc.) is not built in — users can integrate via custom nodes or MCP Server, with data staying on their intranet.

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
Invoke 330+ skill templates      YAML workflow
  ↓                              ↓
Tool execution → Reflect →      DAG scheduled execution
  Optimize
```

**Agent Mode**: Launch via `aflare chat` or `aflare agent`. Built-in ReAct reasoning loop, 330+ pre-built skill templates (17 domains), 6 pluggable capability types (reflection, human-in-the-loop, utility-driven optimization, memory, etc.).

**Workflow Mode**: `aflare create` converts descriptions into YAML workflows via keyword matching. The YAML defines exactly what each step does, its dependencies, and failure handling. The Runtime handles DAG scheduling, WAL crash recovery, Saga transaction compensation, circuit breaking, and auditing — every operation is traceable, replayable, and verifiable.

---

## Three-Layer Model

```
L0: Agent        —  "Monitor BTC / US stocks / HK stocks, notify me on threshold"
                    ├── ReAct reasoning loop (think → call tool → observe → answer)
                    ├── 330+ skill templates (17 domains)
                    └── 6 pluggable capabilities (reflection/HITL/utility etc.)
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

## Project Strengths

aflare is built for intranet / local-first users — enterprises and individuals who are sensitive about data privacy and security. Core strengths:

**Local-first, data never leaves your machine** — Single binary with zero runtime deps, runs in ~5MB RAM; workflows, execution history, memory and secrets all stay on local disk; API keys are injected via environment variables or the OS keyring, never written in cleartext to `config.yaml`; fully offline-capable (offline install, `aflare doctor --offline`, WebUI Mermaid offline fallback, 330+ templates embedded in the binary and auto-released on first run).

**Connect your own LLM** — Ollama / vLLM / LM Studio / local DeepSeek / any OpenAI-compatible endpoint, with loopback addresses (127.0.0.1 / localhost) requiring no API key. With a local LLM, the LLM drives intent understanding and dynamic workflow generation (`--ai` / `chat`); without one, keyword matching falls back so offline use still works.

**Connect your own databases and knowledge bases** — SQL Query node connects directly to your database, RAG node + vector store + document parsing hook into your knowledge base, MCP protocol bridges external services, and custom nodes let you write any integration in Go. aflare never exfiltrates your data and telemetry is opt-out — it only does the work, without leaking internal enterprise data.

**Deterministic execution guarantees** — YAML declarative workflows: every step's action, dependencies and failure handling are fully determined. DAG parallel scheduling (TLA+ formally verified), WAL crash recovery + checkpoint (`--resume` from the interruption point), cross-turn session persistence, Saga transactional compensation, idempotency (Idempotency-Key + cross-process lock), retry / rate limit / circuit breaker. Every operation is traceable, replayable, verifiable.

**Dual Agent + Workflow mode** — Conversational Agent (`aflare chat`, ReAct reasoning loop) and daemon Agent (`aflare agent`, multi-source fusion of stdin + scheduled tasks + file watching) share one core; 6 pluggable capabilities (reflection / human-in-loop / utility-driven / memory / planning / workflow); an Agent can degrade into a deterministic workflow, combining flexibility with determinism. 330+ built-in skill templates across 17 domains.

**Security & compliance** — HMAC hash-chain audit log (tamper-evident), AES-GCM encryption + PBKDF2 (600K iterations), automatic secret redaction (10+ patterns: AWS/GitHub/JWT/private keys), SSRF / path-traversal / command-injection whitelisting, outbound-data anomaly monitoring + automatic circuit-breaker isolation, four security levels (L0-L3) tightened on demand.

**One-command onboarding, smooth offline** — `aflare doctor` environment self-check, `aflare init` interactive setup wizard, `aflare template run <id>` one-command template execution (no clone or path lookup needed), smart unknown-command hints (did-you-mean), zero-config examples ready to run immediately.

**Extensible ecosystem** — Custom nodes (Go), MCP Server / Client (`aflare mcp install` for built-in community servers), **Agent Plugins 1.0.0 bidirectional interop** (`aflare marketplace install <dir>` installs any plugin conforming to the open standard; `aflare marketplace export` exports aflare skills to VS Code / Cursor / Copilot and other clients), plugin system (community `.so`), community template contributions (`aflare template submit`), one-command scenario packs (`aflare install-pack`). 330+ skills already cover 17 domains, targeting 1000+.

**Engineering quality** — Expression engine (bytecode IR + vectorized batch evaluation), Prometheus metrics endpoint, CI dual-architecture verification (x86-64 + ARM64), domestic-chip local inference via OpenAI-compatible endpoints (Ascend / Cambricon / Hygon).

---

## Core Capabilities

### Feature Matrix

| Feature | Status | Verification |
|---------|--------|-------------|
| **ReAct Agent Chat** (`aflare chat`) | ✅ | Tested |
| **Daemon-mode Agent** (`aflare agent`) | ✅ | Tested |
| **330+ Skill Templates** (17 domains) | ✅ | Tested |
| **6 Pluggable Capabilities** (reflection/HITL/utility etc.) | ✅ | Tested |
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
| **Agent Plugins 1.0.0 Bidirectional Interop** (`marketplace install/export`, v0.10 dev) | ✅ | Tested |
| **MemHarness Memory Critique-Reconstruction** (`harness_search` + session critique injection, v0.10 dev) | ✅ | Tested |
| **Step-level Output Contracts** (`output_schema`, v0.10 dev) | ✅ | Tested |
| **Bounded Preview Inputs** (`preview_input`, 16KiB, v0.10 dev) | ✅ | Tested |
| LLM Nodes (18 built-in providers, any OpenAI-compatible model usable) | ✅ | Tested |
| Security Levels (L0-L3) | ✅ | Tested |

> See [Experimental](#experimental) below for experimental features.

### Agent Capabilities (Conversational + Daemon)

- **ReAct Reasoning Loop** — Think → Call Tool → Observe → Answer, with native function calling and JSON fallback
- **330+ Pre-built Skill Templates** — Covering 17 domains (Finance, Healthcare, Supply Chain, DevOps, etc.), auto-matched and executed by Agent
- **Unified Event Loop** — Conversational (`aflare chat`) and daemon (`aflare agent`) share the same `AgentLoop` core, supporting stdin / scheduler / filewatch multi-source input fusion
- **6 Pluggable Capabilities** — Enable on demand, mapping the complete Agent type taxonomy:

| Capability | Type | Description |
|------------|------|-------------|
| `reflection` | Self-Critique | Auto-evaluate output quality after each turn, trigger self-correction |
| `human-in-loop` | Human-in-the-Loop | Pause at critical decisions, request human confirmation |
| `utility` | Utility-Driven | 6-dimension scoring (correctness/completeness/efficiency/safety/clarity/actionability), optimize decisions |
| `memory` | Stateful | Cross-session long-term memory, remember user preferences |
| `planning` | Planning | Generate plans before acting, execute step by step |
| `workflow` | Workflow/Pipeline | Prioritize existing templates, stable and predictable |

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
- 18 built-in providers (OpenAI / DeepSeek / Qwen / GLM / Kimi / Anthropic / Gemini / Mistral / Ollama, etc.), any OpenAI-compatible model usable
- Fully offline capable (Ollama local LLM)
- LLM smart routing (EWMA latency prediction + Pareto cost sorting)

### MCP Protocol Support
- Built-in MCP Server, connectable by any MCP client (Claude, VS Code, Cursor, etc.)
- Provides workflow execution, validation, node query, code graph, and other tools
- Built-in MCP Client, workflows can call external MCP services directly
- `aflare mcp install <name>` one-command install of 8 built-in community servers; stdio servers declared by plugins register idempotently via `marketplace install`

### Agent Plugins 1.0.0 Interop
- **Install** (`aflare marketplace install <plugin-dir>`): loads any Agent Plugins 1.0 conformant plugin — `skills/*/SKILL.md` materializes into skills runnable via `aflare run`, stdio servers from `mcp.json` register into `.mcp.json`; nothing from the plugin executes at install time, and directory names / frontmatter names / cwd are all checked against traversal and symlink escapes
- **Export** (`aflare marketplace export`): exports aflare skills in the same standard format for VS Code / Cursor / Copilot / ChatGPT — the export → install round trip is verified

### Memory Critique-Reconstruction (MemHarness mode)
- memory node `harness_search` operation: retrieves candidates with full source state (type/level/confidence/recorded-at/score) and emits a self-contained critique prompt; the LLM critique (keep/rewrite/discard) runs as an explicit, retryable workflow step, outputting `<EMPTY>` instead of inventing when nothing applies
- Agent session injection runs a deterministic critique: stale never-reused memories are dropped, survivors injected with source-state annotations
- Full example at `examples/real-world/memharness-critique/`

### Step-level Typed Output Contracts & Bounded Preview
- `output_schema`: any node's output is validated against a JSON Schema (draft-07 subset); violations fail the step with the first violation location and flow into retry / on_error / capture_error
- `preview_input: true`: inputs over 16KiB are replaced by a bounded head/tail preview while the full value stays in workflow state and is passed untouched to other steps — LLMs see samples, deterministic nodes operate on complete data

### Engineering
- Expression engine: bytecode IR + vectorized batch evaluation
- DAG scheduler formally verified with TLA+ (spec at [`docs/tla/dag_scheduler.tla`](docs/tla/dag_scheduler.tla), bounded model-checking via `dag_formal_test.go`)
- Prometheus metrics endpoint
- Single binary, zero runtime dependencies
- CI validates both architectures (x86-64 + ARM64)

### Experimental
- Local inference services on Ascend / Cambricon / Hygon chips via OpenAI-compatible endpoints (no native SDK integration)

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
│  │  │ ReAct    │  │ 330+     │  │ 6 Pluggable    │  │ │
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
│  │ Software (API/Web/DB/Files)                       │ │
│  │ External devices (custom nodes/MCP, experimental) │ │
│  └──────────────────────────────────────────────────┘ │
└──────────────────────────────────────────────────────┘
```

---

## Roadmap

| Version | Status | Focus |
|---------|--------|-------|
| v0.6 | Done | Agent memory infrastructure, voice AI toolchain, WAL persistence, TLA+ verification |
| v0.7 | Done | Financial scenario enhancement (Saga / Idempotency / Audit chain), ReAct Agent chat, 300+ skill templates, 6 pluggable capabilities, Agent unified event loop |
| **v0.8** | **Done** | Offline/intranet-first experience, privacy/security hardening, smooth local-LLM onboarding, CLI UX improvements (template run / smart command hints), CI speedup |
| **v0.9** | **Done** | National cryptography support (SM3/SM4, opt-in), audit-chain security hardening (random HMAC key, cross-process lock, bundle truncation-forgery defense), `aflare mcp install`, supply-chain scenario pack, loong64 |
| v0.10 | In development | Agent Plugins 1.0.0 interop, MemHarness memory critique-reconstruction, step-level output contracts & bounded preview, watermark deployment tracing, security self-audit fixes; next: domestic chip support refinement, Agent capability deepening |
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

## Market Data & Financial Use Cases

The built-in price-monitoring templates cover four markets out of the box:

| Market | Symbol style in `aflare create` | Data source |
|--------|-------------------------------|-------------|
| Crypto (BTC/ETH) | `BTC`, `ethereum` | CoinGecko public API |
| US stocks | `usAAPL`, `US:TSLA` | Tencent quote API |
| HK stocks | `hk00700`, `hk0700` (auto zero-padded) | Tencent quote API |
| A-shares | `600519` (6xx→SH, 0xx/3xx→SZ) or `sh600519` | Tencent quote API |

Optional data sources: the East Money **MiaoXiang** financial LLM is closed-source (registered model Shanghai-Miaoxiang-20231207), but East Money operates an official API platform (free tier available, covering A-shares / HK / US stocks / funds / bonds, requires an `EM_API_KEY`). It plugs in directly through the `http_request` node — review their terms of service and quota limits before use.

What aflare can and cannot do in finance:

| Use case | Feasibility | Notes |
|----------|-------------|-------|
| Price monitoring / threshold alerts | ✅ Fully supported | schedule → fetch → condition → notify, built-in template ready to run |
| Trade review / research assistant | ✅ Supported | Pull historical quotes, generate review reports via LLM; output is for personal research only |
| Quant research / backtesting | ⚠️ Partial | Workflows handle data fetching, indicator math and scheduling; backtests need your own engine (broker sim API or open-source frameworks); **live trading must go through a licensed broker API (e.g. QMT/PTrade in CN) at your own risk** |
| Investment advisory / stock picks | ❌ Not provided | aflare never produces investment advice — generated content is objective data aggregation only, no predictions or buy/sell recommendations |

> **Disclaimer**: Nothing generated by aflare (data, reports, alerts) constitutes investment advice. Market data may be delayed or inaccurate; verify against official exchange sources. You are solely responsible for how you use this software.

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

We welcome contributions! Beyond code, you can also **submit Skill templates**:

1. **Fork** this repo
2. Create a YAML template under the corresponding domain directory in `templates/` (see [YAML Syntax](docs/getting-started.md#workflow-configuration))
3. Run `go test ./...` to verify
4. Submit a PR with a description of what the template does

330+ Skills already cover 17 domains — your template can fill the missing piece.

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