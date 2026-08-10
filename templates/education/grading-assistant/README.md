# Grading Assistant

Automated essay and assignment grading with rubric-based scoring, detailed feedback, and letter grade calculation.

## Description

This workflow evaluates student submissions against a customizable rubric, calculates scores automatically, generates letter grades, and provides detailed constructive feedback. It supports multiple academic levels from K-12 to graduate studies.

## Usage Example

```yaml
params:
  assignment_text: "The causes of the French Revolution include..."
  rubric: "Thesis clarity (20pts), Evidence use (25pts), Analysis quality (30pts), Organization (15pts), Grammar (10pts)"
  max_score: 100
  grade_level: "undergraduate"
  subject: "History"
  feedback_style: "constructive"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| assignment_text | string | yes | - | Student's assignment text |
| rubric | string | yes | - | Grading rubric criteria |
| max_score | integer | no | 100 | Maximum possible score |
| grade_level | string | no | undergraduate | Academic level |
| subject | string | yes | - | Subject area |
| feedback_style | string | no | constructive | Feedback tone |

## Nodes Used

- `agent` - Evaluate submission and generate feedback
- `code_interpreter` - Calculate scores and letter grades
- `file_write` - Save grading results to file

## Category

Education