# Symptom Checker

> AI symptom analysis and possible condition suggestions with drug safety cross-reference

## Description

This workflow template provides AI-powered symptom analysis, generating differential diagnoses, checking for drug-related side effects via FDA data, and calculating a concern index to recommend appropriate care levels. It combines agent-based clinical reasoning with external API data and code-based scoring.

## Usage

```bash
aflare install healthcare/symptom-checker
aflare run symptom-checker/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| symptoms | List of symptoms | Yes |
| duration | How long symptoms have persisted | Yes |
| severity | mild, moderate, severe, or critical | Yes |
| age | Patient age | Yes |
| gender | Patient gender | No |
| existing_conditions | Known medical conditions | No |
| medications | Current medications | No |
| recent_travel | Recent travel history | No |
| lifestyle | Lifestyle factors | No |
| assessment_date | Date of assessment | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for symptom analysis and drug cross-referencing
- http_request - FDA drug safety API lookup
- code_interpreter - Concern index calculation
- transform - Report formatting
- file_write - Save symptom report

## Category

healthcare