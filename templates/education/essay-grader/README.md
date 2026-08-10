# Essay Grader

Essay grading with rubric-based scoring, weighted criteria, detailed feedback, and actionable improvement suggestions.

## Description

This workflow provides comprehensive essay grading using customizable rubric criteria with weights. It calculates weighted final scores, assigns letter grades, and generates detailed feedback with specific improvement suggestions organized by focus areas.

## Usage Example

```yaml
params:
  essay_text: "Throughout history, revolutions have shaped..."
  essay_prompt: "Discuss the causes and effects of the Industrial Revolution"
  rubric_criteria:
    - name: "Thesis"
      weight: 20
      description: "Clear and arguable thesis statement"
    - name: "Evidence"
      weight: 30
      description: "Use of supporting evidence and examples"
    - name: "Analysis"
      weight: 25
      description: "Depth of critical analysis"
    - name: "Organization"
      weight: 15
      description: "Logical structure and flow"
    - name: "Mechanics"
      weight: 10
      description: "Grammar, spelling, and formatting"
  grade_level: "high_school"
  word_limit: 1000
  focus_areas: ["argument", "evidence", "structure", "grammar"]
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| essay_text | string | yes | - | Essay to grade |
| essay_prompt | string | yes | - | Original essay prompt |
| rubric_criteria | array | yes | - | Rubric dimensions with weights |
| grade_level | string | no | high_school | Academic level |
| word_limit | integer | no | 0 | Expected word count |
| focus_areas | array | no | ["argument","evidence","structure","grammar"] | Focus areas |

## Nodes Used

- `agent` - Analyze essay against rubric and generate feedback
- `code_interpreter` - Calculate weighted scores and final grade
- `file_write` - Save grading results to file

## Category

Education