# Zendesk Ticket Manager

Manage Zendesk support tickets with AI-powered analysis and reporting.

## Description

Fetch recent support tickets from Zendesk, use AI to analyze ticket status, identify aging tickets, suggest responses and priority changes, and generate a comprehensive support report.

## Install

```bash
aflare install zendesk-ticket
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export ZENDESK_SUBDOMAIN="your-subdomain"
export ZENDESK_EMAIL="your-email@example.com"
export ZENDESK_API_TOKEN="your-zendesk-api-token"
```

## Usage

```bash
aflare run templates/zendesk-ticket/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `subdomain` | Zendesk subdomain | Required |
| `auth_token` | Base64 encoded email/token:password | Required |

## Nodes Used

- `http_request` — Fetch tickets, create report ticket
- `json_parse` — Parse ticket data
- `agent` — AI-powered ticket analysis and response suggestions
- `file_write` — Save support report
- `notify` — Display confirmation

## Output

- `zendesk-support-report.md` — Comprehensive support ticket analysis
- Optional report ticket created in Zendesk

## Schedule

```bash
# Daily support review
0 8 * * * aflare run /path/to/templates/zendesk-ticket/workflow.yaml
```

## Category

integrations