# Newsletter Content Generator

> Email newsletter content curator and writer

## Description

This workflow template generates complete email newsletter editions with curated content, original articles, engagement sections, and A/B test-ready subject lines. Perfect for weekly or monthly newsletters.

## Usage

```bash
aflare install creative/newsletter-content
aflare run newsletter-content/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| newsletter_name | Name of the newsletter | Yes |
| edition | Edition number or date | Yes |
| theme | Theme or focus for this edition | Yes |
| industry | Industry context | Yes |
| audience | Target audience description | Yes |
| brand_voice | Brand voice and tone | Yes |
| sources | Content sources to curate from | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative