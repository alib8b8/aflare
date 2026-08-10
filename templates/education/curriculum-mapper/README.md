# Curriculum Mapper

Curriculum mapping and standards alignment tool that maps course units to educational standards frameworks with coverage analysis.

## Description

This workflow creates comprehensive curriculum maps by aligning course units to educational standards frameworks such as Common Core, NGSS, or IB. It analyzes coverage completeness, identifies gaps, and includes 21st century skills and cross-curricular connections.

## Usage Example

```yaml
params:
  course_name: "Biology"
  standards_framework: "NGSS"
  units: ["Cell Biology", "Genetics", "Evolution", "Ecology", "Human Body Systems"]
  grade_level: "10th Grade"
  subject: "Science"
  assessment_types: ["formative", "summative", "performance"]
  include_skills: true
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| course_name | string | yes | - | Course name |
| standards_framework | string | yes | - | Standards framework |
| units | array | yes | - | Course units/modules |
| grade_level | string | yes | - | Target grade level |
| subject | string | yes | - | Subject area |
| assessment_types | array | no | ["formative","summative","performance"] | Assessment types |
| include_skills | boolean | no | true | Include 21st century skills |

## Nodes Used

- `http_request` - Fetch standards framework references
- `agent` - Map curriculum to standards
- `code_interpreter` - Analyze standards coverage
- `transform` - Format curriculum map
- `file_write` - Save map to file

## Category

Education