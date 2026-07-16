# AI News Daily

Fetch and summarize the latest AI/ML news from Hacker News.

## Install

```bash
llm-box install ai-news
```

## Run

```bash
llm-box run templates/ai-news/workflow.yaml
```

## Output

- `ai-news-daily.md` — markdown summary with categorized stories
- Terminal notification with daily digest

## Features

- Fetches AI and ML stories from Hacker News
- Agent-powered summarization with reasoning
- Categorizes by breakthroughs, funding, and releases
- Auto-saves to dated markdown file

## Schedule

```bash
# Daily at 9 AM
0 9 * * * llm-box run /path/to/templates/ai-news/workflow.yaml
```
