# Onboarding Checklist Generator

Role-based new hire onboarding checklist generator with automated timeline and task assignment.

## Description

Generate comprehensive onboarding checklists tailored to the new hire's role type, seniority level, department, IT requirements, and compliance needs. Covers pre-boarding through 90-day milestones with task ownership, priorities, and automated timeline calculation.

## Usage Example

```yaml
params:
  employee_name: "Michael Chen"
  job_title: "Senior Backend Engineer"
  department: "Engineering"
  start_date: "2026-04-01"
  manager_name: "Sarah Johnson"
  role_type: "hybrid"
  seniority_level: "senior"
  it_requirements:
    - "MacBook Pro 16\""
    - "AWS IAM access"
    - "GitHub Enterprise admin"
    - "VPN access"
  compliance_required:
    - "GDPR training"
    - "SOC2 awareness"
    - "Code of conduct"
  output_file: "onboarding_michael_chen.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| employee_name | string | Yes | - | New hire full name |
| job_title | string | Yes | - | Job title |
| department | string | Yes | - | Department name |
| start_date | string | Yes | - | Start date (YYYY-MM-DD) |
| manager_name | string | Yes | - | Reporting manager |
| role_type | string | No | office | office, remote, hybrid, contractor |
| seniority_level | string | No | mid | entry, mid, senior, executive |
| it_requirements | array | No | [] | IT access and equipment needed |
| compliance_required | array | No | [] | Compliance training requirements |
| output_file | string | No | onboarding_{name}.md | Output file path |

## Nodes Used

- **agent**: Generate role-tailored checklist with tasks, owners, priorities, and categories
- **code_interpreter**: Calculate timeline milestones and summary statistics
- **http_request**: Create pre-boarding tasks in external task management system
- **file_write**: Save formatted onboarding checklist as markdown

## Category

HR > Onboarding