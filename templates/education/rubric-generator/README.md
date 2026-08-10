# Rubric Generator

Assessment rubric creator with customizable criteria, multiple performance levels, weighted scoring, and example evidence descriptors.

## Description

This workflow generates professional assessment rubrics for any assignment type. It creates clear, observable descriptors for each performance level, calculates equal-weight scoring schemes, and includes example evidence to guide consistent evaluation.

## Usage Example

```yaml
params:
  assignment_type: "Research Paper"
  criteria: ["Thesis Statement", "Research Quality", "Analysis", "Organization", "Citations"]
  levels: 4
  level_labels: ["Excellent", "Proficient", "Developing", "Beginning"]
  max_score: 100
  subject: "History"
  include_examples: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| assignment_type | string | yes | - | Type of assignment |
| criteria | array | yes | - | Assessment criteria |
| levels | integer | no | 4 | Number of performance levels |
| level_labels | array | no | ["Excellent","Proficient","Developing","Beginning"] | Level labels |
| max_score | integer | no | 100 | Maximum total score |
| subject | string | no | "" | Subject area |
| include_examples | boolean | no | true | Include example evidence |

## Nodes Used

- `agent` - Design rubric with descriptors for each level
- `code_interpreter` - Calculate scoring weights and point distributions
- `transform` - Format rubric into structured table
- `file_write` - Save rubric to file

## Category

Education