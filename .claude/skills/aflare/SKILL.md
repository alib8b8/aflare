---
name: aflare
description: Core skill for creating, validating, and running aflare workflows — YAML-defined automation pipelines that chain nodes for fetching, transforming, LLM inference, file I/O, and more
allowed-tools:
  - bash
  - file_read
  - file_write
  - edit
version: "1.0.0"
author: alib8b8
license: AGPL-3.0
compatibility: ">=0.1.0"
tags:
  - workflow
  - automation
  - llm
  - aflare
  - ai
  - pipeline
---

# aflare Workflow Skill

## Overview

aflare is a workflow engine that chains together **nodes** (steps) to automate tasks. Each workflow is a YAML file defining a sequence of nodes. Supported node types include `fetch_url`, `execute`, `transform`, `ollama` (local LLM), `file_read`, `file_write`, `notify`, `call`, and custom community nodes. Workflows support error handling with retries, parallel execution, conditional branching, variable substitution, and workflow chaining.

## Prerequisites

- **aflare binary** installed and in your PATH
- **Ollama** running locally if your workflows use `ollama` nodes
- **Required models** installed (e.g., `ollama pull llama3`)
- **API access** to any external services your workflow nodes call
- **Secrets configured** if workflows reference `{{secret.*}}` expressions

## Instructions

### 1. Create a Workflow YAML

Every workflow needs a `name`, `description`, and `steps` array. Each step declares a `node` type and `params`:

```yaml
name: my-workflow
description: What this workflow does
steps:
  - node: fetch_url
    params:
      url: "https://example.com"
```

### 2. Validate the Workflow

```bash
aflare validate my-workflow.yaml
```

### 3. Run the Workflow

```bash
aflare run my-workflow.yaml
aflare run my-workflow.yaml -var key=value
aflare run my-workflow.yaml -input "text to process"
```

### 4. Debug Issues

- Review `~/.aflare/logs/audit.log` for execution history
- Use `aflare run --verbose` for detailed step output
- Inspect individual node output for error messages

### 5. Advanced Patterns

- **Parallel steps:** Use `parallel` to run nodes concurrently
- **Retries:** Add `retry` and `delay` for resilience
- **Conditional branching:** Use `condition` to branch on prior output
- **Workflow chaining:** Use `call` nodes to invoke sub-workflows
- **Map over arrays:** Use `map.over` with `concurrency` for batch processing

## Output

Running a workflow produces:
- **Files** written by `file_write` nodes
- **Console output** from `notify` or `execute` nodes
- **Audit log entries** in `~/.aflare/logs/audit.log`
- **Return code** 0 on success, non-zero on failure

## Examples

### Fetch and Summarize

```yaml
name: article-summarizer
description: Fetch an article and summarize it with Ollama
steps:
  - node: fetch_url
    params:
      url: "https://example.com/article"
      mode: "text"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Summarize the key points: {{.steps[0].output}}"
      temperature: 0.3
  - node: file_write
    params:
      path: "summary.md"
```

### Run with Variables

```bash
aflare run workflow.yaml -var url="https://news.example.com"
```

## Resources

- [Workflow Examples](.trae/skills/aflare-workflow/examples.md)
- [Node Reference](docs/nodes-reference.md)
- [Custom Nodes Guide](docs/custom-nodes.md)
- [Batch URL Processor Example](examples/real-world/batch-url-processor/README.md)
- [GitHub Repository](https://github.com/alib8b8/aflare)
- [OpenClaw Plugin](contrib/openclaw/)
