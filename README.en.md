<div align="center">
  <h1>llm-box</h1>
  <p>🌍
    <a href="README.md">中文</a> ·
    <strong>English</strong> ·
    <a href="README.ru.md">Русский</a>
  </p>
  <p><strong>Turn Natural Language Into Executable Workflows</strong></p>
  <p>Agentic Workflow Engine for the Terminal &mdash; deterministic execution meets AI agents. Build self-driving workflows with autonomous agent nodes, tool use, and multi-step reasoning.</p>

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

- [🏆 Awards &amp; Certifications](#-awards--certifications)
- [🚀 Quick Start](#-quick-start)
- [✨ Core Features](#-core-features)
- [🤖 Agent Nodes](#-agent-nodes)
- [📱 HarmonyOS &amp; Mobile Nodes](#-harmonyos--mobile-nodes)
- [🔒 Security](#-security)
- [🌐 Ecosystem](#-ecosystem)
- [📚 Documentation](#-documentation)
- [🛠️ CLI Commands](#-cli-commands)
- [🏗️ Architecture](#-architecture)
- [🗺️ Roadmap](#-roadmap)
- [🌟 Featured Integrations](#-featured-integrations)
- [📖❓ FAQ](#-faq)
- [🤝 Contributing](#-contributing)
- [📦 Code Hosting](#-code-hosting)
- [📄 License](#-license)

---

## 🏆 Awards &amp; Certifications

| Award | Issued By |
|-------|-----------|
| **GitCode G-Star Quality Open Source Project** | GitCode / AtomGit |
| **OpenAtom Open Source Foundation** | OpenAtom Foundation |

---

## 🚀 Quick Start

**One command to install:**

| macOS | Linux | Windows |
|-------|-------|---------|
| `brew install alib8b8/tap/llm-box` | `curl -fsSL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**🌏 China users &mdash; use mirror for faster download:**

| macOS/Linux | Windows |
|-------------|---------|
| `curl -fsSL https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh \| bash` | `irm https://ghproxy.com/https://raw.githubusercontent.com/alib8b8/llm-box/main/install.ps1 \| iex` |

**Or download from [GitCode Releases](https://gitcode.com/llm-box/llm-box/releases) / [GitHub Releases](https://github.com/alib8b8/llm-box/releases)**

📖 [Interactive Download Page →](docs/download.html)

---

### Create &amp; run your first workflow

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
| **Workflow Generation** | Natural language &rarr; YAML via keyword matching, 100+ built-in templates across 20+ categories |
| **Agent Nodes** | 35+ AI agent nodes with ReAct, Chain-of-Thought, tool use, and autonomous reasoning |
| **Edge AI Engine** | ReAct reasoning loop, 3-tier persistent memory (short/working/long-term), local/cloud model routing, privacy analyzer |
| **Skill Self-Evolution** | Agent skills improve with use &mdash; auto-tracks success rate, latency, best practices, known pitfalls; auto-optimizes prompts |
| **HarmonyOS Adaptation** | Ability launch, atomic service, desktop widget, 7-device-type adaptation (phone/foldable/tablet/TV/car/wearable) |
| **Phone Built-in AI** | System event listening (notification/call/SMS/location/battery), on-device LLM inference (1B-8B, INT4/INT8, including SenseNova U1), adaptive power management (eco/balanced/high, battery/thermal-aware), screen understanding, voice input (VAD+wake+ASR), voice output (TTS+voice cloning) |
| **WAIC-Aligned** | Blockchain audit trails for agent interoperability, embodied AI robot control (humanoid/arm/drone), L3 smart agent phone capabilities |
| **Cross-Platform Protocol** | `intent://` and `ohos://` URI schemes, W3C DID identity verification, cross-domain agent messaging |
| **Ascend NPU Adaptation** | 7-agent pipeline (search&rarr;verify&rarr;adapt&rarr;quantize&rarr;optimize&rarr;deploy&rarr;doc), CANN/MindIE/MindStudio integration, INT8/FP8 quantization, 1-hour auto-adapt |
| **Code Intelligence** | Code graph (AST/call graph/dependency for 158 languages), code knowledge graph (semantic vector retrieval, entity/relation/concept extraction, token-optimized), Codex/OpenCode-compatible tools (glob/grep/list_dir/apply_patch) |
| **Subagent Architecture** | Main/sub agent prompt hierarchy (17 specialist templates), borrowed from Grok Build's prompt.md + subagent_prompt.md pattern |
| **Distributed Resilience** | Per-node circuit breaker (Closed/Open/HalfOpen state machine), auto-isolation of failing workers, breaker stats endpoint |
| **Privacy by Design** | Auto secret redaction (.env/keys/tokens) on file read, outbound data volume monitor with anomaly alerting (prevents Grok-Build-style 27800&times; leaks) |
| **File Watching** | Polling-based file watch node (create/modify/delete events) for log-monitor and file-organizer workflows |
| **TUI Rendering** | Terminal Markdown renderer (headings/code/bold/italic/lists/quotes/tables) + Mermaid-to-ASCII converter (flow/sequence diagrams) |
| **Meta Orchestration** | Multi-model router (22+ models: OpenAI/Anthropic/Google/AndesGPT/SenseNova/AntLing/DeepSeek/Qwen), 5 strategies (auto/fastest/cheapest/best_quality/privacy_first), hierarchical agent network (supervisor&rarr;specialist&rarr;worker) |
| **MCP Protocol** | MCP bridge client (7 operations, 5 built-in tools) + MCP server mode (HTTP/WebSocket, tool exposure, session management, auth) |
| **Quality Guard** | Anti-AI-slop detection, 5 assessment types (AI detection/design/code/writing/overall), auto-fix with quality threshold enforcement |
| **Engineer Skills** | 16 pre-built skills across 4 domains (frontend/backend/devops/architecture): React/TypeScript/API/Database/CI-CD/Docker/Design Patterns |
| **Skill Distillation** | Extract methodologies from books/videos/podcasts/articles into callable skills: workflow/decision/analysis/creative/prompt/checklist |
| **Video Editing** | AI video editing: smart_cut/merge/effects/subtitle/storyboard/upscale, 4 styles, 720p/1080p/4k |
| **AI Gateway** | OmniRoute unified layer: 15+ providers, 6 routing strategies (auto/fastest/cheapest/best_quality/availability/custom_fallback), Claude Code/Cursor/Cline/llm-box compatibility |
| **Agent Memory** | Three-level memory infrastructure (short/medium/long): 8 operations (store/retrieve/delete/search/summary/forget/transfer/merge), LRU eviction, cross-session long-term memory |
| **Voice AI Toolchain** | Full voice studio: TTS + voice cloning + ASR transcription + speaker diarization + voice analysis, 11 languages, 5 ASR engines |
| **Agent Teamization** | 200+ professional specialist roles across 12+ domains, Agency workflow (8 phases: Discovery&rarr;Strategy&rarr;Design&rarr;Development&rarr;QA&rarr;Deployment&rarr;Launch&rarr;Post-Launch) |
| **Utility Nodes** | 40+ built-in nodes: LLM providers, fetch, execute, transform, file I/O, JSON, notify, condition, combine, call, template |
| **Data &amp; Knowledge** | RAG retrieval, knowledge graph extraction/query/traversal, smart model router, multimodal image analysis |
| **Code &amp; Tools** | Python code interpreter sandbox, node marketplace, MCP integration, plugin system |
| **Distributed Execution** | Coordinator/Worker architecture, horizontal scaling, heartbeat monitoring, circuit breaker |
| **Scheduling** | Cron-based scheduled workflows, interval triggers, CLI management |
| **Security** | SSRF protection, path traversal prevention, command injection defense, AES-GCM secrets, audit logging, secret redaction, ANSI injection defense, data race fixes, DoS prevention (subscription limits, log rotation, memory caps), 92+ vuln audited |
| **Ecosystem** | GitCode G-Star, HarmonyOS Agent Skills, ohpm SDK, OPPO X-OmniClaw Skill Adapter, OPPO 小布技能, AndesGPT, SenseNova, Ant Ling |
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
| `supervisor` | Oversees multi-agent workflows with sequential/parallel/hierarchical/MoE/MindSearch/Agency strategies and 200+ domain specialists |
| `code_review` | Automated code review and suggestions |
| `router` | Routes inputs to appropriate handlers |
| `human_in_loop` | Pauses for human approval |
| `meta_orchestrator` | Multi-model router with 22+ models, 5 strategies, hierarchical agent network |
| `code_knowledge_graph` | Semantic code knowledge graph: 158 languages, vector retrieval, entity/relation/concept extraction, MCP tool exposure, token-efficient review |
| `moe_streaming` | MoE expert streaming: consumer hardware runs 744B models with on-demand loading |
| `cli_session` | Interactive terminal session with context persistence, streaming output, auto-completion |
| `plugin_system` | Plugin extension: install/uninstall/update from local/git/url/market with sandbox isolation |
| `mcp_server` | MCP server mode: expose tools via HTTP/WebSocket with session management and auth |
| `mcp_bridge` | MCP client bridge: 7 operations, 5 built-in tools, protocol-compatible |
| `quality_guard` | Anti-AI-slop detection: 5 assessment types, auto-fix, quality threshold enforcement |
| `engineer_skills` | 16 pre-built skills: React/TypeScript/API/Database/CI-CD/Docker/Design Patterns |
| `skill_distill` | Distill methodologies from books/videos/podcasts into callable skills |
| `voice_output` | Voice AI toolchain: TTS + voice cloning + ASR transcription + speaker diarization + voice analysis, 11 languages, 5 ASR engines |
| `doc_gen` | AI document generation: 7 types (readme/api/function/module/changelog/tutorial/architecture) |
| `video_edit` | AI video editing: smart_cut/merge/effects/subtitle/storyboard/upscale |
| `omniroute` | AI gateway unified layer: 15+ providers, 6 routing strategies, Claude Code/Cursor/Cline compatibility |
| `memory` | Agent memory infrastructure: three-level memory (short/medium/long), 8 operations, LRU eviction, cross-session long-term memory |

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
| `andesgpt` | OPPO AndesGPT integration: Tiny (on-device 1B) / Turbo (edge-cloud 7B) / Titan (cloud 100B+), PersonaX personalization, end-cloud collaboration |

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

### Code Intelligence &amp; Tool Compatibility Nodes

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
| **Secret Redaction** | Auto-detect &amp; mask 10+ secret patterns (AWS/GitHub/Slack/JWT/private keys) on file read; `.env`/credentials fully masked by default |
| **Outbound Monitoring** | Sliding-window data volume monitor with anomaly alerting (prevents Grok-Build-style 27800&times; data leaks) |
| **Circuit Breaker** | Per-worker breaker (Closed&rarr;Open&rarr;HalfOpen), auto-isolation of failing nodes prevents cascade failures |
| **Atomic Patches** | `apply_patch` validates-then-commits with temp staging + atomic rename; no partial writes on failure |
| **ANSI Injection** | TUI Markdown/Mermaid renderers strip terminal control sequences (CSI/OSC/DCS) from user input |
| **Tool Portability** | Codex/OpenCode-compatible tools (glob/grep/list_dir/apply_patch) with full path/symlink/DoS hardening |
| **Session Limits** | CLI sessions auto-expire after 24h (max 500), MCP sessions capped at 1000 with cleanup |
| **Plugin Limits** | Max 100 plugins, HTTPS-only URLs, restricted git hosts (GitHub/GitLab/GitCode/Gitee) |
| **Resource Limits** | Code knowledge graph: max 5000 files/depth 5; video edit: shell metacharacter filtering |
| **Concurrent Safety** | Per-session rand mutex, RWMutex for shared state, no global mutable state without locks |
| **Auto Fix** | CI auto-runs gofmt/go vet/gosec/govulncheck, auto-commits fixes on detection |

📖 [Security Guide →](SECURITY.md) | [Audit Logs →](docs/getting-started.md#audit-logs)

---

## 🌐 Ecosystem

llm-box participates in multiple open-source ecosystems:

| Ecosystem | Status | Description |
|-----------|--------|-------------|
| **GitCode G-Star** | Applied | Compute support, traffic exposure, HarmonyOS certification |
| **HarmonyOS Agent Skills** | Published | 8 skills: ability launch, atomic service, widget, device adapt, cross-app, agent message, intent router, device state |
| **ohpm SDK** | Published | `@llm-box/workflow-engine` &mdash; ArkTS SDK with WorkflowEngine, 30+ node types, device adaptation, intent protocol |
| **Ascend NPU Adaptation** | Active | 7-agent auto-adapt pipeline, 3 workflow templates (end-to-end/quick/performance-tune), CANN/MindIE integration |
| **SenseNova** | Active | API integration (6 models), on-device U1-Lite support (8B/A3B MoE), 8 skills for SenseNova ecosystem |
| **Ant Ling** | Active | API integration (4 models: ling-2.6-flash/ling-2.6-1t/ring-2.6-1t/ming-flash-omni-2.0), OpenAI-compatible endpoint |
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
| Atlas 300I Duo | Edge Inference | &lt;13B |
| 310P | Edge Inference | &lt;7B |

📖 [G-Star Application →](ecosystem/GSTAR_APPLICATION.md) | [HarmonyOS Skills →](ecosystem/harmonyos-skills/) | [ohpm SDK →](ecosystem/ohpm/) | [Ascend Adaptation →](ecosystem/ascend-adaptation/ASCEND_ADAPTATION.md)

---

## 📚 Documentation

### Getting Started

- [Getting Started Guide](docs/getting-started.md) &mdash; Installation, first workflow, next steps
- [Examples](examples/) &mdash; 10 ready-to-use workflow templates

### Core Concepts

- [Dataflow &amp; Variables](docs/dataflow.md) &mdash; Step-to-step data passing, `{{input}}`, `{{step.N}}`, `{{var.NAME}}`, `{{secret.GROUP.KEY}}`
- [Distributed Execution](docs/distributed.md) &mdash; Coordinator/Worker setup, configuration, scaling
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
- **Ascend Adaptation** &mdash; 7-agent pipeline for Ascend NPU model adaptation (search/verify/adapt/quantize/optimize/deploy/doc)
- **Code Intelligence** &mdash; Code graph extraction, Codex/OpenCode-compatible tool nodes (glob/grep/list_dir/apply_patch)
- **Subagent Prompts** &mdash; 17 specialist prompt templates, main/sub agent hierarchy (Grok Build pattern)
- **Circuit Breaker** &mdash; Per-worker Closed/Open/HalfOpen state machine for distributed resilience
- **Privacy Layer** &mdash; Secret redaction on file read, outbound data volume anomaly monitor
- **Coordinator/Worker** &mdash; Distributed task scheduling and execution with circuit breaker protection

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
| **v0.6.0** | **Current** | **Ant Ling ecosystem, AI Gateway (OmniRoute), Agent Memory Infrastructure, Voice AI Toolchain (ASR/diarization/analysis), Agent Teamization (200+ roles + Agency workflow)** |
| **v1.0** | 📅 Q3 2026 | Stable API, full documentation, LTS |

📖 [Full Roadmap &rarr;](ROADMAP.md)

---

## 🌟 Featured Integrations

Excellent open-source projects built with llm-box:

| Project | Description |
|---------|-------------|
| [AI News Assistant]() | AI news aggregation and summary system based on llm-box workflows |
| [Code Review Agent]() | Automated code review tool leveraging code knowledge graph nodes |
| [Research Assistant]() | Academic research workflow combining researcher + knowledge_graph nodes |

> If your project uses llm-box, feel free to submit a PR to add it here!

---

## 📖❓ FAQ

### 1. How is llm-box different from other agent frameworks?

llm-box focuses on the **combination of deterministic workflows and AI agents**: workflows ensure reliable and reproducible execution, while agent nodes provide intelligent reasoning capabilities. We don't rely on a single model provider, supporting 22+ models with 5 routing strategies. The core is written in Go with zero dependencies — fast startup and low memory footprint.

### 2. Which large language models are supported?

Currently supporting **22+ models** across mainstream providers:
- **Domestic**: SenseNova, Ant Ling, AndesGPT (OPPO), DeepSeek, Qwen (Alibaba)
- **International**: OpenAI, Anthropic Claude, Google Gemini, Inkling (Thinking Machines)
- **On-device**: llama.cpp, ONNX Runtime, SenseNova U1 (INT4/INT8)

### 3. Can it be used in enterprise environments?

Absolutely. llm-box uses the MIT license and provides:
- **Tiered security configuration** (L0-L3), supporting security gradients from development to production
- **Secret redaction** + **outbound data monitoring** to prevent data leaks
- **Audit logging** for traceability of all operations
- **Distributed circuit breakers** for system stability

### 4. How do I extend with custom nodes?

llm-box supports three extension methods:
- **Plugin System**: Install plugins from local/git/URL/market with sandbox isolation
- **MCP Protocol**: Connect external tools via Model Context Protocol
- **Custom Nodes**: Implement standard interfaces in any language — see [custom nodes guide](docs/custom-nodes.md)

### 5. What's the long-term roadmap?

Short-term (v0.6-v0.9): Complete agent team collaboration, multimodal capabilities, performance optimization
Long-term (v1.0+): Stable API, LTS releases, enterprise support, more hardware adaptation (Ascend/Cambricon/Hygon)
See [Roadmap →](#-roadmap) for details.

---

## 📦 Code Hosting

llm-box is synced across multiple platforms — follow and contribute on your preferred platform:

| Platform | Link |
|----------|------|
| **GitHub** | https://github.com/alib8b8/llm-box |
| **GitCode / AtomGit** | https://gitcode.com/llm-box/llm-box |

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

📖 [Full Contributing Guide &rarr;](CONTRIBUTING.md)

---

## 📄 License

MIT License &mdash; see [LICENSE](LICENSE) for details.

---

<div align="center">
  <p>Built with ❤️ for developers who love the terminal</p>
  <p>
    <a href="https://github.com/alib8b8/llm-box">GitHub</a>
    &middot;
    <a href="https://github.com/alib8b8/llm-box/issues">Issues</a>
    &middot;
    <a href="https://github.com/alib8b8/llm-box/discussions">Discussions</a>
  </p>
</div>