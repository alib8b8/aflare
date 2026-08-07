---
name: setup
description: Guide for installing and configuring aflare, including binary installation, source builds, workflows directory setup, and initial workflow creation
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
  - setup
  - installation
  - configuration
  - aflare
---

# Setup Skill

## Overview

The setup skill walks you through installing aflare and getting your first workflow running. aflare is a workflow engine that chains nodes (steps) to automate tasks — fetching URLs, executing commands, transforming data, calling local LLMs via Ollama, reading/writing files, and more. This skill covers binary installation, building from source, directory structure, configuration, secrets, and verification.

## Prerequisites

- **Go 1.25.12+** — required only if building from source
- **Node.js >=22.0.0** — required for OpenClaw plugin development
- **Git** — for cloning the repository and version control
- **Ollama** (optional) — needed for local LLM inference in workflow nodes
- **Docker** (optional) — for containerized deployments

## Instructions

### 1. Install aflare

**Binary release:** Download the latest release asset for your platform from the [GitHub Releases](https://github.com/alib8b8/aflare/releases) page.

**Build from source:**

```bash
git clone https://github.com/alib8b8/aflare.git
cd aflare
go build -o aflare ./cmd/aflare
sudo mv aflare /usr/local/bin/
```

### 2. Verify Installation

```bash
aflare --version
```

### 3. Create Workflows Directory

```bash
mkdir -p ~/.aflare/workflows
```

### 4. Configure Secrets (Optional)

```bash
aflare secrets init
aflare secrets set --group api --key service <password>
```

### 5. Test with a Sample Workflow

```bash
aflare run examples/content-processor.yaml
```

## Output

After a successful setup you should have:
- A working `aflare` binary available in your PATH
- A workflows directory at `~/.aflare/workflows/` (or a custom path)
- An optional secrets store for sensitive parameters
- A test run confirming the engine can execute workflows

## Examples

```bash
# Full installation from source
git clone https://github.com/alib8b8/aflare.git
cd aflare
go build -o aflare ./cmd/aflare
sudo mv aflare /usr/local/bin/

# Create and run your first workflow
mkdir -p ~/.aflare/workflows
cat > ~/.aflare/workflows/hello.yaml << 'EOF'
name: hello-world
description: Simple hello world workflow
steps:
  - node: file_write
    params:
      path: "output.txt"
    input: "Hello from aflare!"
EOF
aflare run ~/.aflare/workflows/hello.yaml
```

## Resources

- [aflare GitHub Repository](https://github.com/alib8b8/aflare)
- [Node Reference Documentation](docs/nodes-reference.md)
- [Custom Nodes Guide](docs/custom-nodes.md)
- [Example Workflows](examples/)
- [OpenClaw Plugin](contrib/openclaw/)
