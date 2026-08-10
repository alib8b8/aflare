# Garden Planner

> Generate a personalized garden planting plan

## Description

This workflow template provides a ready-to-use solution for generate a personalized garden planting plan.

## Usage

```bash
aflare install lifestyle/garden-planner
aflare run garden-planner/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| garden_type | Type of garden (vegetable, flower, herb, mixed) | Yes |
| climate_zone | USDA climate zone | Yes |
| garden_size | Garden size (sq ft or sq m) | Yes |
| sunlight | Sunlight exposure (full sun, partial, shade) | Yes |
| soil_type | Soil type (clay, sandy, loam, etc.) | No |
| season | Current or target season | Yes |
| experience_level | Gardening experience level | No |
| goals | Gardening goals (food production, aesthetics, etc.) | No |
| budget | Budget for the garden | No |
| maintenance | Preferred maintenance level (low, medium, high) | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- researcher - Research and information gathering
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

lifestyle