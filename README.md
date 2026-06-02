# llm-box

<p align="center">
  <img src="docs/logo.svg" alt="llm-box" width="200"/>
</p>

<p align="center">
  <strong>Build terminal workflows using plain English.</strong>
</p>

<p align="center">
  <a href="https://github.com/alib8b8/llm-box/releases">
    <img src="https://img.shields.io/github/v/release/alib8b8/llm-box?style=flat-square" alt="release"/>
  </a>
  <a href="https://golang.org/">
    <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square" alt="go"/>
  </a>
  <a href="LICENSE">
    <img src="https://img.shields.io/github/license/alib8b8/llm-box?style=flat-square" alt="license"/>
  </a>
  <a href="https://github.com/alib8b8/llm-box/stargazers">
    <img src="https://img.shields.io/github/stars/alib8b8/llm-box?style=flat-square" alt="stars"/>
  </a>
</p>

---

No YAML. No drag-and-drop builders. No boilerplate.

Turn repetitive terminal tasks into reusable workflows directly from your terminal.

---

## Demo

![llm-box demo](docs/demo.gif)

> **30 seconds to see it in action:** Run `vhs docs/demo.tape` to generate the demo GIF locally.

---

## Why llm-box

Most workflow tools force developers to choose between:

| Approach | Problem |
|----------|---------|
| Complex shell scripts | Hard to read, maintain, or share |
| Heavy visual builders | Slow, opaque, require GUI |
| Endless configuration files | Steep learning curve, verbose syntax |

**llm-box provides a lightweight terminal-first approach.**

- Describe a workflow naturally
- Execute it instantly
- Keep everything transparent and scriptable

---

## Features

- **Terminal First** — Native command-line experience, no GUI required
- **Natural Language Workflow Creation** — Define workflows in plain English
- **Lightweight Runtime** — Single static binary, zero dependencies
- **Workflow Reusability** — Save, share, and version control your workflows
- **Fast Setup** — 60 seconds from zero to running workflow
- **Extensible Architecture** — Build custom nodes in any language
- **Open Source** — MIT licensed, community-driven

---

## Quick Start

### Installation (60 seconds)

**Linux / macOS:**
```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh | bash
```

**Windows:**
```powershell
# Download from releases page
# https://github.com/alib8b8/llm-box/releases/latest
Invoke-WebRequest -Uri "https://github.com/alib8b8/llm-box/releases/latest/download/llm-box-windows-amd64.exe" -OutFile llm-box.exe
```

**Build from source:**
```bash
git clone https://github.com/alib8b8/llm-box.git
cd llm-box
go install ./cmd/llm-box
```

### Create Your First Workflow

```bash
llm-box create "Fetch the top 5 Hacker News stories and save to file"
```

### Run It

```bash
llm-box run my_workflow.yaml
```

### See Results

```
✅ Fetched HN stories
✅ Saved to hn_stories.txt

Workflow completed in 3.2s
```

---

## Examples

Here are 10 practical workflows you can build with llm-box:

### 1. Daily GitHub Summary

**Goal:** Get an overview of your GitHub activity

**Input:** GitHub username

**Workflow:**
```yaml
name: "GitHub Daily Summary"
steps:
  - node: fetch_url
    params:
      url: "https://github.com/{username}"
  - node: transform
    params:
      operation: "extract_repos"
  - node: file_write
    params:
      path: "daily_summary.txt"
```

**Output:** List of recent repositories with star counts

---

### 2. Research Assistant

**Goal:** Collect and summarize research materials

**Input:** List of URLs

**Workflow:**
```yaml
name: "Research Summary"
steps:
  - node: fetch_url
    params:
      url: "{{input}}"
  - node: transform
    params:
      operation: "extract_key_points"
  - node: file_write
    params:
      path: "research_notes.md"
```

**Output:** Structured markdown notes from web sources

---

### 3. Documentation Generator

**Goal:** Auto-generate README from code structure

**Input:** Repository path

**Workflow:**
```yaml
name: "Docs Generator"
steps:
  - node: execute
    params:
      command: "find . -name '*.go' | head -20"
  - node: transform
    params:
      operation: "extract_functions"
  - node: file_write
    params:
      path: "API.md"
```

