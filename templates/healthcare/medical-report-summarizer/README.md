# Medical Report Summarizer

> Summarize complex medical reports into patient-friendly language

## Description

This workflow template takes complex medical reports (lab results, radiology reports, pathology reports, etc.) and transforms them into easy-to-understand patient summaries. It uses a two-stage agent pipeline: first extracting clinical findings, then translating them into plain language at an appropriate reading level.

## Usage

```bash
aflare install healthcare/medical-report-summarizer
aflare run medical-report-summarizer/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| report_type | Type of medical report (lab, radiology, pathology, etc.) | Yes |
| report_content | Full text of the medical report | Yes |
| specialty | Medical specialty context | No |
| report_date | Date of the report | No |
| patient_name | Patient name for the summary | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for clinical analysis and patient-friendly translation
- transform - Template-based formatting of the final report
- file_write - Save the summarized report to file
- notify - Send completion notification

## Category

healthcare