# llm-box Architecture

This document explains how llm-box is structured and how it works under the hood.

---

## High-Level Architecture

```
┌───────────────────────────────────────────────────────────────────────┐
│                         User (Terminal)                                │
│  llm-box create "fetch HN and save"                                   │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│                    Natural Language Parser                        │
│  Converts plain English description → → → → → → → → → → → → → → →    │
│  to structured workflow YAML                                          │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│                      Task Planner                                  │
│  Validates workflow, checks dependencies,                              │
│  creates execution plan                                           │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│                    Execution Engine                              │
│  ┌─────────────────────────────────────────────────────────┐      │
│  │  Node Registry                                     │      │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐   │      │
│  │  │ fetch    │  │ transform│  │ execute  │   │      │
│  │  └──────────┘  └──────────┘  └──────────┘   │      │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐   │      │
│  │  │ file     │  │ notify    │  │ custom   │   │      │
│  │  └──────────┘  └──────────┘  └──────────┘   │      │
│  └─────────────────────────────────────────────────────────┘      │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│                   TUI Renderer                                      │
│  Shows beautiful real-time progress to user                        │
└───────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌───────────────────────────────────────────────────────────────────────┐
│                     Output Handler                                │
│  Saves to file, displays terminal, sends notifications            │
└───────────────────────────────────────────────────────────────────────┘
```

---

## Key Components

### 1. CLI Layer

Located in `cmd/llm-box/main.go`

Responsibilities:
- Parses command-line arguments
- Routes commands (create, run, help)
- Initializes dependencies

### 2. Parser

Located in `internal/parser/`

Responsibilities:
- Converts plain English descriptions to workflow YAML
- Validates workflow structure

### 3. Planner

Located in `internal/planner/`

Responsibilities:
- Validates workflow YAML
- Checks that all nodes exist
- Creates execution plan
- Handles dependencies

### 4. Engine

Located in `internal/engine/`

Responsibilities:
- Executes workflow steps
- Manages state
- Reports progress
- Handles errors

### 5. Node System

Located in `internal/nodes/`

Built-in nodes:
- `fetch_url`: Fetch content from URLs
- `transform`: Transform data
- `execute`: Run shell commands
- `file_write`: Save to files
- `notify`: Send alerts
- `ollama`: Integrate with local Ollama

Custom nodes:
- Can be written in any language
- Follow simple stdin/stdout/stderr protocol

### 6. TUI

Located in `internal/tui/`

Responsibilities:
- Beautiful terminal UI
- Real-time progress updates
- Status indicators (✅ done, ⏳ running, ❌ failed)

---

## Project Structure

```
llm-box/
├── cmd/
│   └── llm-box/
│       └── main.go          # CLI entry point
├── internal/
│   ├── workflow/
│   │   ├── parser.go   # Parser
│   │   ├── executor.go # Engine
│   │   └── types.go    # Data structures
│   ├── nodes/            # Nodes implementations
│   └── tui/             # Terminal UI
├── nodes/                 # Community nodes (user-contributed
├── examples/              # Example workflows
├── docs/                  # Documentation
├── README.md
├── CONTRIBUTING.md
├── LICENSE
└── go.mod
```

---

## Data Flow

1. User runs `llm-box create "..."`
2. Parser converts description → workflow YAML
3. User runs `llm-box run workflow.yaml`
4. Planner loads and validates workflow
5. Engine executes steps sequentially
6. TUI shows progress in real time
7. Engine writes output

---

## Extending llm-box

See [Contributing](contributing.md) for how to add custom nodes.
