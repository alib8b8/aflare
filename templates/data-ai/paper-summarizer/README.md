# Academic Paper Summarizer

> Summarize academic papers into key insights

## Description

This workflow template provides a ready-to-use solution for summarize academic papers into key insights.

## Usage

```bash
llm-box install research-analysis/paper-summarizer
llm-box run paper-summarizer/workflow.yaml
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

research-analysis
