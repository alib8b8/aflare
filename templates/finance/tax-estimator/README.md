# Tax Estimator

> Tax liability estimation and planning with deduction optimization and credit analysis

## Description

This workflow template estimates tax liability based on income data and taxpayer profile. It analyzes deductions vs standard deduction, identifies applicable tax credits, calculates federal and state taxes, and provides tax planning recommendations.

## Usage

```bash
aflare install finance/tax-estimator
aflare run tax-estimator/workflow.yaml \
  --params.income_file="/path/to/income.json" \
  --params.filing_status="single" \
  --params.state="CA" \
  --params.tax_year="2025" \
  --params.country="US"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| income_file | Yes | - | Path to income data file |
| filing_status | Yes | - | Tax filing status (single, married_joint, married_separate, head_of_household) |
| state | Yes | - | State of residence |
| dependents | No | 0 | Number of dependents |
| age | No | - | Taxpayer age |
| tax_year | No | Current | Tax year for estimation |
| country | No | US | Country for tax rules |

## Nodes Used

- file_read - Read income data
- agent - AI agent for tax analysis and planning
- code_interpreter - Python-based tax bracket calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance