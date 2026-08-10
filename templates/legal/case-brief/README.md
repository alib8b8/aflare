# Legal Case Brief Generator

## Description
A legal case brief generator that creates structured case briefs from court opinions. Supports URL fetching or direct text input, extracts case metadata and procedural history, and generates comprehensive briefs in standard, detailed, or student formats with IRAC structure.

## Usage Example
```yaml
workflow: legal/case-brief
params:
  case_url: "https://www.courtlistener.com/opinion/12345/smith-v-jones/"
  case_name: "Smith v. Jones"
  brief_format: "detailed"
  output_path: "output/smith_v_jones_brief.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| case_text | string | No | - | Full text of the court opinion |
| case_url | string | No | - | URL to the court opinion |
| case_name | string | No | - | Name of the case |
| brief_format | string | No | standard | Brief format (standard, detailed, student) |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/case_brief.md | Output file path |

## Nodes Used
- **http_request** (fetch_opinion): Fetches court opinion from URL
- **agent** (extract_metadata): Extracts case metadata and procedural history
- **agent** (generate_brief): Generates the structured case brief
- **file_write** (save_brief): Saves the case brief document

## Category
legal