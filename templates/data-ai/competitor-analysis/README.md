# Competitor Analysis

> Analyze competitors and their strategies

## Description

This workflow template provides a ready-to-use solution for analyze competitors and their strategies.

## Usage

```bash
aflare install research-analysis/competitor-analysis
aflare run competitor-analysis/workflow.yaml
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

research-analysis
