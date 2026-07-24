# Market Scan & Intelligence

Financial market scanning and investment intelligence workflow.

## Features

- Real-time market data aggregation
- Multi-platform news and sentiment analysis (Reddit, HN, GitHub, Google)
- Swarm intelligence investment thesis generation
- Risk/reward ranking of opportunities
- Watchlist and position sizing recommendations

## Usage

```bash
llm-box install finance/market-scan
llm-box run market-scan/workflow.yaml \
  --params.query="AI sector stocks Q3 outlook" \
  --params.strategy="growth" \
  --params.risk_tolerance="medium" \
  --params.time_horizon="6-12 months" \
  --params.capital="100000" \
  --params.market_data_url="https://api.example.com/market"
```

## Workflow Steps

1. **Market Data** - Fetch market data from API
2. **News Aggregation** - Aggregate news from multiple platforms using search_aggregate
3. **Swarm Intelligence** - 5 financial specialists collaborate via swarm communication
4. **Save Report** - Write investment thesis to markdown
5. **Notify** - Output results

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| query | Yes | - | Search query for market scan |
| strategy | No | growth | Investment strategy |
| risk_tolerance | No | medium | Risk tolerance level |
| time_horizon | No | 6-12 months | Investment time horizon |
| capital | No | 100000 | Available capital |
| market_data_url | Yes | - | Market data API URL |
