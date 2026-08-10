# Prescription Validator

> Validate prescriptions for safety, dosing, interactions, and contraindications

## Description

This workflow template performs comprehensive prescription validation including dose checking, allergy cross-reactivity, drug interactions, renal/hepatic adjustments, pregnancy/lactation safety, therapeutic duplication, and FDA label cross-referencing. It generates a safety score and clear dispensing recommendation.

## Usage

```bash
aflare install healthcare/prescription-validator
aflare run prescription-validator/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| medication | Medication name (brand or generic) | Yes |
| dose | Prescribed dose | Yes |
| frequency | Dosing frequency | Yes |
| route | Route of administration | Yes |
| duration | Treatment duration | Yes |
| age | Patient age | Yes |
| weight_kg | Patient weight in kg | No |
| diagnosis | Indication/diagnosis | No |
| crcl | Creatinine clearance (renal function) | No |
| liver_function | Liver function status | No |
| allergies | Known allergies | No |
| current_medications | Current medication list | No |
| pregnancy_status | Pregnancy/lactation status | No |
| patient_name | Patient name | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for prescription validation and FDA cross-reference
- http_request - FDA drug label API
- code_interpreter - Safety score calculation
- file_write - Save validation report

## Category

healthcare