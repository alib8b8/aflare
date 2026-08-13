# OpenClaw aflare Plugin

<p align="center">
  <img src="https://raw.githubusercontent.com/alib8b8/aflare/main/docs/logo.svg" width="200" alt="aflare logo"/>
</p>

<p align="center">
  <strong>Invoke aflare workflows as tools in OpenClaw conversations</strong>
</p>

<p align="center">
  <a href="https://github.com/alib8b8/aflare">
    <img src="https://img.shields.io/badge/aflare-v1.0.0-blue" alt="aflare"/>
  </a>
  <a href="https://github.com/alib8b8/openclaw-llmbox">
    <img src="https://img.shields.io/badge/OpenClaw-Plugin-green" alt="OpenClaw Plugin"/>
  </a>
  <a href="https://github.com/alib8b8/openclaw-llmbox/blob/main/LICENSE">
    <img src="https://img.shields.io/badge/License-AGPL%20v3.0-yellow" alt="License"/>
  </a>
</p>

---

## Overview

This plugin integrates **aflare** with **OpenClaw**, allowing you to invoke aflare workflows as callable tools within OpenClaw conversations. Your AI assistant can now execute complex, multi-step workflows built with aflare directly from chat.

## Features

- **Workflow Discovery** - List all available aflare workflows in your configured directory
- **Workflow Execution** - Execute any workflow by name with optional input parameters
- **Workflow Inspection** - Get detailed descriptions of what each workflow does
- **Multi-Provider Support** - Leverage 20+ LLM providers through aflare (Ollama, Kimi, DeepSeek, Qwen, etc.)

## How It Works

```
┌─────────────────────────────────────────────────────────────┐
│                        OpenClaw Agent                         │
│                                                               │
│  You: "Summarize this article using my kimi_summary workflow"│
│                                                               │
│  Agent → aflare_list_workflows (tool)                        │
│         ↓                                                    │
│  Agent → aflare_run_workflow(workflow_file="kimi_summary")   │
│         ↓                                                    │
│  aflare executes:                                            │
│    [STEP 1] fetch_url → Extract article content              │
│    [STEP 2] kimi → Generate summary                          │
│         ↓                                                    │
│  Agent: "Here's the summary: ..."                            │
└─────────────────────────────────────────────────────────────┘
```

## Prerequisites

- **OpenClaw** installed and configured
- **aflare** installed (`go install github.com/alib8b8/aflare@latest`)
- Some aflare workflows in your workflows directory

## Installation

```bash
# Install the plugin
openclaw plugin install alib8b8/openclaw-llmbox

# Or clone and install locally
git clone https://github.com/alib8b8/openclaw-llmbox.git
cd openclaw-llmbox
npm install
npm run build
openclaw plugin install ./dist
```

## Configuration

Edit your OpenClaw config file (`~/.openclaw/config.json`):

```json
{
  "plugins": {
    "openclaw-llmbox": {
      "workflowDir": "./workflows",
      "llmboxPath": "aflare",
      "enableAutoDiscovery": true
    }
  }
}
```

| Config Option | Default | Description |
|--------------|---------|-------------|
| `workflowDir` | `./workflows` | Directory containing your aflare workflow YAML files |
| `llmboxPath` | `aflare` | Path to aflare binary (must be in PATH or use full path) |
| `enableAutoDiscovery` | `true` | Auto-discover workflows in the configured directory |

> **Note:** `llmboxPath` is a historical naming convention inherited from the project's earlier name (llmbox). It points to the `aflare` binary — the two names refer to the same tool.
>
> **v0.8 migration:** The `openclaw-llmbox` plugin id, package name, and `llmboxPath` config key will be renamed to `openclaw-aflare` / `aflarePath` in v0.8. See [#59](https://github.com/alib8b8/aflare/issues/59).

## Available Tools

### `aflare_list_workflows`

List all available aflare workflows in your configured directory.

**Parameters:** None

**Example:**
```
User: What workflows do I have available?

Agent calls aflare_list_workflows:
→ {"workflows": [
    {"name": "kimi_summary", "file": "kimi_summary.yaml", "steps": 2},
    {"name": "deepseek_coder", "file": "deepseek_coder.yaml", "steps": 3},
    {"name": "multi_step", "file": "multi_step.yaml", "steps": 5}
  ], "count": 3}
```

### `aflare_run_workflow`

Execute a specific aflare workflow.

**Parameters:**
- `workflow_file` (required): The workflow filename (e.g., `"kimi_summary.yaml"`)
- `input` (optional): Input text to pass to the workflow as `{{input}}`

**Example:**
```
User: Summarize https://example.com/article using kimi_summary

Agent calls aflare_run_workflow:
→ {"workflow_file": "kimi_summary.yaml", "input": "https://example.com/article", ...}
```

### `aflare_describe_workflow`

Get detailed information about a specific workflow.

**Parameters:**
- `workflow_file` (required): The workflow filename

**Example:**
```
User: What does the deepseek_coder workflow do?

Agent calls aflare_describe_workflow:
→ {"workflow": "deepseek_coder.yaml", "name": "deepseek_coder", 
   "description": "Code review and fix workflow using DeepSeek Coder",
   "steps": [
     {"step": 1, "node": "fetch_url", "params": {"url": "{{input}}"}},
     {"step": 2, "node": "deepseek", "params": {"model": "deepseek-coder", "prompt": "Review this code..."}}
   ]}
```

## Creating Workflows for OpenClaw

Here are some workflow patterns that work great with OpenClaw:

### 1. Simple URL Processing

```yaml
# workflows/url_summarize.yaml
name: URL Summarizer
description: Fetch a URL and summarize its content
steps:
  - node: fetch_url
    params:
      url: "{{input}}"
  - node: deepseek
    params:
      model: deepseek-chat
      prompt: "Summarize the following content in 3 bullet points:\n\n{{output}}"
```

### 2. Multi-Provider Pipeline

```yaml
# workflows/research_assistant.yaml
name: Research Assistant
description: Research a topic using multiple sources
steps:
  - node: fetch_url
    params:
      url: "{{input}}"
  - node: ollama
    params:
      model: llama3
      prompt: "Extract key information from: {{output}}"
  - node: kimi
    params:
      model: moonshot-v1-128k
      prompt: "Format the following research notes:\n\n{{output}}"
```

### 3. Code Review

```yaml
# workflows/code_review.yaml
name: Code Review
description: Review code for bugs and improvements
steps:
  - node: file_read
    params:
      path: "{{input}}"
  - node: deepseek
    params:
      model: deepseek-coder
      prompt: "Review this code for:\n1. Bugs\n2. Security issues\n3. Performance improvements\n\nCode:\n{{output}}"
```

## Example Usage

### Chat Mode

```bash
$ openclaw chat

🤖: How can I help you today?

You: I want to summarize this article: https://example.com/tech-news

🤖: Let me use the kimi_summary workflow to summarize that article.

[Calling aflare_run_workflow]
→ Workflow: kimi_summary.yaml
→ Steps executed: 2
→ Duration: 3.2s

🤖: Here's a summary of the article:

• **Main Topic**: The article discusses the latest developments in AI technology
• **Key Points**: 
  - New LLM models achieving state-of-the-art results
  - Increased focus on efficiency and cost reduction
  - More open-source releases
• **Conclusion**: The AI field continues to evolve rapidly...
```

### Command Mode

```bash
$ openclaw "Summarize https://example.com/article using my kimi_summary workflow"
```

## Requirements

- Node.js >= 22.0.0
- OpenClaw
- aflare

## License

GNU Affero General Public License v3.0 - See [LICENSE](LICENSE)

## Contributing

Contributions are welcome! Please open an issue or submit a PR.

## Links

- [aflare](https://github.com/alib8b8/aflare) - Terminal-based AI workflow engine
- [OpenClaw](https://github.com/alib8b8/openclaw) - Personal AI assistant
- [OpenClaw Plugins](https://github.com/alib8b8/awesome-openclaw) - Discover more plugins
