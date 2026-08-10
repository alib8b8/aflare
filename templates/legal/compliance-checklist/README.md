# Regulatory Compliance Checklist Generator

## Description
A comprehensive regulatory compliance checklist generator that creates tailored checklists for any industry and regulatory framework. The workflow fetches the latest regulatory updates, generates detailed compliance checklists with priority levels, creates prioritized action plans, and calculates compliance coverage metrics.

## Usage Example
```yaml
workflow: legal/compliance-checklist
params:
  industry: "healthcare"
  regulations: ["HIPAA", "HITECH", "PCI-DSS"]
  organization_size: "enterprise"
  regions: ["US", "EU"]
  risk_profile: "high"
  output_path: "output/compliance_checklist.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| industry | string | Yes | - | Industry sector |
| regulations | array | Yes | - | List of applicable regulations |
| organization_size | string | No | medium | Organization size (small, medium, enterprise) |
| regions | array | No | [US] | Operating regions |
| risk_profile | string | No | moderate | Risk profile (low, moderate, high) |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/compliance_checklist.md | Output file path |

## Nodes Used
- **http_request** (fetch_regulatory_data): Fetches latest regulatory updates
- **agent** (generate_checklist): Generates comprehensive compliance checklist
- **agent** (prioritize_actions): Creates prioritized compliance action plan
- **code_interpreter** (calculate_coverage): Calculates compliance coverage metrics
- **file_write** (save_checklist): Saves the checklist report

## Category
legal