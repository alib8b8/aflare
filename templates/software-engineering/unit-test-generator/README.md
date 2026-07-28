# Unit Test Generator

> Generate unit test skeleton for Go functions

## Description

This workflow template provides a ready-to-use solution for generate unit test skeleton for go functions.

## Usage

```bash
llm-box install developer-tools/unit-test-generator
llm-box run unit-test-generator/workflow.yaml
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
