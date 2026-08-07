<div align="center">
  <h1>aflare</h1>
  <p>🌍
    <a href="README.md">中文</a> ·
    <strong>English</strong>
  </p>
  <p><strong>Stop chatting with AI — make it your execution co-pilot.</strong></p>
  <p>Natural language in, deterministic YAML out. More elegant than Bash, lighter than n8n, more reliable than pure AI agents. Built-in financial Saga transactions, multimodal analysis, robot control. Deeply adapted for domestic chips (Ascend / Cambricon / Hygon).</p>
  <p><em>What if your computer just understood what you meant?</em></p>

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
    <a href="https://github.com/alib8b8/aflare/actions/workflows/release.yml">
      <img src="https://github.com/alib8b8/aflare/actions/workflows/release.yml/badge.svg" alt="Release status" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://img.shields.io/badge/AtomGit-GitCode-green?style=flat-square&logo=data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCI+PHBhdGggZmlsbD0iIzI1MjUyNSIgZD0iTTIyIDJoLTJWMGgydi0yaDJ2MmgydjItMmgydjItMmgydjJ6bTAgMTZIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6bTAgLThIMnYtMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjItMmgydjJ6Ii8+PC9zdmc+" alt="GitCode" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/star/new_badge.svg" alt="AtomGit G-Star" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/star/badge.svg" alt="AtomGit Star" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/fork/badge.svg" alt="AtomGit Fork" />
    </a>
    <a href="https://gitcode.com/llm-box/llm-box">
      <img src="https://gitcode.com/llm-box/llm-box/download/badge.svg" alt="AtomGit Download" />
    </a>
  </p>

</div>

---

## 📋 Table of Contents

