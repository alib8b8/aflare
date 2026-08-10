# SMS Alert

Send SMS alerts and notifications via Twilio with intelligent message formatting.

## Description

Send SMS messages via Twilio with AI-powered message optimization for the 160-character SMS limit. Includes delivery confirmation and logging of all sent messages.

## Install

```bash
aflare install sms-alert
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export TWILIO_ACCOUNT_SID="your-account-sid"
export TWILIO_AUTH_TOKEN="your-auth-token"
export TWILIO_PHONE="+1234567890"
export TO_PHONE="+0987654321"
```

## Usage

```bash
# Write a message to send
echo "Server CPU usage exceeded 90% threshold!" > sms-message.txt

# Run the workflow
aflare run templates/sms-alert/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `account_sid` | Twilio Account SID | Required |
| `auth_token` | Twilio Auth Token | Required |
| `twilio_phone` | Twilio phone number (sender) | Required |
| `to_phone` | Recipient phone number | Required |
| `message` | SMS content (from sms-message.txt) | "Alert from aflare workflow" |

## Nodes Used

- `file_read` — Read message from input file
- `template_render` — Format message with timestamp
- `agent` — AI-powered SMS truncation to 160 chars
- `http_request` — Send SMS via Twilio API
- `json_parse` — Parse Twilio response
- `file_write` — Save SMS log to JSON
- `notify` — Display delivery confirmation

## Output

- `sms-log.json` — Log of all sent messages with SIDs and timestamps
- Terminal notification with message SID

## Schedule

```bash
# Alert check every 15 minutes
*/15 * * * * aflare run /path/to/templates/sms-alert/workflow.yaml
```

## Category

integrations