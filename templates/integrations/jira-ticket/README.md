# Jira Ticket Manager

Create and track Jira issues with AI-powered ticket generation.

## Description

Search existing Jira issues, analyze project context, and use AI to generate properly formatted tickets with summaries, descriptions, priority suggestions, and recommended assignees.

## Install

```bash
aflare install jira-ticket
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export JIRA_BASE_URL="your-domain.atlassian.net"
export JIRA_EMAIL="your-email@example.com"
export JIRA_API_TOKEN="your-jira-api-token"
export JIRA_PROJECT="PROJ"
```

## Usage

```bash
# Create a JSON file with issue details
echo '{"title": "Fix login bug", "description": "Users cannot login with SSO"}' > jira-issue.json

# Run the workflow
aflare run templates/jira-ticket/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `jira_base_url` | Jira instance URL | Required |
| `auth_token` | Base64 encoded email:api_token | Required |
| `project` | Jira project key | Required |
| `issue_data` | Issue details from jira-issue.json | Optional |

## Nodes Used

- `file_read` — Read issue data from JSON file
- `http_request` — Search existing issues, create new issue
- `json_parse` — Parse issue lists
- `agent` — AI-powered issue generation
- `file_write` — Save created issue details
- `notify` — Display confirmation

## Output

- `jira-issue-created.json` — Created issue details
- New Jira issue in the specified project

## Schedule

```bash
# Daily issue triage
0 10 * * * aflare run /path/to/templates/jira-ticket/workflow.yaml
```

## Category

integrations