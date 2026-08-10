# Expense Tracker

> Personal and business expense tracking with intelligent categorization and spending analysis

## Description

This workflow template provides automated expense tracking and categorization. It reads transaction data, uses AI to categorize each expense, performs statistical calculations, and generates a comprehensive spending report.

## Usage

```bash
aflare install finance/expense-tracker
aflare run expense-tracker/workflow.yaml \
  --params.transactions_file="/path/to/transactions.csv" \
  --params.categories="Food,Transport,Housing,Utilities,Entertainment,Healthcare,Shopping"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| transactions_file | Yes | - | Path to transactions file (CSV/JSON) |
| categories | No | Standard set | Comma-separated list of expense categories |
| tax_deductible | No | false | Whether to flag tax-deductible expenses |

## Nodes Used

- file_read - Read transaction data from file
- agent - AI agent for intelligent categorization
- code_interpreter - Python-based statistical calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance