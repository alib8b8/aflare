# Training Needs Assessment

Skills gap analysis and training needs assessment with budget-aware planning.

## Description

Identify skill gaps by comparing employee skills against required competencies, prioritize training needs using severity scoring, and generate comprehensive training plans with budget allocation, quarterly calendars, and individual development paths.

## Usage Example

```yaml
params:
  department: "Engineering"
  employee_skills:
    - name: "Alice Wang"
      skills:
        - name: "Python"
          level: 4
        - name: "AWS"
          level: 3
        - name: "Kubernetes"
          level: 2
        - name: "Machine Learning"
          level: 2
    - name: "Bob Chen"
      skills:
        - name: "Python"
          level: 3
        - name: "AWS"
          level: 2
        - name: "Go"
          level: 4
        - name: "Terraform"
          level: 3
  required_competencies:
    - name: "Python"
      required_level: 4
      category: "Technical"
    - name: "Kubernetes"
      required_level: 4
      category: "Technical"
    - name: "Machine Learning"
      required_level: 3
      category: "Technical"
    - name: "System Design"
      required_level: 4
      category: "Architecture"
    - name: "Technical Leadership"
      required_level: 3
      category: "Leadership"
  strategic_priorities:
    - "AI/ML Platform Migration"
    - "Cloud Cost Optimization"
    - "Developer Productivity"
  budget_constraint: 50000
  assessment_method: "self_assessment"
  output_file: "engineering_training_plan.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| department | string | Yes | - | Department to assess |
| employee_skills | array | Yes | - | Current employee skills inventory |
| required_competencies | array | Yes | - | Required competencies for roles |
| strategic_priorities | array | No | [] | Company strategic priorities |
| budget_constraint | number | No | 0 | Annual training budget |
| assessment_method | string | No | self_assessment | self_assessment, manager, 360, skills_test |
| output_file | string | No | training_needs_plan.json | Output file path |

## Nodes Used

- **agent** (×2): Identify skill gaps and assess severity; generate comprehensive training plan
- **code_interpreter**: Prioritize training needs with severity scoring and budget-aware selection
- **file_write**: Save training needs plan as JSON

## Category

HR > Learning & Development