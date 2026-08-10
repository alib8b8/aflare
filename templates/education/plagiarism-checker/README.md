# Plagiarism Checker

Text similarity and plagiarism detection with web search, detailed similarity reports, risk assessment, and proper citation recommendations.

## Description

This workflow checks text for potential plagiarism by searching the web, comparing against reference texts, and analyzing similarity patterns. It distinguishes between exact matches, paraphrased content, and structural similarity, providing a risk assessment and citation recommendations.

## Usage Example

```yaml
params:
  text: "Climate change is one of the most pressing issues of our time..."
  reference_texts: []
  check_web: true
  sensitivity: "medium"
  min_match_length: 10
  report_format: "detailed"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| text | string | yes | - | Text to check |
| reference_texts | array | no | [] | Reference texts |
| check_web | boolean | no | true | Search web for matches |
| sensitivity | string | no | medium | Detection sensitivity |
| min_match_length | integer | no | 10 | Minimum word sequence |
| report_format | string | no | detailed | Report detail level |

## Nodes Used

- `http_request` - Search web for similar content
- `agent` - Analyze text similarity and generate report
- `code_interpreter` - Compute similarity scores and risk assessment
- `file_write` - Save plagiarism report to file

## Category

Education