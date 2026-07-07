---
name: grok-workflow
description: Use llm-box to generate and execute YAML workflows from plain English. For Grok/xAI integration via MCP or CLI.
invocation: both
---

# llm-box Grok Integration

## Overview

llm-box is a terminal-first workflow automation engine. It generates and executes
YAML workflows from natural language descriptions. This skill enables Grok to
use llm-box as a tool for multi-step automation tasks.

## Integration Methods

### Method 1: MCP Server (Recommended for Grok Web)

Grok supports Remote MCP via Streamable HTTP/SSE. To connect llm-box to Grok:

1. Start the Remote MCP server:
   ```bash
   llm-box --mcp-remote --port 8080
   ```

2. In Grok, go to **grok.com/connectors** and add a custom MCP:
   - Type: Remote MCP (Streamable HTTP)
   - URL: `http://localhost:8080/mcp`

3. Grok will discover all 5 tools automatically.

### Method 2: Grok Build CLI (Local Development)

For Grok Build CLI, use stdio MCP:

1. Install llm-box:
   ```bash
   curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
   bash install.sh
   ```

2. Add to your Grok Build project's `.mcp.json`:
   ```json
   {
     "mcpServers": {
       "llm-box": {
         "type": "stdio",
         "command": "llm-box",
         "args": ["--mcp-server"]
       }
     }
   }
   ```

### Method 3: xAI API Function Calling

Use llm-box tools as xAI API function definitions. See `grok-mcp-server/tools.json`
for the complete tool schema compatible with xAI's function-calling API.

## Available MCP Tools

| Tool | Description |
|------|-------------|
| `create_workflow` | Generate a YAML workflow from a plain English description |
| `run_workflow` | Execute a workflow from a YAML file path |
| `run_workflow_yaml` | Execute a workflow from raw YAML content |
| `list_nodes` | List all available llm-box nodes |
| `validate_workflow` | Validate a workflow YAML without executing |

## Quick CLI Reference

```bash
# Generate workflow from description
llm-box create "fetch Hacker News top stories and save to file"

# Run a workflow
llm-box run workflow.yaml

# List available nodes
llm-box list

# Validate without running
llm-box validate workflow.yaml

# Safe mode (disables execute node)
llm-box --safe-mode run workflow.yaml
```

## Available Nodes

**Utility:** fetch_url, http_request, file_read, file_write, execute, json_parse,
template_render, transform, combine, notify

**LLM:** ollama, deepseek, openai, qwen, glm, kimi, mistral, yi

**Control:** condition, call

## Example: Grok-Triggered Workflow

When a user asks Grok to automate something, Grok can call the `create_workflow`
tool, then `run_workflow_yaml` to execute it immediately:

1. User: "Fetch the weather for Beijing and save it to a file"
2. Grok calls `create_workflow` with description "fetch weather for Beijing and save to file"
3. llm-box returns the YAML workflow
4. Grok calls `run_workflow_yaml` with the generated YAML
5. Result is returned to the user

## Security

- SSRF protection on URL fetching
- Path traversal protection on file operations
- Command injection prevention on execute node
- Safe mode available to disable execute node entirely
- Resource limits (file size, response body, step count, timeouts)
