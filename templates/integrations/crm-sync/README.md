# CRM Sync

Synchronize CRM contacts and deals across platforms like HubSpot.

## Description

Fetch contacts and deals from HubSpot CRM, use AI to analyze pipeline health, identify stale contacts and duplicates, and generate sync recommendations. Extensible to Salesforce and other CRMs.

## Install

```bash
aflare install crm-sync
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export HUBSPOT_API_KEY="your-hubspot-api-key"
```

## Usage

```bash
aflare run templates/crm-sync/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `hubspot_api_key` | HubSpot API key | Required |

## Nodes Used

- `http_request` — Fetch contacts and deals from HubSpot
- `json_parse` — Parse contact and deal data
- `agent` — AI-powered CRM analysis
- `file_write` — Save sync report
- `notify` — Display confirmation

## Output

- `crm-sync-report.md` — CRM analysis with segmentation, pipeline health, and recommendations
- Terminal summary with contact and deal counts

## Schedule

```bash
# Daily CRM sync
0 7 * * * aflare run /path/to/templates/crm-sync/workflow.yaml
```

## Category

integrations