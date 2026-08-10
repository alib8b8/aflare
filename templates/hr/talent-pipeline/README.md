# Talent Pipeline Analytics

Recruitment funnel analytics with conversion metrics, bottleneck detection, and hiring forecasts.

## Description

Analyze recruitment funnel performance with stage-by-stage conversion metrics, source effectiveness comparison, bottleneck identification, and hiring forecasts based on active pipeline. Helps optimize recruitment processes and predict hiring outcomes.

## Usage Example

```yaml
params:
  pipeline_data:
    sourced: 500
    applied: 320
    screened: 180
    phone_interview: 95
    onsite: 42
    offer: 15
    accepted: 11
    hired: 10
    avg_time_to_fill_days: 52
    total_recruitment_cost: 250000
    time_in_stage:
      sourced_to_applied: 5
      applied_to_screened: 3
      screened_to_phone: 7
      phone_to_onsite: 10
      onsite_to_offer: 8
      offer_to_accepted: 5
      accepted_to_hired: 14
    source_breakdown:
      linkedin:
        applied: 120
        hired: 4
        total_cost: 30000
      indeed:
        applied: 80
        hired: 2
        total_cost: 15000
      referral:
        applied: 40
        hired: 3
        total_cost: 15000
      direct:
        applied: 80
        hired: 1
        total_cost: 5000
    active_candidates:
      "Senior Engineer":
        onsite: 3
        phone_interview: 8
        screened: 15
  time_period:
    start: "2026-01-01"
    end: "2026-06-30"
  roles:
    - title: "Senior Engineer"
      open_positions: 4
    - title: "Product Manager"
      open_positions: 2
  target_fill_days: 45
  output_file: "talent_pipeline_h1_2026.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| pipeline_data | object | Yes | - | Recruitment funnel data by stage |
| time_period | object | Yes | - | Analysis time period |
| roles | array | Yes | - | Roles being recruited |
| source_channels | array | No | [] | Recruitment source channels |
| target_fill_days | integer | No | 45 | Target time-to-fill |
| cost_per_stage | object | No | {} | Cost per stage |
| output_file | string | No | talent_pipeline_report.json | Output file path |

## Nodes Used

- **code_interpreter** (×2): Compute funnel metrics, conversion rates, and source effectiveness; generate hiring forecasts from active pipeline
- **agent**: Identify bottlenecks and optimization opportunities with impact analysis
- **file_write**: Save pipeline analytics report as JSON

## Category

HR > Recruitment