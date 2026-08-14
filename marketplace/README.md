# aflare Workflow Marketplace

The aflare Workflow Marketplace is a package manager that lets you discover, install, and manage reusable workflow packages — just like `npm` or `pip`, but for aflare workflows.

## What is a Workflow Package?

A workflow package is a self-contained aflare workflow YAML definition bundled with metadata (name, version, description, category, author, tags). Each package is ready to run after installation — just supply your own parameters.

## Quick Start

### Install a workflow

```bash
aflare install btc-monitor
```

This downloads the workflow YAML to `~/.aflare/workflows/btc-monitor.yaml`. You can then run it:

```bash
aflare run ~/.aflare/workflows/btc-monitor.yaml
```

### List available workflows

```bash
aflare marketplace list
```

Output:

```
Available Workflows (5)
----------------------------------------
  arxiv-daily        research   Fetch latest arXiv papers and generate summaries
  btc-monitor        finance    Monitor Bitcoin price movements and send alerts
  financial-aml      finance    AML transaction screening and risk scoring
  github-alert       devops     Aggregate GitHub activity and send daily digest
```

### Search for workflows

```bash
aflare marketplace search finance
aflare marketplace search robot
aflare marketplace search alert
```

### List by category

```bash
aflare marketplace category finance
aflare marketplace category research
```

### Show package details

```bash
aflare marketplace show btc-monitor
```

### Uninstall a workflow

```bash
aflare uninstall btc-monitor
```

## Available Categories

| Category   | Description                                    |
|------------|------------------------------------------------|
| `finance`  | Financial analysis, trading, AML, compliance   |
| `devops`   | CI/CD, monitoring, infrastructure automation   |
| `research` | Academic research, paper analysis, literature  |
| `robot`    | Robotics, autonomous systems, sensor workflows |

## Built-in Packages

| Package          | Category  | Description                                              |
|------------------|-----------|----------------------------------------------------------|
| `btc-monitor`    | finance   | Monitor Bitcoin price and send Telegram alerts            |
| `github-alert`   | devops    | Aggregate GitHub repo activity into daily digest          |
| `arxiv-daily`    | research  | Fetch latest arXiv papers and generate AI summaries       |
| `financial-aml`  | finance   | AML transaction screening with risk scoring               |

## Publishing Your Own Workflow

To publish a workflow to the marketplace:

1. **Create your workflow YAML** — write a standard aflare workflow file:

```yaml
name: "My Cool Workflow"
description: "Does something amazing"
steps:
  - node: fetch_url
    id: step1
    params:
      url: "https://api.example.com/data"
  - node: llm
    id: step2
    params:
      prompt: "Analyze: {{data}}"
```

2. **Add package metadata** — create a `package.yaml` alongside your workflow:

```yaml
name: my-cool-workflow
version: "1.0.0"
description: "Does something amazing"
category: devops
author: your-name
tags:
  - automation
  - api
  - analysis
```

3. **Submit to the registry** — add your package entry to `marketplace/registry.yaml` and open a pull request.

## Package Format Specification

Every package in the marketplace registry follows this schema:

```yaml
packages:
  - name: <string>            # Unique package identifier (lowercase, hyphens)
    version: <string>         # Semver version (e.g., "1.0.0")
    description: <string>     # Short description of what the workflow does
    category: <string>        # Category: finance, devops, research, robot, etc.
    author: <string>          # Package author or organization
    install_command: <string> # CLI command to install (e.g., "aflare install <name>")
    tags:                     # Search keywords
      - <string>
      - <string>
```

### Package Name Rules

- Only lowercase letters, digits, hyphens, and underscores
- 1-100 characters
- Must be unique within the registry

## Installation Location

Installed workflows are stored in `~/.aflare/workflows/`:

```
~/.aflare/
  └── workflows/
      ├── btc-monitor.yaml
      ├── github-alert.yaml
      ├── arxiv-daily.yaml
      └── financial-aml.yaml
```

Set `AFLARE_DATA` to override the base directory:
```bash
export AFLARE_DATA=/custom/path/.aflare
```

## Registry File

The marketplace registry is defined in `marketplace/registry.yaml` at the root of the aflare repository. This file is the source of truth for all available packages and their metadata.