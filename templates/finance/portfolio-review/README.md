# Investment Portfolio Review

> Review and optimize investment portfolio

## Description

This workflow template provides a ready-to-use solution for review and optimize investment portfolio.

## Usage

```bash
llm-box install finance-investing/portfolio-review
llm-box run portfolio-review/workflow.yaml
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
