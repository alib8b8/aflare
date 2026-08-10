# Succession Planning

Succession planning and talent readiness assessment for critical role continuity.

## Description

Assess talent readiness for critical roles, build succession matrices with coverage ratios, identify vacancy risks, and generate targeted development plans for high-potential successors. Supports immediate, 1-2 year, and 3-5 year planning horizons.

## Usage Example

```yaml
params:
  critical_roles:
    - title: "VP Engineering"
      incumbent: "Sarah Chen"
      department: "Engineering"
      criticality: "Critical"
    - title: "Head of Product"
      incumbent: "Mike Torres"
      department: "Product"
      criticality: "High"
    - title: "Director of Sales"
      incumbent: "James Wilson"
      department: "Sales"
      criticality: "High"
  talent_pool:
    - name: "Alice Wang"
      current_role: "Senior Engineering Manager"
      performance_rating: "Exceeds"
      potential_rating: "High"
      years_in_role: 3
      skills: ["Technical Leadership", "Team Building", "Architecture"]
    - name: "Bob Martinez"
      current_role: "Engineering Manager"
      performance_rating: "Meets"
      potential_rating: "Medium"
      years_in_role: 2
      skills: ["People Management", "Agile", "Cloud Infrastructure"]
  time_horizon: "1-2_years"
  risk_tolerance: "moderate"
  output_file: "succession_plan_q3.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| critical_roles | array | Yes | - | Critical roles with incumbent and criticality |
| talent_pool | array | Yes | - | Internal talent with skills, performance, potential |
| time_horizon | string | No | immediate | immediate, 1-2_years, 3-5_years |
| organization | object | No | {} | Organization context |
| risk_tolerance | string | No | moderate | low, moderate, high |
| output_file | string | No | succession_plan.json | Output file path |

## Nodes Used

- **agent** (×2): Assess readiness of potential successors for each role; generate development plans for high-potential talent
- **code_interpreter**: Build succession matrix with coverage ratios and vacancy risk distribution
- **file_write**: Save succession plan as JSON

## Category

HR > Talent Management