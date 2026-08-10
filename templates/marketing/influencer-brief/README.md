# Influencer Campaign Brief

> Generate influencer marketing campaign briefs with profile matching

## Description

This workflow template creates comprehensive influencer marketing campaign briefs. It fetches influencer statistics from social platforms and generates detailed briefs covering content requirements, deliverables, compensation, compliance, and performance metrics.

## Usage

```bash
aflare install marketing/influencer-brief
aflare run influencer-brief/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand | Your brand name | Yes |
| campaign_goal | Primary campaign objective | Yes |
| budget | Campaign budget allocation | Yes |
| target_audience | Target audience description | Yes |
| influencer_handle | Influencer social handle | Yes |
| platform | Target platform (YouTube, Instagram, TikTok) | Yes |

## Nodes Used

- agent - AI agent for campaign brief generation
- http_request - Fetch influencer statistics from social platforms
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing