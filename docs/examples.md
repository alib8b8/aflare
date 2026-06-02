# llm-box Example Workflows

This document explains the 10 example workflows included in llm-box.

---

## 1. Daily GitHub Summary

**File**: `examples/github-summary.yaml`

**What it does**:
Fetches your recent GitHub activity and saves a summary to a markdown file.

**Usage**:
```bash
llm-box run examples/github-summary.yaml
```

**Customization**:
Change the GitHub username in the `fetch_url` node.

---

## 2. Research Assistant

**File**: `examples/research-assistant.yaml`

**What it does**:
Fetches 3 technical articles and combines them into a single summary document.

**Usage**:
```bash
llm-box run examples/research-assistant.yaml
```

**Customization**:
Update the URLs in the `fetch_url` nodes to point to the articles you want.

---

## 3. Documentation Generator

**File**: `examples/docs-generator.yaml`

**What it does**:
Scans your project's source files and generates an API overview document.

**Usage**:
```bash
llm-box run examples/docs-generator.yaml
```

**Customization**:
Modify the `find` command to include the file types you want to scan.

---

## 4. Log Monitor

**File**: `examples/log-monitor.yaml`

**What it does**:
Tails your server log file, filters for errors, and alerts you.

**Usage**:
```bash
llm-box run examples/log-monitor.yaml
```

**Customization**:
Change the log file path in the `execute` node.

---

## 5. Release Notes Creator

**File**: `examples/release-notes.yaml`

**What it does**:
Takes git commit history from the last 2 weeks and generates structured release notes.

**Usage**:
```bash
llm-box run examples/release-notes.yaml
```

**Customization**:
Adjust the `--since` parameter in the `git log` command.

---

## 6. Data Collector

**File**: `examples/data-collector.yaml`

**What it does**:
Fetches data from multiple APIs and combines it into a single report.

**Usage**:
```bash
llm-box run examples/data-collector.yaml
```

**Customization**:
Update the API URLs to the endpoints you want to fetch from.

---

## 7. File Organizer

**File**: `examples/file-organizer.yaml`

**What it does**:
Organizes your Downloads folder by file type.

**Usage**:
```bash
llm-box run examples/file-organizer.yaml
```

**Customization**:
Add/remove file extensions and folders as needed.

---

## 8. Content Workflow

**File**: `examples/content-workflow.yaml`

**What it does**:
Takes a markdown blog post, converts it to HTML, and adds a standard header.

**Usage**:
```bash
llm-box run examples/content-workflow.yaml
```

**Customization**:
Change the input file path in the `fetch_url` node.

---

## 9. DevOps Automation

**File**: `examples/devops-automation.yaml`

**What it does**:
Builds a Docker image, deploys it, and runs a health check.

**Usage**:
```bash
llm-box run examples/devops-automation.yaml
```

**Customization**:
Update the Docker image name and health check URL.

---

## 10. Team Reporting

**File**: `examples/team-reporting.yaml`

**What it does**:
Compiles weekly stats: issues closed, PRs merged, commits made.

**Usage**:
```bash
llm-box run examples/team-reporting.yaml
```

**Customization**:
Change the repository and date range in the commands.

---

## How to Share Your Workflows

1. Fork the repo
2. Add your workflow to the `examples/` directory
3. Open a PR!

We'd love to see what you build.
