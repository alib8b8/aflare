# Scheduled Workflows

aflare supports scheduling workflows to run at specified times using cron expressions.

## Quick Start

### Schedule a Workflow via CLI

```bash
# Schedule workflow to run daily at 9:00 AM
aflare schedule --cron "0 9 * * *" my-workflow.yaml

# Schedule workflow to run every hour
aflare schedule --cron "0 * * * *" my-workflow.yaml

# Schedule workflow with custom task ID
aflare schedule --id daily-report --cron "0 9 * * *" my-workflow.yaml
```

### List Scheduled Tasks

```bash
# List all scheduled tasks
aflare schedule --list

# Get task details
aflare schedule --info daily-report
```

### Remove a Scheduled Task

```bash
aflare schedule --remove daily-report
```

## Cron Expression Syntax

aflare uses standard 5-field cron expressions:

```
┌───────────── minute (0 - 59)
│ ┌───────────── hour (0 - 23)
│ │ ┌───────────── day of month (1 - 31)
│ │ │ ┌───────────── month (1 - 12)
│ │ │ │ ┌───────────── day of week (0 - 6) (Sunday=0)
│ │ │ │ │
│ │ │ │ │
* * * * *
```

### Supported Features

| Feature | Example | Description |
|---------|---------|-------------|
| Wildcard | `*` | Matches any value |
| Specific value | `5` | Exactly 5 |
| Range | `1-5` | Values 1 through 5 |
| List | `1,3,5` | Values 1, 3, and 5 |
| Step | `*/2` | Every 2 units |

### Common Schedules

| Schedule | Cron Expression |
|----------|----------------|
| Every minute | `* * * * *` |
| Every hour | `0 * * * *` |
| Daily at 9 AM | `0 9 * * *` |
| Daily at 5 PM | `0 17 * * *` |
| Every weekday at 9 AM | `0 9 * * 1-5` |
| Every Monday at 9 AM | `0 9 * * 1` |
| Every 15 minutes | `*/15 * * * *` |
| Every hour during work hours | `0 9-17 * * 1-5` |
| Monthly on the 1st | `0 0 1 * *` |
| Monthly on the last day | `0 0 L * *` |
| Every Sunday at midnight | `0 0 * * 0` |

## Configuration

### CLI Options

| Option | Description | Required |
|--------|-------------|----------|
| `--cron` | Cron expression | Yes |
| `--id` | Unique task identifier | No (auto-generated) |
| `--retry` | Retry a failed workflow up to N extra times (0–10, default 0) | No |
| `--retry-delay` | Base backoff delay between retries (default `30s`) | No |
| `--list` | List all scheduled tasks | No |
| `--info` | Show task details | No |
| `--remove` | Remove a task by ID | No |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `AFLARE_SCHEDULER_LOG_FILE` | Path to scheduler log file |

## Scheduled Workflow Example

### YAML Configuration

Workflows can include scheduling metadata:

```yaml
name: daily-report
schedule:
  cron: "0 9 * * *"
  enabled: true

vars:
  api_key: "{{secret.api.service}}"

steps:
  - node: http_request
    params:
      url: "https://api.example.com/metrics"
      headers: "Authorization: Bearer {{var.api_key}}"
    id: fetch_data

  - node: agent
    input: "{{step.fetch_data}}"
    params:
      model: gpt-4o
      prompt: "Generate a daily summary report from this data"
    id: generate_report

  - node: file_write
    params:
      path: "reports/daily-report-{{env.DATE}}.md"
    input: "{{step.generate_report}}"

  - node: notify
    params:
      channel: "email"
      to: "team@example.com"
      subject: "Daily Report - {{env.DATE}}"
      message: "{{step.generate_report}}"
```

### Dynamic Input

Scheduled workflows can use environment variables for dynamic input:

```yaml
name: hourly-check
schedule:
  cron: "0 * * * *"

steps:
  - node: agent
    params:
      model: gpt-4o
      prompt: "Perform hourly health check at {{env.TIME}}. Check system status."
```

## Logs and Monitoring

### Scheduler Logs

```bash
# View scheduler logs
tail -f ~/.aflare/logs/scheduler.log

# Search for specific task logs
grep "daily-report" ~/.aflare/logs/scheduler.log
```

