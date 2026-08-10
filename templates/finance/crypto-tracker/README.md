# Crypto Tracker

> Cryptocurrency portfolio tracker with real-time price alerts and performance analysis

## Description

This workflow template tracks cryptocurrency portfolios with real-time price data from CoinGecko. It provides portfolio valuation, profit/loss tracking, technical analysis signals, risk assessment, and rebalancing suggestions.

## Usage

```bash
aflare install finance/crypto-tracker
aflare run crypto-tracker/workflow.yaml \
  --params.coin_ids="bitcoin,ethereum,solana,cardano" \
  --params.portfolio_file="/path/to/portfolio.json" \
  --params.alert_thresholds="price_change:10,volume_spike:3x"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| coin_ids | Yes | - | Comma-separated CoinGecko coin IDs |
| portfolio_file | Yes | - | Path to portfolio holdings file |
| alert_thresholds | No | - | Alert thresholds (e.g., price_change:10,volume_spike:3x) |

## Nodes Used

- http_request - Fetch real-time prices from CoinGecko API
- file_read - Read portfolio holdings
- agent - AI agent for portfolio analysis and alerts
- code_interpreter - Python-based portfolio calculations
- file_write - Write output report
- notify - Send completion notification

## Category

finance