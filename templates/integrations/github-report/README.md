# GitHub Weekly Report

Generate a weekly activity report for your GitHub repository.

## Install

```bash
llm-box install github-report
```

## Configure

Edit `workflow.yaml` and set your repo and author:

```yaml
params:
  repo: "your-org/your-repo"
  author: "your-github-username"
```

## Run

```bash
llm-box run templates/github-report/workflow.yaml
```

## Output

- `github-weekly-report.md` — full markdown report with stats
- Terminal summary of weekly activity

## Features

- Fetches issues, PRs, and commits from the past week
- AI-powered summary and categorization
- Auto-generates markdown report
- Supports any public or private repo (with gh auth)

## Schedule

```bash
# Every Monday at 9 AM
0 9 * * 1 llm-box run /path/to/templates/github-report/workflow.yaml
```
