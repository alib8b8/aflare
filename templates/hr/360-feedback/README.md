# 360-Degree Feedback

Multi-source feedback collection, aggregation, and analysis with self-awareness insights.

## Description

Collect and aggregate 360-degree feedback from managers, peers, direct reports, and self-assessments. Identify strengths, development areas, blind spots, and hidden strengths with competency-level scoring and professional development reporting.

## Usage Example

```yaml
params:
  subject_name: "Jennifer Lee"
  subject_title: "Engineering Manager"
  feedback_period:
    start: "2026-01-01"
    end: "2026-06-30"
  reviewers:
    - name: "Self"
      relationship: "self"
      email: "jennifer@company.com"
    - name: "David Chen"
      relationship: "manager"
      email: "david@company.com"
    - name: "Peer A"
      relationship: "peer"
      email: "peera@company.com"
    - name: "Peer B"
      relationship: "peer"
      email: "peerb@company.com"
    - name: "Report 1"
      relationship: "direct_report"
      email: "report1@company.com"
    - name: "Report 2"
      relationship: "direct_report"
      email: "report2@company.com"
    - name: "Report 3"
      relationship: "direct_report"
      email: "report3@company.com"
  competency_model:
    - name: "Technical Leadership"
      description: "Ability to guide technical decisions and architecture"
      rating_scale: 5
    - name: "People Management"
      description: "Effectiveness in managing, coaching, and developing team"
      rating_scale: 5
    - name: "Communication"
      description: "Clarity, frequency, and effectiveness of communication"
      rating_scale: 5
    - name: "Strategic Thinking"
      description: "Ability to think long-term and align with business goals"
      rating_scale: 5
    - name: "Collaboration"
      description: "Cross-functional partnership and teamwork"
      rating_scale: 5
    - name: "Execution"
      description: "Ability to deliver results and meet commitments"
      rating_scale: 5
  anonymize_peers: true
  include_self_assessment: true
  minimum_reviewers: 3
  output_file: "360_feedback_jennifer_lee.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| subject_name | string | Yes | - | Person receiving feedback |
| subject_title | string | Yes | - | Job title |
| feedback_period | object | Yes | - | Feedback collection period |
| reviewers | array | Yes | - | Reviewers with relationship types |
| competency_model | array | Yes | - | Competency framework with scales |
| anonymize_peers | boolean | No | true | Anonymize peer/report feedback |
| include_self_assessment | boolean | No | true | Include self-assessment |
| minimum_reviewers | integer | No | 3 | Minimum per category |
| output_file | string | No | 360_feedback_{name}.md | Output file path |

## Nodes Used

- **http_request**: Send feedback requests and collect responses
- **code_interpreter**: Aggregate multi-source scores, compute gaps, identify blind spots and hidden strengths
- **agent**: Generate comprehensive feedback report with development recommendations
- **file_write**: Save feedback report as markdown

## Category

HR > Performance Management