# Course Evaluation Analyzer

Course evaluation and feedback analysis with sentiment analysis, category breakdowns, and actionable improvement recommendations.

## Description

This workflow processes student course evaluations, performing sentiment analysis across customizable categories. It generates statistical summaries, identifies strengths and weaknesses, and provides prioritized improvement recommendations with implementation timelines.

## Usage Example

```yaml
params:
  course_name: "Introduction to Psychology"
  feedback_data: "The lectures were engaging and clear. The textbook was hard to follow..."
  evaluation_categories: ["teaching_quality", "course_content", "materials", "assessments", "engagement"]
  rating_scale: 5
  include_recommendations: true
  semester: "Fall 2026"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| course_name | string | yes | - | Course name |
| feedback_data | string | yes | - | Raw student feedback |
| evaluation_categories | array | no | ["teaching_quality","course_content","materials","assessments","engagement"] | Evaluation categories |
| rating_scale | integer | no | 5 | Rating scale |
| include_recommendations | boolean | no | true | Generate recommendations |
| semester | string | no | "" | Academic term |

## Nodes Used

- `agent` - Analyze feedback and generate recommendations
- `code_interpreter` - Compute sentiment statistics
- `notify` - Send evaluation results notification

## Category

Education