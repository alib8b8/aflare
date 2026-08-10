# ICD-10 Medical Coding Assistant

> AI-powered ICD-10 medical coding assistant with code validation

## Description

This workflow template automates ICD-10-CM medical coding from clinical documentation. It extracts diagnoses and procedures from clinical notes, looks up ICD-10 codes via NIH API, maps concepts to appropriate codes with guidelines, validates code format, and identifies HCC/risk adjustment opportunities and documentation gaps.

## Usage

```bash
aflare install healthcare/medical-coding
aflare run medical-coding/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| clinical_notes | Full clinical documentation | Yes |
| encounter_type | inpatient, outpatient, ED, etc. | Yes |
| specialty | Medical specialty context | No |
| provider_notes | Provider-specific notes | No |
| primary_diagnosis | Primary diagnosis for lookup | Yes |
| encounter_date | Date of encounter | No |
| code_year | ICD-10-CM code year | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for clinical extraction and code mapping
- http_request - NIH ICD-10-CM code lookup API
- code_interpreter - Code format validation and metrics
- transform - Report formatting
- file_write - Save coding report

## Category

healthcare