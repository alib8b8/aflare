# Financial Report Generator

> Automated financial report generation with ratio analysis, benchmarking, and executive summary

## Description

This workflow template generates comprehensive financial reports from raw financial data. It includes income statement analysis, balance sheet review, cash flow analysis, financial ratio calculations, industry benchmarking, and an executive summary.

## Usage

```bash
aflare install finance/financial-report
aflare run financial-report/workflow.yaml \
  --params.data_file="/path/to/financials.json" \
  --params.company_name="Example Inc" \
  --params.report_type="quarterly" \
  --params.period="Q3 2025"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| data_file | Yes | - | Path to financial data file |
| company_name | Yes | - | Company name for report |
| report_type | No | quarterly | Report type (quarterly, annual, monthly) |
| period | Yes | - | Reporting period |
| comparison_period | No | - | Comparison period for YoY/QoQ |
| benchmark_api_url | No | - | API URL for industry benchmark data |

## Nodes Used

- file_read - Read financial data
- http_request - Fetch benchmark data from API
- agent - AI agent for report generation
- code_interpreter - Python-based ratio calculations
- template_render - Render final report format
- file_write - Write output report
- notify - Send completion notification

## Category

finance