- [🚀 Quick Start](#-quick-start)
- [💡 Why aflare?](#-why-aflare)
- [📖 The Story](#-the-story)
- [✨ Core Features](#-core-features)
- [💰 Financial Scenario Capabilities](#-financial-scenario-capabilities)
- [🏛️ Xinchuang &amp; Domestic IT](#-xinchuang--domestic-it)
- [🤖 Agent Nodes](#-agent-nodes)
- [📱 HarmonyOS &amp; Mobile Nodes](#-harmonyos--mobile-nodes)
- [🤖 Unitree Robot Integration](#-unitree-robot-integration)
- [🔒 Security](#-security)
- [🌐 Ecosystem](#-ecosystem)
- [📚 Documentation](#-documentation)
- [🛠️ CLI Commands](#-cli-commands)
- [🏗️ Architecture](#-architecture)
- [🗺️ Roadmap](#-roadmap)
- [🌟 Featured Integrations](#-featured-integrations)
- [🛒 Featured Skills Marketplace](#-featured-skills-marketplace)
- [📖❓ FAQ](#-faq)
- [🤝 Contributing](#-contributing)
- [📦 Code Hosting](#-code-hosting)
- [📄 License](#-license)

---

## 🚀 Quick Start

**One command to install:**

| macOS | Linux | Windows |
|-------|-------|---------|
| `brew install alib8b8/tap/aflare` | `curl -fsSL https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh \| bash` | `irm https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 \| iex` |

**🌏 China users &mdash; use mirror for faster download:**

| macOS/Linux | Windows |
|-------------|---------|
| `curl -fsSL https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/aflare/main/install.sh \| bash` | `irm https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/aflare/main/install.ps1 \| iex` |

**Or download from [GitCode Releases](https://gitcode.com/llm-box/llm-box/releases) / [GitHub Releases](https://github.com/alib8b8/aflare/releases)**

📖 [Interactive Download Page →](docs/download.html)

---

### Generate Workflows in One Line

Tell aflare what you want in plain English, and it generates YAML workflows:

```bash
# Monitor BTC price every 10 minutes
aflare create "monitor BTC price every 10 minutes and send telegram alert when > 70000"

# Watch GitHub repo stars and notify at threshold
aflare create "watch my github repo alib8b8/aflare and notify me when star > 100"

# Download and summarize arxiv AI papers daily
aflare create "download arxiv AI papers every day and summarize top 5"

# Watch TSLA stock and send Telegram alerts
aflare create "watch TSLA stock and send telegram alert when price drops 5%"

# Monitor Shanghai weather and email if rain
aflare create "monitor Shanghai weather and email me if it will rain tomorrow"
```

Then run the generated workflow:

```bash
aflare run btc-monitor.yaml
```

📖 [Full Getting Started Guide →](docs/getting-started.md)

---

## 💡 Why aflare?

> aflare is not an AI assistant — it's a **deterministic execution engine**.
>
> AI understands your intent. YAML guarantees execution.

| Tool | The Problem | aflare's Approach |
|------|-------------|-------------------|
| **Bash** | Too hard to write, hard to maintain, error-prone | Natural language generation, YAML is readable and editable |
| **n8n** | Too heavy — one automation task needs Docker | Single binary, works right in your terminal |
| **Zapier** | SaaS, your data isn't yours, expensive | Local execution, fully controllable |
| **Claude Code** | Code-focused, not great at workflow automation | General-purpose workflow engine, not just code |
| **AI Agent** | Too unpredictable, output is all hallucination | AI only translates intent, execution is guaranteed by YAML |

---

## 📖 The Story

> I built aflare because I was tired of writing Bash scripts.
>
> Every time I wanted to automate something simple — monitor a price, scrape news, send a timed message — I'd end up writing dozens of lines of Bash, handling errors, retries, dependencies.
>
> What I wanted was: **tell the computer what I want, and it just does it.**
>
> As simple as GitHub Actions — but running on my laptop.

---

## ✨ Core Features

| Category | Features |
|----------|----------|
| **🎯 Natural Language &rarr; YAML** | Describe your needs in one sentence, auto-generate executable workflows. AI only translates, doesn't decide |
| **📦 Homebrew-like Experience** | One command to install, one command to create, templates at your fingertips. Install workflows like packages |
| **⚙️ Deterministic Execution** | YAML workflows = deterministic output. No hallucinations, no randomness, same results every time |
| **🤖 Personal AI Superintelligence** | Your private AI agent running on your laptop. Not SaaS, not cloud — yours |
| **🔌 Fully Offline Capable** | Works with Ollama — everything runs locally. Works without internet, data never leaves your device |
| **🔄 Scheduled Execution** | Built-in Cron scheduling. Every 10 minutes, daily, weekly — however you want it |
| **🧩 100+ Built-in Templates** | BTC monitor, GitHub Star alerts, Arxiv paper summaries, weather reminders… ready to use |
| **🔌 Plugin System** | Extend like Homebrew taps. `aflare install btc-monitor` |
| **🌐 Multi-Model Support** | Ollama / OpenAI / DeepSeek / Qwen / Kimi / GLM / Mistral / **Ascend**, local and cloud |
| **🔒 Privacy First** | Local execution by default, auto secret redaction, complete audit logs, 98+ vulnerabilities audited |
| **🛡️ Enterprise-Grade Security** | SSRF protection, path traversal defense, command injection whitelist, AES-GCM encryption |
| **⚙️ Engineering Depth** | WAL persistence engine (crash recovery), bytecode-IR expression engine + vectorized evaluation, EWMA latency prediction + Pareto routing, TLA+ formal verification of DAG scheduler |

---

## 💰 Financial Scenario Capabilities

aflare ships the core capabilities required for real-world financial scenarios (read-only analysis + controlled writes):

| Capability | Implementation | Status |
|------------|----------------|--------|
| Audit log (HMAC hash chain) | executor auto-flush, tamper-proof | ✅ |
| LLM response cache | performance optimization (cache by model+prompt+params+seed+API key hash); audit relies on audit log + trace | ✅ |
| Idempotency (prevents double-debit) | Idempotency-Key + atomic placeholder + cross-process lock | ✅ |
| HTTP rate limiting / retry | per-host token bucket + exponential backoff | ✅ |
| Quota persistence + multi-tenancy | FileQuotaStore + per-tenant isolation | ✅ |
| Trace redaction | LLM I/O redacted (API keys/JWT/private keys) before persistence | ✅ |
| **Saga transaction compensation** | forward steps + reverse compensate, best-effort compensation, `{{var.error}}` context | ✅ |
| **LLM cost attribution** | token usage × price table → estimated USD cost in audit log `cost_usd`/`total_tokens`; `AFLARE_PRICING_FILE` overrides prices | ✅ |

### Applicable Scenarios
- ✅ Read-only analysis: AML review, investment research, portfolio review, end-of-day reconciliation (templates available)
- ✅ Controlled writes: idempotent transfer (requires server-side dedup)
- ✅ Cross-step transactions: saga transfer compensation (multi-step writes auto-rolled-back in reverse on failure)
- ⚠️ Strong-consistency distributed transactions: 2PC not yet implemented; strong-consistency scenarios still need database transactions

### Examples
- [AML Suspicious Transaction Review](examples/finance/aml-review/) — read-only analysis
- [AML Review (Ascend NPU)](examples/finance/aml-review-ascend/) — read-only analysis, LLM inference on Ascend NPU
- [Idempotent Transfer](examples/finance/idempotent-transfer/) — controlled write
- [Saga Cross-bank Transfer](examples/finance/saga-transfer/) — cross-step transaction compensation
- [Saga Transfer (Ascend NPU)](examples/finance/saga-transfer-ascend/) — cross-step transaction compensation + Ascend LLM fraud detection
- [End-of-day Reconciliation](examples/finance/reconciliation/) — read-only analysis

---

## 🏛️ Xinchuang &amp; Domestic IT

aflare is a domestic AI workflow engine built for China's Xinchuang (信息技术应用创新) ecosystem — from chip ISA to OS, from AI compute to mobile, the entire stack runs on domestic technology.

### Xinchuang Compatibility Matrix

| Layer | Supported | Notes |
|-------|-----------|-------|
| **AI Chips** | Ascend 910B/310P, Cambricon MLU 370/590, Hygon DCU K100/Z100 | Via OpenAI-compatible inference servers, `ascend`/`cambricon`/`hygon` provider nodes |
| **CPU Architecture** | x86-64, ARM64 (Kunpeng 920) | CI validates both architectures, single binary cross-platform |
| **Operating Systems** | openEuler, Kylin, UOS | Go single binary, zero runtime dependencies |
| **Mobile** | HarmonyOS | 19 nodes: Ability launch, atomic service, widget, device adapt, cross-app |
| **AI Models** | DeepSeek, Qwen, GLM, Kimi, Ant Ling | All domestic models, local/cloud deployment |
| **Code Hosting** | GitCode / AtomGit | Domestic mirror, Xinchuang community engagement |

### Xinchuang Scenarios

| Scenario | Example | Chip |
|----------|---------|------|
| Financial Risk | [AML Review (Ascend)](examples/finance/aml-review-ascend/) | Ascend 910B |
| Transaction Compensation | [Saga Transfer (Ascend)](examples/finance/saga-transfer-ascend/) | Ascend 910B |
| Smart Patrol | [Unitree Go2 Patrol](examples/robot/unitree-go2-patrol/) | Edge + Ascend cloud |

### Xinchuang Differentiators

| Dimension | aflare | Domestic Alternatives |
|-----------|---------|----------------------|
| **Chip Support** | Ascend + Cambricon + Hygon (3 vendors) | Mostly NVIDIA CUDA only |
| **Deployment** | Single binary, fully offline/air-gapped | Mostly SaaS or Docker |
| **HarmonyOS** | 19 native nodes, full device coverage | None or basic |
| **License** | AGPL v3.0, fully open source | Mostly source-available or partial |
| **Financial Compliance** | HMAC audit chain + idempotency + redaction + saga compensation | Most lack financial capabilities |

> **Strategic positioning**: Enter through financial Xinchuang, become the benchmark for "domestic AI workflow engines." The Xinchuang market has databases, OSes, and chips — what it lacks is a good AI automation orchestration tool.

---

## 🤖 Agent Nodes

Specialized AI agent nodes for autonomous reasoning:

| Node | Description |
|------|-------------|
| `agent` | General-purpose ReAct agent with tool use, Chain-of-Thought mode |
| `planner` | Breaks tasks into step-by-step plans |
| `researcher` | Web research and information gathering |
| `critic` | Reviews and provides constructive feedback |
| `evaluator` | Evaluates outputs against criteria |
| `reflector` | Reflects on process and suggests improvements |
| `supervisor` | Oversees multi-agent workflows with sequential/parallel/hierarchical/MoE/MindSearch/Agency strategies and **195 domain specialists**, **9 collaboration templates** |
| `code_review` | Automated code review and suggestions |
| `human_in_loop` | Pauses for human approval |
| `code_knowledge_graph` | Semantic code knowledge graph: 158 languages, vector retrieval, entity/relation/concept extraction, MCP tool exposure, token-efficient review |
| `moe_streaming` | MoE expert streaming: consumer hardware runs 744B models with on-demand loading |
| `cli_session` | Interactive terminal session with context persistence, streaming output, auto-completion |
| `plugin_system` | Plugin extension: install/uninstall/update from local/git/url/market with sandbox isolation |
| `engineer_skills` | 16 pre-built skills: React/TypeScript/API/Database/CI-CD/Docker/Design Patterns |
| `skill_distill` | Distill methodologies from books/videos/podcasts into callable skills |
| `voice_output` | Voice AI toolchain: TTS + voice cloning + ASR transcription + speaker diarization + voice analysis, 11 languages, 5 ASR engines |
| `doc_gen` | AI document generation: 7 types (readme/api/function/module/changelog/tutorial/architecture) |
| `video_edit` | AI video editing: smart_cut/merge/effects/subtitle/storyboard/upscale |
| `memory` | Agent memory infrastructure: three-level memory (short/medium/long), 16 operations, LRU eviction, cross-session long-term memory |

### HarmonyOS &amp; Mobile Nodes

| Node | Description |
|------|-------------|
| `harmony_ability` | Launch HarmonyOS Ability (page/slice/service/data) with type validation |
| `harmony_atomic_service` | Launch atomic service (card-based lightweight app, no install needed) |
| `harmony_widget` | Manage desktop widgets: add, update, remove, query |
| `harmony_device_adapt` | Detect 7 device types (phone/foldable/tablet/TV/car/wearable), generate UI adaptation plan |
| `app_launch` | Launch mobile apps (Android/iOS/HarmonyOS) with platform auto-detection |
| `ui_automate` | UI automation: click, scroll, type, swipe, screenshot (whitelist-validated actions) |
| `cross_app_action` | Cross-app workflows: share content, save for later, compare prices |
| `intent_router` | Route user intents to appropriate handlers with domain classification |
| `device_state` | Query device state: battery, network, location, apps, storage |
| `agent_message` | Send cross-domain messages between agents using W3C DID identity |
| `agent_inbox` | Query and manage agent message inbox |
| `system_event` | Listen for mobile system events: notification, call, SMS, location, battery, alarm, screen state |
| `ondevice_llm` | Run LLM inference locally on device (1B-7B models, INT4/INT8/FP16, llama.cpp/MLC-LLM/ONNX backends) |
| `power_manager` | Adaptive power management: eco/balanced/high profiles, battery-aware &amp; thermal-aware throttling |
| `blockchain_audit` | Record workflow execution on blockchain for tamper-proof audit trails (Ethereum/Hyperledger/simulated). Aligns with WAIC Agent Interoperability Initiative |
| `screen_understanding` | L3-level screen content understanding: parse UI elements, identify actionable items, generate interaction plans for agent phones |
| `voice_input` | Voice pipeline: VAD, wake word detection, speech-to-text with on-device support |
| `robot_control` | Plan and execute robot action sequences for embodied AI: humanoid/mobile_base/arm/drone/dog/wheelchair with safety checks |
| `unitree_robot` | Dedicated Unitree robot control: Go2/B2/H1/G1 and 9 models, 14 actions, simulate/API dual mode, natural language control |
| `andesgpt` | OPPO AndesGPT integration: Tiny (on-device 1B) / Turbo (edge-cloud 7B) / Titan (cloud 100B+), PersonaX personalization, end-cloud collaboration |

### Code Intelligence Nodes

| Node | Description |
|------|-------------|
| `file_watch` | Watch a path for file changes (create/modify/delete), polling-based, context-aware |

---

## 🤖 Unitree Robot Integration

aflare ships with a built-in `unitree_robot` node that brings Unitree (宇树) robots into YAML workflow orchestration, supporting both quadruped and humanoid robot control and status monitoring.

| Node | Description |
|------|-------------|
| `unitree_robot` | Control Unitree quadruped/humanoid robots with 14 actions, 9 models, simulate/API dual mode |

### Supported Models

Go2, B2, B2-W, Go1, A1, H1, H1-2, G1, G1-Humanoid

### Supported Actions

| Category | Actions |
|----------|---------|
| Basic Movement | stand, sit, walk, run, stop |
| Navigation & Patrol | patrol, navigate |
| Posture Control | step_up, step_down |
| Humanoid Interaction | wave_hand, shake_hands |
| Perception Query | get_status, get_camera |
| Performance | dance |

### Two Modes

| Mode | Description |
|------|-------------|
| **Simulate (default)** | No hardware required; returns simulated IMU/GPS/joint state/battery data — ideal for development, debugging, and CI testing |
| **API (direct)** | Controls real robot hardware via HTTP API, with support for custom API endpoints and API key authentication |

### Natural Language Control

Chinese natural language input is supported with automatic action inference: say "巡逻仓库" → `patrol`, "站起来" → `stand`, "挥手" → `wave_hand` (humanoid models), "检查电池" → `get_status`.

### Safety Checks

Built-in safety validation engine: high-speed operation warnings, safety zone checks, collision-avoidance alerts for humanoid arm movements. Results include a `safety_checks` field in JSON output.

### Examples

- [Go2 Warehouse Patrol](examples/robot/unitree-go2-patrol/workflow.yaml) — Quadruped simulated patrol: stand → patrol → camera → status → sit
- [H1 Natural Language Interaction](examples/robot/unitree-h1-interact/workflow.yaml) — Humanoid NL commands with auto-inferred wave/shake/stand actions
- [B2 API Direct Control](examples/robot/unitree-b2-api/workflow.yaml) — Industrial robot HTTP API direct control, industrial park patrol + real-time status

---

## 🔒 Security

aflare takes security seriously. Key protections:

| Protection | Implementation |
|------------|----------------|
| **SSRF Protection** | Custom `DialContext` validates IPs at connect time (prevents DNS rebinding) |
| **Path Traversal** | Input validation + symlink resolution, all paths confined to working directory |
| **Command Injection** | Allowlist mode blocks shell metacharacters; safe mode disables execution entirely |
| **Secrets Management** | AES-GCM encryption with PBKDF2 (600K iterations), file permissions `0600` |
| **Timing Attack** | `subtle.ConstantTimeCompare` for authentication tokens |
| **Fail-Closed Auth** | Empty token = request rejected (503) |
| **Audit Logging** | All commands logged with redacted secrets, `0600` permissions, **HMAC hash chain tamper-proofing** |
| **DID Identity** | W3C DID format validation, signature verification, cross-domain message auth |
| **Memory Safety** | Layered memory with LRU eviction, symlink-resistant persistence, 0600 file perms |
| **Prompt Injection** | Skill evolution sanitizes best practices/pitfalls before injecting into prompts |
| **Secret Redaction** | Auto-detect &amp; mask 10+ secret patterns (AWS/GitHub/Slack/JWT/private keys) on file read; `.env`/credentials fully masked by default |
| **Outbound Monitoring** | Sliding-window data volume monitor with anomaly alerting (prevents Grok-Build-style 27800&times; data leaks) |
| **Circuit Breaker** | Per-node breaker (Closed&rarr;Open&rarr;HalfOpen), auto-isolation of failing nodes prevents cascade failures |
| **ANSI Injection** | TUI Markdown/Mermaid renderers strip terminal control sequences (CSI/OSC/DCS) from user input |
| **Session Limits** | CLI sessions auto-expire after 24h (max 500), MCP sessions capped at 1000 with cleanup |
| **Plugin Limits** | Max 100 plugins, HTTPS-only URLs, restricted git hosts (GitHub/GitLab/GitCode/Gitee) |
| **Resource Limits** | Code knowledge graph: max 5000 files/depth 5; video edit: shell metacharacter filtering |
| **Concurrent Safety** | Per-session rand mutex, RWMutex for shared state, no global mutable state without locks |
| **Auto Fix** | CI auto-runs gofmt/go vet/gosec/govulncheck, auto-commits fixes on detection |

📖 [Security Guide →](SECURITY.md) | [Audit Logs →](docs/getting-started.md#audit-logs)

---

## 🌐 Ecosystem

aflare participates in multiple open-source ecosystems:

| Ecosystem | Status | Description |
|-----------|--------|-------------|
| **GitCode G-Star** | Applied | Compute support, traffic exposure, HarmonyOS certification |
| **HarmonyOS &amp; Mobile Nodes** | Built-in | 19 nodes: ability launch, atomic service, widget, device adapt, cross-app, agent message, intent router, device state, UI automation, system events, etc. |
| **SenseNova** | Active | API integration (6 models), on-device U1-Lite support (8B/A3B MoE) |
| **Ant Ling** | Active | API integration (4 models: ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0), OpenAI-compatible endpoint |
| **OPPO AndesGPT** | Active | API integration (Tiny/Turbo/Titan tiers), PersonaX personalization, end-cloud collaboration |
| **GitHub** | Active | CI/CD, CodeQL security scan, automated releases |
| **Ascend** | Integrated | Ascend NPU (MindIE inference), `ascend` provider node, financial risk/transfer templates |
| **Cambricon** | Integrated | Cambricon MLU (MLU 370/590), `cambricon` provider node, OpenAI-compatible inference |
| **Hygon** | Integrated | Hygon DCU (K100/Z100), `hygon` provider node, ROCm-compatible inference |
| **Unitree** | Integrated | Unitree robot control (Go2/B2/H1/G1), 14 actions, simulate/API dual mode, natural language control |

### HarmonyOS Device Support

| Device Type | Key Capabilities |
|-------------|-----------------|
| Phone (Standard) | touch, camera, gps, nfc, biometrics |
| Phone (Dual Fold) | foldable_screen, multi_window, drag_to_split |
| Phone (Triple Fold) | foldable_screen, multi_window, drag_to_split |
| Tablet | stylus, multi_window, split_screen |
| Smart Screen | voice, gesture, remote_control |
| Car | steering_wheel_control, hud, voice |
| Wearable | heart_rate, accelerometer, gyroscope |

---

## 📚 Documentation

### Getting Started

- [Getting Started Guide](docs/getting-started.md) &mdash; Installation, first workflow, next steps
- [Examples](examples/) &mdash; 10 ready-to-use workflow templates

### Core Concepts

- [Dataflow &amp; Variables](docs/dataflow.md) &mdash; Step-to-step data passing, `{{input}}`, `{{step.N}}`, `{{var.NAME}}`, `{{secret.GROUP.KEY}}`
- [Distributed Execution](docs/distributed.md) &mdash; Coordinator/Worker architecture (design doc, not yet implemented)
- [Scheduling](docs/scheduling.md) &mdash; Cron workflows, schedule management
- [MCP Integration](docs/mcp.md) &mdash; Connect external tools via Model Context Protocol
- [Plugins](docs/plugins.md) &mdash; Install and manage community plugins

### Advanced

- [Web UI Editor](docs/webui.md) &mdash; Visual workflow builder
- [Visualizer](docs/visualizer.md) &mdash; Mermaid/JSON/DOT/ASCII diagrams
- [Tenant Isolation](docs/tenants.md) &mdash; Multi-tenant workspace separation
- [Custom Nodes](docs/custom-nodes.md) &mdash; Build nodes in any language
- [Troubleshooting](docs/troubleshooting.md) &mdash; Error codes, common issues, FAQ

### Reference

- [Workflow YAML Syntax &rarr;](docs/getting-started.md#workflow-configuration)
- [Node Reference &rarr;](docs/custom-nodes.md#built-in-nodes)
- [CLI Reference &rarr;](docs/getting-started.md#cli-command-reference)
- [Error Codes &rarr;](docs/troubleshooting.md#error-codes)

---

## 🛠️ CLI Commands

```bash
aflare create [description]    Generate workflow from description
aflare run <file>             Run a workflow
aflare run --resume <file>    Resume workflow from last checkpoint
aflare secrets add            Store an encrypted secret
aflare secrets list           List secrets in a group
aflare schedule add           Add a scheduled task
aflare schedule list          List scheduled tasks
aflare schedule remove        Remove a scheduled task
aflare schedule start         Start the scheduler
aflare ui                     Start web UI editor
aflare visualize <file>       Visualize a workflow
aflare validate <file>        Validate a workflow file
aflare node install           Install external node
aflare plugin install         Install a plugin
aflare version                Show version
aflare help                   Show full help
```

📖 [Full CLI Reference &rarr;](docs/getting-started.md#cli-command-reference)

---

## 🏗️ Architecture

```
┌─────────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐     ┌────────┐
│  Prompt │────▶│ Planner │────▶│ Workflow YAML│────▶│ Executor │────▶│ Result │
└─────────┘     └─────────┘     └──────────────┘     └──────────┘     └────────┘
                                                          │
                                              ┌───────────┴───────────┐
                                              ▼                       ▼
                                      ┌──────────────┐         ┌──────────────┐
                                      │  Agent Nodes │         │ Utility Nodes│
                                      │  ReAct Loop  │         │ fetch, exec  │
                                      └──────────────┘         └──────────────┘
```

**Key components:**
- **Generator** &mdash; Keyword-based workflow generation from natural language, 100+ ready-to-use templates
- **Parser** &mdash; YAML workflow validation and parsing
- **Executor** &mdash; Deterministic step execution with dependency tracking
- **Expression Engine** &mdash; Variable substitution, secrets injection, file reading
- **Registry** &mdash; 50+ built-in nodes plus external node discovery and loading
- **Edge Router** &mdash; ReAct reasoning loop, 3-tier persistent memory, local/cloud model routing
- **Skill Evolution** &mdash; Self-improving agent skills with success rate tracking and prompt optimization
- **Intent Protocol** &mdash; `intent://` and `ohos://` URI schemes, W3C DID identity, cross-domain messaging
- **Subagent Prompts** &mdash; 17 specialist prompt templates, main/sub agent hierarchy (Grok Build pattern)
- **Circuit Breaker** &mdash; Per-node Closed/Open/HalfOpen state machine, auto-isolation prevents cascade failures
- **Privacy Layer** &mdash; Secret redaction on file read, outbound data volume anomaly monitor
- **Checkpoint/Resume** &mdash; WAL persistence engine (append-only + CRC32 + atomic compaction), replaces full JSON rewrites; `--resume` recovers from interruption
- **Shared HTTP Client** &mdash; Unified connection-pool tuning and SSRF defense, proxy env passthrough
- **DAG Parallel Execution** &mdash; Topological-sort dependency scheduling, concurrent execution of independent steps
- **Saga Transaction Compensation** &mdash; forward steps run in order; on any failure, completed steps run their `compensate` in reverse order; best-effort compensation never aborts earlier rollbacks, `{{var.error}}` exposes the triggering cause
- **Multi-Error Aggregation** &mdash; `ProviderMultiError` aggregates all provider failures, supports `errors.Is/As` traversal
- **Observability** &mdash; Prometheus metrics endpoint, audit-log HMAC hash chain tamper-proofing, log rotation
- **Expression Engine** &mdash; Bytecode IR (flat opcodes + switch dispatch) + `EvaluateParamsVectorized` vectorized batch evaluation, eliminates virtual call overhead
- **LLM Smart Routing** &mdash; EWMA latency prediction, full circuit-breaker state machine (Closed→Open→HalfOpen), Pareto cost-latency frontier sorting
- **Formal Verification** &mdash; TLA+ spec + Go model checking (500 random DAGs) verifies DAG scheduler safety invariants and liveness

---

## 🗺️ Roadmap

| Version | Status | Features |
|---------|--------|----------|
| **v0.1** | ✅ Released | Core workflow engine, 10 utility nodes |
| **v0.2** | ✅ Released | LLM nodes, MCP integration, external nodes |
| **v0.3** | ✅ Released | Agent nodes, Web UI, scheduling |
| **v0.4** | ✅ Released | Code interpreter, RAG, knowledge graph, smart router, multimodal, node marketplace, 100+ templates, 16 specialists, Chain-of-Thought |
| **v0.5** | ✅ Released | ReAct engine, layered memory, skill self-evolution, HarmonyOS adaptation (7 device types), cross-platform protocol (intent:// + ohos://), W3C DID identity, cross-domain agent messaging, GitCode G-Star + ohpm ecosystem |
| **v0.5.1** | ✅ Released | Ascend NPU adaptation (7-agent pipeline, 3 workflow templates, CANN/MindIE integration) |
| **v0.5.2** | ✅ Released | Grok Build-inspired capabilities: code graph, subagent prompt hierarchy, circuit breaker, secret redaction, file watch, TUI Markdown/Mermaid rendering (15-vuln audited), unified LLM routing (3 consolidated to 1) |
| **v0.6.0** | ✅ Released | **Ant Ling ecosystem, AI Gateway (OmniRoute), Agent Memory Infrastructure, Voice AI Toolchain (ASR/diarization/analysis), Agent Teamization (200+ roles + Agency workflow), Engineering Depth (WAL persistence + bytecode-IR expression engine + EWMA/Pareto routing + TLA+ formal DAG verification)** |
| **v0.7.0** | **Current** | **Financial scenario + Xinchuang: ✅ HMAC hash-chain audit, ✅ idempotency, ✅ HTTP rate limiting/retry, ✅ LLM response cache, ✅ quota persistence + multi-tenancy, ✅ trace redaction, ✅ WAL crash recovery, ✅ saga transaction compensation, ✅ LLM cost attribution, ✅ Unitree robot integration, ✅ Cambricon MLU + Hygon DCU triple-chip support, ✅ ARM64 Kunpeng CI** |
| **v1.0** | 📅 Q3 2026 | Stable API, full documentation, LTS |

📖 [Full Roadmap &rarr;](ROADMAP.md)

---

## 🌟 Featured Integrations

Xinchuang (domestic IT) landing cases built on aflare core capabilities, covering five key domains:

| Domain | Case | Chip | Core Capabilities |
|--------|------|------|-------------------|
| 🏦 Finance | [AML Review on Ascend](docs/cases/xinchuang-cases.md#案例一金融信创--全栈国产化-aml-反洗钱智能审查系统) | Ascend 910B | map concurrency + HMAC audit chain + LLM cost attribution |
| 🏭 Industry | [Ascend + Unitree Robot Factory Patrol](docs/cases/xinchuang-cases.md#案例二工业信创--基于昇腾--宇树机器人的智能产线巡检系统) | Ascend 310P | unitree_robot patrol + vision defect detection + saga loop |
| 🏛️ Government | [Work Order Dispatch on Kylin + HarmonyOS](docs/cases/xinchuang-cases.md#案例三政务信创--国产化工单自动分派与督办系统) | Domestic GPU | LLM dispatch + saga coordination + harmony_widget push |
| ⚡ Energy | [Substation Inspection with Dual-Chip](docs/cases/xinchuang-cases.md#案例四能源信创--变电站智能巡检与缺陷闭环管理系统) | Ascend + Hygon DCU | 200-station concurrency + saga defect loop + reconciliation |
| 📡 Telecom | [Network Alarm Self-Healing on Cambricon](docs/cases/xinchuang-cases.md#案例五运营商信创--国产化网络告警自愈编排平台) | Cambricon MLU 590 | LLM alarm convergence + saga self-healing + cost attribution |

> Each case includes: pain point analysis → architecture diagram → aflare capability mapping → xinchuang value quantification. See [full case studies](docs/cases/xinchuang-cases.md)

---

## 🛒 Featured Skills Marketplace

aflare ships with 100+ ready-to-use workflow templates covering development, ops, marketing, research, and more. One command to install and use.

### Installation

```bash
# Install from the skills marketplace
aflare create from templates/software-engineering/unit-test-generator

# Or install directly from GitHub
aflare create from https://github.com/alib8b8/aflare/tree/main/templates/data-ai/prompt-engineering
```

### Popular Skills

| Category | Skill | Description | Install |
|----------|-------|-------------|---------|
| 🤖 AI/ML | Prompt Engineering | LLM prompt engineering template | `aflare create from templates/data-ai/prompt-engineering` |
| 🤖 AI/ML | LLM Fine-tune | Large model fine-tuning pipeline | `aflare create from templates/data-ai/llm-finetune` |
| 🤖 AI/ML | Model Evaluation | Model evaluation and benchmarking | `aflare create from templates/data-ai/model-evaluation` |
| 💻 Dev Tools | Unit Test Generator | Auto-generate unit tests | `aflare create from templates/software-engineering/unit-test-generator` |
| 💻 Dev Tools | API Docs Generator | Auto-generate API docs | `aflare create from templates/software-engineering/api-docs-generator` |
| 💻 Dev Tools | Code Duplicate Finder | Code duplication detection | `aflare create from templates/software-engineering/code-duplicate-finder` |
| 💻 Dev Tools | Dependency Checker | Dependency security check | `aflare create from templates/software-engineering/dependency-checker` |
| 🔧 DevOps | Log Analyzer | Intelligent log analysis | `aflare create from templates/devops-infra/log-analyzer` |
| 🔧 DevOps | Docker Cleaner | Docker resource cleanup | `aflare create from templates/devops-infra/docker-cleaner` |
| 🔧 DevOps | SSL Cert Checker | SSL certificate expiry check | `aflare create from templates/devops-infra/ssl-cert-checker` |
| 📊 Data | CSV Analyzer | CSV data analysis | `aflare create from templates/data-ai/csv-analyzer` |
| 📊 Data | A/B Test Analyzer | A/B test analysis | `aflare create from templates/data-ai/ab-test-analyzer` |
| 📊 Data | Financial Analyzer | Financial data analysis | `aflare create from templates/data-ai/financial-analyzer` |
| 📝 Content | Blog Outline Generator | Blog outline generation | `aflare create from templates/marketing/blog-outline-generator` |
| 📝 Content | SEO Keyword Research | SEO keyword research | `aflare create from templates/marketing/seo-keyword-research` |
| 🔬 Research | Literature Review | Literature review | `aflare create from templates/data-ai/literature-review` |
| 🔬 Research | Paper Summarizer | Paper summarization | `aflare create from templates/data-ai/paper-summarizer` |
| 🔬 Research | Competitor Analysis | Competitor analysis | `aflare create from templates/data-ai/competitor-analysis` |
| 📈 Business | Business Plan | Business plan writing | `aflare create from templates/business/business-plan` |
| 📈 Business | SaaS Pricing | SaaS pricing strategy | `aflare create from templates/business/saas-pricing` |
| 🔒 Security | Security Audit | Security auditing | `aflare create from templates/devops-infra/security-audit` |
| 🔒 Security | Incident Response | Incident response | `aflare create from templates/devops-infra/incident-response` |
| 📚 Docs | README Generator | README auto-generation | `aflare create from templates/software-engineering/readme-generator` |
| 📚 Docs | API Docs Builder | API docs builder | `aflare create from templates/software-engineering/api-docs-builder` |
| 🏗️ Arch | Microservices Design | Microservices design | `aflare create from templates/software-engineering/microservices-design` |
| 🏗️ Arch | Cloud Architecture | Cloud architecture design | `aflare create from templates/devops-infra/cloud-architecture` |

> See the [templates/](templates/) directory for the full list — 120+ skills and growing.

---

## 📖❓ FAQ

### 1. How is aflare different from other agent frameworks?

aflare focuses on the **combination of deterministic workflows and AI agents**: workflows ensure reliable and reproducible execution, while agent nodes provide intelligent reasoning capabilities. We don't rely on a single model provider, supporting 22+ models with 5 routing strategies. The core is written in Go with zero dependencies — fast startup and low memory footprint.

In terms of engineering depth: the expression engine uses bytecode IR + vectorized batch evaluation; persistence uses append-only WAL (CRC32 + atomic compaction) replacing full JSON rewrites; LLM routing integrates EWMA latency prediction + full circuit-breaker state machine + Pareto sorting; the DAG scheduler is verified via TLA+ formal spec + Go model checking for safety invariants and liveness.

### 2. Which large language models are supported?

Currently supporting **22+ models** across mainstream providers:
- **Domestic**: SenseNova, Ant Ling, AndesGPT (OPPO), DeepSeek, Qwen (Alibaba)
- **International**: OpenAI, Anthropic Claude, Google Gemini, Inkling (Thinking Machines)
- **On-device**: llama.cpp, ONNX Runtime, SenseNova U1 (INT4/INT8)

### 3. Can it be used in enterprise environments?

Absolutely. aflare uses the GNU Affero General Public License v3.0 and provides:
- **Tiered security configuration** (L0-L3), supporting security gradients from development to production
- **Secret redaction** + **outbound data monitoring** to prevent data leaks
- **Audit logging** for traceability of all operations
- **Node-level circuit breakers** for system stability

### 4. How do I extend with custom nodes?

aflare supports three extension methods:
- **Plugin System**: Install plugins from local/git/URL/market with sandbox isolation
- **MCP Protocol**: Connect external tools via Model Context Protocol
- **Custom Nodes**: Implement standard interfaces in any language — see [custom nodes guide](docs/custom-nodes.md)

### 5. What's the long-term roadmap?

Short-term (v0.6-v0.9): Complete agent team collaboration, multimodal capabilities, performance optimization
Long-term (v1.0+): Stable API, LTS releases, enterprise support, more hardware adaptation (Ascend/Cambricon/Hygon)
See [Roadmap →](#-roadmap) for details.

### 6. Can aflare be used in real financial scenarios?

The core financial capabilities (audit / idempotency / rate limiting / redaction / caching / saga transaction compensation) are in place, suitable for read-only analysis, controlled-write, and cross-step transaction scenarios.
Saga supports automatic reverse compensation when multi-step writes fail; 2PC is not yet implemented, so strong-consistency scenarios still need database transactions.
See [Financial Scenario Capabilities](#-financial-scenario-capabilities) for details.

---

## 📦 Code Hosting

aflare is synced across multiple platforms — follow and contribute on your preferred platform:

| Platform | Link |
|----------|------|
| **GitHub** | https://github.com/alib8b8/aflare |
| **GitCode / AtomGit** | https://gitcode.com/llm-box/llm-box |

---

## 🤝 Contributing

We welcome contributions from the community!

### Good First Issues

New to the project? Start here:

- 🐛 [Bug fixes labeled "good first issue"](https://github.com/alib8b8/aflare/labels/good%20first%20issue)
- 📝 Documentation improvements
- ✅ Add test coverage for low-coverage packages
- 🔧 New utility nodes (see [custom nodes guide](docs/custom-nodes.md))
- 🌐 Add or improve translations (i18n)

### How to Contribute

1. Fork the repository
2. Create a branch: `git checkout -b feature/your-feature`
3. Make changes and add tests
4. Run tests: `go test ./...`
5. Submit a pull request

### Review Process

- CI must pass (build, tests, lint, security scan)
- At least one approval from a CODEOWNER
- Code follows Go conventions (`gofmt`, `go vet`)
- New features include tests and documentation

📖 [Full Contributing Guide &rarr;](CONTRIBUTING.md)

---

## 📄 License

GNU Affero General Public License v3.0 &mdash; see [LICENSE](LICENSE) for details.

---

<div align="center">
  <p>Built with ❤️ for developers who love the terminal</p>
  <p>
    <a href="https://github.com/alib8b8/aflare">GitHub</a>
    &middot;
    <a href="https://github.com/alib8b8/aflare/issues">Issues</a>
    &middot;
    <a href="https://github.com/alib8b8/aflare/discussions">Discussions</a>
  </p>
</div>