**Output:** Markdown documentation of code structure

---

### 4. Log Monitoring

**Goal:** Real-time monitoring with alerts

**Input:** Log file path and pattern

**Workflow:**
```yaml
name: "Log Monitor"
steps:
  - node: execute
    params:
      command: "tail -f {{input}}"
  - node: transform
    params:
      operation: "filter_errors"
  - node: notify
    params:
      channel: "stdout"
```

**Output:** Filtered log stream with error highlights

---

### 5. Release Notes Creation

**Goal:** Generate changelog from git commits

**Input:** Git repository path

**Workflow:**
```yaml
name: "Release Notes"
steps:
  - node: execute
    params:
      command: "git log --oneline -20"
  - node: transform
    params:
      operation: "group_by_type"
  - node: file_write
    params:
      path: "RELEASE_NOTES.md"
```

**Output:** Structured release notes grouped by commit type

---

### 6. Data Collection

**Goal:** Aggregate data from multiple sources

**Input:** List of API endpoints

**Workflow:**
```yaml
name: "Data Aggregator"
steps:
  - node: fetch_url
    params:
      url: "{{item}}"
  - node: transform
    params:
      operation: "extract_json"
  - node: combine
    params:
      format: "csv"
  - node: file_write
    params:
      path: "data.csv"
```

**Output:** Combined CSV file from multiple sources

---

### 7. File Organization

**Goal:** Auto-organize downloads folder

**Input:** Downloads directory path

**Workflow:**
```yaml
name: "File Organizer"
steps:
  - node: execute
    params:
      command: "ls -la {{input}}"
  - node: transform
    params:
      operation: "group_by_extension"
  - node: execute
    params:
      command: "mkdir -p images documents archives && mv *.jpg *.png images/ 2>/dev/null; true"
```

**Output:** Organized folder structure

---

### 8. Content Workflow

**Goal:** Process markdown files for publishing

**Input:** Markdown file path

**Workflow:**
```yaml
name: "Content Processor"
steps:
  - node: fetch_url
    params:
      url: "file://{{input}}"
  - node: transform
    params:
      operation: "add_frontmatter"
  - node: transform
    params:
      operation: "optimize_images"
  - node: file_write
    params:
      path: "_site/{{basename}}.html"
```

**Output:** HTML file ready for publishing

---

### 9. DevOps Automation

**Goal:** Deploy with zero downtime

**Input:** Service name and environment

**Workflow:**
```yaml
name: "Zero Downtime Deploy"
steps:
  - node: execute
    params:
      command: "docker build -t {{service}} ."
  - node: execute
    params:
      command: "docker-compose up -d --no-deps {{service}}"
  - node: execute
    params:
      command: "sleep 5 && curl -f http://localhost/health"
  - node: notify
    params:
      channel: "slack"
```

**Output:** Deployed service with health verification

---

### 10. Team Reporting

**Goal:** Generate weekly team metrics

**Input:** Date range and team members

**Workflow:**
```yaml
name: "Team Report"
steps:
  - node: execute
    params:
      command: "gh issue list --assignee @me --since '{{start}}' --state all"
  - node: transform
    params:
      operation: "count_by_label"
  - node: execute
    params:
      command: "git log --author '{{author}}' --since '{{start}}' --oneline"
  - node: file_write
    params:
      path: "weekly_report.md"
```

**Output:** Markdown report with issues and commits

---

## Why Not Alternatives

| Tool | Learning Curve | Configuration | Visual Builder | Terminal Native |
|------|----------------|---------------|----------------|-----------------|
| Bash | Medium | Scripts | No | Yes |
| Makefile | High | Makefiles | No | Yes |
| Zapier | Low | GUI | Yes | No |
| n8n | Medium | GUI + YAML | Yes | No |
| Airflow | High | Python | Yes | No |
| **llm-box** | **Low** | **Plain Text** | **No** | **Yes** |

**llm-box wins on:** Learning curve, configuration simplicity, and terminal-native experience.

