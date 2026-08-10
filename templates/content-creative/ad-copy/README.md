# Ad Copy Generator

> Advertising copy generator for multiple platforms

## Description

This workflow template generates comprehensive advertising copy across search, social, display, and video formats. Includes campaign strategy, A/B testing plans, and platform-specific optimizations.

## Usage

```bash
aflare install creative/ad-copy
aflare run ad-copy/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| product | Product or service name | Yes |
| platform | Primary ad platform | Yes |
| audience | Target audience description | Yes |
| goal | Campaign goal (awareness, conversion, etc.) | Yes |
| budget_tier | Budget level (low, medium, high) | Yes |
| usp | Unique selling proposition | Yes |
| competitors | Key competitors | No |
| brand_voice | Brand voice and tone | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative