# Calendar Bridge

Bridge calendar events between Google Calendar and Microsoft Outlook.

## Description

Fetch events from Google Calendar and Microsoft Outlook, compare event lists with AI, identify missing events across platforms, and sync events to ensure both calendars stay in sync.

## Install

```bash
aflare install calendar-bridge
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export GOOGLE_CALENDAR_ID="primary"
export GOOGLE_TOKEN="your-google-oauth-token"
export OUTLOOK_CLIENT_ID="your-outlook-client-id"
export OUTLOOK_CLIENT_SECRET="your-outlook-client-secret"
export OUTLOOK_TOKEN="your-outlook-oauth-token"
```

## Usage

```bash
aflare run templates/calendar-bridge/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `google_calendar_id` | Google Calendar ID | "primary" |
| `google_token` | Google OAuth access token | Required |
| `outlook_token` | Microsoft Graph access token | Required |

## Nodes Used

- `http_request` — Fetch Google Calendar events, fetch Outlook events, sync events
- `json_parse` — Parse event lists
- `agent` — AI-powered event comparison and sync planning
- `file_write` — Save bridge report
- `notify` — Display confirmation

## Output

- `calendar-bridge-report.md` — Sync plan with identified gaps and conflicts
- Events synced to Outlook (missing events from Google)

## Schedule

```bash
# Hourly sync
0 * * * * aflare run /path/to/templates/calendar-bridge/workflow.yaml
```

## Category

integrations