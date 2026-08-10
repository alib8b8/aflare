# Offer Letter Generator

AI-powered offer letter generation with compensation details and customizable templates.

## Description

Generate professional offer letters with validated compensation data, integrated benefits information, and support for multiple template styles (standard, executive, startup). Includes automatic compensation validation and total comp calculation.

## Usage Example

```yaml
params:
  candidate_name: "Jane Smith"
  position_title: "Senior Product Manager"
  department: "Product"
  start_date: "2026-03-15"
  company_name: "Acme Corp Inc."
  company_address: "123 Main St, San Francisco, CA 94105"
  template_style: "standard"
  compensation:
    base_salary: 175000
    target_bonus_pct: 15
    equity_grant:
      shares: 10000
      vesting_schedule: "4-year with 1-year cliff"
    signing_bonus: 15000
  special_terms:
    - "Relocation assistance up to $15,000"
    - "Subject to background check completion"
  output_file: "offer_letter_jane_smith.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| candidate_name | string | Yes | - | Full name of the candidate |
| position_title | string | Yes | - | Job title being offered |
| department | string | Yes | - | Department name |
| start_date | string | Yes | - | Proposed start date (YYYY-MM-DD) |
| compensation | object | Yes | - | Compensation details (base, bonus, equity, signing) |
| company_name | string | Yes | - | Company legal name |
| company_address | string | No | "" | Company address |
| special_terms | array | No | [] | Additional special terms |
| template_style | string | No | standard | standard, executive, or startup |
| output_file | string | No | offer_letter_{name}.md | Output file path |

## Nodes Used

- **code_interpreter**: Validate compensation data and calculate total annual comp
- **http_request**: Fetch current benefits data from internal API
- **agent**: Generate professional offer letter with all sections
- **file_write**: Save offer letter as markdown document

## Category

HR > Hiring & Onboarding