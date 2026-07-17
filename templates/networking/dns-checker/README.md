# DNS Records Checker

> Check all DNS records for a domain

## Description

This workflow template provides a ready-to-use solution for check all dns records for a domain.

## Usage

```bash
llm-box install networking/dns-checker
llm-box run dns-checker/workflow.yaml
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

networking
