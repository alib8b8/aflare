# Medical Billing & Coding Workflow

> Automated medical billing and coding workflow with validation and error checking

## Description

This workflow template automates the medical billing process from clinical documentation. It extracts ICD-10 and CPT/HCPCS codes, validates against NCCI edits and payer guidelines, calculates claim readiness scores, estimates RVUs, and identifies errors and warnings before claim submission.

## Usage

```bash
aflare install healthcare/medical-billing
aflare run medical-billing/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| clinical_documentation | Full clinical encounter notes | Yes |
| encounter_type | inpatient, outpatient, ED, etc. | Yes |
| place_of_service | POS code and location | Yes |
| provider | Rendering provider NPI/name | Yes |
| insurance_type | Medicare, Medicaid, commercial, etc. | Yes |
| encounter_date | Date of service | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for code extraction and validation
- code_interpreter - Claim readiness scoring and RVU calculation
- transform - Billing report formatting
- file_write - Save billing report

## Category

healthcare