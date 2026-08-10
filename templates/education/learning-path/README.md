# Learning Path Designer

Personalized learning path designer with adaptive milestones, resource recommendations, and progress tracking based on learning style preferences.

## Description

This workflow creates customized learning paths by analyzing the gap between current and target proficiency levels. It generates week-by-week plans with specific resources, practice exercises, checkpoint assessments, and milestone achievements tailored to the learner's preferred style.

## Usage Example

```yaml
params:
  subject: "Python Programming"
  current_level: "beginner"
  target_level: "intermediate"
  timeframe_weeks: 12
  hours_per_week: 5
  learning_style: "mixed"
  resources_preference: ["videos", "books", "exercises", "projects"]
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| subject | string | yes | - | Subject to learn |
| current_level | string | yes | - | Current proficiency |
| target_level | string | yes | - | Desired proficiency |
| timeframe_weeks | integer | no | 12 | Timeframe in weeks |
| hours_per_week | integer | no | 5 | Study hours per week |
| learning_style | string | no | mixed | Preferred learning style |
| resources_preference | array | no | ["videos","books","exercises","projects"] | Preferred resources |

## Nodes Used

- `agent` - Analyze learning gap and design path
- `code_interpreter` - Calculate progress metrics and milestones
- `file_write` - Save learning path to file

## Category

Education