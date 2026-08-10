# Hospital Discharge Summary Generator

> Generate comprehensive hospital discharge summaries with patient instructions

## Description

This workflow template creates structured hospital discharge summaries from clinical data. It uses dual AI agents to generate both the clinical summary for medical records and patient-friendly discharge instructions. Includes length-of-stay calculation and medication reconciliation.

## Usage

```bash
aflare install healthcare/discharge-summary
aflare run discharge-summary/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| patient_name | Patient full name | Yes |
| age | Patient age | Yes |
| gender | Patient gender | No |
| mrn | Medical record number | Yes |
| admission_date | Date of admission (YYYY-MM-DD) | Yes |
| discharge_date | Date of discharge (YYYY-MM-DD) | Yes |
| admitting_diagnosis | Initial diagnosis | Yes |
| hospital_course | Summary of hospital stay | Yes |
| procedures | Procedures performed | No |
| significant_findings | Key clinical findings | No |
| discharge_diagnoses | Final diagnoses | Yes |
| disposition | Discharge disposition | Yes |
| attending | Attending physician name | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for clinical summary and patient instructions
- code_interpreter - Length of stay calculation
- file_write - Save discharge summary
- notify - Send completion notification

## Category

healthcare