# Employee Engagement Survey

End-to-end employee engagement survey creation, distribution, and analysis workflow.

## Description

Generate comprehensive employee engagement surveys with AI-crafted questions, distribute them to targeted audiences, analyze responses with sentiment analysis, and produce actionable insights with trend comparison against historical data.

## Usage Example

```yaml
params:
  survey_title: "Q3 2026 Employee Engagement Pulse"
  survey_focus:
    - engagement
    - satisfaction
    - culture
    - leadership
    - well-being
  target_audience:
    departments: ["Engineering", "Product", "Design"]
    all: false
  anonymous_mode: true
  survey_duration_days: 14
  custom_questions:
    - question: "How satisfied are you with the new hybrid work policy?"
      type: "likert_5"
      category: "workplace"
    - question: "What would make your work experience better?"
      type: "open_ended"
      category: "general"
      required: false
  previous_survey_data: "q2_2026_survey_results.json"
  output_file: "q3_2026_engagement_report.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| survey_title | string | Yes | - | Title of the survey |
| survey_focus | array | No | [engagement, satisfaction, culture] | Focus areas |
| target_audience | object | No | {all: true} | Audience filters |
| anonymous_mode | boolean | No | true | Anonymous responses |
| custom_questions | array | No | [] | Custom questions to append |
| survey_duration_days | integer | No | 14 | Duration in days |
| previous_survey_data | string | No | "" | Previous survey for trend analysis |
| output_file | string | No | employee_survey_report.json | Output file path |

## Nodes Used

- **agent** (×2): Generate survey questions by focus area; analyze responses with sentiment analysis
- **http_request**: Distribute survey via external survey platform API
- **code_interpreter**: Build executive summary and score aggregations
- **file_write**: Save final analysis report as JSON

## Category

HR > Employee Experience