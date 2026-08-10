# Lead Scoring & Qualification

> Score and qualify leads for sales prioritization

## Description

This workflow template scores leads using a multi-dimensional model covering demographic, behavioral, and firmographic factors. It fetches lead data from CRM, applies Python-based scoring algorithms, and generates tiered qualification reports with actionable sales handoff recommendations.

## Usage

```bash
aflare install marketing/lead-scoring
aflare run lead-scoring/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| campaign_source | Campaign or source of leads | Yes |

## Nodes Used

- agent - AI agent for qualification analysis and recommendations
- http_request - Fetch lead data from CRM
- code_interpreter - Python-based multi-factor lead scoring model
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing