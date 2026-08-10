# Cap Table Manager

> Startup cap table and equity management with dilution analysis and exit scenario modeling

## Description

This workflow template manages startup capitalization tables. It tracks ownership structure, vesting schedules, models dilution from new funding rounds, runs exit scenario waterfalls, and checks compliance with 409A and securities regulations.

## Usage

```bash
aflare install finance/cap-table
aflare run cap-table/workflow.yaml \
  --params.cap_table_file="/path/to/cap-table.json" \
  --params.scenario="Series A raise of $5M at $20M pre-money" \
  --params.stage="Seed" \
  --params.valuation="15000000"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| cap_table_file | Yes | - | Path to cap table data file |
| scenario | Yes | - | Scenario to model (e.g., new funding round, exit) |
| stage | No | Seed | Company stage (Pre-seed, Seed, Series A, etc.) |
| valuation | Yes | - | Current company valuation |

## Nodes Used

- file_read - Read cap table data
- agent - AI agent for cap table analysis and scenario modeling
- code_interpreter - Python-based ownership calculations and waterfall analysis
- file_write - Write output report
- notify - Send completion notification

## Category

finance