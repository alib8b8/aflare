---
name: llm-box-setup
description: Guide for setting up and connecting the llm-box MCP server
---

# llm-box Setup Guide

Welcome to llm-box! This guide will help you set up the MCP server connection.

## Prerequisites

1. **Install llm-box**: You need the llm-box binary installed on your system.

## Installation

### Option 1: Install Script (recommended for Linux/macOS)

```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh
```

### Option 2: Go Install

```bash
go install github.com/alib8b8/llm-box/cmd/llm-box@latest
```

### Option 3: Download from Releases

Download the appropriate binary for your platform from:
https://github.com/alib8b8/llm-box/releases

## Verify Installation

```bash
llm-box list
```

This should display the list of available nodes.

## MCP Server Configuration

The plugin comes with a pre-configured `.mcp.json` file that connects to the llm-box MCP server via stdio:

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

Claude will automatically start the MCP server when the plugin is activated.

## Post-Setup Verification

After installation, run this test command to verify everything works:

```bash
llm-box create "Fetch the GitHub trending page and save to file" --dry-run
```

## Next Steps

Once installed, you can use these slash commands:
- `/llm-box:create <description>` - Generate a workflow from natural language
- `/llm-box:run <workflow.yaml>` - Execute a workflow file
- `/llm-box:list` - Show available nodes
- `/llm-box:validate <workflow.yaml>` - Validate a workflow

## Configuration

You can configure llm-box by creating a `~/.llm-box/config.yaml` file:

```yaml
# Global settings
safe_mode: false
default_model: "ollama://llama3"

# API keys for LLM providers
api_keys:
  openai: "your-api-key"
  deepseek: "your-api-key"
```

## Troubleshooting

If the MCP server fails to start:

1. Check if llm-box is in your PATH
2. Verify the installation with `llm-box --version`
3. Check that you have Go runtime or the correct binary for your architecture

## Resources

- Documentation: https://github.com/alib8b8/llm-box/tree/main/docs
- Examples: https://github.com/alib8b8/llm-box/tree/main/examples
- GitHub: https://github.com/alib8b8/llm-box