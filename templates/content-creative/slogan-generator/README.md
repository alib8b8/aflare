# Slogan Generator

> Brand slogan and tagline generator

## Description

This workflow template generates a comprehensive set of brand slogans and taglines across multiple categories, with evaluation criteria and top recommendations. Produces 40+ slogan options plus tagline variants.

## Usage

```bash
aflare install creative/slogan-generator
aflare run slogan-generator/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand | Brand name | Yes |
| industry | Industry sector | Yes |
| personality | Brand personality traits | Yes |
| values | Core brand values | Yes |
| audience | Target audience description | Yes |
| usp | Unique selling proposition | Yes |
| competitor_slogans | Known competitor slogans | No |
| language | Target language | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative