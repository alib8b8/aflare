# Social Media Caption Generator

> Social media caption generator for Instagram/LinkedIn/Twitter

## Description

This workflow template generates platform-optimized social media captions with multiple variations, hashtag strategies, and engagement tactics. Supports Instagram, LinkedIn, Twitter/X, Facebook, and TikTok.

## Usage

```bash
aflare install creative/social-caption
aflare run social-caption/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| platform | Target platform (Instagram, LinkedIn, Twitter, etc.) | Yes |
| content_type | Type of content (photo, carousel, reel, poll, etc.) | Yes |
| topic | Topic or theme of the post | Yes |
| brand_voice | Brand voice and tone | Yes |
| goal | Primary goal (engagement, traffic, awareness, etc.) | Yes |
| audience | Target audience description | Yes |
| hashtag_strategy | Hashtag approach (broad, niche, mixed) | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative