# Loan Calculator

> Loan comparison and amortization schedule calculator with prepayment analysis

## Description

This workflow template compares multiple loan options and generates detailed amortization schedules. It supports mortgages, auto loans, personal loans, student loans, and business loans. It calculates monthly payments, total interest, APR, and the impact of extra payments.

## Usage

```bash
aflare install finance/loan-calculator
aflare run loan-calculator/workflow.yaml \
  --params.loan_type="mortgage" \
  --params.loan_scenarios='[{"name":"30yr Fixed","principal":300000,"rate":6.5,"term_months":360},{"name":"15yr Fixed","principal":300000,"rate":5.75,"term_months":180}]'
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| loan_type | Yes | - | Type of loan (mortgage, auto, personal, student, business) |
| loan_scenarios | Yes | - | JSON array of loan scenarios with principal, rate, term_months |
| borrower_profile | No | - | Borrower profile for personalized recommendations |

## Nodes Used

- agent - AI agent for loan analysis and comparison
- code_interpreter - Python-based amortization calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance