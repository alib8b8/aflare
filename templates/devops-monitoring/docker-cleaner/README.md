# Docker Image Cleaner

> Clean up unused Docker images and containers

## Description

This workflow template provides a ready-to-use solution for clean up unused docker images and containers.

## Usage

```bash
llm-box install devops-monitoring/docker-cleaner
llm-box run docker-cleaner/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| - | Check workflow.yaml for configurable parameters | - |

## Nodes Used

- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications
- http_request - Make HTTP requests (when applicable)
- json_parse - Parse JSON responses (when applicable)
- execute - Execute shell commands (when applicable)

## Category

devops-monitoring
