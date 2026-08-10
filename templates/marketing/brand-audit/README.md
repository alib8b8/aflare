# Brand Health Audit

> Audit brand health and competitive positioning

## Description

This workflow template performs comprehensive brand health audits by analyzing brand identity, competitive positioning, digital presence, and customer sentiment. It fetches live data from review platforms and the brand's website to ground analysis in real-world data.

## Usage

```bash
aflare install marketing/brand-audit
aflare run brand-audit/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| brand_name | Name of the brand to audit | Yes |
| brand_website | Brand website URL | Yes |
| industry | Industry category | Yes |
| competitors | Comma-separated list of competitors | Yes |

## Nodes Used

- agent - AI agent for brand analysis and recommendations
- http_request - Fetch brand website and review data
- json_parse - Parse JSON responses
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

marketing