# Drug Interaction Checker

> Check drug interactions and contraindications using FDA and RxNav data

## Description

This workflow template performs comprehensive drug interaction screening by querying FDA drug labels and RxNav interaction APIs, then using AI agents to analyze and cross-reference findings. It checks drug-drug, drug-condition, drug-allergy interactions, and provides risk assessment with severity scoring.

## Usage

```bash
aflare install healthcare/drug-interaction-checker
aflare run drug-interaction-checker/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| drug_name | Primary drug to check | Yes |
| patient_medications | List of all patient medications | Yes |
| rxcui_list | RxNorm concept unique identifiers | No |
| age | Patient age | No |
| conditions | Existing medical conditions | No |
| allergies | Known allergies | No |
| pregnancy | Pregnancy status | No |
| renal_function | Renal function status | No |
| hepatic_function | Hepatic function status | No |
| output_path | Output file path | No |

## Nodes Used

- http_request - FDA drug label API and RxNav interaction API
- agent - AI agent for interaction analysis and clinical reasoning
- code_interpreter - Risk level calculation and severity counting
- file_write - Save interaction report

## Category

healthcare