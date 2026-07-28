# SEO Keyword Research

> Research and analyze SEO keywords for a topic

## Description

This workflow template provides a ready-to-use solution for research and analyze seo keywords for a topic.

## Usage

```bash
llm-box install content-marketing/seo-keyword-research
llm-box run seo-keyword-research/workflow.yaml
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

content-marketing
