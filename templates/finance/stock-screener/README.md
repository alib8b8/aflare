# Stock Screener

> Stock screening and filtering tool with multi-factor scoring and ranking

## Description

This workflow template screens stocks based on customizable fundamental and technical criteria. It fetches market data, applies filters on valuation, growth, profitability, and financial health metrics, then ranks stocks using a composite scoring system.

## Usage

```bash
aflare install finance/stock-screener
aflare run stock-screener/workflow.yaml \
  --params.market_api_url="https://api.example.com/stocks" \
  --params.sector="Technology" \
  --params.market_cap_min="1000000000" \
  --params.pe_max="30" \
  --params.roe_min="15" \
  --params.revenue_growth_min="10"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| market_api_url | Yes | - | Market data API URL |
| api_key | No | - | API authentication key |
| sector | No | - | Target sector filter |
| market_cap_min | No | - | Minimum market cap |
| market_cap_max | No | - | Maximum market cap |
| pe_min | No | - | Minimum P/E ratio |
| pe_max | No | - | Maximum P/E ratio |
| div_yield_min | No | - | Minimum dividend yield (%) |
| revenue_growth_min | No | - | Minimum revenue growth (%) |
| de_ratio_max | No | - | Maximum debt/equity ratio |
| roe_min | No | - | Minimum ROE (%) |
| beta_min | No | - | Minimum beta |
| beta_max | No | - | Maximum beta |
| price_min | No | - | Minimum stock price |
| price_max | No | - | Maximum stock price |

## Nodes Used

- http_request - Fetch market data from API
- agent - AI agent for stock screening and analysis
- code_interpreter - Python-based composite scoring
- file_write - Write output results
- notify - Send completion notification

## Category

finance