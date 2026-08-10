# Infographic Brief Generator

> Infographic content brief and data outline

## Description

This workflow template creates comprehensive infographic content briefs with data breakdowns, visualization suggestions, wireframe outlines, and distribution plans. Perfect for handing off to designers or creating data-driven visual content.

## Usage

```bash
aflare install creative/infographic-brief
aflare run infographic-brief/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| topic | Infographic topic | Yes |
| industry | Industry context | Yes |
| audience | Target audience description | Yes |
| infographic_type | Type (statistical, timeline, process, comparison) | Yes |
| data_sources | Data sources to reference | No |
| brand_style | Brand visual style guidelines | Yes |
| goal | Primary goal (traffic, backlinks, awareness, etc.) | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative