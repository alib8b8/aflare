# Config Auditor

> Audit configuration files for security and best practices

## Description

This workflow template provides a ready-to-use solution for audit configuration files for security and best practices.

## Usage

```bash
aflare install devops-infra/config-auditor
aflare run config-auditor/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| config_path | Path to configuration files directory | Yes |

## Nodes Used

- execute - Execute shell commands for scanning configs
- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

devops-infra