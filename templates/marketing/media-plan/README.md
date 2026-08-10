# Media Buying Plan

> Media buying plan and budget allocator with channel optimization

## Description

This workflow template generates comprehensive media buying plans with data-driven budget allocation. It uses market intelligence data and channel benchmarks to optimize budget distribution across channels, projects impressions/clicks/conversions, and provides a weekly flight schedule with optimization checkpoints.

## Usage

```bash
aflare install marketing/media-plan
aflare run media-plan/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| campaign_name | Campaign name | Yes |
| budget | Total campaign budget | Yes |
| duration_weeks | Campaign duration in weeks | Yes |
| target_audience | Target audience description | Yes |
| target_audience_size | Estimated audience size | No |
| channels | List of channels to use | Yes |
| industry | Industry for market intel | Yes |
| period | Analysis period | Yes |

## Nodes Used

- agent - AI agent for media plan strategy generation
- http_request - Fetch market intelligence data
- code_interpreter - Python-based budget allocation and projection calculations
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing