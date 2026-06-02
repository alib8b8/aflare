# 10 30-Second Demo Scenarios

---

## Scenario 1: Daily GitHub Summary

### User Input
```bash
llm-box create "fetch my recent GitHub activity and save to summary.md"
```

### Generated Workflow
```yaml
name: Daily GitHub Summary
steps:
  - node: fetch_url
    params:
      url: https://github.com/your-username
  - node: transform
    params:
      operation: extract_repos_and_commits
  - node: file_write
    params:
      path: summary.md
```

### Execution
```bash
llm-box run summary-workflow.yaml
```

### Output
```
✅ workflow completed in 4.2s
Saved to summary.md
```

---

## Scenario 2: Download Organizer

### User Input
```bash
llm-box create "organize my downloads folder by file type"
```

### Generated Workflow
```yaml
name: Download Organizer
steps:
  - node: execute
    params:
      command: ls -la ~/Downloads
  - node: transform
    params:
      operation: group_by_extension
  - node: execute
    params:
      command: mkdir -p ~/Downloads/images ~/Downloads/docs ~/Downloads/archives
  - node: execute
    params:
      command: mv ~/Downloads/*.jpg ~/Downloads/*.png ~/Downloads/images/
```

### Execution
```bash
llm-box run organize-downloads.yaml
```

### Output
```
✅ workflow completed in 2.8s
Moved 12 files to organized folders
```

---

## Scenario 3: Release Notes

### User Input
```bash
llm-box create "turn git commits from last 2 weeks into release notes"
```

### Generated Workflow
```yaml
name: Release Notes Generator
steps:
  - node: execute
    params:
      command: git log --oneline --since="2 weeks ago"
  - node: transform
    params:
      operation: group_by_commit_type
  - node: file_write
    params:
      path: RELEASE-NOTES.md
```

### Execution
```bash
llm-box run release-notes.yaml
```

### Output
```
✅ workflow completed in 3.1s
RELEASE-NOTES.md generated successfully
```

---

## Scenario 4: Weather Report

### User Input
```bash
llm-box create "fetch 7-day weather forecast and save to report"
```

### Generated Workflow
```yaml
name: Daily Weather Report
steps:
  - node: fetch_url
    params:
      url: https://api.weather.gov/gridpoints/SFO/98,41/forecast
  - node: transform
    params:
      operation: extract_forecast_summary
  - node: file_write
    params:
      path: weather-report.md
```

### Execution
```bash
llm-box run weather-report.yaml
```

### Output
```
✅ workflow completed in 1.9s
Saved 7-day forecast to weather-report.md
```

---

## Scenario 5: Blog Post Prep

### User Input
```bash
llm-box create "convert markdown post to HTML and add header"
```

### Generated Workflow
```yaml
name: Blog Post Processor
steps:
  - node: fetch_url
    params:
      url: file://blog-post.md
  - node: transform
    params:
      operation: markdown_to_html
  - node: file_write
    params:
      path: blog-post.html
```

### Execution
```bash
llm-box run blog-prep.yaml
```

### Output
```
✅ workflow completed in 0.9s
Saved to blog-post.html
```

---

## Scenario 6: Server Health Check

### User Input
```bash
llm-box create "check local server health and log results"
```

### Generated Workflow
```yaml
name: Server Health Check
steps:
  - node: execute
    params:
      command: curl -f http://localhost/health
  - node: execute
    params:
      command: df -h
  - node: file_write
    params:
      path: health-log.txt
```

### Execution
```bash
llm-box run health-check.yaml
```

### Output
```
✅ workflow completed in 2.3s
Health check passed. Saved log to health-log.txt
```

---

## Scenario 7: File Backup

### User Input
```bash
llm-box create "backup my documents folder to archives"
```

### Generated Workflow
```yaml
name: Document Backup
steps:
  - node: execute
    params:
      command: tar -czf docs-backup-$(date +%Y%m%d).tar.gz ~/Documents
  - node: execute
    params:
      command: mv docs-backup-*.tar.gz ~/Archives/
  - node: notify
    params:
      channel: stdout
      message: Backup completed successfully
```

### Execution
```bash
llm-box run backup-docs.yaml
```

### Output
```
✅ workflow completed in 8.4s
Backup saved to ~/Archives/
```

---

## Scenario 8: Project Stats

### User Input
```bash
llm-box create "count lines of code in my project and save stats"
```

### Generated Workflow
```yaml
name: Project Stats Generator
steps:
  - node: execute
    params:
      command: wc -l $(find . -name "*.go" -name "*.ts" -name "*.py" 2>/dev/null)
  - node: file_write
    params:
      path: project-stats.txt
```

### Execution
```bash
llm-box run project-stats.yaml
```

### Output
```
✅ workflow completed in 1.2s
Total lines of code: 18,472
Stats saved to project-stats.txt
```

---

## Scenario 9: Content Aggregator

### User Input
```bash
llm-box create "fetch 3 tech articles and combine into digest"
```

### Generated Workflow
```yaml
name: Tech Digest Generator
steps:
  - node: fetch_url
    params:
      url: https://news.ycombinator.com/rss
  - node: fetch_url
    params:
      url: https://lobste.rs/rss
  - node: combine
    params:
      format: markdown
  - node: file_write
    params:
      path: tech-digest.md
```

### Execution
```bash
llm-box run tech-digest.yaml
```

### Output
```
✅ workflow completed in 5.6s
Saved digest to tech-digest.md
```

---

## Scenario 10: Test Runner

### User Input
```bash
llm-box create "run all tests and email results if failures"
```

### Generated Workflow
```yaml
name: Test Runner & Notifier
steps:
  - node: execute
    params:
      command: go test ./...
  - node: transform
    params:
      operation: extract_test_results
  - node: notify
    params:
      channel: stdout
```

### Execution
```bash
llm-box run test-and-notify.yaml
```

### Output
```
✅ workflow completed in 12.8s
All 82 tests passed
```
