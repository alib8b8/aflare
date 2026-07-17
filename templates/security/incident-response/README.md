# Incident Response Plan

> Generate incident response playbook

## Description

This workflow template provides a ready-to-use solution for generate incident response playbook.

## Usage

```bash
llm-box install security/incident-response
llm-box run incident-response/workflow.yaml
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

security
