# Promotional Campaign Planner

An AI-powered promotional campaign planning engine that designs multi-channel campaigns, allocates budgets, selects products, forecasts impact, and generates creative direction.

## Description

This workflow template provides end-to-end campaign planning:
- **Campaign Design**: AI-generated campaign themes, discount structures, and creative direction
- **Budget Allocation**: Optimized distribution across channels (email, social, site, push, SMS)
- **Product Selection**: Smart product selection based on margins and audience
- **Impact Forecasting**: Python-based ROI and conversion predictions
- **A/B Testing**: Built-in testing plan for campaign optimization

The workflow learns from historical campaign data to improve forecasting accuracy.

## Usage Example

```yaml
params:
  campaign_goal: "revenue"
  budget: 50000
  duration_days: 14
  target_audience: "all"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| campaign_goal | string | Yes | - | Campaign goal (revenue, acquisition, clearance, engagement) |
| budget | number | Yes | - | Total campaign budget |
| duration_days | integer | No | 14 | Campaign duration in days |
| target_audience | string | No | all | Target audience segment |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for campaign design |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch historical campaigns and eligible products
- **agent**: AI-powered campaign design and creative direction
- **code_interpreter**: Python-based ROI and impact forecasting
- **file_write**: Save complete campaign plan

## Category

ecommerce