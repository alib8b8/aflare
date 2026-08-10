# Study Guide Generator

Comprehensive study guide generator with chapter summaries, key concepts, practice questions, glossaries, and study strategies.

## Description

This workflow creates detailed study guides from topic lists, incorporating academic reference lookups, key concept extraction, practice questions, and study strategy recommendations. It supports multiple depth levels and target audiences.

## Usage Example

```yaml
params:
  subject: "Organic Chemistry"
  topics: ["Functional Groups", "Reaction Mechanisms", "Stereochemistry", "Spectroscopy"]
  depth: "comprehensive"
  include_practice: true
  format: "markdown"
  target_audience: "student"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| subject | string | yes | - | Subject/course name |
| topics | array | yes | - | Topics to cover |
| depth | string | no | comprehensive | Detail level |
| include_practice | boolean | no | true | Include practice questions |
| format | string | no | markdown | Output format |
| target_audience | string | no | student | Target audience |

## Nodes Used

- `http_request` - Fetch academic reference works
- `agent` - Build comprehensive study guide
- `code_interpreter` - Generate guide metadata and statistics
- `file_write` - Save study guide to file

## Category

Education