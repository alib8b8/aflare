# Regulatory Filing Preparation

## Description
A regulatory filing preparation workflow that supports multiple filing types (SEC, FDA, FCC, EPA, etc.). It researches the latest filing requirements and forms, drafts compliant filing documents with all required sections and disclosures, and performs compliance verification before submission.

## Usage Example
```yaml
workflow: legal/regulatory-filing
params:
  filing_type: "SEC-10K"
  organization_name: "PublicTech Corp"
  reporting_period: "FY2025"
  jurisdiction: "US"
  deadline: "2026-03-31"
  data_points:
    revenue: "$500M"
    net_income: "$45M"
    total_assets: "$1.2B"
  output_path: "output/10k_filing.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| filing_type | string | Yes | - | Type of regulatory filing (SEC-10K, FDA-510k, etc.) |
| organization_name | string | Yes | - | Name of the filing organization |
| reporting_period | string | No | - | Reporting period |
| jurisdiction | string | Yes | - | Regulatory jurisdiction |
| data_points | object | No | - | Key data points for the filing |
| deadline | string | No | - | Filing deadline (YYYY-MM-DD) |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/regulatory_filing.md | Output file path |

## Nodes Used
- **agent** (fetch_requirements): Researches filing requirements and latest forms
- **agent** (draft_filing): Drafts the complete regulatory filing content
- **agent** (compliance_check): Verifies filing compliance and completeness
- **file_write** (save_filing): Saves the regulatory filing document

## Category
legal