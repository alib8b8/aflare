# Telegram Notification Bot

Send notifications to Telegram from your workflows.

## Install

```bash
aflare install telegram-bot
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export TELEGRAM_BOT_TOKEN="your-bot-token"
export TELEGRAM_CHAT_ID="your-chat-id"
```

## Run

```bash
# Write a message
echo "Deployment successful!" > message.txt

# Send it
aflare run templates/telegram-bot/workflow.yaml
```

## Features

- Send any text message to Telegram
- Supports Markdown formatting
- Reads message from `message.txt` or uses default
- Confirmation of delivery status

## Use with other workflows

Chain this template after any workflow to send results to Telegram:

```yaml
steps:
  - node: agent
    # ... do something ...
    id: result

  - node: file_write
    params:
      path: message.txt
    input: result

  # Then run telegram-bot
```
