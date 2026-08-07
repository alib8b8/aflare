# Travel Itinerary Planner

> Generate detailed travel itinerary

## Description

This workflow template provides a ready-to-use solution for generate detailed travel itinerary.

## Usage

```bash
aflare install travel/travel-itinerary
aflare run travel-itinerary/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| - | Check workflow.yaml for configurable parameters | - |

## Nodes Used

- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications
- http_request - Make HTTP requests (when applicable)
- json_parse - Parse JSON responses (when applicable)
- execute - Execute shell commands (when applicable)

## Category

travel
