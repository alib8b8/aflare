# Data Cleaning Pipeline

> Generate data cleaning and preprocessing pipeline

## Description

This workflow template provides a ready-to-use solution for generate data cleaning and preprocessing pipeline.

## Usage

```bash
llm-box install ai-ml/data-cleaning
llm-box run data-cleaning/workflow.yaml
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

ai-ml
