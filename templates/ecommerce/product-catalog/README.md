# Product Catalog Enrichment

A product catalog enrichment and optimization engine that generates AI-powered product descriptions, structured attributes, SEO metadata, and schema.org markup.

## Description

This workflow template automates product catalog enrichment:
- **AI Descriptions**: Compelling product titles, short and long descriptions
- **Structured Attributes**: Size, color, material, weight, dimensions
- **SEO Optimization**: Meta titles, descriptions, URL slugs, alt text
- **Schema.org Markup**: Structured data for rich search results
- **Content Validation**: Quality scoring and issue detection

## Usage Example

```yaml
params:
  product_ids: ["prod_111", "prod_222"]
  enrichment_type: "full"
  language: "en"
  seo_keywords: ["wireless", "bluetooth", "noise cancelling"]
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| product_ids | array | No | - | Specific products to enrich (empty for all missing) |
| enrichment_type | string | No | full | Enrichment type (description, attributes, seo, images, full) |
| language | string | No | en | Target language for content |
| seo_keywords | array | No | - | Target SEO keywords |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for content generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch products needing enrichment
- **agent**: AI-powered content generation and enrichment
- **code_interpreter**: Python-based content validation and quality scoring
- **file_write**: Save enriched product catalog

## Category

ecommerce