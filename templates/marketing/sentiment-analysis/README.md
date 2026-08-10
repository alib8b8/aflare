# Brand Sentiment Analysis

> Analyze brand sentiment from social media and review platforms

## Description

This workflow template performs comprehensive brand sentiment analysis by fetching social media mentions and customer reviews, then applying keyword-based sentiment scoring in Python. It generates detailed reports with sentiment distribution, common themes, crisis detection, and actionable recommendations.

## Usage

```bash
aflare install marketing/sentiment-analysis
aflare run sentiment-analysis/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand_name | Brand name to analyze | Yes |
| industry | Industry context for benchmarking | Yes |

## Nodes Used

- agent - AI agent for sentiment report generation
- http_request - Fetch social mentions and reviews from APIs
- code_interpreter - Python-based sentiment scoring and theme extraction
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing