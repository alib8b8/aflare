# Insurance Comparison Analyzer

> Insurance policy comparison analyzer with cost-effectiveness scoring and coverage gap detection

## Description

This workflow template compares multiple insurance policies side by side. It analyzes coverage details, costs, benefits, financial strength ratings, and provides value-based recommendations. Supports health, auto, life, home, and business insurance types.

## Usage

```bash
aflare install finance/insurance-comparison
aflare run insurance-comparison/workflow.yaml \
  --params.policies_file="/path/to/policies.json" \
  --params.insurance_type="auto" \
  --params.coverage_needs="comprehensive" \
  --params.budget="2000"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| policies_file | Yes | - | Path to policies data file |
| insurance_type | Yes | - | Type of insurance (auto, health, life, home, business) |
| coverage_needs | No | standard | Coverage level needed (basic, standard, comprehensive) |
| budget | Yes | - | Annual budget for insurance |

## Nodes Used

- file_read - Read policy data
- agent - AI agent for policy comparison and analysis
- code_interpreter - Python-based cost-effectiveness calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance