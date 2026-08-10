# Peer Review Workflow

Peer review workflow and rubric for structured student-to-student feedback with anonymous option, criteria-based scoring, and actionable suggestions.

## Description

This workflow facilitates structured peer review by analyzing submissions against criteria, generating constructive feedback, and calculating review scores. It supports anonymous reviews and provides specific evidence-based suggestions for improvement.

## Usage Example

```yaml
params:
  assignment_type: "Research Paper Draft"
  review_criteria: ["Thesis clarity", "Evidence quality", "Organization", "Writing style", "Citations"]
  submission_text: "This paper examines the impact of climate change..."
  reviewer_name: "Jane Smith"
  review_guidelines: "Be constructive, specific, and respectful"
  anonymous: false
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| assignment_type | string | yes | - | Type of assignment |
| review_criteria | array | yes | - | Review criteria |
| submission_text | string | yes | - | Student work to review |
| reviewer_name | string | no | "" | Reviewer name |
| review_guidelines | string | no | "Be constructive, specific, and respectful" | Review guidelines |
| anonymous | boolean | no | true | Anonymous review |

## Nodes Used

- `agent` - Analyze submission and generate peer review
- `code_interpreter` - Calculate review scores and statistics
- `file_write` - Save peer review to file

## Category

Education