---

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                         User                                │
│                    (Terminal Input)                         │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   Natural Language Parser                   │
│            "Fetch HN stories and summarize"                │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Task Planner                            │
│           Converts intent to executable steps               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Execution Engine                         │
│  ┌─────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  │
│  │ fetch   │  │ transform │  │ execute   │  │ file     │  │
│  │  _url   │  │          │  │           │  │ _write   │  │
│  └─────────┘  └──────────┘  └───────────┘  └──────────┘  │
│                                                              │
│  ┌─────────┐  ┌──────────┐  ┌───────────┐  ┌──────────┐  │
│  │ ollama  │  │  notify  │  │ combine   │  │  custom  │  │
│  │         │  │          │  │           │  │  nodes   │  │
│  └─────────┘  └──────────┘  └───────────┘  └──────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                       Output                               │
│              (Terminal / File / Notification)               │
└─────────────────────────────────────────────────────────────┘
```

**Components:**

1. **Natural Language Parser** — Interprets plain English commands
2. **Task Planner** — Breaks down workflows into executable steps
3. **Execution Engine** — Runs nodes sequentially with dependency management
4. **Node System** — Extensible set of built-in and custom nodes
5. **Output Handler** — Formats and delivers results

---

## Built-in Nodes

### fetch_url
Fetch content from web pages.
```yaml
- node: fetch_url
  params:
    url: "https://example.com"
```

### ollama
Call local LLM models for processing.
```yaml
- node: ollama
  params:
    model: "llama3"
    prompt: "Summarize: {{input}}"
```

### file_write
Save output to files.
```yaml
- node: file_write
  params:
    path: "output.txt"
```

### execute
Run shell commands.
```yaml
- node: execute
  params:
    command: "git status"
```

### notify
Send notifications.
```yaml
- node: notify
  params:
    channel: "slack"
    message: "Deployment complete!"
```

---

## Roadmap

### v0.1 — Initial Release ✓
- [x] Basic workflow creation
- [x] Workflow execution engine
- [x] Built-in nodes (fetch_url, file_write, ollama)
- [x] Terminal UI

### v0.2 — Community Features
- [ ] Plugin system for custom nodes
- [ ] Workflow templates marketplace
- [ ] Workflow sharing via URL

### v0.3 — Collaboration
- [ ] Team workflow library
- [ ] Workflow versioning
- [ ] Cloud sync

### v0.4 — Enterprise
- [ ] Team collaboration features
- [ ] Access control
- [ ] Audit logging

### v1.0 — Stable Release
- [ ] Production-ready
- [ ] Comprehensive documentation
- [ ] Long-term support

---

## Contributing

We welcome contributions from developers of all skill levels!

### Ways to Contribute

- **Go Developers** — Build new nodes, improve the core engine
- **Documentation Contributors** — Improve docs, write tutorials
- **Workflow Designers** — Share your workflows, create templates
- **Community Builders** — Help others, report bugs, suggest features

### Quick Start

1. Fork the repository
2. Create your branch: `git checkout -b feature/amazing-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Commit: `git commit -m 'feat: add amazing feature'`
6. Push: `git push origin feature/amazing-feature`
7. Open a Pull Request

### Development Setup

```bash
# Clone the repository
git clone https://github.com/alib8b8/llm-box.git
cd llm-box

# Install dependencies
go mod download

# Run tests
go test ./...

# Build locally
go build -o llm-box ./cmd/llm-box
```

---

## Project Structure

```
llm-box/
├── cmd/
│   └── llm-box/
│       └── main.go           # Entry point
├── internal/
│   ├── workflow/             # Workflow parsing & execution
│   │   ├── parser.go
│   │   ├── executor.go
│   │   └── types.go
│   ├── nodes/                # Built-in nodes
│   │   ├── fetch_url.go
│   │   ├── file_write.go
│   │   ├── ollama.go
│   │   └── node.go
│   └── tui/                  # Terminal UI
│       └── model.go
├── nodes/                    # Community nodes
├── examples/                 # Example workflows
├── docs/                     # Documentation
├── CONTRIBUTING.md
├── LICENSE
└── README.md
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=alib8b8/llm-box&type=Timeline)](https://star-history.com/#alib8b8/llm-box&Timeline)

---

<p align="center">
  <strong>If this project helps you, please give it a ⭐</strong>
</p>
