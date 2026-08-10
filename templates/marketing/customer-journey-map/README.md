# Customer Journey Map

> Map customer journey and analyze touchpoints across all stages

## Description

This workflow template creates comprehensive customer journey maps covering all stages from awareness through advocacy. It identifies pain points, emotional states, and improvement opportunities at each touchpoint.

## Usage

```bash
aflare install marketing/customer-journey-map
aflare run customer-journey-map/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| persona | Target customer persona description | Yes |
| product | Product or service being mapped | Yes |
| industry | Industry context | Yes |
| goal | Customer's primary goal | Yes |
| discovery_channels | How customers discover the product | Yes |

## Nodes Used

- agent - AI agent for journey analysis and mapping
- transform - Data transformation and structuring
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing