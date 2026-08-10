# Trello Sync

Manage Trello boards and cards with automated sync reports.

## Description

Fetch all lists and cards from a Trello board, use AI to analyze the board state, identify overdue cards, suggest card movements, and create a sync report card. Keeps your Trello board organized.

## Install

```bash
aflare install trello-sync
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export TRELLO_API_KEY="your-trello-api-key"
export TRELLO_TOKEN="your-trello-token"
export TRELLO_BOARD_ID="your-board-id"
```

## Usage

```bash
aflare run templates/trello-sync/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `api_key` | Trello API key | Required |
| `token` | Trello API token | Required |
| `board_id` | Trello board ID | Required |

## Nodes Used

- `http_request` — Fetch lists, fetch cards, create report card
- `json_parse` — Parse list and card data
- `agent` — AI-powered board analysis
- `file_write` — Save sync report
- `notify` — Display confirmation

## Output

- `trello-sync-report.md` — Board analysis and sync report
- New report card created on the board

## Schedule

```bash
# Weekly board sync on Monday
0 8 * * 1 aflare run /path/to/templates/trello-sync/workflow.yaml
```

## Category

integrations