# Benefits Comparison Analyzer

Employee benefits comparison with total compensation modeling and personalized recommendations.

## Description

Compare multiple benefits packages with comprehensive cost modeling across health, retirement, PTO, equity, and perks. Generate personalized total compensation estimates, category winners, and negotiation-ready recommendations.

## Usage Example

```yaml
params:
  current_benefits:
    package_name: "Acme Corp Standard"
    health:
      employee_monthly_premium: 150
      annual_deductible: 2000
      oop_max: 6000
      coverage_type: "PPO"
    retirement:
      employer_match_pct: 50
      match_limit_pct: 6
      vesting_schedule: "4-year graded"
    time_off:
      pto_days: 20
      holidays: 11
      parental_leave_weeks: 12
    equity:
      type: "RSU"
      annual_value: 25000
      vesting_schedule: "4-year with 1-year cliff"
    perks:
      wellness_stipend: 600
      education_stipend: 2000
  comparison_benefits:
    - package_name: "TechCo Premium"
      health:
        employee_monthly_premium: 0
        annual_deductible: 500
        oop_max: 3000
      retirement:
        employer_match_pct: 100
        match_limit_pct: 6
      time_off:
        pto_days: 25
        holidays: 12
      equity:
        type: "ISO"
        annual_value: 40000
    - package_name: "StartupX Standard"
      health:
        employee_monthly_premium: 200
        annual_deductible: 3000
        oop_max: 8000
      retirement:
        employer_match_pct: 0
        match_limit_pct: 0
      time_off:
        pto_days: 15
        holidays: 10
      equity:
        type: "ISO"
        annual_value: 80000
  employee_profile:
    annual_salary: 150000
    dependents: 2
    age: 35
    career_stage: "mid-career"
    health_needs: "moderate"
  include_cost_analysis: true
  output_file: "benefits_comparison_report.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| current_benefits | object | Yes | - | Current benefits package |
| comparison_benefits | array | Yes | - | Benefits packages to compare |
| employee_profile | object | Yes | - | Employee profile for personalization |
| comparison_categories | array | No | [health, retirement, time_off, perks, insurance, equity] | Categories to compare |
| include_cost_analysis | boolean | No | true | Include out-of-pocket costs |
| currency | string | No | USD | Currency code |
| output_file | string | No | benefits_comparison_report.json | Output file path |

## Nodes Used

- **agent** (×2): Normalize benefits packages into consistent format; generate personalized recommendations
- **code_interpreter**: Compute total compensation, health costs, retirement value, and package differences
- **file_write**: Save comparison report as JSON

## Category

HR > Compensation & Benefits