# Exit Interview Analysis

Comprehensive exit interview analysis with turnover pattern detection and retention insights.

## Description

Analyze exit interview data to identify turnover patterns, extract themes using sentiment analysis, detect early warning signals, and generate actionable retention recommendations with stakeholder notifications.

## Usage Example

```yaml
params:
  exit_interviews:
    - employee_id: "E001"
      department: "Engineering"
      exit_date: "2026-06-15"
      exit_type: "voluntary"
      tenure_months: 18
      primary_reason: "Career growth"
      interview_notes: "I felt there was no clear path to senior..."
      manager: "John Smith"
      compensation_satisfaction: 2
      culture_satisfaction: 3
      would_recommend: false
    - employee_id: "E002"
      department: "Engineering"
      exit_date: "2026-07-01"
      exit_type: "voluntary"
      tenure_months: 24
      primary_reason: "Compensation"
      interview_notes: "Got an offer for 30% more..."
      manager: "John Smith"
      compensation_satisfaction: 1
      culture_satisfaction: 4
      would_recommend: true
  analysis_period:
    start: "2026-01-01"
    end: "2026-08-01"
  department_filter: "Engineering"
  min_tenure_months: 3
  include_historical: true
  trigger_alerts: true
  notification_channel: "slack"
  notification_recipients: "#hr-leadership"
  output_file: "exit_interview_analysis_h1.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| exit_interviews | array | Yes | - | Collection of exit interview data |
| analysis_period | object | Yes | - | Date range for analysis |
| department_filter | string | No | "" | Filter to specific department |
| min_tenure_months | integer | No | 0 | Minimum tenure for inclusion |
| include_historical | boolean | No | true | Include historical data |
| trigger_alerts | boolean | No | true | Trigger alerts for patterns |
| notification_channel | string | No | email | Notification channel |
| notification_recipients | string | No | hr-leadership@company.com | Alert recipients |
| output_file | string | No | exit_interview_report.json | Output file path |

## Nodes Used

- **code_interpreter**: Preprocess and filter exit interview data with statistical summaries
- **agent** (×2): Analyze themes, sentiment, and patterns; generate insights and recommendations
- **notify**: Send alerts and summary to stakeholders
- **file_write**: Save comprehensive analysis report as JSON

## Category

HR > Employee Experience