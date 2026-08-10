# RFM Customer Segmentation

An RFM (Recency, Frequency, Monetary) customer segmentation engine that analyzes transaction history to classify customers into actionable segments.

## Description

This workflow template provides comprehensive customer segmentation:
- **RFM Scoring**: Calculates recency, frequency, and monetary scores
- **Segment Classification**: Champions, loyal customers, potential loyalists, at-risk, lost, big spenders
- **Percentile-Based Scoring**: Dynamic thresholds based on actual data distribution
- **Configurable Weights**: Adjust R/F/M importance for scoring
- **Strategy Generation**: AI-powered marketing strategies per segment

## Usage Example

```yaml
params:
  segmentation_model: "rfm"
  recency_weight: 0.35
  frequency_weight: 0.35
  monetary_weight: 0.30
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| segmentation_model | string | No | rfm | Segmentation model (rfm, behavioral, demographic, hybrid) |
| recency_weight | number | No | 0.35 | Weight for recency in RFM scoring |
| frequency_weight | number | No | 0.35 | Weight for frequency in RFM scoring |
| monetary_weight | number | No | 0.30 | Weight for monetary value in RFM scoring |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for strategy generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch customer transaction data
- **code_interpreter**: Python-based RFM scoring and segment classification
- **agent**: AI-powered segment-specific marketing strategy generation
- **file_write**: Save segmentation report

## Category

ecommerce