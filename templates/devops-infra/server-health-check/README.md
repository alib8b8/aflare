# Server Health Check

> Check server health metrics and generate report

## Description

This workflow template provides a ready-to-use solution for check server health metrics and generate report.

## Usage

```bash
llm-box install devops-monitoring/server-health-check
llm-box run server-health-check/workflow.yaml
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
