# Loyalty Program Designer

A loyalty program design and optimization engine that analyzes customer engagement data to create tiered loyalty structures with complete program designs.

## Description

This workflow template provides comprehensive loyalty program design:
- **Program Types**: Points-based, tiered, subscription, hybrid, gamified
- **Tier Structure**: Data-driven tier thresholds based on spend distribution
- **Points Economics**: Liability calculation, redemption rate estimation, ROI projection
- **Benefit Design**: Tier-specific benefits from basic to premium
- **Complete Program Design**: Branding, earning rules, redemption options, communication plan
- **Launch Planning**: Timeline and success metrics

## Usage Example

```yaml
params:
  program_type: "points_based"
  tier_count: 3
  points_per_dollar: 10
  point_value: 0.01
  target_retention_rate: 0.80
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| program_type | string | No | points_based | Loyalty program type (points_based, tiered, subscription, hybrid, gamified) |
| tier_count | integer | No | 3 | Number of tiers in the program |
| points_per_dollar | number | No | 10 | Base points earned per dollar spent |
| point_value | number | No | 0.01 | Dollar value of each point |
| target_retention_rate | number | No | 0.80 | Target customer retention rate |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for program design |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch customer engagement data
- **code_interpreter**: Python-based tier structure design and economics calculation
- **agent**: AI-powered complete program design and branding
- **file_write**: Save loyalty program design document

## Category

ecommerce