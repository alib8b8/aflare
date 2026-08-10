# Slack Bot

Send messages and manage Slack channels from your workflows.

## Description

Automate Slack communication by sending messages to channels, listing available channels, and generating activity reports. Uses the Slack Web API with bot token authentication.

## Install

```bash
aflare install slack-bot
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export SLACK_BOT_TOKEN="xoxb-your-bot-token"
export SLACK_CHANNEL="general"
```

## Usage

```bash
# Write a message to send
echo "Deployment completed successfully!" > slack-message.txt

# Run the workflow
aflare run templates/slack-bot/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `slack_token` | Slack Bot User OAuth token | Required |
| `channel` | Target Slack channel ID or name | Required |
| `message` | Message content (from slack-message.txt) | "Message from aflare workflow" |

## Nodes Used

- `file_read` — Read message from input file
- `template_render` — Format message with template variables
- `http_request` — Send message via Slack API, list channels
- `agent` — Parse and summarize API responses
- `file_write` — Save report to markdown file
- `notify` — Display confirmation to stdout

## Output

- `slack-report.md` — Markdown report with message delivery status and channel list
- Terminal notification with delivery confirmation

## Schedule

```bash
# Daily standup reminder at 9 AM
0 9 * * 1-5 aflare run /path/to/templates/slack-bot/workflow.yaml
```

## Category

integrations