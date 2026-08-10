# Cash Flow Forecast

> Cash flow projection and analysis with scenario modeling and runway calculation

## Description

This workflow template projects future cash flows based on historical financial data and growth assumptions. It models inflows and outflows, calculates cash runway, burn rate, and runs best/worst-case scenarios to identify potential liquidity issues.

## Usage

```bash
aflare install finance/cash-flow-forecast
aflare run cash-flow-forecast/workflow.yaml \
  --params.financial_data="/path/to/financials.json" \
  --params.forecast_months="12" \
  --params.starting_cash="50000" \
  --params.revenue_growth="5" \
  --params.expense_growth="3"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| financial_data | Yes | - | Path to historical financial data file |
| forecast_months | Yes | 12 | Number of months to forecast |
| starting_cash | Yes | - | Current cash balance |
| revenue_growth | No | 5 | Monthly revenue growth rate (%) |
| expense_growth | No | 3 | Monthly expense growth rate (%) |
| seasonality | No | none | Seasonality pattern (none, quarterly, holiday) |

## Nodes Used

- file_read - Read financial data
- agent - AI agent for cash flow projection
- code_interpreter - Python-based runway and burn rate calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance