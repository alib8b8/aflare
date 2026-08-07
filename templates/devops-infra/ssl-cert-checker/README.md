# SSL Certificate Checker

> Check SSL certificate expiry for domains

## Description

This workflow template provides a ready-to-use solution for check ssl certificate expiry for domains.

## Usage

```bash
aflare install devops-monitoring/ssl-cert-checker
aflare run ssl-cert-checker/workflow.yaml
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
