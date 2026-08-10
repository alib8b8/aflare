# Employee Handbook Generator

Comprehensive employee handbook generator with compliance-aware policy creation.

## Description

Generate professional employee handbooks with jurisdiction-specific compliance requirements, customizable policy sections, company culture integration, and acknowledgment workflows. Fetches legal requirements automatically based on location and industry.

## Usage Example

```yaml
params:
  company_name: "Acme Corp"
  company_info:
    founded: 2019
    industry: "Technology"
    size: 250
    locations: ["San Francisco, CA", "New York, NY", "Remote"]
  policies:
    - "employment_basics"
    - "compensation_benefits"
    - "time_off_leave"
    - "work_hours_attendance"
    - "code_of_conduct"
    - "technology_data"
    - "health_safety"
    - "performance_development"
    - "separation"
  jurisdiction:
    country: "US"
    state: "California"
    localities: ["San Francisco"]
  employment_type: "at-will"
  work_model: "hybrid"
  company_culture: "We are a mission-driven team that values transparency, innovation, and work-life balance. Our culture is built on trust, autonomy, and continuous learning."
  custom_policies:
    - title: "Unlimited PTO Policy"
      content: "Acme offers unlimited paid time off..."
    - title: "Home Office Stipend"
      content: "All employees receive a $2,000 annual home office stipend..."
  output_file: "acme_employee_handbook.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| company_name | string | Yes | - | Company legal name |
| company_info | object | Yes | - | Company details |
| policies | array | Yes | - | Policy sections to include |
| jurisdiction | object | Yes | - | Legal jurisdiction details |
| employment_type | string | No | at-will | Employment type |
| work_model | string | No | hybrid | Office, remote, hybrid |
| company_culture | string | No | "" | Culture and values |
| custom_policies | array | No | [] | Custom policy content |
| output_file | string | No | employee_handbook.md | Output file path |

## Nodes Used

- **http_request**: Fetch jurisdiction-specific compliance requirements
- **agent**: Generate comprehensive handbook with all policy sections
- **code_interpreter**: Generate table of contents and document statistics
- **file_write**: Save formatted handbook with acknowledgment section

## Category

HR > Compliance & Documentation