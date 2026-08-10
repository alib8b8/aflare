# Stripe Report

Generate Stripe payment, revenue, and customer analytics reports.

## Description

Fetch charges, balance, and customer data from the Stripe API, then use AI to analyze revenue trends, calculate success rates, identify top customers, and generate a comprehensive financial report.

## Install

```bash
aflare install stripe-report
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export STRIPE_SECRET_KEY="sk_live_your-stripe-secret-key"
```

## Usage

```bash
aflare run templates/stripe-report/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `stripe_secret_key` | Stripe secret API key | Required |

## Nodes Used

- `http_request` — Fetch charges, balance, and customers from Stripe
- `json_parse` — Parse Stripe API responses
- `agent` — AI-powered revenue analysis and report generation
- `file_write` — Save revenue report to markdown
- `notify` — Display confirmation

## Output

- `stripe-revenue-report.md` — Comprehensive revenue report with:
  - Total revenue and fees
  - Charge success rate
  - Top customers by spend
  - Refund analysis
  - Revenue trends
  - Monthly recurring revenue estimate

## Schedule

```bash
# Weekly financial report every Monday
0 9 * * 1 aflare run /path/to/templates/stripe-report/workflow.yaml
```

## Category

integrations