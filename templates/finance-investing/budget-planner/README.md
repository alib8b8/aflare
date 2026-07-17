# Budget Planner

> Create personal budget plan and tracker

## Description

This workflow template provides a ready-to-use solution for create personal budget plan and tracker.

## Usage

```bash
llm-box install finance-investing/budget-planner
llm-box run budget-planner/workflow.yaml
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

finance-investing
