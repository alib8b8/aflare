# BTC Price Alert

Monitor Bitcoin price and get alerted when it crosses your threshold.

## Install

```bash
llm-box install btc-alert
```

## Configure

Edit `workflow.yaml` and set your threshold:

```yaml
params:
  threshold: "100000"
```

## Run

```bash
llm-box run templates/btc-alert/workflow.yaml
```

## Features

- Fetches real-time BTC price from CoinGecko API
- Compares against configurable threshold
- Logs all alerts to `btc-alert.log`
- Supports agent-based reasoning for complex alert conditions

## Schedule

```bash
# Run every 5 minutes
*/5 * * * * llm-box run /path/to/templates/btc-alert/workflow.yaml
```
