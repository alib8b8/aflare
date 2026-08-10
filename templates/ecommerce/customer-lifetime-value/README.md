# Customer Lifetime Value Calculator

A customer lifetime value calculation and segmentation engine that analyzes transaction history, predicts future CLV, and generates tailored marketing strategies.

## Description

This workflow template provides comprehensive CLV analysis:
- **Historical CLV**: Calculates actual revenue per customer
- **Predictive CLV**: Forecasts future value using discounted cash flow
- **Tier Segmentation**: Classifies customers into platinum, gold, silver, bronze
- **Strategy Generation**: AI-powered marketing strategies per segment
- **CAC Ratio Analysis**: Compares acquisition cost to lifetime value

## Usage Example

```yaml
params:
  segment_field: "clv_tier"
  prediction_horizon: 12
  discount_rate: 0.10
  min_transactions: 1
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| segment_field | string | No | clv_tier | Field to segment customers by |
| prediction_horizon | integer | No | 12 | Months to forecast CLV |
| discount_rate | number | No | 0.10 | Discount rate for future cash flows |
| min_transactions | integer | No | 1 | Minimum transactions for inclusion |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for strategy generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch customer transaction and order data
- **code_interpreter**: Python-based CLV calculation with DCF modeling
- **agent**: AI-powered segment strategy generation
- **file_write**: Save comprehensive CLV analysis report

## Category

ecommerce