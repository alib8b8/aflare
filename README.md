<div align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200" />
  <h1>llm-box</h1>
  <p><strong>Turn Natural Language Into Executable Workflows</strong></p>
  <p>Agentic Workflow Engine for the Terminal — deterministic execution meets AI agents. Build self-driving workflows with autonomous agent nodes, tool use, and multi-step reasoning.</p>

  <p>
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
      <img src="https://github.com/alib8b8/llm-box/actions/workflows/release.yml/badge.svg" alt="CI status" />
    </a>
    <a href="https://cobusgreyling.github.io/loop-engineering/">
      <img src="https://img.shields.io/badge/Loop_Ready-L2_(100%2F100)-58a6ff?style=flat-square" alt="Loop Ready L2 (100/100)" />
    </a>
  </p>


</div>

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
# Download from releases: https://github.com/alib8b8/llm-box/releases/latest
Invoke-WebRequest -Uri "https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-windows-amd64.exe" -OutFile llm-box.exe
```

Create and run your first workflow in seconds:

```bash
# DevOps — monitor server health
llm-box create "Check my server CPU every 5 minutes and alert if high"

# AI Research — stay updated
llm-box create "Summarize today's AI news"

# Crypto Trading — automated alerts
llm-box create "Alert me when Bitcoin exceeds 100000"

# Run any workflow
llm-box run server_monitor.yaml
```

---

## 💡 Why Choose llm-box?

**Not Another AI Chatbot**

Most workflow tools force developers to choose between:

| Approach | Problem |
|----------|---------|
| Complex bash scripts | Hard to read, maintain, share |
| Heavy visual builders | Slow, opaque, require GUI |
| Endless config files | Steep learning curve, verbose syntax |

**llm-box is not just another workflow tool — it's a deterministic execution engine with agentic superpowers.**

- ✅ **Predictable & Auditable** — Workflow steps are deterministic, agent behavior is constrained
- ✅ **Agentic Nodes** — 10+ AI agent nodes: `agent` (ReAct), `planner`, `researcher`, `critic`, `evaluator`, `reflector`, `supervisor`, `code_review`, `router`, `human_in_loop`
- ✅ **Local-First** — Your data never leaves your terminal
- ✅ **Transparent & Reproducible** — Same workflow produces same results
- ✅ **MIT Open Source** — No vendor lock-in, no hidden barriers

> 💡 AI agents plan and reason, but llm-box's deterministic code execution keeps everything reliable.

---

## ✨ Features

1. **Generate workflows from plain English** — describe what you want, llm-box writes the YAML
2. **Run them locally** — single binary, zero dependencies, your data never leaves your machine
3. **Share them as reusable templates** — version, publish, and reuse workflows across projects

---

## ⚙️ How It Works

```
┌─────────┐     ┌─────────┐     ┌──────────────┐     ┌──────────┐     ┌────────┐
│  Prompt │────▶│ Planner │────▶│ Workflow YAML│────▶│ Executor │────▶│ Result │
└─────────┘     └─────────┘     └──────────────┘     └──────────┘     └────────┘
     │               │                  │                  │
     │               │                  │                  ▼
     │               │                  │           ┌──────────────┐
     │               │                  │           │  Agent Nodes │
     │               │                  │           │  ReAct Loop  │
     │               │                  │           └──────────────┘
     │               │                  │
     │               │                  ▼
     │               │           ┌──────────────┐
     │               │           │ Utility Nodes│
     │               │           │ fetch, exec  │
     │               │           └──────────────┘
     │               │
     ▼               ▼
┌─────────────────────────────────────────────────────────────┐
│          "Check server CPU every 5 minutes"                  │
│                      ↓                                       │
│      ┌─────────────────────────────┐                        │
│      │ steps:                      │                        │
│      │   - node: execute           │                        │
│      │     params:                 │                        │
│      │       command: top -bn1     │                        │
│      │   - node: transform         │                        │
│      │     params:                 │                        │
│      │       operation: extract_cpu│                        │
│      │   - node: notify            │                        │
│      │     params:                 │                        │
│      │       channel: stdout       │                        │
│      └─────────────────────────────┘                        │
└─────────────────────────────────────────────────────────────┘
```

1. **Prompt** — describe what you want in plain English
2. **Planner** — AI breaks it down into executable steps
3. **Workflow YAML** — generates a structured, versioned workflow file
4. **Executor** — runs each step deterministically (agent nodes use ReAct reasoning)
5. **Result** — output to terminal, file, or notification

---

## 🔒 Security

llm-box takes security seriously. Here's how we protect you:

### Security Protections

| Protection | Description | Implementation |
|------------|-------------|----------------|
| **SSRF Protection** | Blocks access to localhost and private IPs | Custom `DialContext` validates resolved IPs at connect time, preventing DNS rebinding attacks |
| **Path Traversal** | Blocks `../` and absolute paths | Input validation rejects paths starting with `/` or containing `..` |
| **Command Injection** | Blocks shell metacharacters | Allowlist mode strips `;`, `|`, `&`, `` ` ``, `$`, `>` from commands |
| **Template SSTI** | Prevents server-side template injection | Expression engine escapes all output, no arbitrary code evaluation |
| **Timing Attack** | Constant-time token comparison | Uses `subtle.ConstantTimeCompare` for authentication tokens |
| **Fail-Closed Auth** | Rejects requests when auth token is empty | Authentication middleware defaults to deny |
| **Git Option Injection** | Blocks `-` prefixed branch names | Uses `--` delimiter to prevent option injection in git commands |

### Command Execution Safety (`execute` node)

The `execute` node runs shell commands, which is inherently powerful and risky. llm-box implements multiple layers of defense:

