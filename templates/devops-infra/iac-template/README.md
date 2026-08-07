# Infrastructure as Code Template

> Generate Terraform/IaC templates

## Description

This workflow template provides a ready-to-use solution for generate terraform/iac templates.

## Usage

```bash
aflare install cloud-infra/iac-template
aflare run iac-template/workflow.yaml
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

cloud-infra
