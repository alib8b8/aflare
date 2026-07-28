# Email Newsletter Generator

> Generate weekly email newsletter content

## Description

This workflow template provides a ready-to-use solution for generate weekly email newsletter content.

## Usage

```bash
llm-box install content-marketing/email-newsletter
llm-box run email-newsletter/workflow.yaml
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

content-marketing
