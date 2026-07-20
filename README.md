<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Turn Natural Language Into Executable Workflows</strong></p>
  <p>Agentic Workflow Engine for the Terminal — deterministic execution meets AI agents. Build self-driving workflows with autonomous agent nodes, tool use, and multi-step reasoning.</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/llm-box/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat-square" alt="Go" />
    </a>
    <a href="LICENSE">
      <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="license" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/release.yml">
      <img src="https://github.com/alib8b8/llm-box/actions/workflows/release.yml/badge.svg" alt="Release status" />
    </a>
  </p>

</div>

---

## 📋 Table of Contents

- [🚀 Quick Start](#-quick-start)
- [✨ Core Features](#-core-features)
- [🤖 Agent Nodes](#-agent-nodes)
- [📱 HarmonyOS & Mobile Nodes](#-harmonyos--mobile-nodes)
- [🔒 Security](#-security)
- [🌐 Ecosystem](#-ecosystem)
- [📚 Documentation](#-documentation)
- [🛠️ CLI Commands](#️-cli-commands)
- [🏗️ Architecture](#️-architecture)
- [🗺️ Roadmap](#️-roadmap)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🚀 Quick Start

**One command to install:**

| macOS | Linux | Windows |
|-------|-------|---------|
| `brew install alib8b8/tap/llm-box` | `curl -fsSL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**🌏 China users — use mirror for faster download:**

| macOS/Linux | Windows |
|-------------|---------|
| `curl -fsSL https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**Or download from [GitCode Releases](https://gitcode.com/llm-box/llm-box/-/releases) / [GitHub Releases](https://github.com/alib8b8/llm-box/releases)**

📖 [Interactive Download Page →](docs/download.html)

---

### Create & run your first workflow

```bash
# Generate a workflow from natural language
llm-box create "Summarize today's AI news"

# Or use a built-in template
llm-box create --template research-assistant

# Run it
llm-box run ai-news-summary.yaml
```

📖 [Full Getting Started Guide →](docs/getting-started.md)

---

## ✨ Core Features

| Category | Features |
|----------|----------|
| **Workflow Generation** | Natural language → YAML via keyword matching, 100+ built-in templates across 20+ categories |
| **Agent Nodes** | 10+ AI agent nodes with ReAct, Chain-of-Thought, tool use, and autonomous reasoning |
| **Edge AI Engine** | ReAct reasoning loop, 3-tier persistent memory (short/working/long-term), local/cloud model routing, privacy analyzer |
| **Skill Self-Evolution** | Agent skills improve with use — auto-tracks success rate, latency, best practices, known pitfalls; auto-optimizes prompts |
| **HarmonyOS Adaptation** | Ability launch, atomic service, desktop widget, 7-device-type adaptation (phone/foldable/tablet/TV/car/wearable) |
| **Cross-Platform Protocol** | `intent://` and `ohos://` URI schemes, W3C DID identity verification, cross-domain agent messaging |
| **Ascend NPU Adaptation** | 7-agent pipeline (search→verify→adapt→quantize→optimize→deploy→doc), CANN/MindIE/MindStudio integration, INT8/FP8 quantization, 1-hour auto-adapt |
| **Code Intelligence** | Code graph node (AST/call graph/dependency extraction for Go/Python/JS/TS), Codex/OpenCode-compatible tools (glob/grep/list_dir/apply_patch) |
| **Subagent Architecture** | Main/sub agent prompt hierarchy (17 specialist templates), borrowed from Grok Build's prompt.md + subagent_prompt.md pattern |
| **Distributed Resilience** | Per-node circuit breaker (Closed/Open/HalfOpen state machine), auto-isolation of failing workers, breaker stats endpoint |
| **Privacy by Design** | Auto secret redaction (.env/keys/tokens) on file read, outbound data volume monitor with anomaly alerting (prevents Grok-Build-style 27800× leaks) |
| **File Watching** | Polling-based file watch node (create/modify/delete events) for log-monitor and file-organizer workflows |
| **TUI Rendering** | Terminal Markdown renderer (headings/code/bold/italic/lists/quotes/tables) + Mermaid-to-ASCII converter (flow/sequence diagrams) |
| **Utility Nodes** | 40+ built-in nodes: LLM providers, fetch, execute, transform, file I/O, JSON, notify, condition, combine, call, template |
| **Data & Knowledge** | RAG retrieval, knowledge graph extraction/query/traversal, smart model router, multimodal image analysis |
| **Code & Tools** | Python code interpreter sandbox, node marketplace, MCP integration, plugin system |
| **Distributed Execution** | Coordinator/Worker architecture, horizontal scaling, heartbeat monitoring, circuit breaker |
| **Scheduling** | Cron-based scheduled workflows, interval triggers, CLI management |
| **Security** | SSRF protection, path traversal prevention, command injection defense, AES-GCM secrets, audit logging, secret redaction, ANSI injection defense, 76-vuln audited |
| **Ecosystem** | GitCode G-Star, HarmonyOS Agent Skills, ohpm SDK (@llm-box/workflow-engine) |
| **Developer Experience** | Web UI editor, workflow visualizer (Mermaid/JSON/DOT/ASCII), TUI with Markdown/Mermaid rendering, 9 languages |

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
| `supervisor` | Oversees multi-agent workflows with sequential/parallel/hierarchical/MoE/MindSearch strategies and 16 domain specialists |
| `code_review` | Automated code review and suggestions |
| `router` | Routes inputs to appropriate handlers |
| `human_in_loop` | Pauses for human approval |

### HarmonyOS & Mobile Nodes

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

### Ascend NPU Adaptation Nodes

| Node | Description |
|------|-------------|
| `ascend_model_search` | Search models in AtomGit/HuggingFace ModelZoo |
| `ascend_model_verify` | Verify model manifest, dependencies, License, Ascend compatibility |
| `ascend_model_adapt` | Adapt model to Ascend NPU via msTransplant, handle operator patches |
| `ascend_model_quantize` | INT8/FP8/W8A8 quantization via msModelSlim with accuracy comparison |
| `ascend_model_optimize` | Performance tuning via msProf/msprof-analyze, bottleneck analysis |
| `ascend_model_deploy` | MindIE Service deployment, OpenAI API compatibility test |
| `ascend_model_doc` | Auto-generate benchmark report and reproduction guide |
| `ascend_model_agent` | End-to-end orchestrator (mode: full/quick/tune) |

### Code Intelligence & Tool Compatibility Nodes

| Node | Description |
|------|-------------|
| `code_graph` | Extract code structure (imports/functions/calls) for Go/Python/JS/TS, output JSON or Mermaid graph |
| `file_watch` | Watch a path for file changes (create/modify/delete), polling-based, context-aware |
| `glob` | Recursive file glob matching (`**/*.go`), depth-limited, Codex-compatible |
| `grep` | Recursive content search with regex, binary-skip, Codex/OpenCode-compatible |
| `list_dir` | List directory contents (optional recursive), Codex-compatible |
| `apply_patch` | Apply unified diff patches atomically (validate-then-commit), Codex-compatible |

---

## 🔒 Security

llm-box takes security seriously. Key protections:

| Protection | Implementation |
|------------|----------------|
| **SSRF Protection** | Custom `DialContext` validates IPs at connect time (prevents DNS rebinding) |
| **Path Traversal** | Input validation + symlink resolution, all paths confined to working directory |
| **Command Injection** | Allowlist mode blocks shell metacharacters; safe mode disables execution entirely |
| **Secrets Management** | AES-GCM encryption with PBKDF2 (600K iterations), file permissions `0600` |
| **Timing Attack** | `subtle.ConstantTimeCompare` for authentication tokens |
| **Fail-Closed Auth** | Empty token = request rejected (503) |
| **Audit Logging** | All commands logged with redacted secrets, `0600` permissions |
| **DID Identity** | W3C DID format validation, signature verification, cross-domain message auth |
| **Memory Safety** | Layered memory with LRU eviction, symlink-resistant persistence, 0600 file perms |
| **Prompt Injection** | Skill evolution sanitizes best practices/pitfalls before injecting into prompts |
| **Secret Redaction** | Auto-detect & mask 10+ secret patterns (AWS/GitHub/Slack/JWT/private keys) on file read; `.env`/credentials fully masked by default |
| **Outbound Monitoring** | Sliding-window data volume monitor with anomaly alerting (prevents Grok-Build-style 27800× data leaks) |
| **Circuit Breaker** | Per-worker breaker (Closed→Open→HalfOpen), auto-isolation of failing nodes prevents cascade failures |
| **Atomic Patches** | `apply_patch` validates-then-commits with temp staging + atomic rename; no partial writes on failure |
| **ANSI Injection** | TUI Markdown/Mermaid renderers strip terminal control sequences (CSI/OSC/DCS) from user input |
| **Tool Portability** | Codex/OpenCode-compatible tools (glob/grep/list_dir/apply_patch) with full path/symlink/DoS hardening |

📖 [Security Guide →](SECURITY.md) | [Audit Logs →](docs/getting-started.md#audit-logs)

---

## 🌐 Ecosystem

llm-box participates in multiple open-source ecosystems:

| Ecosystem | Status | Description |
|-----------|--------|-------------|
| **GitCode G-Star** | Applied | Compute support, traffic exposure, HarmonyOS certification |
| **HarmonyOS Agent Skills** | Published | 8 skills: ability launch, atomic service, widget, device adapt, cross-app, agent message, intent router, device state |
| **ohpm SDK** | Published | `@llm-box/workflow-engine` — ArkTS SDK with WorkflowEngine, 30+ node types, device adaptation, intent protocol |
| **Ascend NPU Adaptation** | Active | 7-agent auto-adapt pipeline, 3 workflow templates (end-to-end/quick/performance-tune), CANN/MindIE integration |
| **GitHub** | Active | CI/CD, CodeQL security scan, automated releases |

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

### Ascend NPU Hardware Support

| Hardware | Position | Model Scale |
|----------|----------|-------------|
| Ascend 910B | Training/Inference | 7B-70B |
| Ascend 910C | Training/Inference | 7B-170B |
| Atlas 800I A2 | Inference Server | 7B-70B |
| Atlas 300I Duo | Edge Inference | <13B |
| 310P | Edge Inference | <7B |

📖 [G-Star Application →](ecosystem/GSTAR_APPLICATION.md) | [HarmonyOS Skills →](ecosystem/harmonyos-skills/) | [ohpm SDK →](ecosystem/ohpm/) | [Ascend Adaptation →](ecosystem/ascend-adaptation/ASCEND_ADAPTATION.md)

---

## 📚 Documentation

### Getting Started

- [Getting Started Guide](docs/getting-started.md) — Installation, first workflow, next steps
- [Examples](examples/) — 10 ready-to-use workflow templates

### Core Concepts

- [Dataflow & Variables](docs/dataflow.md) — Step-to-step data passing, `{{input}}`, `{{step.N}}`, `{{var.NAME}}`, `{{secret.GROUP.KEY}}`
- [Distributed Execution](docs/distributed.md) — Coordinator/Worker setup, configuration, scaling
- [Scheduling](docs/scheduling.md) — Cron workflows, schedule management
- [MCP Integration](docs/mcp.md) — Connect external tools via Model Context Protocol
- [Plugins](docs/plugins.md) — Install and manage community plugins

### Advanced

- [Web UI Editor](docs/webui.md) — Visual workflow builder
- [Visualizer](docs/visualizer.md) — Mermaid/JSON/DOT/ASCII diagrams
- [Tenant Isolation](docs/tenants.md) — Multi-tenant workspace separation
- [Custom Nodes](docs/custom-nodes.md) — Build nodes in any language
- [Troubleshooting](docs/troubleshooting.md) — Error codes, common issues, FAQ

### Reference

- [Workflow YAML Syntax →](docs/getting-started.md#workflow-configuration)
- [Node Reference →](docs/custom-nodes.md#built-in-nodes)
- [CLI Reference →](docs/getting-started.md#cli-command-reference)
- [Error Codes →](docs/troubleshooting.md#error-codes)

---

## 🛠️ CLI Commands

```bash
llm-box create [description]    Generate workflow from description
llm-box run <file>             Run a workflow
llm-box secrets add            Store an encrypted secret
llm-box secrets list           List secrets in a group
llm-box schedule create        Create a scheduled workflow
llm-box schedule list          List scheduled workflows
llm-box coordinator            Start distributed coordinator
llm-box worker                 Start distributed worker
llm-box ui                     Start web UI editor
llm-box visualize <file>       Visualize a workflow
llm-box validate <file>        Validate a workflow file
llm-box node install           Install external node
llm-box plugin install         Install a plugin
llm-box version                Show version
llm-box help                   Show full help
```

📖 [Full CLI Reference →](docs/getting-started.md#cli-command-reference)

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
- **Generator** — Keyword-based workflow generation from natural language, 100+ ready-to-use templates
- **Parser** — YAML workflow validation and parsing
- **Executor** — Deterministic step execution with dependency tracking
- **Expression Engine** — Variable substitution, secrets injection, file reading
- **Registry** — 50+ built-in nodes plus external node discovery and loading
- **Edge Router** — ReAct reasoning loop, 3-tier persistent memory, local/cloud model routing
- **Skill Evolution** — Self-improving agent skills with success rate tracking and prompt optimization
- **Intent Protocol** — `intent://` and `ohos://` URI schemes, W3C DID identity, cross-domain messaging
- **Ascend Adaptation** — 7-agent pipeline for Ascend NPU model adaptation (search/verify/adapt/quantize/optimize/deploy/doc)
- **Code Intelligence** — Code graph extraction, Codex/OpenCode-compatible tool nodes (glob/grep/list_dir/apply_patch)
- **Subagent Prompts** — 17 specialist prompt templates, main/sub agent hierarchy (Grok Build pattern)
- **Circuit Breaker** — Per-worker Closed/Open/HalfOpen state machine for distributed resilience
- **Privacy Layer** — Secret redaction on file read, outbound data volume anomaly monitor
- **Coordinator/Worker** — Distributed task scheduling and execution with circuit breaker protection

---

## 🗺️ Roadmap

| Version | Status | Features |
|---------|--------|----------|
| **v0.1** | ✅ Released | Core workflow engine, 10 utility nodes |
| **v0.2** | ✅ Released | LLM nodes, MCP integration, external nodes |
| **v0.3** | ✅ Released | Agent nodes, distributed execution, Web UI, scheduling |
| **v0.4** | ✅ Released | Code interpreter, RAG, knowledge graph, smart router, multimodal, node marketplace, 100+ templates, 16 specialists, Chain-of-Thought |
| **v0.5** | ✅ Released | ReAct engine, layered memory, skill self-evolution, HarmonyOS adaptation (7 device types), cross-platform protocol (intent:// + ohos://), W3C DID identity, cross-domain agent messaging, GitCode G-Star + ohpm ecosystem |
| **v0.5.1** | ✅ Released | Ascend NPU adaptation (7-agent pipeline, 3 workflow templates, CANN/MindIE integration) |
| **v0.5.2** | ✅ Released | Grok Build-inspired capabilities: code graph, subagent prompt hierarchy, circuit breaker, secret redaction, file watch, Codex/OpenCode tool compat, TUI Markdown/Mermaid rendering (15-vuln audited) |
| **v1.0** | 📅 Q3 2026 | Stable API, full documentation, LTS |

📖 [Full Roadmap →](ROADMAP.md)

---

## 🤝 Contributing

We welcome contributions from the community!

### Good First Issues

New to the project? Start here:

- 🐛 [Bug fixes labeled "good first issue"](https://github.com/alib8b8/llm-box/labels/good%20first%20issue)
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

📖 [Full Contributing Guide →](CONTRIBUTING.md)

---

## 📄 License

MIT License — see [LICENSE](LICENSE) for details.

---

<div align="center">
  <p>Built with ❤️ for developers who love the terminal</p>
  <p>
    <a href="https://github.com/alib8b8/llm-box">GitHub</a>
    ·
    <a href="https://github.com/alib8b8/llm-box/issues">Issues</a>
    ·
    <a href="https://github.com/alib8b8/llm-box/discussions">Discussions</a>
  </p>
</div>
