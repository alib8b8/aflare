# Competitor Price Tracker

> Monitor and analyze competitor pricing strategies with gap detection

## Description

This workflow template tracks competitor pricing by fetching live pricing data from market APIs and competitor websites. It performs statistical analysis of market positioning, identifies pricing gaps, and generates strategic recommendations for price optimization.

## Usage

```bash
aflare install marketing/competitor-price-tracker
aflare run competitor-price-tracker/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| competitor_pricing_page | URL of competitor pricing page | Yes |
| product_category | Product or service category | Yes |
| market | Target market/region | Yes |
| our_prices | JSON object with your pricing tiers | Yes |

## Nodes Used

- agent - AI agent for pricing strategy recommendations
- http_request - Fetch competitor pricing pages and market data
- code_interpreter - Python-based statistical pricing analysis
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing