# API Documentation Generator

> Generate API documentation from code comments

## Description

This workflow template provides a ready-to-use solution for generate api documentation from code comments.

## Usage

```bash
aflare install developer-tools/api-docs-generator
aflare run api-docs-generator/workflow.yaml
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

developer-tools
