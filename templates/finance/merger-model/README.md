# Merger Model

> M&A financial model with accretion/dilution analysis, synergy modeling, and sensitivity tables

## Description

This workflow template builds comprehensive M&A merger models. It combines acquirer and target financials, calculates pro forma statements, performs accretion/dilution analysis, models synergies, and generates sensitivity tables for deal evaluation.

## Usage

```bash
aflare install finance/merger-model
aflare run merger-model/workflow.yaml \
  --params.acquirer_financials="/path/to/acquirer.json" \
  --params.target_financials="/path/to/target.json" \
  --params.acquirer_name="Acme Corp" \
  --params.target_name="TargetCo" \
  --params.offer_price="45.00" \
  --params.premium="25" \
  --params.financing_mix="60% stock, 40% cash" \
  --params.synergies="50000000"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| acquirer_financials | Yes | - | Path to acquirer financial data file |
| target_financials | Yes | - | Path to target financial data file |
| acquirer_name | Yes | - | Acquirer company name |
| target_name | Yes | - | Target company name |
| offer_price | Yes | - | Offer price per share |
| premium | Yes | - | Premium over current price (%) |
| transaction_type | No | merger | Transaction type (merger, acquisition, stock, cash, mixed) |
| financing_mix | No | 100% stock | Financing mix |
| synergies | No | 0 | Estimated annual synergies |
| integration_costs | No | 0 | Estimated integration costs |

## Nodes Used

- file_read - Read acquirer and target financials
- agent - AI agent for merger model construction and analysis
- code_interpreter - Python-based accretion/dilution calculations and sensitivity analysis
- file_write - Write output report
- notify - Send completion notification

## Category

finance