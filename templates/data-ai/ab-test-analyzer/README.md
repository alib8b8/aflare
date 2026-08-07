# A/B Test Analyzer

> Analyze A/B test results and determine winner

## Description

This workflow template provides a ready-to-use solution for analyze a/b test results and determine winner.

## Usage

```bash
aflare install data-analytics/ab-test-analyzer
aflare run ab-test-analyzer/workflow.yaml
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

data-analytics