| Layer | What it does | How to enable |
|-------|-------------|---------------|
| **Safe Mode** | Completely disables the `execute` node | `llm-box --safe-mode` or `LLM_BOX_SAFE_MODE=1` |
| **Allowlist** | Only allows known-safe commands; blocks shell metacharacters (`;`, `|`, `&`, `` ` ``, etc.) | `LLM_BOX_EXECUTE_ALLOWLIST=1` |
| **Audit Logging** | Every command logged to `~/.llm-box/logs/audit.log` with timestamp, user, and redacted secrets (0600 permissions) | Enabled automatically |
| **Dry Run** | Preview commands before execution without running them | `dry_run: true` param on `execute` node |
| **Timeout** | Commands auto-terminate after a configurable duration (default: 5m, max: 30m) | `timeout: 30s` param on `execute` node |
| **Input Validation** | Empty commands rejected; shell injection patterns blocked | Always on |

**Recommended setup for CI/production:**
```bash
export LLM_BOX_EXECUTE_ALLOWLIST=1
export LLM_BOX_SAFE_MODE=1   # if you don't need shell execution at all
```

### Secrets Management

API keys are **never stored in plain text**:

- AES-GCM encryption with PBKDF2 key derivation (600K iterations)
- Master password protected
- File permissions `0600`
- Values masked in listings (e.g., `sk-****bc`)
- Automatic redaction in audit logs (Bearer tokens, Authorization headers, URL credentials)

**CLI Commands:**
```bash
# Add a secret
llm-box secrets add --group llm --key openai --value sk-...

# List secrets in a group
llm-box secrets list llm

# Remove a secret
llm-box secrets remove --group llm --key openai

# Export secrets (encrypted)
llm-box secrets export llm --output secrets-backup.json

# Import secrets
llm-box secrets import --input secrets-backup.json
```

#### Using Secrets in Workflows

Reference stored secrets in your workflow YAML using the `{{secret.GROUP.KEY}}` syntax:

```yaml
name: AI Summary Workflow
steps:
  - node: openai
    params:
      model: gpt-4o
      api_key: "{{secret.llm.openai}}"
    input: "Summarize this article"

  - node: http_request
    params:
      url: "https://api.example.com/data"
      headers: "Authorization: Bearer {{secret.api.example}}"
```

**Secrets Expression Syntax:**

| Syntax | Description | Example |
|--------|-------------|---------|
| `{{secret.GROUP.KEY}}` | Retrieve secret by group and key | `{{secret.llm.openai}}` |

**Supported Expression Syntax (Full Reference):**

| Expression | Description | Example |
|------------|-------------|---------|
| `{{secret.llm.openai}}` | Secret value from group/key | API key |
| `{{var.name}}` | Workflow variable | Defined in vars section |
| `{{env.NAME}}` | Environment variable | OS env var (allowed list only) |
| `{{step.0}}` | Output of step 0 (0-indexed) | Previous step output |
| `{{step.name}}` | Output by step name | Named step output |
| `{{step.0.jsonpath:$.field}}` | JSONPath extraction from step output | Extract specific field |
| `{{input}}` | Workflow initial input | User-provided input |
| `{{file.path}}` | File contents | Read from local file |
| `{{loop.item}}` | Current loop item | Inside loop context |
| `{{loop.index}}` | Current loop index | Inside loop context |
| `{{loop.count}}` | Total loop iterations | Inside loop context |

**Setup:**
1. Set `LLM_BOX_SECRETS_PASSWORD` environment variable (required for secrets operations)
2. Add secrets via CLI: `llm-box secrets add --group llm --key openai --value sk-...`
3. Reference in YAML: `{{secret.llm.openai}}`

**Security:**
- Secrets are decrypted only at runtime, never stored in memory longer than needed
- Missing secrets cause workflow execution to fail with a clear error message
- Secrets in audit logs are automatically redacted
- Secret files are stored with `0600` permissions (read/write only by owner)

**Example: Multiple secrets in a workflow**
```yaml
name: Multi-API Workflow
steps:
  - node: http_request
    params:
      url: "https://api.github.com/user"
      headers: |
        Authorization: Bearer {{secret.api.github}}
        Accept: application/vnd.github.v3+json
  - node: openai
    params:
      model: gpt-4o
      api_key: "{{secret.llm.openai}}"
      prompt: "Analyze this GitHub profile data: {{input}}"
  - node: file_write
    params:
      path: "analysis.txt"
```

#### Audit Logs

All commands are automatically logged with the following details:

```bash
# Default log path
~/.llm-box/logs/audit.log

# View recent logs
tail -f ~/.llm-box/logs/audit.log

# Search logs by keyword
grep "openai" ~/.llm-box/logs/audit.log

# Search failed operations
grep -i "error\|failed" ~/.llm-box/logs/audit.log
```

**Log Format:**

```json
{
  "time": "2026-07-16T10:00:00Z",
  "level": "info",
  "command": "run",
  "workflow": "my-workflow.yaml",
  "steps": 5,
  "duration_ms": 12345,
  "user": "john",
  "hostname": "workstation",
  "secrets_redacted": true
}
```

**Log Fields:**

| Field | Description |
|-------|-------------|
| `time` | ISO 8601 timestamp |
| `level` | Log level (info, warn, error) |
| `command` | Command executed |
| `workflow` | Workflow file name |
| `steps` | Number of steps |
| `duration_ms` | Execution duration in milliseconds |
| `user` | Current user |
| `hostname` | Machine hostname |
| `secrets_redacted` | Whether secrets were redacted |

**Security:**
- File permissions: `0600` (read/write only by owner)
- Secrets are automatically redacted from logs
- Environment variable override: `LLM_BOX_LOG_FILE=/path/to/audit.log`

### Secure Installation

The `curl | bash` pattern is convenient but carries supply-chain risk. For maximum safety:

```bash
# Option 1: Download + verify checksum
curl -sL https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-linux-amd64 -o llm-box
curl -sL https://github.com/alib8b8/llm-box/releases/latest/download/checksums.txt -o checksums.txt
sha256sum -c checksums.txt
chmod +x llm-box

# Option 2: Build from source
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go build -o llm-box ./cmd/llm-box
```

### Automated Security Scanning

Every PR is scanned with:
- **gosec** — security-focused static analysis
- **CodeQL** — GitHub's semantic analysis engine
- **go vet** — Go compiler static analysis
- **gofmt** — format consistency

See [SECURITY.md](SECURITY.md) for our vulnerability disclosure policy.

---

## 🔄 When To Use llm-box

We recommend the right tool for the job:

| Scenario | Recommended Tool | Why |
|----------|-----------------|-----|
| **Terminal automation & scripts** | **llm-box** | Natural language to YAML, deterministic execution, no dependencies |
| **Enterprise workflow orchestration** | n8n / Dify | Visual builder, enterprise integrations, role-based access |
| **AI-powered coding assistant** | Claude Code / Cursor | Deep IDE integration, codebase awareness, iterative coding |
| **Pure AI agent orchestration** | CrewAI / LangGraph | Multi-agent frameworks, Python-native, research-oriented |
| **Data ETL pipelines** | Apache Airflow / Dagster | DAG scheduling, data lineage, observability at scale |
| **Infrastructure as Code** | Terraform / Pulumi | Cloud resource management, state tracking, plan/apply workflow |
| **Scheduled cron jobs** | systemd timers / cron | Unix-native, zero overhead, battle-tested |
| **API testing & mocking** | Postman / Insomnia | Interactive request builder, collection sharing, collaboration |

### llm-box is built for engineers who want to:

- Replace fragile bash scripts with **structured, versioned workflows**
- Add **AI reasoning** to terminal automation without leaving the command line
- Keep **full transparency** — every step is YAML, auditable, and reproducible
- Stay **local-first** — no cloud dependency, no vendor lock-in

> 📖 [Full comparison →](docs/comparison.md)

---

## 🤖 Agent Nodes

llm-box includes **10 specialized AI agent nodes** that bring autonomous reasoning to your workflows:

### Core Agents

| Node | Description | Use Case |
|------|-------------|----------|
| `agent` | General-purpose ReAct agent with tool use | Autonomous task completion |
| `supervisor` | Supervisor agent that decomposes tasks and delegates to specialists | Orchestrating multi-agent workflows |

### Specialist Agents

| Node | Description | Use Case |
|------|-------------|----------|
| `planner` | Task decomposition agent | Break complex goals into steps |
| `researcher` | Research agent with web sources | Information gathering & synthesis |
| `critic` | Quality review agent that critiques and suggests improvements | Output quality control |
| `evaluator` | Scoring agent with rubrics and pass/fail thresholds | Quality gates & assessment |
| `reflector` | Self-reflection agent that iteratively improves output | Self-correction & refinement |
| `code_review` | Code review specialist agent | Automated code quality checks |
| `router` | Classification & routing agent | Smart workflow branching |

### Control Nodes

| Node | Description | Use Case |
|------|-------------|----------|
| `human_in_loop` | Human approval gate for workflow steps | Human review & sign-off |

### Example: Autonomous Research Workflow

```yaml
name: AI Research Agent
steps:
  - node: agent
    params:
      provider: ollama
      model: llama3
      tools: fetch_url,json_parse
      max_iterations: 5
    input: |
      Research the latest trends in agentic workflow engines.
      Fetch information from 2 sources and summarize key findings.
```

### Example: Multi-Agent Quality Pipeline

```yaml
name: Content Quality Pipeline
steps:
  - node: file_read
    params:
      path: draft.md
  - node: critic
    params:
      provider: ollama
      model: llama3
      role: writing
      suggest_improvements: "true"
  - node: evaluator
    params:
      provider: ollama
      model: llama3
      rubric: quality
      scale: "1-10"
      threshold: "7"
  - node: reflector
    params:
      provider: ollama
      model: llama3
      iterations: "2"
      reflection_focus: all
  - node: human_in_loop
    params:
      mode: file
      approval_file: .content-approved
  - node: file_write
    params:
      path: final.md
```

---

## 🎬 Demo

### 1. Generate Workflow from Plain English

```bash
$ llm-box create "check server CPU every 5 minutes and alert if > 80%"

✓ Generated workflow: cpu-monitor.yaml

steps:
  - node: execute
    params:
      command: "top -bn1 | grep 'Cpu(s)'"
  - node: transform
    params:
      operation: extract_cpu_percent
  - node: agent
    params:
      provider: ollama
      model: llama3
    input: "Alert if CPU usage exceeds 80%"
  - node: notify
    params:
      channel: stdout
```

### 2. Run the Workflow

### `llm-box create` Command Reference

Generate workflows from natural language descriptions using rule-based keyword matching.

**Usage**:
```bash
llm-box create "your natural language description" [options]
```

**Options**:
| Option | Description |
|--------|-------------|
| `-o`, `--output` | Output file path (default: auto-generated) |
| `--name` | Workflow name |
| `--dry-run` | Preview generated workflow without saving |
| `--provider` | Preferred LLM provider (openai, ollama, etc.) |

**Examples**:
```bash
# Basic creation
llm-box create "Fetch weather and save to file"

# Specify output file
llm-box create "Summarize GitHub activity" -o github-summary.yaml

# Preview without saving
llm-box create "Monitor server logs" --dry-run

# Specify preferred LLM provider
llm-box create "Analyze code" --provider openai
```

**Generated YAML**:
```bash
$ llm-box create "check server CPU every 5 minutes and alert if > 80%"

✓ Generated workflow: cpu-monitor.yaml

steps:
  - node: execute
    params:
      command: "top -bn1 | grep 'Cpu(s)'"
  - node: transform
    params:
      operation: extract_cpu_percent
  - node: agent
    params:
      provider: ollama
      model: llama3
    input: "Alert if CPU usage exceeds 80%"
  - node: notify
    params:
      channel: stdout
```

**Supported Keywords**:
| Keyword | Action | Example |
|---------|--------|---------|
| `fetch`, `get`, `download` | HTTP request | "Fetch API data" |
| `read`, `load` | File read | "Read config file" |
| `write`, `save` | File write | "Save to report.md" |
| `summarize`, `analyze`, `review` | LLM agent | "Summarize document" |
| `notify`, `alert`, `send` | Notification | "Notify on Slack" |
| `execute`, `run`, `command` | Shell execute | "Run backup script" |
| `json`, `parse` | JSON parse | "Parse JSON response" |
| `combine`, `merge` | Combine outputs | "Combine two files" |

**Limitations**:
- Rule-based generation (not AI-powered)
- Supports a fixed set of keywords
- For complex workflows, define YAML directly

### 2. Run the Workflow

```bash
$ llm-box run cpu-monitor.yaml

▶ cpu-monitor.yaml
  [1/4] execute     → top -bn1 | grep 'Cpu(s)'          ✓ 0.3s
  [2/4] transform   → extract_cpu_percent                ✓ 0.1s
  [3/4] agent       → llama3 reasoning...                ✓ 2.1s
  [4/4] notify      → channel=stdout                     ✓ 0.0s

Result: CPU at 45% — within normal range

✓ Workflow completed in 2.5s
```

### 3. Retry on Failure with Logs

```bash
$ llm-box run api-health-check.yaml

▶ api-health-check.yaml
  [1/3] http_request → GET https://api.example.com/health
  ⚠ Request failed: connection timeout
  ↻ Retry 1/3 after 5s...                                  ✓ 5.8s
  [2/3] json_parse   → status                             ✓ 0.1s
  [3/3] notify       → channel=stdout                     ✓ 0.0s

Result: API healthy — status: "ok"

✓ Workflow completed in 6.2s (1 retry)

📋 Audit log: ~/.llm-box/logs/2026-07-16_14-32-10_api-health-check.log
```

> **Record your own GIF**
> Use [vhs](https://github.com/charmbracelet/vhs) with `docs/demo.tape` to create high-quality terminal recordings.

---

## 🔧 Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        User Interfaces                             │
│  ┌──────────────────────┐  ┌────────────────────────────────────┐  │
│  │      Terminal CLI    │  │            Web UI Editor           │  │
│  │   llm-box create/run │  │  Visual workflow builder & preview │  │
│  └──────────────────────┘  └────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────┘
                             │                    │
                             ▼                    ▼
┌─────────────────────────────────────────────────────────────────────┐
│                  Natural Language Parser + Task Planner            │
│            "Fetch HN stories and summarize" → Executable Steps    │
└─────────────────────────────────────────────────────────────────────┘
                             │
          ┌──────────────────┼──────────────────┐
          ▼                  ▼                  ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│   Coordinator   │  │   Coordinator   │  │   Coordinator   │
│  (Node Manager) │  │  (Task Router)  │  │  (Heartbeat)    │
└─────────────────┘  └─────────────────┘  └─────────────────┘
          │                  │                  │
          └──────────────────┼──────────────────┘
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                     Distributed Worker Nodes                        │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ ┌───────────┐ ┌────────┐ │
│  │fetch_url │ │transform  │ │execute_cmd │ │file_write │ │  LLM   │ │
│  └──────────┘ └───────────┘ └────────────┘ └───────────┘ └────────┘ │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ ┌───────────┐ ┌────────┐ │
│  │http_req  │ │json_parse │ │template    │ │secrets    │ │plugins│ │
│  └──────────┘ └───────────┘ └────────────┘ └───────────┘ └────────┘ │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ ┌───────────┐ ┌────────┐ │
│  │  agent   │ │ planner   │ │ researcher │ │  critic   │ │evaluator│
│  │ (ReAct)  │ │           │ │            │ │           │ │        │
│  └──────────┘ └───────────┘ └────────────┘ └───────────┘ └────────┘ │
│  ┌──────────┐ ┌───────────┐ ┌────────────┐ ┌───────────┐ ┌────────┐ │
│  │reflector │ │supervisor │ │code_review │ │  router   │ │human_in│
│  │          │ │           │ │            │ │           │ │_loop   │
│  └──────────┘ └───────────┘ └────────────┘ └───────────┘ └────────┘ │
└─────────────────────────────────────────────────────────────────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────────────┐
│                       Output + Visualization                        │
│            (Terminal / File / Notification / Mermaid Diagram)      │
└─────────────────────────────────────────────────────────────────────┘
```

**Components:**
1. **Parser** - Interprets plain English commands
2. **Planner** - Breaks down into executable steps
3. **Coordinator** - Manages distributed nodes, assigns tasks, monitors heartbeats
4. **Workers** - Execute workflow steps across multiple machines
5. **Nodes** - Built-in and extensible actions (17+ utility and LLM nodes)
6. **Web UI** - Visual workflow editor with real-time preview
7. **Visualizer** - Generates Mermaid/JSON/DOT/ASCII diagrams
8. **Plugin System** - Extend core behavior with community plugins
9. **Custom Nodes** - Build your own nodes in any language
10. **Output** - Formatted results to terminal, file, or notifications

## 🌐 Distributed Execution

llm-box supports distributed workflow execution across multiple machines using a Coordinator/Worker architecture. The Coordinator manages nodes, assigns tasks, and monitors heartbeats. Workers execute workflow steps and report results back.

### Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        Coordinator                          │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐ │
│  │ Registry │ │ Tasks    │ │ Heartbeat│ │ Load Balancer  │ │
│  └────┬─────┘ └────┬─────┘ └────┬─────┘ └───────┬────────┘ │
└───────┼────────────┼────────────┼────────────────┼──────────┘
        │            │            │                │
        └────────────┴────────────┴────────────────┘
                              │
                    ┌─────────┴──────────┐
                    │   Worker Nodes     │
                    │ (Capacity varies)  │
                    └────────────────────┘
```

### Quick Start

**1. Start the Coordinator:**
```bash
# Default port 8090
llm-box coordinator --auth-token my-secret-token

# Custom port
llm-box coordinator --port 8090 --auth-token my-secret-token
```

**2. Start Workers (on each worker machine):**
```bash
# Default capacity (5 tasks)
llm-box worker --coordinator http://coordinator-host:8090 --auth-token my-secret-token

# Custom capacity
llm-box worker --coordinator http://coordinator-host:8090 --auth-token my-secret-token --capacity 10 --port 8091
```

**3. Submit a workflow:**
```bash
llm-box run --distributed http://coordinator-host:8090 my-workflow.yaml
```

### Configuration Reference

**Coordinator Options:**

| Option | Default | Required | Description |
|--------|---------|----------|-------------|
| `--port` | `8090` | No | HTTP port to listen on |
| `--auth-token` | (empty) | Yes* | Authentication token for worker registration |

**Worker Options:**

| Option | Default | Required | Description |
|--------|---------|----------|-------------|
| `--port` | `8091` | No | HTTP port to listen on |
| `--coordinator` | `http://localhost:8090` | Yes | Coordinator URL |
| `--auth-token` | (empty) | Yes* | Authentication token matching coordinator |
| `--capacity` | `5` | No | Maximum concurrent tasks this worker handles |

\* Required for production deployments.

**Environment Variables:**

| Variable | Description |
|----------|-------------|
| `LLM_BOX_COORDINATOR` | Default coordinator URL |
| `LLM_BOX_AUTH_TOKEN` | Default authentication token |

### Security & Networking

- **Authentication**: Use `--auth-token` on both Coordinator and Workers; every request validates the token via constant-time comparison.
- **TLS/HTTPS**: For production, place an HTTPS reverse proxy (e.g., nginx + Let's Encrypt) in front of the Coordinator.
- **Firewall**: Restrict ports so only Workers can reach the Coordinator.
- **Secrets Sync**: Each worker stores its own secrets. Use one of the following:
  - **Manual sync**: `llm-box secrets export > secrets.json` on Coordinator, then `llm-box secrets import < secrets.json` on each Worker.
  - **Shared volume**: Mount an encrypted secrets volume accessible to all Workers (permissions `0600`).
  - **Environment**: Set `LLM_BOX_SECRETS_PASSWORD` on all nodes.

### Example Distributed Workflow

Workflows are authored the same way as local ones:

```yaml
name: distributed-data-processing
vars:
  api_key: "{{secret.api.service}}"

steps:
  - node: http_request
    id: fetch_data
    params:
      url: "https://api.example.com/data"
      headers: "Authorization: Bearer {{var.api_key}}"

  - node: json_parse
    id: parse_data
    input: fetch_data
    params:
      path: results

  - node: agent
    id: analyze
    input: "Analyze this data: {{step.parse_data}}"
    params:
      provider: ollama
      model: llama3

  - node: file_write
    id: save_report
    input: analyze
    params:
      path: analysis-report.md
```

> 📖 [Full Distributed Execution Documentation →](docs/distributed.md)

---

## 🌐 Web UI Editor

A built-in web-based workflow editor with visualization capabilities.

**Start the Web UI:**
```bash
# Default port 8081
llm-box webui

# Custom port
llm-box webui --port 8080

# Custom workflows directory
llm-box webui --dir ./workflows
```

**Access:** `http://localhost:8081`

**Features:**
- Syntax-highlighted YAML editor
- Real-time workflow validation
- Visual preview (Mermaid/JSON/DOT/ASCII)
- REST API endpoints for programmatic access

> 📖 [Full Web UI Documentation →](docs/webui.md)

---

## 📊 Workflow Visualizer

Generate visual diagrams from workflow YAML files.

**Usage:**
```bash
# Generate Mermaid diagram (default)
llm-box visualize workflow.yaml

# Specific format
llm-box visualize workflow.yaml --format mermaid
llm-box visualize workflow.yaml --format dot
llm-box visualize workflow.yaml --format ascii
llm-box visualize workflow.yaml --format json

# Output to file
llm-box visualize workflow.yaml -o diagram.md
```

**Supported formats:** Mermaid (interactive), DOT (Graphviz), ASCII (text-based), JSON (custom rendering)

> 📖 [Full Visualizer Documentation →](docs/visualizer.md)

---

## 🔗 MCP Integration

Connect llm-box to external AI applications via the Model Context Protocol.

**Start MCP server:**
```bash
# Stdin/stdout mode (default)
llm-box mcp

# HTTP mode
llm-box mcp --port 8082
```

**Available tools:** `workflow_run`, `workflow_create`, `workflow_list`, `secrets_list`, `secrets_add`

> 📖 [Full MCP Documentation →](docs/mcp.md)

---

## 🔌 Plugin System

llm-box supports a plugin system for extending functionality with community-contributed plugins. Plugins can add new workflow nodes or extend core behavior.

### Plugin Types

| Type | Description | Example |
|------|-------------|---------|
| **Node Plugin** | Adds new nodes to the workflow engine | Custom LLM provider, database connector |
| **Extension Plugin** | Extends core functionality | Custom authentication, logging |

### Plugin Management

```bash
# List installed plugins
llm-box plugins list

# List by type
llm-box plugins list --type node
llm-box plugins list --type extension

# Enable/disable plugins
llm-box plugins enable my-plugin
llm-box plugins disable my-plugin

# Check plugin status
llm-box plugins info my-plugin
```

### Directory Layout

Plugins are stored in `~/.llm-box/plugins/`:

```
~/.llm-box/plugins/
├── my-plugin/
│   ├── plugin.yaml      # Plugin metadata
│   ├── main.go          # Implementation
│   └── nodes/
│       └── custom_node.py
└── another-plugin/
    ├── plugin.yaml
    └── extension.js
```

### plugin.yaml Format

```yaml
name: "my-plugin"
version: "1.0.0"
description: "A custom plugin for llm-box"
author: "John Doe"
type: "node"          # or "extension"
dependencies:
  - "base-plugin"
```

### Node Plugin Example (Go)

```go
package main

import "github.com/alib8b8/llm-box/internal/plugins"

type MyNodePlugin struct {
    info plugins.PluginInfo
}

func NewMyNodePlugin() *MyNodePlugin {
    return &MyNodePlugin{
        info: plugins.PluginInfo{
            Name:         "my-node-plugin",
            Version:      "1.0.0",
            Description:  "Custom node plugin",
            Author:       "John Doe",
            Type:         plugins.PluginTypeNode,
            Dependencies: []string{},
        },
    }
}

func (p *MyNodePlugin) GetInfo() plugins.PluginInfo { return p.info }
func (p *MyNodePlugin) Init() error                { return nil }
func (p *MyNodePlugin) Shutdown() error            { return nil }
func (p *MyNodePlugin) GetNodes() []interface{}   { return []interface{}{"my_custom_node"} }
```

### Registering the Plugin

```go
pm := plugins.NewPluginManager()
pm.Register(NewMyNodePlugin())
pm.Enable("my-node-plugin")
```

### Plugin Lifecycle

1. **Register** — Plugin is added to the plugin manager
2. **Enable** — Plugin is initialized and its nodes/resources become available
3. **Execute** — Plugin nodes are used in workflows
4. **Disable** — Plugin is shut down and removed from available resources
5. **Unregister** — Plugin is completely removed

### Publishing Plugins

1. Create a GitHub repository for your plugin
2. Add a `plugin.yaml` file
3. Implement the plugin interface
4. Add documentation
5. Tag releases with semantic versioning

> 📖 [Full Plugin Documentation →](docs/plugins.md)

---

## 🏢 Tenant Isolation

Run multi-tenant deployments with resource isolation.

**Tenant management:**
```bash
# Create tenant
llm-box tenant create --id acme --name "Acme Corp" --max-workflows 10

# List tenants
llm-box tenant list

# Check quota
llm-box tenant quota acme
```

**Resource isolation:** Workflows, execution history, and secrets are completely isolated per tenant.

> 📖 [Full Tenant Documentation →](docs/tenants.md)

---

## 🛠️ Custom Nodes

Build custom nodes in any programming language. Custom nodes extend the workflow engine with new functionality by communicating via stdin/stdout using a JSON protocol.

### Supported Languages

- Python
- Node.js
- Bash
- Go
- Any language that can read/write JSON

### Interface Specification

**Input via stdin (JSON):**

```json
{
  "input": "input text from previous step",
  "params": {
    "param1": "value1",
    "param2": "value2"
  }
}
```

**Output via stdout (JSON):**

```json
{
  "output": "result text to pass to next step"
}
```

**Environment variables available to the node:**

| Variable | Description |
|----------|-------------|
| `LLM_BOX_NODE_NAME` | Name of the node |
| `LLM_BOX_WORKFLOW_NAME` | Name of the current workflow |
| `LLM_BOX_STEP_INDEX` | Zero-based step index |
| `LLM_BOX_SECRETS_PASSWORD` | Secrets password (if set) |

**Sensitive data filtering:** Parameters matching `api_key`, `key`, `secret`, `password`, `token`, or `credential` are automatically filtered before being passed to external nodes.

### Directory Structure

Custom nodes are stored in `~/.llm-box/nodes/`:

```
~/.llm-box/nodes/
├── my_custom_node/
│   ├── node.json      # Node metadata
│   └── main.py        # Node implementation
└── another_node/
    ├── node.json
    └── main.js
```

### Node Metadata (node.json)

```json
{
  "name": "my_custom_node",
  "description": "A custom node that does X",
  "version": "1.0.0",
  "author": "John Doe",
  "entrypoint": "main.py",
  "input_type": "string",
  "output_type": "string",
  "params": {
    "prefix": {
      "type": "string",
      "description": "Prefix to add to input",
      "required": false,
      "default": "Result: "
    }
  }
}
```

### Metadata Fields

| Field | Required | Description |
|-------|----------|-------------|
| `name` | Yes | Unique node name used in workflow YAML |
| `description` | Yes | Human-readable description |
| `version` | Yes | Semantic version |
| `author` | No | Author name |
| `entrypoint` | Yes | Script to execute |
| `input_type` | Yes | Input type: `string`, `json`, or `binary` |
| `output_type` | Yes | Output type: `string`, `json`, or `binary` |
| `params` | No | Parameter schema |

### Step-by-Step Example

**1. Create the node directory:**
```bash
mkdir -p ~/.llm-box/nodes/echo_prefix
```

**2. Create metadata:**
```json
// ~/.llm-box/nodes/echo_prefix/node.json
{
  "name": "echo_prefix",
  "description": "Echo input with custom prefix",
  "version": "1.0.0",
  "entrypoint": "main.py",
  "input_type": "string",
  "output_type": "string",
  "params": {
    "prefix": {
      "type": "string",
      "description": "Prefix to add",
      "required": false,
      "default": "Result: "
    }
  }
}
```

**3. Create implementation (Python):**
```python
#!/usr/bin/env python3
import sys
import json

payload = json.loads(sys.stdin.read())
input_data = payload.get("input", "")
params = payload.get("params", {})
prefix = params.get("prefix", "Result: ")

print(json.dumps({"output": prefix + input_data}))
```

**4. Make executable:**
```bash
chmod +x ~/.llm-box/nodes/echo_prefix/main.py
```

**5. Use in workflow:**
```yaml
name: test-custom-node
steps:
  - node: echo_prefix
    params:
      prefix: "Processed: "
    input: "Hello World"
```

### Multi-Language Examples

**Node.js:**
```javascript
#!/usr/bin/env node
const data = [];
process.stdin.on('data', chunk => data.push(chunk));
process.stdin.on('end', () => {
  const payload = JSON.parse(data.join(''));
  const result = `Processed: ${payload.input}`;
  console.log(JSON.stringify({ output: result }));
});
```

**Bash:**
```bash
#!/bin/bash
read -r payload
input=$(echo "$payload" | jq -r '.input // ""')
echo "{\"output\": \"Processed: $input\"}"
```

> 📖 [Full Custom Nodes Documentation →](docs/custom-nodes.md)

---

## 🏪 Marketplace

Browse and install ready-made workflow templates from the community:

| Template | Description | Install |
|----------|-------------|---------|
| **btc-alert** | Monitor Bitcoin price and alert on threshold | `llm-box install btc-alert` |
| **ai-news** | Daily AI/ML news summary from Hacker News | `llm-box install ai-news` |
| **github-report** | Weekly GitHub activity report (issues, PRs, commits) | `llm-box install github-report` |
| **telegram-bot** | Send notifications to Telegram | `llm-box install telegram-bot` |
| **seo-writer** | Generate SEO-optimized articles with audit | `llm-box install seo-writer` |

### Install a template

```bash
llm-box install btc-alert
llm-box run templates/btc-alert/workflow.yaml
```

### Submit your own

1. Create a directory under `templates/<your-template>/`
2. Include `workflow.yaml` and `README.md`
3. Open a PR — the community will review and merge

**See [templates/](templates/) for all available templates.**

---

## ⏰ Scheduled Workflows

Schedule workflows to run automatically using cron expressions.

**Quick start:**
```bash
# Run daily at 9 AM
llm-box schedule --cron "0 9 * * *" my-workflow.yaml

# List scheduled tasks
llm-box schedule --list

# Remove a task
llm-box schedule --remove daily-report
```

**Common schedules:**

| Schedule | Cron Expression |
|----------|----------------|
| Every hour | `0 * * * *` |
| Daily at 9 AM | `0 9 * * *` |
| Every weekday at 9 AM | `0 9 * * 1-5` |
| Every 15 minutes | `*/15 * * * *` |
| Monthly on the 1st | `0 0 1 * *` |

**YAML configuration:**
```yaml
name: daily-report
schedule:
  cron: "0 9 * * *"
  enabled: true
```

> 📖 [Full Scheduling Documentation →](docs/scheduling.md)

---

## 📦 External Nodes & Registry

Extend llm-box with community nodes from the registry.

**Manage nodes:**
```bash
# Sync registry first
llm-box registry sync

# Install a node
llm-box install weather-api

# Uninstall a node
llm-box uninstall weather-api

# List available nodes
llm-box registry list

# Search for nodes
llm-box registry search weather
```

**Available External Nodes (7):**

| Node | Category | Description |
|------|----------|-------------|
| `weather-api` | Data | Fetch weather info from Open-Meteo API |
| `rss-reader` | Data | Read and parse RSS/Atom feeds |
| `github-issues` | DevOps | Fetch and filter GitHub issues |
| `crypto-price` | Finance | Fetch crypto prices from CoinGecko |
| `ip-info` | Network | IP address geolocation lookup |
| `uuid-generator` | Utility | Generate UUIDs in various formats |
| `qr-code` | Utility | Generate QR codes from text or URLs |

> 💡 Total: **17 built-in + 7 external = 24 nodes**

---

## 🌍 Internationalization (i18n)

llm-box supports multi-language UI via environment variables:

```bash
# English (default)
export LLM_BOX_LANG=en

# Auto-detect from system LANG env var
unset LLM_BOX_LANG
```

**Supported languages:** English (`en`)

> 📝 More languages coming soon. Community translations welcome!

---

## 📊 Resource Limits

llm-box enforces default resource limits for safety and performance:

| Limit | Default Value | Notes |
|-------|--------------|-------|
| Max file size | 10 MB | Workflow YAML, state files |
| Max parallel steps | 50 | Per `parallel` block |
| Max retries per step | 10 | Exponential backoff |
| Max retry delay | 5 minutes | Backoff cap |
| Node download size | 1 MB | External node files |
| Registry size | 5 MB | Registry JSON |

All limits are enforced at runtime to prevent resource exhaustion.

---

## ❌ Error Codes & Troubleshooting

### Workflow Execution Errors

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| WF001 | `node '%s' not found in registry` | The specified node doesn't exist or isn't registered | Check node name spelling, run `llm-box list` to see available nodes, ensure the node is installed |
| WF002 | `step %d (%s) failed: %w` | A step execution failed | Check the step's input/parameters, verify credentials, review node-specific error messages |
| WF003 | `workflow timed out during retry delay` | Workflow exceeded timeout while waiting for retry | Increase `max_timeout`, reduce retry count, optimize step execution time |
| WF004 | `condition evaluation failed: %w` | A condition expression couldn't be evaluated | Check condition syntax, ensure referenced variables exist |
| WF005 | `expression evaluation failed: %w` | An expression like `{{step.0}}` couldn't be evaluated | Verify step indices/names exist, check variable references |
| WF006 | `too many parallel steps (%d, max %d)` | Exceeded maximum parallel steps limit (50) | Reduce the number of parallel steps |
| WF007 | `invalid workflow name: %s` | Workflow name contains invalid characters | Use only alphanumeric characters, hyphens, and underscores |

### Node-Specific Errors

#### HTTP/Network

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| ND001 | `failed to fetch URL: %w` | Network request failed | Check network connectivity, verify URL, check firewall rules |
| ND002 | `HTTP request failed: %d %s` | HTTP request returned non-200 status | Check API endpoint, verify authentication, check rate limits |
| ND003 | `connection timeout` | Connection to remote server timed out | Increase timeout parameter, check server availability |
| ND004 | `API key invalid or missing` | Authentication failed | Verify API key, check secrets config, ensure `LLM_BOX_SECRETS_PASSWORD` is set |

#### File Operations

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| ND005 | `permission denied` | Insufficient file permissions | Check file/directory permissions, run with appropriate privileges |
| ND006 | `file not found: %s` | Specified file doesn't exist | Verify file path, check spelling, ensure file exists |
| ND007 | `invalid mode: %s` | Invalid file write mode | Use only `write` or `append` for the mode parameter |
| ND008 | `file too large` | File exceeds size limit | Reduce file size, check `MaxFileSize` limit |

#### LLM/AI

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| ND009 | `model not found: %s` | Specified model doesn't exist | Verify model name, check provider availability |
| ND010 | `rate limit exceeded` | API rate limit reached | Wait and retry, implement caching, upgrade provider plan |
| ND011 | `insufficient quota` | API usage quota exhausted | Check provider billing, increase quota, reduce usage |
| ND012 | `model unavailable` | Model is temporarily unavailable | Try again later, use a different model |

#### Execute Node

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| ND013 | `command not allowed in safe mode` | Attempted to run execute node in safe mode | Disable safe mode with `--safe-mode=false` or use allowlist |
| ND014 | `command not in allowlist` | Command not in allowlist | Add command to allowlist or disable allowlist mode |
| ND015 | `command execution timed out` | Command exceeded timeout | Increase timeout parameter, optimize command |
| ND016 | `shell injection detected` | Command contains dangerous characters | Remove shell metacharacters (`;`, `|`, `&`, `` ` ``) |

### YAML/Parsing Errors

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| YML001 | `only .yaml and .yml workflow files are allowed` | Invalid file extension | Rename file to use `.yaml` or `.yml` extension |
| YML002 | `invalid workflow file path: %w` | File path is invalid | Verify path, check for special characters |
| YML003 | `YAML parse error: %w` | Invalid YAML syntax | Check YAML formatting, ensure proper indentation |
| YML004 | `invalid filename: %s` | Invalid characters in filename | Use only alphanumeric characters and underscores |
| YML005 | `missing required field: %s` | Required field is missing | Add the required field to the workflow YAML |

### Secrets Management Errors

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| SEC001 | `secrets password not set` | Master password not configured | Set `LLM_BOX_SECRETS_PASSWORD` environment variable |
| SEC002 | `invalid secret type: %s` | Unknown secret type | Use only `normal` or `secret` type |
| SEC003 | `file too short: invalid format` | Secrets file is corrupted or empty | Restore from backup, recreate secrets file |
| SEC004 | `failed to decrypt secrets: %w` | Incorrect master password | Verify `LLM_BOX_SECRETS_PASSWORD` is correct |
| SEC005 | `secret not found: %s/%s` | Requested secret doesn't exist | Add the secret using `llm-box secrets add` |

### Scheduler Errors

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| SCH001 | `invalid cron expression: %w` | Invalid cron syntax | Use 5-field cron format: `minute hour day month weekday` |
| SCH002 | `task with id %q already exists` | Duplicate task ID | Use a different task ID or remove the existing task |
| SCH003 | `task with id %q not found` | Task doesn't exist | Check task ID, list tasks with `llm-box schedule --list` |
| SCH004 | `invalid step value %q` | Invalid step value in cron expression | Step must be a positive integer |
| SCH005 | `value %d out of range [%d, %d]` | Cron field value is out of bounds | Use valid ranges (minute: 0-59, hour: 0-23, etc.) |

### Distributed Execution Errors

| Code | Error Message | Cause | Solution |
|------|---------------|-------|----------|
| DST001 | `invalid port: %s` | Invalid port number | Use numeric port between 1-65535 |
| DST002 | `invalid coordinator URL: %s` | Invalid coordinator address | Verify URL format, ensure coordinator is running |
| DST003 | `authentication failed` | Invalid auth token | Ensure auth tokens match between coordinator and workers |
| DST004 | `no available workers` | No workers registered or all at capacity | Start more workers, increase worker capacity |
| DST005 | `heartbeat timeout` | Worker didn't respond | Check worker health, verify network connectivity |

### Common Issues

**Workflow fails with "node not found"**
1. Verify the node name is spelled correctly
2. Check available nodes: `llm-box list`
3. Ensure the node is built-in or installed as a plugin

**API key authentication fails**
1. Set `LLM_BOX_SECRETS_PASSWORD` environment variable
2. Verify the secret exists: `llm-box secrets list <group>`
3. Check the reference syntax: `{{secret.group.key}}`

**Network timeout**
1. Verify network connectivity to the target server
2. Increase timeout parameter: `timeout: "30s"`
3. Try accessing the URL directly with `curl`

**YAML parsing error**
1. Check YAML indentation (use spaces, not tabs)
2. Ensure colons are followed by spaces
3. Validate YAML with an online validator

> 📖 [Full Troubleshooting Guide →](docs/troubleshooting.md)

---

## 📚 10 Real Use Cases

| # | Use Case | Command | Example |
|---|----------|---------|---------|
| 1 | GitHub Daily Digest | `llm-box create "fetch my recent GitHub activity and save summary"` | [github-daily-digest.yaml](examples/github-daily-digest.yaml) |
| 2 | Research Assistant | `llm-box create "fetch 3 tech blog posts and save key takeaways"` | [research-assistant.yaml](examples/research-assistant.yaml) |
| 3 | Documentation Generator | `llm-box create "scan my Go project and generate API overview"` | [docs-generator.yaml](examples/docs-generator.yaml) |
| 4 | Log Monitor | `llm-box create "monitor server logs for 5xx errors and alert"` | [log-monitor.yaml](examples/log-monitor.yaml) |
| 5 | Release Notes Creator | `llm-box create "turn git commit history into release notes"` | [release-notes.yaml](examples/release-notes.yaml) |
| 6 | Data Collector | `llm-box create "fetch weather and stock data, combine into report"` | [data-collector.yaml](examples/data-collector.yaml) |
| 7 | File Organizer | `llm-box create "organize downloads folder by file type"` | [file-organizer.yaml](examples/file-organizer.yaml) |
| 8 | Content Workflow | `llm-box create "take markdown post and generate HTML version"` | [content-processor.yaml](examples/content-processor.yaml) |
| 9 | DevOps Automation | `llm-box create "build docker image and deploy with health check"` | [devops-deploy.yaml](examples/devops-deploy.yaml) |
| 10 | Team Reporting | `llm-box create "compile weekly issue and commit stats"` | [team-report.yaml](examples/team-report.yaml) |

**See [examples/](examples/) for full workflow definitions.**

---

## ❓ FAQ

### What makes this different from Bash scripts?
llm-box adds structure, reusability, and a beautiful UI without losing the power of the terminal.

### Do I have to write YAML?
No for getting started — describe what you want in plain English, and llm-box generates the YAML for you. For advanced use, you can edit the generated YAML directly for fine-grained control.

### Can I extend it?
Yes! Build custom nodes in any language. See [docs/contributing.md](docs/contributing.md).

### Is it production-ready?
v0.3 is the current stable version with core features including workflow engine, 15+ LLM providers, distributed execution, Web UI, and MCP integration. v1.0 is planned for Q3 2026.

### Which platforms are supported?
Linux, macOS, and Windows are fully supported.

### Where can I get help?
Open a [GitHub Discussion](https://github.com/alib8b8/llm-box/discussions) or file an issue.

---

## 📝 Workflow Configuration

### Variables (`vars` field)

Define reusable variables at the workflow level using the `vars` field. Variables can be referenced anywhere in the workflow using the `{{var.name}}` expression syntax.

**Basic example:**
```yaml
name: Report Generator
vars:
  output_file: "report.md"
  api_endpoint: "https://api.example.com/data"
  max_retries: "3"

steps:
  - node: http_request
    params:
      url: "{{var.api_endpoint}}"
  - node: file_write
    params:
      path: "{{var.output_file}}"
```

**Use cases:**
- Configuration values shared across multiple steps
- Environment-specific settings
- Template parameters
- Default values that can be overridden via `call` node

**Variable scope and usage:**
- Workflow-level vars are available to all steps
- Vars passed via `call` node's `vars` parameter override existing vars with the same name
- Variables can be used in any `params` value, in `condition` expressions, and in step `input`
- Variable values are strings (use string type for all values)

**Example: Using variables in conditions**
```yaml
name: Conditional Workflow
vars:
  threshold: "error"
  alert_channel: "stdout"

steps:
  - node: http_request
    params:
      url: "https://api.example.com/health"
  - node: notify
    params:
      channel: "{{var.alert_channel}}"
    condition: "contains:{{var.threshold}}"
```

> 💡 For more details about expression syntax, see the [full expression reference](#secrets-management).

### Data Flow Between Steps

By default, each step receives the output of the previous step as its input (`{{input}}`). llm-box supports flexible data flow between steps:

| Reference | Description | Example |
|-----------|-------------|---------|
| `{{input}}` | Output of the immediately previous step | Implicit flow |
| `{{step.0}}` | Output of step by 0-based index | Any step |
| `{{step.name}}` | Output of step by its `id` | Named step |
| `{{var.name}}` | Workflow-level variable | Defined in `vars` section |
| `{{secret.GROUP.KEY}}` | Secret value from secure storage | API keys |

Use the `combine` node to merge outputs from multiple steps:
```yaml
- node: combine
  input:
    - "{{step.fetch_users}}"
    - "{{step.fetch_orders}}"
  params:
    separator: "\n---\n"
```

> 📖 [Full Data Flow Documentation →](docs/dataflow.md)

---

## 🔧 Built-in Utility Nodes

llm-box includes many utility nodes for common tasks:

### file_read
Reads content from a local file.

**Parameters:**
- `path` (required) - Path to the file to read

**Example:**
```yaml
- node: file_read
  params:
    path: "input.txt"
```

### file_write
Writes input content to a file.

**Parameters:**
- `path` (required) - Path to the output file

**Example:**
```yaml
- node: file_write
  params:
    path: "output.txt"
```

### fetch_url
Fetches content from a URL.

**Parameters:**
- `url` (required) - URL to fetch

**Example:**
```yaml
- node: fetch_url
  params:
    url: "https://example.com"
```

### execute
Executes a shell command.

**Parameters:**
- `command` (required) - Command to execute

**Example:**
```yaml
- node: execute
  params:
    command: "ls -la"
```

### transform
Deterministic text transformation using pure string/regex operations — no LLM calls. Supported operations: `upper`, `lower`, `trim`, `lines`, `words`, `chars`, `first_line`, `first_500`, `first_1000`, `summary`, `reverse`, `unique_lines`, `sort_lines`, `remove_blank_lines`, `filter_errors`, `extract_urls`, `extract_emails`, `markdown_to_html`, `html_to_markdown`, and domain-specific helpers like `extract_repos_and_activity`, `group_by_commit_type`, `group_by_extension`.

**Parameters:**
- `operation` - Operation to perform (upper, lower, trim, etc.)

**Example:**
```yaml
- node: transform
  params:
    operation: "upper"
```

### combine
Combines multiple inputs.

### notify
Sends a desktop notification.

### json_parse
Parses JSON and extracts specific fields using dot notation.

**Parameters:**
- `path` (optional) - JSON path to extract (e.g., `user.name`, `items.[0].title`). If omitted, returns formatted JSON.

**Example:**
```yaml
- node: json_parse
  params:
    path: "name"
```

### http_request
Makes HTTP requests to any API endpoint. More flexible than `fetch_url`.

**Parameters:**
- `url` (required) - URL to request
- `method` (optional) - HTTP method (GET, POST, PUT, DELETE, etc.). Default: GET
- `body` (optional) - Request body. Uses step input if not provided
- `content_type` (optional) - Content-Type header. Default: application/json for POST/PUT
- `headers` (optional) - Additional headers, one per line, format: `Key: Value`
- `timeout` (optional) - Request timeout (e.g., `30s`, `2m`). Default: 60s

**Example:**
```yaml
- node: http_request
  params:
    url: "https://api.example.com/data"
    method: "POST"
    content_type: "application/json"
    headers: |
      Authorization: Bearer token123
      X-Custom-Header: value
```

### template_render
Renders a Go template with input data and parameters.

**Parameters:**
- `template` or `template_file` (required) - Template string or path to template file
- Additional params are available as template variables

**Available template functions:** `upper`, `lower`, `title`, `trim`, `split`, `join`, `len`, `replace`

**Example:**
```yaml
- node: template_render
  params:
    template: |
      # Report
      Name: {{ .name }}
      Date: {{ .date }}
    name: "My Report"
    date: "2026-06-29"
```

### condition
Evaluates a condition against the input and returns "true" or "false". Used within steps for conditional logic.

**Parameters:**
- `expr` (required) - Condition expression (or `condition` as alias)

**Supported operators:**

| Operator | Description | Example |
|----------|-------------|---------|
| `contains:keyword` | Returns true if input contains the keyword | `contains:error` |
| `equals:value` | Returns true if input exactly matches | `equals:success` |
| `starts_with:prefix` | Returns true if input starts with prefix | `starts_with:https` |
| `ends_with:suffix` | Returns true if input ends with suffix | `ends_with:.json` |
| `regex:pattern` | Returns true if input matches regex | `regex:^\d+$` |
| `empty` | Returns true if input is empty | `empty` |
| `not_empty` | Returns true if input is not empty | `not_empty` |
| `true` / `false` | Literal boolean values | `true` |

**Negation:** Prefix any expression with `not ` to negate it.

**Example:**
```yaml
- node: condition
  params:
    expr: "contains:error"
```

**Example with negation:**
```yaml
- node: condition
  params:
    expr: "not contains:success"
```

**Example with regex:**
```yaml
- node: condition
  params:
    expr: "regex:^[A-Za-z0-9]+$"
```

**Example with variables:**
```yaml
- node: condition
  params:
    expr: "equals:{{var.expected_value}}"
```

### Condition Execution (Workflow-level)
In addition to the `condition` node, llm-box supports workflow-level conditional execution using the `if` field on steps, and branching with `if`/`else` blocks.

#### Step-level condition (`condition` field)
Skip a step based on a condition evaluated against the previous step's output:

```yaml
name: Conditional Workflow
steps:
  - node: http_request
    params:
      url: "https://api.example.com/health"
  - node: notify
    params:
      channel: stdout
    condition: "contains:error"
```

#### If/Else branching
Execute different branches based on conditions:

```yaml
name: Approval Workflow
steps:
  - node: evaluator
    params:
      criteria: "Is this content ready for publication?"
      threshold: "8"
  - if:
      condition: "contains:pass"
      then:
        - node: file_write
          params:
            path: "approved.txt"
        - node: notify
          params:
            channel: stdout
      else:
        - node: reflector
          params:
            model: gpt-4o
        - node: human_in_loop
          params:
            prompt: "Review and approve?"
```

**If/Else features:**
- Supports nested if/else (max depth: 20)
- Conditions use the same operators as the `condition` node
- Branch steps are executed as sub-workflows
- Output from the executed branch becomes the input for subsequent steps

### agent
Generic LLM agent node that supports multiple providers.

**Parameters:**
- `provider` - LLM provider (openai, ollama, coze, fastgpt, ima)
- `model` - Model name
- `prompt` - Prompt template
- `stream` (optional) - Enable streaming output. Default: true

**Example:**
```yaml
- node: agent
  params:
    provider: openai
    model: gpt-4o
    prompt: "Summarize this: {{input}}"
```

### openai
Calls OpenAI's API.

**Parameters:**
- `model` (required) - Model name (e.g., gpt-4o, gpt-4-turbo)
- `api_key` (optional) - API key (uses secret if not provided)
- `prompt` - Prompt template
- `temperature` (optional) - Temperature (0-2). Default: 0.7

**Example:**
```yaml
- node: openai
  params:
    model: gpt-4o
    api_key: "{{secret.llm.openai}}"
    prompt: "Analyze this data: {{input}}"
```

### ollama
Calls local Ollama models.

**Parameters:**
- `model` (required) - Model name (e.g., llama3, mistral)
- `prompt` - Prompt template
- `temperature` (optional) - Temperature (0-2). Default: 0.7

**Example:**
```yaml
- node: ollama
  params:
    model: llama3
    prompt: "Summarize this: {{input}}"
```

### coze
Calls Coze AI API.

**Parameters:**
- `model` (required) - Model name
- `api_key` (optional) - API key (uses secret if not provided)
- `prompt` - Prompt template

**Example:**
```yaml
- node: coze
  params:
    model: coze-chat
    api_key: "{{secret.llm.coze}}"
```

### fastgpt
Calls FastGPT API.

**Parameters:**
- `model` (required) - Model name
- `api_key` (optional) - API key (uses secret if not provided)
- `prompt` - Prompt template

**Example:**
```yaml
- node: fastgpt
  params:
    model: fastgpt-chat
    api_key: "{{secret.llm.fastgpt}}"
```

### ima
Calls IMA (Intelligent Multi-Agent) API.

**Parameters:**
- `model` (required) - Model name
- `api_key` (optional) - API key (uses secret if not provided)
- `prompt` - Prompt template

**Example:**
```yaml
- node: ima
  params:
    model: ima-7b
    api_key: "{{secret.llm.ima}}"
```

### call
Calls another workflow file, enabling workflow chaining and modular reuse.

**Parameters:**
- `workflow` (required) - Relative path to the workflow file to call (absolute paths are not allowed for security)
- `vars` (optional) - Variables to inject into the called workflow. Supports two formats:
  - JSON: `{"key":"value","key2":"value2"}`
  - Key=value pairs: `key=value,key2=value2`

**Security:**
- Path validation prevents arbitrary file read (only relative paths allowed)
- Maximum call depth of 10 to prevent infinite recursion
- Workflow file size limit: 10MB

**Example: Basic workflow chaining**
```yaml
name: Main Workflow
steps:
  - node: fetch_url
    params:
      url: "https://api.example.com/data"
  - node: call
    params:
      workflow: "process-data.yaml"
  - node: file_write
    params:
      path: "result.txt"
```

**Example: Passing variables to sub-workflow**
```yaml
name: Main Workflow
steps:
  - node: call
    params:
      workflow: "generate-report.yaml"
      vars: '{"topic":"AI Trends","format":"markdown"}'
```

**Example: Key=value format for variables**
```yaml
steps:
  - node: call
    params:
      workflow: "deploy.yaml"
      vars: "env=production,region=us-west"
```

**Sub-workflow example (`process-data.yaml`):**
```yaml
name: Process Data
vars:
  format: json
steps:
  - node: json_parse
    params:
      path: "items"
  - node: transform
    params:
      operation: "summary"
```

### planner
Generates a plan for complex tasks.

**Parameters:**
- `task` (required) - Task description
- `model` (optional) - LLM model to use

**Example:**
```yaml
- node: planner
  params:
    task: "Create a web scraper for example.com"
```

### code_review
Reviews code for issues and improvements.

**Parameters:**
- `model` (optional) - LLM model to use

**Example:**
```yaml
- node: code_review
  input: "{{step.code_output}}"
  params:
    model: gpt-4o
```

### critic
Provides critical feedback on content.

**Parameters:**
- `model` (optional) - LLM model to use
- `focus` (optional) - Focus area (quality, accuracy, clarity)

**Example:**
```yaml
- node: critic
  input: "{{step.report}}"
  params:
    model: gpt-4o
    focus: "accuracy"
```

### evaluator
Evaluates content against criteria.

**Parameters:**
- `criteria` (required) - Evaluation criteria
- `model` (optional) - LLM model to use

**Example:**
```yaml
- node: evaluator
  input: "{{step.response}}"
  params:
    criteria: "Is this response accurate and helpful?"
```

### router
Routes input to different nodes based on conditions.

**Parameters:**
- `routes` (required) - Array of route conditions and targets

**Example:**
```yaml
- node: router
  params:
    routes: |
      if contains(input, "code") then code_review
      if contains(input, "question") then agent
      else file_write
```

### supervisor
Monitors and manages workflow execution.

**Parameters:**
- `max_steps` (optional) - Maximum steps allowed
- `timeout` (optional) - Overall timeout

**Example:**
```yaml
- node: supervisor
  params:
    max_steps: 100
    timeout: "30m"
```

### researcher
Performs research and information gathering.

**Parameters:**
- `query` (required) - Research query
- `model` (optional) - LLM model to use

**Example:**
```yaml
- node: researcher
  params:
    query: "Latest developments in AI"
```

### human_in_loop
Pauses for human input/approval.

**Parameters:**
- `prompt` (required) - Prompt for human operator

**Example:**
```yaml
- node: human_in_loop
  params:
    prompt: "Review the generated code and approve?"
```

### reflector
Reflects on previous steps and provides insights.

**Parameters:**
- `model` (optional) - LLM model to use

**Example:**
```yaml
- node: reflector
  input: "{{step.execution_history}}"
```

---

## 🤖 Supported LLMs

llm-box supports multiple LLM providers out of the box:

### DeepSeek (Cloud API)

The `deepseek` node calls DeepSeek's official API. Perfect when you don't want to run models locally.

**Setup:**
```bash
export DEEPSEEK_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: DeepSeek Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: deepseek
    params:
      model: "deepseek-chat"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `deepseek-chat` - General purpose chat model
- `deepseek-coder` - Code generation model
- `deepseek-reasoner` - Reasoning model (R1)

### Coze (Cloud API)

The `coze` node calls ByteDance's Coze API (OpenAI compatible). Great for Chinese language tasks.

**Setup:**
```bash
export COZE_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: Coze Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: coze
    params:
      model: "glm-4"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `glm-4` - General purpose high-performance model
- `glm-4v` - Vision-capable model
- `glm-3-turbo` - Fast and cost-effective model

### Zhipu GLM (Cloud API)

The `glm` node calls Zhipu AI's GLM API (OpenAI compatible). Native Chinese language support.

**Setup:**
```bash
export GLM_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: GLM Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: glm
    params:
      model: "glm-4"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `glm-4` - Flagship model with strong reasoning
- `glm-4v` - Vision language model
- `glm-3-turbo` - Fast, cost-effective option
- `glm-4-plus` - High intelligence, longer context

### Kimi (Cloud API)

The `kimi` node calls Moonshot AI's Kimi API (OpenAI compatible). Known for long context windows.

**Setup:**
```bash
export KIMI_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: Kimi Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: kimi
    params:
      model: "moonshot-v1-8k"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `moonshot-v1-8k` - 8K context, standard
- `moonshot-v1-32k` - 32K context, long documents
- `moonshot-v1-128k` - 128K context, massive files

### MiniMax (Cloud API)

The `minimax` node calls MiniMax's API (OpenAI compatible). Strong Chinese language understanding.

**Setup:**
```bash
export MINIMAX_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: MiniMax Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: minimax
    params:
      model: "abab6.5s-chat"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `abab6.5s-chat` - Fast & balanced
- `abab6.5t-chat` - Text focused
- `abab7-chat` - Latest generation

### Qwen (Cloud API)

The `qwen` node calls Alibaba's Tongyi Qianwen API (OpenAI compatible). Strong ecosystem integration with Alibaba Cloud.

**Setup:**
```bash
export QWEN_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: Qwen Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: qwen
    params:
      model: "qwen-turbo"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Available models:**
- `qwen-turbo` - Fast & cost-effective
- `qwen-plus` - Balanced performance
- `qwen-max` - Maximum capability
- `qwen-long` - Long context (10M tokens)
- `qwen-vl-max` - Vision language model

### XVERSE (Cloud API)

The `xverse` node calls XVERSE's API (OpenAI compatible).

**Setup:**
```bash
export XVERSE_API_KEY="your-api-key"
```

**Available models:**
- `XVERSE-7B-Chat` - Lightweight fast model
- `XVERSE-13B-Chat` - Balanced performance
- `XVERSE-65B-Chat` - High capability

### Yi (Lingyiwanwu) (Cloud API)

The `yi` node calls Lingyiwanwu's Yi API (OpenAI compatible).

**Setup:**
```bash
export YI_API_KEY="your-api-key"
```

**Available models:**
- `yi-lightning` - Lightning fast
- `yi-large` - Large high-quality model
- `yi-medium` - Balanced
- `yi-vision` - Vision capability

### Baichuan (Cloud API)

The `baichuan` node calls Baichuan's API (OpenAI compatible).

**Setup:**
```bash
export BAICHUAN_API_KEY="your-api-key"
```

**Available models:**
- `Baichuan4` - Latest flagship model
- `Baichuan3-Turbo` - Fast & cost-effective
- `Baichuan2` - Previous generation

### InternLM (Open-Source) (Cloud API)

The `internlm` node calls Shanghai AI Lab's InternLM API (OpenAI compatible).

**Setup:**
```bash
export INTERNLM_API_KEY="your-api-key"
```

**Available models:**
- `internlm3-latest` - Latest generation
- `internlm2.5-latest` - v2.5 series
- `internlm2-latest` - v2 series
- `internlm-xcomposer` - Vision-language

### Mistral AI (Cloud API)

The `mistral` node calls Mistral AI's API (OpenAI compatible).

**Setup:**
```bash
export MISTRAL_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: Mistral Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: mistral
    params:
      model: "mistral-large-latest"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "mistral_summary.txt"
```

**Available models:**
- `mistral-large-latest` - Latest flagship model
- `mistral-medium-latest` - Balanced performance
- `mistral-small-latest` - Fast & cost-effective
- `open-mistral-nemo` - Open source model

### Xiaomi MiMo (Cloud API)

The `mimo` node calls Xiaomi MiMo's API (OpenAI compatible).

**Setup:**
```bash
export MIMO_API_KEY="your-api-key"
```

**Example workflow:**
```yaml
name: MiMo Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: mimo
    params:
      model: "mimo-v2.5-pro"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "mimo_summary.txt"
```

**Available models:**
- `mimo-v2.5-pro` - Latest flagship model
- `mimo-v2.5-plus` - Enhanced version
- `mimo-v2.5-lite` - Lightweight version

### IMA Copilot (Cloud API)

The `ima` node connects to IMA Copilot's OpenAI-compatible API endpoint.

**Setup:**
```bash
export IMA_API_KEY="your-api-key"
export IMA_API_BASE="https://your-ima-endpoint/v1"
```

**Example workflow:**
```yaml
name: IMA Copilot Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: ima
    params:
      model: "gpt-4o"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Supported models:**
- `gpt-4o` - High capability
- `gpt-4o-mini` - Fast & cost-effective
- `gpt-4.1` - Latest generation
- `gpt-5` - Most capable

### FastGPT (Knowledge Base Platform)

The `fastgpt` node connects to [FastGPT](https://github.com/labring/FastGPT) knowledge base applications. Perfect for querying enterprise knowledge bases, documentation, and custom datasets.

**Setup:**
```bash
export FASTGPT_API_KEY="your-api-key"
export FASTGPT_BASE_URL="https://your-fastgpt-domain.com/api/v1"
```

**Example workflow:**
```yaml
name: FastGPT Knowledge Query
steps:
  - node: fastgpt
    params:
      app_id: "your-app-id"
      api_key: "your-api-key"
      endpoint: "https://your-fastgpt-domain.com/api/v1"
  - node: file_write
    params:
      path: "answer.txt"
```

**Parameters:**
- `app_id` - FastGPT application ID (required)
- `api_key` - API key (or set `FASTGPT_API_KEY` env var)
- `endpoint` - FastGPT API base URL (or set `FASTGPT_BASE_URL` env var)
- `chat_id` - Conversation ID for context persistence (optional)
- `system` - System prompt (optional)

**💡 Use cases:**
- Query enterprise documentation from the terminal
- Batch process knowledge base queries
- Build automated QA pipelines
- Combine with `file_read` to import local docs into FastGPT via API

### Ollama (Local)

The `ollama` node runs models locally via Ollama. Great for privacy and offline use.

**Setup:**
```bash
# Install Ollama
curl -fsSL https://ollama.com/install.sh -o ollama-install.sh
sh ollama-install.sh

# Pull a model
ollama pull llama3
```

### OpenAI Compatible (Any Provider)

The `openai` node works with **any** API that follows the OpenAI format — SiliconFlow, Together AI, 腾讯混元, and hundreds more.

**Setup:**
```bash
export OPENAI_API_KEY="your-api-key"
export OPENAI_API_BASE="https://api.siliconflow.cn/v1"
```

**Example — SiliconFlow (30+ models):**
```yaml
name: SiliconFlow Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: openai
    params:
      model: "deepseek-ai/DeepSeek-V3"
      endpoint: "https://api.siliconflow.cn/v1"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Example — OpenRouter (200+ models):**
```yaml
name: OpenRouter Summary
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
  - node: openai
    params:
      model: "openai/gpt-4o"
      endpoint: "https://openrouter.ai/api/v1"
      system: "You are a helpful assistant that summarizes text concisely."
  - node: file_write
    params:
      path: "summary.txt"
```

**Setup for OpenRouter:**
```bash
export OPENAI_API_KEY="your-openrouter-api-key"
export OPENAI_API_BASE="https://openrouter.ai/api/v1"
```

**Works with:**
- [OpenRouter](https://openrouter.ai) - 200+ models from top providers
- SiliconFlow (硅基流动) - 30+ models, 0.5元/百万token起
- 腾讯混元 (Hunyuan)
- Together AI
- Anyscale
- Any OpenAI-compatible endpoint

---

## 📖 CLI Command Reference

### Core Commands

| Command | Description |
|---------|-------------|
| `llm-box create "<description>"` | Generate workflow YAML from natural language |
| `llm-box run <file>` | Execute a workflow file |
| `llm-box validate <file>` | Validate workflow YAML without running |
| `llm-box list` | List available nodes |
| `llm-box version` | Show version information |
| `llm-box help` | Show help message |

### Registry & Node Management

| Command | Description |
|---------|-------------|
| `llm-box install <node>` | Install an external node from registry |
| `llm-box uninstall <node>` | Uninstall an external node |
| `llm-box registry sync` | Sync node registry from GitHub |
| `llm-box registry list` | List available nodes in registry |
| `llm-box registry search <query>` | Search for nodes by name/description/tags |

### Workflow Visualization

| Command | Description |
|---------|-------------|
| `llm-box visualize <file>` | Generate workflow diagram (default: Mermaid) |
| `llm-box visualize <file> --format mermaid` | Generate Mermaid diagram |
| `llm-box visualize <file> --format dot` | Generate DOT format |
| `llm-box visualize <file> --format ascii` | Generate ASCII diagram |
| `llm-box visualize <file> --format json` | Generate JSON data |
| `llm-box visualize <file> -o <output>` | Output to file |

### Web UI

| Command | Description |
|---------|-------------|
| `llm-box webui` | Start Web UI on default port (8081) |
| `llm-box webui --port <port>` | Start Web UI on custom port |
| `llm-box webui --dir <dir>` | Load workflows from custom directory |

### MCP Server

| Command | Description |
|---------|-------------|
| `llm-box mcp` | Start MCP server (stdin/stdout mode) |
| `llm-box mcp --port <port>` | Start MCP server in HTTP mode |

### Scheduled Workflows

| Command | Description |
|---------|-------------|
| `llm-box schedule --cron "<expr>" <file>` | Schedule a workflow |
| `llm-box schedule --list` | List scheduled tasks |
| `llm-box schedule --info <id>` | Get task details |
| `llm-box schedule --remove <id>` | Remove a scheduled task |

### Tenant Management

| Command | Description |
|---------|-------------|
| `llm-box tenant create --id <id> --name <name>` | Create a new tenant |
| `llm-box tenant list` | List all tenants |
| `llm-box tenant info <id>` | Get tenant details |
| `llm-box tenant quota <id>` | Check tenant quota usage |
| `llm-box tenant delete <id>` | Delete a tenant |

### Secrets Management

| Command | Description |
|---------|-------------|
| `llm-box secrets add --group <g> --key <k> --value <v>` | Add a secret |
| `llm-box secrets list <group>` | List secrets in a group |
| `llm-box secrets remove --group <g> --key <k>` | Remove a secret |
| `llm-box secrets export <group> --output <file>` | Export secrets (encrypted) |
| `llm-box secrets import --input <file>` | Import secrets |

### Upgrade Commands

| Command | Description |
|---------|-------------|
| `llm-box self-update` | Check for and install updates |
| `llm-box autoupgrade status` | Show auto-upgrade status |
| `llm-box autoupgrade enable` | Enable automatic updates |
| `llm-box autoupgrade disable` | Disable automatic updates |
| `llm-box autoupgrade monitor` | Enable monitor mode (notify only) |
| `llm-box autoupgrade run` | Manually trigger upgrade check |
| `llm-box autoupgrade config <key>=<value>` | Configure auto-upgrade settings |

### Global Options

| Option | Description |
|--------|-------------|
| `--safe-mode` | Disable execute node and other dangerous features |
| `--lang <lang>` | Set language (en, zh) |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `LLM_BOX_SECRETS_PASSWORD` | Master password for secrets |
| `LLM_BOX_SAFE_MODE` | Enable safe mode |
| `LLM_BOX_EXECUTE_ALLOWLIST` | Enable execute allowlist mode |
| `LLM_BOX_LOG_FILE` | Path to audit log file |
| `LLM_BOX_LOG_LEVEL` | Log level (debug, info, warn, error) |
| `LLM_BOX_LANG` | UI language |

---

## 🗺️ Roadmap

| Version | Milestone | Status |
|---------|-----------|--------|
| **v0.1** | Workflow engine + core nodes | ✓ Done |
| **v0.2** | 15+ LLM providers + plugin system | ✓ Done |
| **v0.3** | Distributed execution + Web UI + MCP | ✓ Done |
| **v0.4** | **Workflow Marketplace** — `llm-box install <template>` | In Progress |
| **v0.5** | **100 Built-in Templates** — curated by category | Planned |
| **v0.6** | **Community Template Hub** — anyone can publish | Planned |
| **v0.7** | **Template Ranking** — stars, downloads, reviews | Planned |
| **v1.0** | **Workflow Store** — full template ecosystem | Planned |

---

## 🤝 Contributing

We welcome contributors of all skill levels!

### Ways to Contribute
- **Go Developers** - Build new nodes, improve core
- **Documentation** - Improve docs, write tutorials
- **Workflow Designers** - Share your workflows
- **Community Builders** - Help others on Discussions

### Quick Start

```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go mod download
go test ./...
go build -o llm-box ./cmd/llm-box
./llm-box help
```

See [docs/contributing.md](docs/contributing.md) for guidelines.

---

## 📄 License

MIT License - see [LICENSE](LICENSE) for full details.

---

<div align="center">
  <p>If this project helps you, please give it a ⭐</p>
  <p>
    <a href="https://github.com/alib8b8/llm-box/stargazers">
      <img src="https://api.star-history.com/svg?repos=alib8b8/llm-box&type=Timeline" alt="Star History" />
    </a>
  </p>
</div>
