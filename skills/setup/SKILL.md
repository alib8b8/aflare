---
name: setup
description: Install and configure llm-box, including the MCP server connection for Claude Code.
invocation: user
allowed-tools: Read, Edit, Write, Bash
version: 0.6.0
author: llm-box
license: MIT
compatibility: claude-code >= 0.7.0
tags: [setup, installation, configuration, mcp]
---

## Overview

This skill installs and configures llm-box, an AI-powered terminal workflow engine, and sets up the MCP server connection for Claude Code integration. It handles binary installation, PATH configuration, and MCP server setup.

## Prerequisites

- Go 1.21+ (for building from source) OR a pre-built binary
- Git (for cloning or installing)
- Bash shell (Linux/macOS) or PowerShell (Windows)

## Instructions

### Step 1: Install llm-box

Choose one of the following installation methods:

**Option 1: Install Script (Linux/macOS)**
```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh
```

**Option 2: Go Install**
```bash
go install github.com/alib8b8/llm-box/cmd/llm-box@latest
```

**Option 3: Download from Releases**
Download the binary for your platform:
https://github.com/alib8b8/llm-box/releases

### Step 2: Verify Installation

```bash
llm-box --version
llm-box list
```

### Step 3: Configure MCP Server

The MCP server is pre-configured in `.mcp.json`:

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

Claude Code will automatically start the MCP server when the plugin is activated.

### Step 4: Create Configuration (Optional)

Create `~/.llm-box/config.yaml`:

```yaml
safe_mode: false
default_model: "ollama://llama3"
api_keys:
  openai: "your-api-key"
  deepseek: "your-api-key"
```

## Output

After successful setup:
- llm-box CLI is installed and available in PATH
- MCP server is configured for Claude Code
- Configuration file created at `~/.llm-box/config.yaml`
- Verify with: `llm-box --version`

## Examples

**Example 1: Fresh Install on Linux**
```bash
curl -sL https://raw.githubusercontent.com/alib8b8/llm-box/main/install.sh -o install.sh
bash install.sh
llm-box --version
```

**Example 2: Update via Go**
```bash
go install github.com/alib8b8/llm-box/cmd/llm-box@latest
llm-box --version
```

**Example 3: Configure API Keys**
```bash
mkdir -p ~/.llm-box
cat > ~/.llm-box/config.yaml <<EOF
safe_mode: false
default_model: "deepseek-chat"
api_keys:
  deepseek: "sk-your-key"
  openai: "sk-your-key"
EOF
```

## Resources

- **GitHub**: https://github.com/alib8b8/llm-box
- **GitCode**: https://gitcode.com/llm-box/llm-box
- **Releases**: https://github.com/alib8b8/llm-box/releases
- **Documentation**: https://gitcode.com/llm-box/llm-box/blob/main/README.md
- **Issues**: https://github.com/alib8b8/llm-box/issues
- **Troubleshooting**:
  - Check PATH: `which llm-box`
  - Verify version: `llm-box --version`
  - Test MCP server: `llm-box --mcp-server`
- **License**: MIT