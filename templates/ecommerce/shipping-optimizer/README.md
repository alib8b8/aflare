# Shipping Cost and Carrier Optimizer

A shipping cost and carrier optimization engine that compares real-time rates, analyzes delivery performance, and recommends the optimal shipping method.

## Description

This workflow template provides comprehensive shipping optimization:
- **Multi-Carrier Comparison**: UPS, FedEx, USPS, DHL rate comparison
- **Performance Analysis**: On-time delivery and damage rate tracking
- **Multi-Factor Scoring**: Balances cost, speed, and reliability
- **Preference Modes**: Cheapest, fastest, most reliable, or balanced
- **Label Generation**: Optional automatic shipping label creation
- **Savings Calculation**: Shows potential savings vs. most expensive option

## Usage Example

```yaml
params:
  origin_zip: "90210"
  destination_zip: "10001"
  package_weight: 5.5
  package_dimensions: { "length": 12, "width": 8, "height": 4 }
  delivery_preference: "balanced"
  api_base: "https://api.example.com/v1"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| order_id | string | No | - | Specific order to optimize shipping for |
| origin_zip | string | Yes | - | Origin warehouse zip code |
| destination_zip | string | Yes | - | Destination zip code |
| package_weight | number | Yes | - | Package weight in pounds |
| package_dimensions | object | No | - | Package dimensions (length, width, height) |
| delivery_preference | string | No | balanced | Preference (cheapest, fastest, balanced, reliable) |
| api_base | string | Yes | - | Base URL for ecommerce API |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch carrier rates, delivery performance, and generate labels
- **code_interpreter**: Python-based multi-factor scoring and optimization
- **file_write**: Save shipping optimization report

## Category

ecommerce