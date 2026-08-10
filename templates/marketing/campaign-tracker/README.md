# Campaign Performance Tracker

> Track multi-channel marketing campaign performance with ROI analysis

## Description

This workflow template provides comprehensive multi-channel campaign performance tracking. It fetches metrics from analytics and ad platforms, performs Python-based calculations for CTR, CVR, CPA, ROAS, and generates actionable optimization recommendations.

## Usage

```bash
aflare install marketing/campaign-tracker
aflare run campaign-tracker/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| campaign_name | Name of the campaign | Yes |
| campaign_id | Campaign identifier | Yes |
| date_range | Date range for analysis | Yes |
| channels | List of marketing channels | Yes |
| avg_order_value | Average order value for ROAS calculation | Yes |

## Nodes Used

- agent - AI agent for performance report generation
- http_request - Fetch campaign metrics from analytics APIs
- code_interpreter - Python-based metric calculations and analysis
- json_parse - Parse JSON responses
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

marketing