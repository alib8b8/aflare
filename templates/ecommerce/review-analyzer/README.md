# Customer Review Analyzer

A customer review sentiment analysis and insight extraction engine that processes product reviews, classifies sentiment, identifies key topics, and generates actionable product improvement recommendations.

## Description

This workflow template transforms raw customer reviews into actionable business intelligence. It performs:
- **Sentiment Classification**: Positive/negative/neutral with scoring
- **Topic Extraction**: Key themes and features discussed
- **Pain Point Identification**: Common customer complaints and issues
- **Competitive Analysis**: Strengths and weaknesses vs. competitors

The workflow fetches reviews from multiple sources, runs dual-stage AI analysis (sentiment then insights), and generates a comprehensive review report.

## Usage Example

```yaml
params:
  product_id: "prod_45678"
  review_source: "all"
  date_range: "90d"
  language: "auto"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_id | string | Yes | - | Product to analyze reviews for |
| review_source | string | No | all | Review source (all, website, marketplace, social) |
| date_range | string | No | 90d | Review date range to analyze |
| language | string | No | auto | Review language filter |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for analysis |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch product reviews from API
- **agent**: Dual-stage AI analysis (sentiment classification and insight extraction)
- **file_write**: Save comprehensive review analysis report

## Category

ecommerce