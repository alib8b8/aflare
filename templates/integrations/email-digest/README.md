# Email Digest

Generate and send daily/weekly email digests with news and trending content.

## Description

Automatically fetch top news headlines and trending GitHub repositories, then use AI to generate a formatted HTML email digest. Sends via SMTP and saves a local copy.

## Install

```bash
aflare install email-digest
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export NEWS_API_KEY="your-newsapi-key"
export SMTP_HOST="smtp.gmail.com"
export SMTP_PORT="587"
export SMTP_USER="your-email@gmail.com"
export SMTP_PASS="your-app-password"
export RECIPIENT="recipient@example.com"
```

## Usage

```bash
aflare run templates/email-digest/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `news_api_key` | NewsAPI.org API key | Required |
| `smtp_host` | SMTP server hostname | Required |
| `smtp_port` | SMTP server port | Required |
| `smtp_user` | SMTP username | Required |
| `smtp_pass` | SMTP password | Required |
| `recipient` | Email recipient address | Required |

## Nodes Used

- `http_request` — Fetch news from NewsAPI, fetch GitHub trending
- `json_parse` — Parse news articles
- `agent` — Generate HTML email digest with AI
- `http_request` — Send email via SMTP
- `file_write` — Save digest to HTML file
- `notify` — Display confirmation

## Output

- `email-digest-{date}.html` — Full HTML email digest
- Email sent to configured recipient

## Schedule

```bash
# Daily at 8 AM
0 8 * * * aflare run /path/to/templates/email-digest/workflow.yaml
```

## Category

integrations