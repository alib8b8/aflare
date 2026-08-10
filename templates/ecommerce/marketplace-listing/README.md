# Multi-Marketplace Listing Optimizer

A multi-marketplace listing optimization engine that adapts product content for Amazon, eBay, Walmart, and other platforms with platform-specific requirements.

## Description

This workflow template streamlines marketplace listing creation:
- **Platform Adaptation**: Generates optimized content for each marketplace
- **Search Optimization**: Platform-specific keyword and search term strategies
- **Content Validation**: Validates listings against platform requirements
- **Category Mapping**: Maps products to correct marketplace categories
- **Pricing Strategy**: Considers platform fees in pricing recommendations
- **Enhanced Content**: A+ Content and enhanced brand content suggestions

## Usage Example

```yaml
params:
  product_id: "prod_33333"
  marketplaces: ["amazon", "ebay", "walmart"]
  optimize_for: "conversion"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_id | string | Yes | - | Product to create marketplace listings for |
| marketplaces | array | No | [amazon, ebay, walmart] | Target marketplaces |
| optimize_for | string | No | conversion | Optimization goal (conversion, visibility, ranking) |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for content generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch product master data and marketplace requirements
- **agent**: AI-powered platform-specific listing generation
- **code_interpreter**: Python-based listing validation against platform rules
- **file_write**: Save marketplace listings

## Category

ecommerce