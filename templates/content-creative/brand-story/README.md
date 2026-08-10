# Brand Story Generator

> Brand narrative and origin story writer

## Description

This workflow template crafts compelling brand stories including origin narratives, multiple length versions (from one-liner to full About page), storytelling elements, and a brand voice guide. Built on narrative archetype frameworks.

## Usage

```bash
aflare install creative/brand-story
aflare run brand-story/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand | Brand name | Yes |
| founder | Founder background and story | Yes |
| industry | Industry sector | Yes |
| mission | Brand mission statement | Yes |
| values | Core brand values | Yes |
| origin | Origin details and founding story | Yes |
| milestones | Key company milestones | No |
| audience | Target audience description | Yes |
| tone | Desired tone (inspirational, humble, etc.) | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative