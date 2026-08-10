# Credit Score Analyzer

> Credit score analysis and improvement suggestions with score projection modeling

## Description

This workflow template analyzes credit reports and provides actionable improvement plans. It breaks down score components, identifies strengths and weaknesses, projects future scores, and recommends specific strategies to reach target credit scores.

## Usage

```bash
aflare install finance/credit-score-analyzer
aflare run credit-score-analyzer/workflow.yaml \
  --params.credit_report="/path/to/credit-report.json" \
  --params.current_score="650" \
  --params.goal_score="720" \
  --params.timeline="12 months"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| credit_report | Yes | - | Path to credit report data file |
| current_score | Yes | - | Current credit score |
| score_model | No | FICO | Credit score model (FICO, VantageScore) |
| goal_score | Yes | - | Target credit score |
| timeline | No | 12 months | Desired timeline to reach goal |

## Nodes Used

- file_read - Read credit report data
- agent - AI agent for credit analysis and recommendations
- code_interpreter - Python-based score projection calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance