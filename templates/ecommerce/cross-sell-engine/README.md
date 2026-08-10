# Cross-Sell and Bundle Engine

A cross-sell and bundle recommendation engine that analyzes purchase patterns, product relationships, and customer behavior to generate optimized product bundles.

## Description

This workflow template automates cross-sell and bundle creation:
- **Complementary Products**: Finds products that naturally pair together
- **Bundle Generation**: Creates optimized product bundles with pricing
- **Discount Strategy**: Calculates optimal bundle discounts
- **Marketing Content**: Generates compelling descriptions and taglines
- **Multiple Strategies**: Supports complementary, upgrade, accessory, and bundle approaches

The workflow combines Python-based combinatorial optimization with AI-generated marketing content.

## Usage Example

```yaml
params:
  product_id: "prod_78901"
  bundle_size: 3
  discount_threshold: 0.10
  strategy: "complementary"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_id | string | Yes | - | Source product for cross-sell recommendations |
| bundle_size | integer | No | 3 | Maximum items in bundle |
| discount_threshold | number | No | 0.10 | Maximum bundle discount rate |
| strategy | string | No | complementary | Cross-sell strategy (complementary, upgrade, accessory, bundle) |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for marketing content |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch product info and cross-sell candidates
- **code_interpreter**: Python-based bundle generation and pricing optimization
- **agent**: AI-powered marketing description generation
- **file_write**: Save cross-sell bundle results

## Category

ecommerce