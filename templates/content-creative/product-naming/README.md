# Product Naming Generator

> Product and brand name brainstorming

## Description

This workflow template generates 80+ product or brand name ideas across 8 naming categories, with a detailed evaluation matrix, domain/trademark guidance, and top recommendations with rationale.

## Usage

```bash
aflare install creative/product-naming
aflare run product-naming/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| product_description | Description of the product or brand | Yes |
| industry | Industry sector | Yes |
| audience | Target audience description | Yes |
| brand_architecture | Brand architecture (master, sub, endorsed) | Yes |
| naming_style | Preferred naming style | No |
| competitor_names | Known competitor names | No |
| keywords | Keywords to incorporate | No |
| language | Target language considerations | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative