# Contract Review & Risk Analysis

## Description
An AI-powered workflow for comprehensive contract review and risk analysis. This template automates the process of reviewing legal contracts by classifying contract types, identifying potential risks, checking for missing or problematic clauses, and generating an executive summary with actionable recommendations.

## Usage Example
```yaml
workflow: legal/contract-review
params:
  contract_text: "Full contract text here..."
  contract_type: "vendor"
  jurisdiction: "US"
  risk_threshold: "medium"
  output_path: "output/review_report.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| contract_text | string | Yes | - | Full text of the contract to review |
| contract_type | string | No | general | Type of contract (employment, vendor, lease, msa) |
| jurisdiction | string | No | US | Governing jurisdiction for legal analysis |
| risk_threshold | string | No | medium | Risk sensitivity threshold (low, medium, high) |
| model | string | No | gpt-4 | AI model to use for analysis |
| output_path | string | No | output/contract_review_report.md | Output file path |

## Nodes Used
- **agent** (classify_contract): Classifies contract type and extracts metadata
- **agent** (risk_analysis): Identifies and analyzes legal risks with severity ratings
- **agent** (clause_check): Checks for missing and problematic clauses
- **agent** (summary_report): Generates consolidated executive summary
- **file_write** (save_report): Saves the full review report to a file

## Category
legal