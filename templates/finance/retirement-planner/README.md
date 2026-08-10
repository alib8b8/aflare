# Retirement Planner

> Retirement savings and withdrawal planning with gap analysis and tax optimization

## Description

This workflow template creates comprehensive retirement plans. It projects savings growth, calculates sustainable withdrawal rates, identifies savings shortfalls, recommends asset allocations, and provides tax-efficient withdrawal strategies.

## Usage

```bash
aflare install finance/retirement-planner
aflare run retirement-planner/workflow.yaml \
  --params.current_age="35" \
  --params.retirement_age="65" \
  --params.life_expectancy="90" \
  --params.current_savings="100000" \
  --params.annual_income="85000" \
  --params.monthly_contribution="1500" \
  --params.expected_return="7" \
  --params.inflation_rate="3" \
  --params.desired_income="60000"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| current_age | Yes | - | Current age |
| retirement_age | Yes | 65 | Target retirement age |
| life_expectancy | No | 90 | Life expectancy |
| current_savings | Yes | - | Current retirement savings |
| annual_income | Yes | - | Current annual income |
| monthly_contribution | Yes | - | Monthly retirement contribution |
| expected_return | No | 7 | Expected annual return (%) |
| inflation_rate | No | 3 | Expected inflation rate (%) |
| desired_income | Yes | - | Desired annual retirement income |

## Nodes Used

- agent - AI agent for retirement planning and strategy
- code_interpreter - Python-based savings projection and withdrawal calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance