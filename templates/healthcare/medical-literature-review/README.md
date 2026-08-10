# Medical Literature Search & Review

> Search and review medical literature from PubMed with evidence grading

## Description

This workflow template performs comprehensive medical literature searches using PubMed's E-utilities API. It builds structured PICO-based search strategies, retrieves and analyzes articles, assesses study quality and evidence levels, and synthesizes findings into a graded evidence summary with clinical recommendations.

## Usage

```bash
aflare install healthcare/medical-literature-review
aflare run medical-literature-review/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| clinical_question | PICO-formatted clinical question | Yes |
| study_type | RCT, meta-analysis, cohort, etc. | No |
| date_range | Publication date range | No |
| population | Target patient population | No |
| intervention | Intervention or exposure | Yes |
| outcome | Outcome of interest | Yes |
| search_date | Date of search | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for search strategy and literature synthesis
- http_request - PubMed E-utilities (esearch, esummary)
- code_interpreter - Evidence level assessment and quality scoring
- file_write - Save literature review

## Category

healthcare