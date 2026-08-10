# DEI Report Generator

Diversity, equity, and inclusion report generator with comprehensive metrics analysis.

## Description

Generate professional DEI reports with workforce demographic analysis, leadership representation gaps, pay equity metrics, hiring funnel diversity, and promotion data. Includes actionable goal setting and initiative tracking.

## Usage Example

```yaml
params:
  company_name: "Acme Corp"
  workforce_demographics:
    total_employees: 500
    total_leadership: 75
    gender:
      Men: 280
      Women: 200
      Non-Binary: 15
      Not_Disclosed: 5
    ethnicity:
      White: 260
      Asian: 120
      Black: 55
      Hispanic: 40
      Two_or_More: 15
      Other: 10
    leadership:
      gender:
        Men: 52
        Women: 20
        Non-Binary: 2
        Not_Disclosed: 1
      ethnicity:
        White: 50
        Asian: 15
        Black: 5
        Hispanic: 3
        Two_or_More: 1
        Other: 1
    pay_data:
      by_gender:
        Men:
          avg_salary: 125000
          median_salary: 120000
          count: 280
        Women:
          avg_salary: 112000
          median_salary: 108000
          count: 200
  reporting_period:
    start: "2026-01-01"
    end: "2026-12-31"
  comparison_year: 2025
  include_pay_equity: true
  include_hiring_funnel: true
  include_promotion_data: true
  dei_initiatives:
    - "Employee Resource Groups (ERGs) launched: Women in Tech, Pride@Acme"
    - "Unconscious bias training completed by 95% of managers"
    - "Rooney Rule implemented for leadership roles"
  output_file: "dei_report_2026.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| workforce_demographics | object | Yes | - | Demographic data by category |
| reporting_period | object | Yes | - | Reporting period dates |
| company_name | string | Yes | - | Company name |
| comparison_year | integer | No | 0 | Previous year for comparison |
| include_pay_equity | boolean | No | true | Include pay equity analysis |
| include_hiring_funnel | boolean | No | true | Include hiring funnel data |
| include_promotion_data | boolean | No | true | Include promotion data |
| dei_initiatives | array | No | [] | Current DEI initiatives |
| output_file | string | No | dei_report.md | Output file path |

## Nodes Used

- **code_interpreter** (×2): Compute representation metrics and leadership gaps; calculate pay equity gaps
- **agent**: Generate comprehensive DEI report with narrative and action plan
- **file_write**: Save DEI report as markdown

## Category

HR > DEI & Compliance