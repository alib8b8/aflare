# Discord Notifier

Send webhook notifications to Discord channels.

## Description

Send rich embed notifications to Discord channels via webhook URLs. Supports custom embeds with titles, descriptions, colors, and timestamps.

## Install

```bash
aflare install discord-notifier
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export DISCORD_WEBHOOK_URL="https://discord.com/api/webhooks/..."
```

## Usage

```bash
# Write a message to send
echo "New release deployed to production!" > discord-message.txt

# Run the workflow
aflare run templates/discord-notifier/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `webhook_url` | Discord webhook URL | Required |
| `message` | Message content (from discord-message.txt) | "Notification from aflare workflow" |

## Nodes Used

- `file_read` — Read message from input file
- `template_render` — Format message with embed support
- `http_request` — POST to Discord webhook URL
- `json_parse` — Parse webhook response
- `notify` — Display delivery confirmation

## Output

- Terminal notification with delivery status
- Discord embed with workflow notification details

## Schedule

```bash
# After every deployment
0 10 * * 1-5 aflare run /path/to/templates/discord-notifier/workflow.yaml
```

## Category

integrations