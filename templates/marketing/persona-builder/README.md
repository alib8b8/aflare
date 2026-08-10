# Buyer Persona Builder

> Research and generate buyer personas for targeted marketing

## Description

This workflow template creates detailed buyer personas by combining audience insights from professional networks with AI-powered profile generation. It covers demographics, professional background, goals, pain points, buying behavior, and communication preferences.

## Usage

```bash
aflare install marketing/persona-builder
aflare run persona-builder/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| persona_name | Name for the persona (e.g., "Marketing Mary") | Yes |
| industry | Target industry | Yes |
| job_title | Target job title | Yes |
| company_size | Target company size range | Yes |
| product | Your product or service | Yes |

## Nodes Used

- agent - AI agent for persona profile generation
- http_request - Fetch audience insights from professional platforms
- transform - Data structuring and JSON transformation
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing