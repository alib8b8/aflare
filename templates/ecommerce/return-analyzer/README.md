# Return Reason Analyzer

A return reason analysis and prevention engine that examines return patterns, identifies root causes, calculates return costs, and generates actionable prevention strategies.

## Description

This workflow template helps ecommerce businesses reduce return rates by:
- **Pattern Analysis**: Identifying common return reasons and trends
- **Cost Calculation**: Quantifying the financial impact of returns
- **Product Flagging**: Highlighting products with abnormally high return rates
- **Prevention Strategies**: AI-generated recommendations to reduce returns

The workflow combines Python-based statistical analysis with AI-powered strategy generation.

## Usage Example

```yaml
params:
  date_range: "90d"
  category_id: "cat_apparel"
  min_return_rate: 0.05
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| date_range | string | No | 90d | Analysis period for returns |
| category_id | string | No | - | Filter by product category |
| min_return_rate | number | No | 0.05 | Minimum return rate threshold for flagging |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for strategy generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch returns data from analytics API
- **code_interpreter**: Python-based return metrics calculation and pattern analysis
- **agent**: AI-powered prevention strategy generation
- **file_write**: Save comprehensive return analysis report

## Category

ecommerce