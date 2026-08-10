# Classroom Management Assistant

Classroom management and behavior tracking with situation analysis, intervention strategies, positive reinforcement systems, and progress monitoring tools.

## Description

This workflow analyzes classroom behavior patterns, designs management strategies based on preferred approaches (positive, restorative, assertive, collaborative), and creates behavior tracking tools with reporting schedules for ongoing monitoring.

## Usage Example

```yaml
params:
  class_name: "Period 3 - Algebra I"
  grade_level: "9th Grade"
  class_size: 28
  behavior_data: "Several students are consistently off-task during independent work..."
  management_style: "positive"
  focus_areas: ["engagement", "disruption", "participation"]
  generate_plan: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| class_name | string | yes | - | Class name |
| grade_level | string | yes | - | Grade level |
| class_size | integer | no | 25 | Number of students |
| behavior_data | string | no | "" | Behavior observations |
| management_style | string | no | positive | Management approach |
| focus_areas | array | no | ["engagement","disruption","participation"] | Focus areas |
| generate_plan | boolean | no | true | Generate management plan |

## Nodes Used

- `agent` - Analyze situation and design strategies
- `code_interpreter` - Create behavior tracking tools
- `transform` - Format management plan
- `notify` - Send plan notification

## Category

Education