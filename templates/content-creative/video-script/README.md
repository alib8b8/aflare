# Video Script Generator

> AI video script generator for YouTube/TikTok

## Description

This workflow template generates professional video scripts optimized for YouTube, TikTok, and other video platforms. It produces SEO-optimized titles, timestamped scripts with visual cues, hooks, CTAs, and production notes.

## Usage

```bash
aflare install creative/video-script
aflare run video-script/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| platform | Target platform (YouTube, TikTok, Instagram Reels) | Yes |
| topic | Video topic | Yes |
| audience | Target audience description | Yes |
| duration | Video duration in minutes | Yes |
| tone | Desired tone (educational, entertaining, etc.) | Yes |
| style | Creator style reference | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative