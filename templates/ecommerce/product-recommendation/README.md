# Product Recommendation Engine

An AI-powered product recommendation engine that leverages customer behavior data, browsing history, and purchase patterns to generate personalized product suggestions.

## Description

This workflow template implements a sophisticated recommendation system that combines multiple strategies:
- **Collaborative Filtering**: Recommends products based on similar customers' preferences
- **Content-Based**: Matches product attributes with customer interests
- **Hybrid**: Combines both approaches for optimal results

The workflow fetches customer profiles and browsing history, runs AI analysis to compute recommendations, enriches results with product details, and saves the output.

## Usage Example

```yaml
params:
  customer_id: "cust_12345"
  top_n: 10
  strategy: "hybrid"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  lookback_days: 30
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| customer_id | string | Yes | - | Target customer identifier |
| top_n | integer | No | 10 | Number of recommendations to return |
| strategy | string | No | collaborative | Recommendation strategy (collaborative, content-based, hybrid) |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for recommendation computation |
| lookback_days | integer | No | 30 | Days of browsing history to analyze |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch customer profile, browsing history, and enrich product details
- **agent**: AI-powered recommendation computation and reasoning
- **file_write**: Save recommendation results to JSON file

## Category

ecommerce