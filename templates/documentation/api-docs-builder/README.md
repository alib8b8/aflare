# API Documentation Builder

> Build API documentation from spec

## Description

This workflow template provides a ready-to-use solution for build api documentation from spec.

## Usage

```bash
llm-box install documentation/api-docs-builder
llm-box run api-docs-builder/workflow.yaml
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

documentation
