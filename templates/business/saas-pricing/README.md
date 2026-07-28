# SaaS Pricing Calculator

> Design SaaS pricing strategy and tiers

## Description

This workflow template provides a ready-to-use solution for design saas pricing strategy and tiers.

## Usage

```bash
llm-box install business-sales/saas-pricing
llm-box run saas-pricing/workflow.yaml
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

business-sales
