# Creative Brief Generator

> Creative project brief template generator

## Description

This workflow template generates comprehensive creative briefs for marketing and design projects. Includes business context, audience analysis, creative direction, deliverables, timelines, and budget breakdowns. Ideal for agencies and in-house creative teams.

## Usage

```bash
aflare install creative/creative-brief
aflare run creative-brief/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| project_name | Project name | Yes |
| project_type | Type of project (campaign, rebrand, launch, etc.) | Yes |
| client | Client or brand name | Yes |
| campaign | Campaign or initiative name | Yes |
| objective | Primary project objective | Yes |
| audience | Target audience description | Yes |
| budget | Budget allocation | No |
| timeline | Project timeline | Yes |
| deliverables | List of expected deliverables | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative