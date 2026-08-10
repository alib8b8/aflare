# Deposition Transcript Summarizer

## Description
A deposition transcript summarizer that produces comprehensive, executive, topic-based, or chronological summaries. Extracts key testimony indexed by topic, identifies contradictions and impeachment opportunities, and provides cross-examination strategies for trial preparation.

## Usage Example
```yaml
workflow: legal/deposition-summary
params:
  case_name: "Johnson v. MegaCorp"
  deponent_name: "Robert Williams"
  deposition_date: "2025-06-15"
  transcript_text: "Q: State your name... [full transcript]"
  summary_type: "comprehensive"
  key_topics: ["safety protocols", "incident timeline", "training records"]
  output_path: "output/deposition_summary.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| case_name | string | Yes | - | Name of the case |
| deponent_name | string | Yes | - | Name of the deponent |
| deposition_date | string | Yes | - | Date of deposition (YYYY-MM-DD) |
| transcript_text | string | No | - | Full deposition transcript text |
| transcript_url | string | No | - | URL to the deposition transcript |
| summary_type | string | No | comprehensive | Summary type (comprehensive, executive, topic, chronological) |
| key_topics | array | No | [] | Key topics to focus on |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/deposition_summary.md | Output file path |

## Nodes Used
- **http_request** (fetch_transcript): Fetches deposition transcript from URL
- **agent** (summarize_deposition): Summarizes the deposition transcript
- **agent** (extract_testimony): Extracts and indexes key testimony by topic
- **agent** (impeachment_analysis): Identifies impeachment material and contradictions
- **file_write** (save_summary): Saves the deposition summary

## Category
legal