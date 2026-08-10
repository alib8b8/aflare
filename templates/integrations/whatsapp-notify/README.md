# WhatsApp Notify

Send WhatsApp message notifications with delivery status tracking.

## Description

Send WhatsApp messages via Twilio's WhatsApp Business API with AI-powered message formatting, WhatsApp text styling support, and delivery status verification.

## Install

```bash
aflare install whatsapp-notify
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export TWILIO_ACCOUNT_SID="your-account-sid"
export TWILIO_AUTH_TOKEN="your-auth-token"
export WHATSAPP_FROM="+1234567890"
export WHATSAPP_TO="+0987654321"
```

## Usage

```bash
# Write a message to send
echo "Meeting reminder: Standup in 10 minutes!" > whatsapp-message.txt

# Run the workflow
aflare run templates/whatsapp-notify/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `account_sid` | Twilio Account SID | Required |
| `auth_token` | Twilio Auth Token | Required |
| `whatsapp_from` | WhatsApp sender number | Required |
| `whatsapp_to` | WhatsApp recipient number | Required |
| `message` | Message content (from whatsapp-message.txt) | "Notification from aflare" |

## Nodes Used

- `file_read` — Read message from input file
- `template_render` — Format message with timestamp
- `agent` — AI-powered WhatsApp formatting
- `http_request` — Send WhatsApp message, check delivery status
- `json_parse` — Parse Twilio responses
- `file_write` — Save WhatsApp log to JSON
- `notify` — Display delivery confirmation

## Output

- `whatsapp-log.json` — Log with message SID, status, and timestamp
- Terminal notification with delivery status

## Schedule

```bash
# Hourly reminder check
0 * * * * aflare run /path/to/templates/whatsapp-notify/workflow.yaml
```

## Category

integrations