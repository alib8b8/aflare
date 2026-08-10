# Podcast Outline Generator

> Podcast episode outline and show notes generator

## Description

This workflow template creates comprehensive podcast episode outlines with segment breakdowns, discussion questions, show notes, and promotion plans. Perfect for solo hosts and interview-based shows.

## Usage

```bash
aflare install creative/podcast-outline
aflare run podcast-outline/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| show_name | Name of the podcast | Yes |
| topic | Episode topic | Yes |
| episode_number | Episode number | No |
| guests | Guest name(s) | No |
| format | Episode format (solo, interview, panel) | Yes |
| duration | Target duration in minutes | Yes |
| audience | Target audience description | Yes |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative