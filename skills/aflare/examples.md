# aflare Workflow Examples

10 ready-to-use workflow patterns. Use these as templates when generating workflows for users.

## 1. Fetch and Save

The simplest pattern — fetch a URL and save to a file.

```yaml
name: fetch-and-save
description: Fetch a URL and save to file
steps:
  - node: fetch_url
    params:
      url: "https://example.com/data"
  - node: file_write
    params:
      path: "data.txt"
```

## 2. Fetch, Parse, and Summarize with LLM

Fetch an article, extract content, summarize with local LLM.

```yaml
name: research-assistant
description: Fetch an article and summarize with LLM
steps:
  - node: fetch_url
    name: fetch
    params:
      url: "https://example.com/article"
      mode: "text"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Summarize the key points: {{step.fetch}}"
      temperature: 0.3
  - node: file_write
    params:
      path: "summary.md"
```

## 3. Multi-source Aggregation (Parallel)

Fetch from multiple APIs in parallel, combine, and analyze.

```yaml
name: daily-report
description: Aggregate data from multiple sources
steps:
  - parallel:
      - node: fetch_url
        params:
          url: "https://api.weather.gov/forecast"
      - node: fetch_url
        params:
          url: "https://api.stock.example.com/quote/ABC"
  - node: combine
    name: merged
    params:
      separator: "\n---\n"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Analyze this combined data and write a daily report: {{step.merged}}"
  - node: file_write
    params:
      path: "daily-report.md"
```

## 4. GitHub Daily Digest

Fetch GitHub activity and generate a summary.

```yaml
name: github-digest
description: Generate GitHub activity digest
steps:
  - node: fetch_url
    params:
      url: "https://github.com/your-username"
      mode: "text"
  - node: transform
    name: repos
    params:
      operation: regex
      pattern: "([a-zA-Z0-9-]+/[a-zA-Z0-9-]+)"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Summarize recent GitHub activity: {{step.repos}}"
  - node: file_write
    params:
      path: "github-digest.md"
```

## 5. Release Notes Generator

Generate release notes from git commit history.

```yaml
name: release-notes
description: Generate release notes from git log
steps:
  - node: execute
    params:
      command: "git log --oneline --since='2 weeks ago'"
  - node: transform
    name: commits
    params:
      operation: replace
      find: "feat:"
      replace: "NEW FEATURE:"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Organize these commits into release notes by category: {{step.commits}}"
  - node: file_write
    params:
      path: "RELEASE-NOTES.md"
```

## 6. Log Monitor & Alert

Monitor logs for errors and alert.

```yaml
name: log-monitor
description: Watch logs for 5xx errors and alert
steps:
  - node: execute
    params:
      command: "tail -n 100 /var/log/server.log"
  - node: transform
    params:
      operation: regex
      pattern: "5\\d{2}"
  - node: notify
    condition: "not_empty"
    params:
      channel: stdout
      message: "ALERT: 5xx errors detected in server logs"
```

## 7. API Documentation Generator

Scan a Go project and generate API docs.

```yaml
name: docs-generator
description: Scan Go project and generate API overview
steps:
  - node: execute
    params:
      command: "find . -name '*.go' -not -path './vendor/*'"
  - node: execute
    name: signatures
    params:
      command: "grep -rn 'func \\(.*\\)(' --include='*.go' ."
  - node: ollama
    params:
      model: "llama3"
      prompt: "Generate API documentation from these function signatures: {{step.signatures}}"
  - node: file_write
    params:
      path: "API.md"
```

## 8. Content Processor (Markdown to HTML)

Convert markdown to HTML.

```yaml
name: content-processor
description: Convert markdown post to HTML
steps:
  - node: file_read
    params:
      path: "post.md"
  - node: transform
    params:
      operation: markdown_to_html
  - node: file_write
    params:
      path: "post.html"
```

## 9. DevOps Deploy with Health Check

Build, deploy, and verify health.

```yaml
name: zero-downtime-deploy
description: Build and deploy with health check
steps:
  - node: execute
    params:
      command: "docker build -t my-service ."
      timeout: "300s"
  - node: execute
    params:
      command: "docker-compose up -d --no-deps my-service"
  - node: execute
    params:
      command: "sleep 30 && curl -f http://localhost/health"
      timeout: "60s"
  - node: notify
    params:
      channel: stdout
      message: "Deploy complete and health check passed"
```

## 10. Team Weekly Report

Compile weekly issue and commit stats.

```yaml
name: team-weekly-report
description: Compile weekly issue and commit stats
steps:
  - parallel:
      - node: execute
        params:
          command: "gh issue list --repo my-org/my-repo --since='1 week ago' --state all"
      - node: execute
        params:
          command: "git log --author='@my-team.com' --since='1 week ago' --oneline"
  - node: combine
    name: merged
    params:
      separator: "\n## Commits\n"
  - node: ollama
    params:
      model: "llama3"
      prompt: "Create a weekly team report from this data: {{step.merged}}"
  - node: file_write
    params:
      path: "team-report.md"
```

## Common Patterns Reference

### Error Handling with Retry

```yaml
- node: http_request
  retry: 3
  delay: "2s"
  params:
    url: "https://flaky-api.example.com/data"
```

### Conditional Branching

The step-level `condition` operates on the step input (previous step output) with operators like `not_empty`, `empty`, `contains:x`, `equals:x`, `regex:x` (prefix with `not ` to negate).

```yaml
- node: fetch_url
  name: fetch
  params:
    url: "https://example.com/api"
- node: ollama
  condition: "not_empty"
  params:
    prompt: "Analyze: {{step.fetch}}"
- node: notify
  condition: "empty"
  params:
    channel: stdout
    message: "No data received, skipping analysis"
```

### Workflow Chaining

```yaml
- node: call
  params:
    workflow: "sub-workflows/parse-data.yaml"
    vars: "input={{step.fetch}}"
```

### Using Variables

```yaml
name: parameterized-workflow
vars:
  target_url: "https://example.com"
  output_file: "result.txt"
steps:
  - node: fetch_url
    params:
      url: "{{var.target_url}}"
  - node: file_write
    params:
      path: "{{var.output_file}}"
```
