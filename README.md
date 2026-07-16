<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Turn Natural Language Into Executable Workflows</strong></p>
  <p>Agentic Workflow Engine for the Terminal — deterministic execution meets AI agents. Build self-driving workflows with autonomous agent nodes, tool use, and multi-step reasoning.</p>

  <p>
    <a href="https://github.com/alib8b8/llm-box/actions/workflows/ci.yml">
      <img src="https://img.shields.io/github/actions/workflow/status/alib8b8/llm-box/ci.yml?branch=main&style=flat-square&label=CI" alt="CI Status" />
    </a>
    <a href="https://codecov.io/gh/alib8b8/llm-box">
      <img src="https://img.shields.io/codecov/c/github/alib8b8/llm-box?style=flat-square&label=coverage" alt="Coverage" />
    </a>
    <a href="https://github.com/alib8b8/llm-box/releases">
      <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?display_name=tag&include_prereleases&style=flat-square" alt="release" />
    </a>
    <a href="https://golang.org/">
      <img src="https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat-square" alt="Go" />
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
- [🔒 Security](#-security)
- [📚 Documentation](#-documentation)
- [🛠️ CLI Commands](#️-cli-commands)
- [🏗️ Architecture](#️-architecture)
- [🗺️ Roadmap](#️-roadmap)
- [🤝 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🚀 Quick Start

Install in 60 seconds:

```bash
# Linux/macOS
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh

# macOS (Homebrew)
brew install alib8b8/tap/llm-box

# Windows
Invoke-WebRequest -Uri "https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-windows-amd64.exe" -OutFile llm-box.exe
```

Create and run your first workflow:

```bash
# Generate a workflow from natural language (keyword-based, no API key needed)
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
| **Workflow Generation** | Natural language → YAML via keyword matching, 10+ built-in templates |
| **Agent Nodes** | 10 AI agent nodes: ReAct agent, planner, researcher, critic, evaluator, reflector, supervisor, code reviewer, router, human-in-the-loop |
| **Utility Nodes** | 20+ built-in nodes: fetch, execute, transform, file I/O, JSON, notify, condition, combine, call, template |
| **Distributed Execution** | Coordinator/Worker architecture, horizontal scaling, heartbeat monitoring |
| **Scheduling** | Cron-based scheduled workflows, interval triggers, CLI management |
| **Security** | SSRF protection, path traversal prevention, command injection defense, AES-GCM secrets, audit logging |
| **Extensibility** | Custom nodes in any language, MCP integration, plugin system, external node registry |
| **Developer Experience** | Web UI editor, workflow visualizer (Mermaid/JSON/DOT/ASCII), TUI, 9 languages |

---

## 🤖 Agent Nodes

10 specialized AI agent nodes for autonomous reasoning:

| Node | Description |
|------|-------------|
| `agent` | General-purpose ReAct agent with tool use |
| `planner` | Breaks tasks into step-by-step plans |
| `researcher` | Web research and information gathering |
| `critic` | Reviews and provides constructive feedback |
| `evaluator` | Evaluates outputs against criteria |
| `reflector` | Reflects on process and suggests improvements |
| `supervisor` | Oversees multi-agent workflows |
| `code_review` | Automated code review and suggestions |
| `router` | Routes inputs to appropriate handlers |
| `human_in_loop` | Pauses for human approval |

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

📖 [Security Guide →](SECURITY.md) | [Audit Logs →](docs/getting-started.md#audit-logs)

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
- **Generator** — Keyword-based workflow generation from natural language
- **Parser** — YAML workflow validation and parsing
- **Executor** — Deterministic step execution with dependency tracking
- **Expression Engine** — Variable substitution, secrets injection, file reading
- **Registry** — Built-in + external node discovery and loading
- **Coordinator/Worker** — Distributed task scheduling and execution

---

## 🗺️ Roadmap

| Version | Status | Features |
|---------|--------|----------|
| **v0.1** | ✅ Released | Core workflow engine, 10 utility nodes |
| **v0.2** | ✅ Released | LLM nodes, MCP integration, external nodes |
| **v0.3** | ✅ Released | Agent nodes, distributed execution, Web UI, scheduling |
| **v0.4** | 🔄 In Progress | Marketplace, multi-language, resource limits |
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
