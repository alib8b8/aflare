# Interview Scorecard

Structured interview scorecard and evaluation workflow with evidence-based scoring.

## Description

Generate structured interview scorecards by parsing interview notes, extracting evidence for each criterion, computing weighted scores, and producing professional summaries with hire/no-hire recommendations. Supports all interview types.

## Usage Example

```yaml
params:
  candidate_name: "Priya Patel"
  position_title: "Staff Software Engineer"
  interview_type: "technical"
  interviewer_name: "Alex Rivera"
  interview_duration_minutes: 60
  evaluation_criteria:
    - name: "System Design"
      weight: 30
      max_score: 5
      score: 4
    - name: "Coding & Algorithms"
      weight: 25
      max_score: 5
      score: 5
    - name: "Problem Solving"
      weight: 20
      max_score: 5
      score: 4
    - name: "Communication"
      weight: 15
      max_score: 5
      score: 3
    - name: "Culture Fit"
      weight: 10
      max_score: 5
      score: 4
  interview_notes: |
    Candidate demonstrated strong system design skills...
    Coding exercise was completed efficiently with clean code...
    Good communication but sometimes jumped to conclusions...
  output_file: "scorecard_priya_patel.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| candidate_name | string | Yes | - | Candidate full name |
| position_title | string | Yes | - | Position title |
| interview_type | string | Yes | - | phone_screen, technical, behavioral, panel, final |
| interviewer_name | string | Yes | - | Interviewer name |
| evaluation_criteria | array | Yes | - | Criteria with weight, max_score, score |
| interview_notes | string | No | "" | Raw interview notes |
| interview_duration_minutes | integer | No | 60 | Interview duration |
| output_file | string | No | scorecard_{name}.md | Output file path |

## Nodes Used

- **agent** (×2): Parse notes and extract structured evidence per criterion; generate professional interview summary
- **code_interpreter**: Compute weighted scores and overall decision classification
- **file_write**: Save formatted scorecard as markdown

## Category

HR > Recruitment