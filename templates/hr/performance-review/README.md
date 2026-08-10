# Performance Review Generator

Comprehensive performance review document generator with goal tracking and competency assessment.

## Description

Generate professional performance review documents with automated goal achievement scoring, multi-source (self/manager/peer) competency assessment, and development planning. Supports annual, semi-annual, quarterly, and probation review types.

## Usage Example

```yaml
params:
  employee_name: "David Kim"
  employee_title: "Product Manager"
  department: "Product"
  review_period:
    start: "2026-01-01"
    end: "2026-06-30"
  review_type: "semi-annual"
  manager_name: "Lisa Thompson"
  goals:
    - name: "Launch mobile app v2"
      weight: 30
      target: 100
      actual: 95
    - name: "Increase user retention by 15%"
      weight: 25
      target: 15
      actual: 18
    - name: "Reduce churn rate to under 3%"
      weight: 20
      target: 3
      actual: 3.2
    - name: "Mentor 2 junior PMs"
      weight: 15
      target: 2
      actual: 2
    - name: "Complete OKR framework migration"
      weight: 10
      target: 100
      actual: 100
  competencies:
    - name: "Strategic Thinking"
      self_rating: 4
      manager_rating: 3.5
      peer_ratings: [4, 4, 3.5]
    - name: "Communication"
      self_rating: 4.5
      manager_rating: 4
      peer_ratings: [4, 5, 4]
    - name: "Execution"
      self_rating: 3.5
      manager_rating: 4
      peer_ratings: [4, 3.5, 4]
  self_assessment: "I believe I made strong progress on..."
  output_file: "review_david_kim.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| employee_name | string | Yes | - | Employee full name |
| employee_title | string | Yes | - | Current job title |
| department | string | Yes | - | Department name |
| review_period | object | Yes | - | Review start and end dates |
| goals | array | Yes | - | Goals with target, actual, weight |
| manager_name | string | Yes | - | Reviewing manager |
| review_type | string | No | annual | annual, semi-annual, quarterly, probation |
| competencies | array | No | [] | Competency ratings |
| self_assessment | string | No | "" | Employee self-assessment |
| output_file | string | No | performance_review_{name}.md | Output file path |

## Nodes Used

- **code_interpreter** (×2): Calculate goal achievement scores and weighted ratings; compute composite competency scores
- **agent**: Generate comprehensive review document with all sections
- **file_write**: Save review as markdown document

## Category

HR > Performance Management