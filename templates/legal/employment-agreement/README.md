# Employment Contract Generator

## Description
A jurisdiction-aware employment contract generator that creates customized employment agreements. The workflow first checks applicable labor laws for the specified jurisdiction, generates a complete contract with role-specific clauses, and validates the contract for legal compliance and enforceability.

## Usage Example
```yaml
workflow: legal/employment-agreement
params:
  employer_name: "InnovateTech Corp"
  employee_name: "Jane Smith"
  job_title: "Senior Software Engineer"
  employment_type: "full-time"
  salary: "$150,000 per annum"
  location: "hybrid"
  benefits: ["health insurance", "401k matching", "stock options", "unlimited PTO"]
  jurisdiction: "California"
  output_path: "output/employment_agreement.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| employer_name | string | Yes | - | Name of the employer |
| employee_name | string | Yes | - | Name of the employee |
| job_title | string | Yes | - | Job title |
| employment_type | string | No | full-time | Employment type |
| salary | string | Yes | - | Annual salary amount |
| location | string | No | on-site | Work location (on-site, remote, hybrid) |
| benefits | array | No | [] | List of benefits offered |
| jurisdiction | string | No | California | Governing jurisdiction |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/employment_agreement.md | Output file path |

## Nodes Used
- **agent** (check_labor_laws): Checks applicable labor laws and requirements
- **agent** (generate_contract): Generates the complete employment contract
- **agent** (validate_contract): Validates the contract for legal compliance
- **file_write** (save_contract): Saves the final contract

## Category
legal