# Presentation Outline Generator

> Presentation slide deck outline generator

## Description

This workflow template creates comprehensive presentation outlines with slide-by-slide content, speaker notes, visual suggestions, storytelling elements, delivery notes, and handout materials. Suitable for pitch decks, keynotes, webinars, and training presentations.

## Usage

```bash
aflare install creative/presentation-outline
aflare run presentation-outline/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| presentation_title | Title of the presentation | Yes |
| presentation_type | Type (pitch, keynote, webinar, training, etc.) | Yes |
| audience | Target audience description | Yes |
| duration | Presentation duration in minutes | Yes |
| objective | Primary objective of the presentation | Yes |
| key_message | Core message to convey | Yes |
| presenter_style | Presenter's speaking style | No |
| visual_style | Preferred visual style | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative