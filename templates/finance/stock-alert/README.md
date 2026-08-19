# A-Share Price Alert

Monitor an A-share stock price and get alerted when it crosses your threshold.

## Install

```bash
aflare install finance/stock-alert
```

## Configure

Pass variables on the command line or edit the defaults in `workflow.yaml`:

```bash
aflare run stock-alert/workflow.yaml --set symbol=sh600519 --set threshold=1400
```

- `symbol`: Tencent quote symbol — `sh600519` (Shanghai: 6xx codes) / `sz000001` (Shenzhen: 0xx/3xx codes)
- `threshold`: alert threshold in CNY

## Run

```bash
aflare run stock-alert/workflow.yaml
```

## Features

- Fetches real-time A-share quotes from the Tencent quote API (`web.ifzq.gtimg.cn`, no auth, UTF-8 JSON)
- Compares the live price against a configurable threshold
- Logs all alerts to `stock-alert.log`

## Schedule

```bash
# Run every 10 minutes
*/10 * * * * aflare run /path/to/stock-alert/workflow.yaml
```

## Disclaimer

Market data comes from the Tencent public quote API and may be delayed or inaccurate. Output of this template is for personal research only and does **not** constitute investment advice. 投资有风险，入市需谨慎。
