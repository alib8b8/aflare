# Lesson Plan Generator

Detailed lesson plan generator with learning objectives, timed activities, differentiation strategies, and standards alignment.

## Description

This workflow creates comprehensive lesson plans following the standard instructional model (hook, direct instruction, guided practice, independent practice, closure). It includes timing breakdowns, differentiation strategies, and formative assessment checkpoints.

## Usage Example

```yaml
params:
  lesson_title: "Introduction to Fractions"
  subject: "Mathematics"
  grade_level: "4th Grade"
  duration_minutes: 60
  learning_objectives: ["Define fractions", "Identify numerator and denominator", "Represent fractions visually"]
  materials_needed: ["Fraction strips", "Whiteboard", "Worksheets"]
  standards: ["CCSS.MATH.CONTENT.4.NF.A.1"]
  accommodations: ["Visual aids", "Extended time", "Peer tutoring"]
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| lesson_title | string | yes | - | Lesson title |
| subject | string | yes | - | Subject area |
| grade_level | string | yes | - | Grade/academic level |
| duration_minutes | integer | no | 60 | Lesson duration |
| learning_objectives | array | yes | - | Learning objectives |
| materials_needed | array | no | [] | Required materials |
| standards | array | no | [] | Curriculum standards |
| accommodations | array | no | [] | Differentiation accommodations |

## Nodes Used

- `agent` - Design lesson content and activities
- `code_interpreter` - Calculate timing breakdowns
- `transform` - Format lesson plan into structured output
- `file_write` - Save lesson plan to file

## Category

Education