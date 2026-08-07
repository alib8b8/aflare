# Social Media Content Planner

> Plan social media content for the week

## Description

This workflow template provides a ready-to-use solution for plan social media content for the week.

## Usage

```bash
aflare install content-marketing/social-media-planner
aflare run social-media-planner/workflow.yaml
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
