# Webhook Relay

Receive, transform, and forward webhooks between services.

## Description

A webhook relay that receives incoming webhooks, transforms payloads with field mapping, enriches data with AI, and forwards to a target URL. Includes logging of all relayed payloads.

## Install

```bash
aflare install webhook-relay
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export RELAY_TARGET_URL="https://your-target-service.com/webhook"
export AUTH_HEADER="Bearer your-auth-token"
```

## Usage

```bash
aflare run templates/webhook-relay/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `relay_target_url` | Target URL to forward webhooks to | Required |
| `auth_header` | Authorization header for target | Required |
| `relay_id` | Unique relay identifier | Auto-generated |

## Nodes Used

- `http_request` — Receive incoming webhook, forward to target
- `json_parse` — Parse webhook payload
- `transform` — Map and transform payload fields
- `agent` — AI-powered payload enrichment
- `file_write` — Save relay log to JSON
- `notify` — Display relay status

## Output

- `webhook-relay-log.json` — Complete relay log with received, transformed, and forwarded payloads
- Terminal notification with relay status

## Schedule

```bash
# Run every 5 minutes for continuous relay
*/5 * * * * aflare run /path/to/templates/webhook-relay/workflow.yaml
```

## Category

integrations