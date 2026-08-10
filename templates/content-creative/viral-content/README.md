# Viral Content Generator

> Viral content idea generator and hook writer

## Description

This workflow template generates viral content strategies with 30 scroll-stopping hooks, 20 viral content ideas, 8 content templates, engagement optimization tactics, and analytics frameworks. Designed for creators and brands aiming to maximize reach.

## Usage

```bash
aflare install creative/viral-content
aflare run viral-content/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand | Brand or creator name | Yes |
| platform | Primary platform (TikTok, Instagram, YouTube, etc.) | Yes |
| niche | Content niche or category | Yes |
| audience | Target audience description | Yes |
| brand_voice | Brand voice and tone | Yes |
| top_content | Examples of previous top-performing content | No |
| trends | Current trends to leverage | No |
| content_format | Preferred content format (short-form, long-form, etc.) | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative