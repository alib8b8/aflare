# Notion Sync

Create Notion pages and synchronize databases automatically.

## Description

Query Notion databases, analyze existing entries with AI, and create new pages with summaries and action items. Perfect for automated knowledge base updates and database synchronization.

## Install

```bash
aflare install notion-sync
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export NOTION_API_KEY="secret_your-notion-api-key"
export NOTION_DATABASE_ID="your-database-id"
```

## Usage

```bash
aflare run templates/notion-sync/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `notion_api_key` | Notion integration API key | Required |
| `database_id` | Notion database ID to sync | Required |

## Nodes Used

- `http_request` — Query Notion database, create new page
- `json_parse` — Parse database results
- `agent` — Analyze entries and generate page content
- `file_write` — Save sync report
- `notify` — Display confirmation

## Output

- `notion-sync-report.md` — Generated page content and analysis
- New Notion page created in the target database

## Schedule

```bash
# Daily sync at 7 AM
0 7 * * * aflare run /path/to/templates/notion-sync/workflow.yaml
```

## Category

integrations