### Log Format

```json
{
  "time": "2026-07-16T09:00:00Z",
  "level": "info",
  "task_id": "daily-report",
  "action": "started",
  "workflow": "daily-report.yaml",
  "cron": "0 9 * * *",
  "next_run": "2026-07-17T09:00:00Z"
}
```

### Task Status

```bash
# Check task status
aflare schedule --info daily-report

# Output:
# Task ID: daily-report
# Workflow: daily-report.yaml
# Cron: 0 9 * * *
# Next Run: 2026-07-17T09:00:00Z
# Status: enabled
# Last Run: 2026-07-16T09:00:00Z
# Last Status: success
```

## Advanced Usage

### Multiple Schedules

A workflow can have multiple schedules by creating separate tasks:

```bash
# Daily morning report
aflare schedule --id morning-report --cron "0 9 * * *" report.yaml

# Daily evening report
aflare schedule --id evening-report --cron "0 18 * * *" report.yaml
```

### Parameterized Schedules

Pass parameters to scheduled workflows:

```bash
aflare schedule --cron "0 9 * * *" --params '{"type":"daily"}' report.yaml
```

### Time Zones

Schedules use the system's local time zone. For specific time zones:

```bash
# Set time zone before scheduling
TZ=UTC aflare schedule --cron "0 9 * * *" report.yaml
```

### Error Handling

Scheduled workflows support the same error handling as regular workflows:

```yaml
name: scheduled-backup
schedule:
  cron: "0 2 * * *"

steps:
  - node: execute
    params:
      command: "backup.sh"
      timeout: "30m"
    fallback: "Backup failed, continuing..."
    continue_on_error: true
```

## Failure Retry

By default a scheduled workflow runs once per trigger and a failure is only
logged (historical behaviour). Workflow-based tasks can opt into automatic
retries with exponential backoff:

```bash
# Retry a failed run up to 3 extra times, starting at 1m backoff
# (1m → 2m → 4m, capped at 5m)
aflare schedule add --cron "0 9 * * *" --retry 3 --retry-delay 1m my-workflow.yaml
```

| Option | Description |
|--------|-------------|
| `--retry <n>` | Retry a failed workflow up to `<n>` extra times (0–10, default 0 = no retry) |
| `--retry-delay <dur>` | Base backoff delay (default `30s`); retries back off exponentially, capped at `5m` |

Behaviour details:

- A panic in the task body counts as a failure and is retried like an error.
- Retries apply to the whole workflow execution; per-step retries inside the
  workflow (`retry` on a step) run first and are independent.
- Scheduler shutdown aborts pending retries (the in-flight attempt's context
  is cancelled) — stopping the scheduler is never blocked by a backoff wait.
- Description-based tasks (executed by `aflare agent`, not the standalone
  scheduler) do not retry; the agent loop owns their failure handling.

## Limitations

- Scheduler must remain running for tasks to execute
- No built-in high availability (consider process managers like systemd)
- Time zone is determined by the scheduler's host system

## Systemd Integration

For production, run the scheduler as a systemd service:

```ini
[Unit]
Description=aflare Scheduler
After=network.target

[Service]
Type=simple
User=youruser
ExecStart=/usr/local/bin/aflare scheduler
Restart=on-failure
Environment=AFLARE_SECRETS_PASSWORD=your-password

[Install]
WantedBy=multi-user.target
```

```bash
# Enable and start
sudo systemctl daemon-reload
sudo systemctl enable aflare-scheduler
sudo systemctl start aflare-scheduler

# Check status
sudo systemctl status aflare-scheduler

# View logs
journalctl -u aflare-scheduler -f
```

## Best Practices

1. **Use descriptive task IDs**: Makes it easier to manage multiple schedules
2. **Test cron expressions**: Verify your cron syntax before deploying
3. **Monitor logs**: Regularly check scheduler logs for failures
4. **Use error handling**: Add `fallback` and `continue_on_error` for robustness
5. **Set appropriate time zones**: Ensure the scheduler runs in your desired time zone
6. **Avoid overlapping runs**: If your workflow takes time, ensure schedules don't overlap
