# Quiz Generator

Auto-generate quizzes with multiple question types including multiple choice, true/false, and short answer from any topic or source material.

## Description

This workflow analyzes source material or a topic description, extracts key concepts, and generates a structured quiz with various question types. It supports configurable difficulty levels and automatically calculates quiz metadata including time estimates and scoring.

## Usage Example

```yaml
params:
  topic: "World War II Causes"
  question_count: 15
  question_types: ["multiple_choice", "true_false", "short_answer"]
  difficulty: "medium"
  source_text: "World War II was caused by..."
  output_format: "json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| topic | string | yes | - | Subject or topic for the quiz |
| question_count | integer | no | 10 | Number of questions to generate |
| question_types | array | no | ["multiple_choice","true_false","short_answer"] | Types of questions |
| difficulty | string | no | medium | Difficulty level (easy/medium/hard) |
| source_text | string | no | "" | Optional source material |
| output_format | string | no | json | Output format (json/markdown/pdf) |

## Nodes Used

- `agent` - Analyze content and generate questions using AI
- `code_interpreter` - Calculate quiz metadata and statistics
- `transform` - Format quiz into structured output
- `file_write` - Save quiz to file

## Category

Education