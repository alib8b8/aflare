# GitHub Issue Bot

Auto-create, label, and triage GitHub issues.

## Description

Fetch open issues from a GitHub repository, use AI to analyze patterns and identify duplicates, auto-apply labels, and suggest new issues based on gaps. Streamlines issue triage and management.

## Install

```bash
aflare install github-issue-bot
```

## Configure

Set environment variables or edit `workflow.yaml`:

```bash
export GITHUB_TOKEN="ghp_your-github-token"
export GITHUB_REPO="owner/repo"
```

## Usage

```bash
aflare run templates/github-issue-bot/workflow.yaml
```

## Parameters

| Parameter | Description | Default |
|-----------|-------------|---------|
| `github_token` | GitHub personal access token | Required |
| `repo` | Repository in owner/repo format | Required |

## Nodes Used

- `http_request` — Fetch open issues, create issues, apply labels
- `json_parse` — Parse issue lists
- `agent` — AI-powered issue analysis and pattern detection
- `file_write` — Save analysis report
- `notify` — Display confirmation

## Output

- `github-issue-report.md` — Issue analysis and triage report
- New issues created and labels applied as needed

## Schedule

```bash
# Daily issue triage
0 9 * * 1-5 aflare run /path/to/templates/github-issue-bot/workflow.yaml
```

## Category

integrations