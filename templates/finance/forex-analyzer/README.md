# Forex Analyzer

> Foreign exchange rate analysis with trend detection, technical indicators, and volatility modeling

## Description

This workflow template provides comprehensive forex analysis. It fetches current and historical exchange rates, applies technical indicators, analyzes fundamental drivers, calculates volatility, and generates trading signals with entry/exit levels.

## Usage

```bash
aflare install finance/forex-analyzer
aflare run forex-analyzer/workflow.yaml \
  --params.base_currency="USD" \
  --params.target_currency="EUR,GBP,JPY" \
  --params.timeframe="daily" \
  --params.analysis_type="technical" \
  --params.historical_days="90"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| base_currency | Yes | - | Base currency code (e.g., USD) |
| target_currency | Yes | - | Target currency code(s) |
| timeframe | No | daily | Timeframe for analysis (daily, weekly, monthly) |
| analysis_type | No | technical | Analysis type (technical, fundamental, combined) |
| historical_days | No | 90 | Number of historical days to analyze |
| historical_api_url | No | - | Historical data API URL |

## Nodes Used

- http_request - Fetch current and historical exchange rates
- agent - AI agent for forex analysis and signal generation
- code_interpreter - Python-based volatility and trend calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance