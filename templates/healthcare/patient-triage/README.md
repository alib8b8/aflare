# Patient Triage & Urgency Assessment

> AI-powered patient triage with urgency scoring and recommendation

## Description

This workflow template provides an AI-driven patient triage system that assesses chief complaints, symptoms, and vital signs to produce an ESI-based triage level with urgency scoring. It uses a multi-step pipeline combining agent-based analysis, code-based scoring calculations, and structured report generation.

## Usage

```bash
aflare install healthcare/patient-triage
aflare run patient-triage/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| chief_complaint | Primary reason for visit | Yes |
| symptoms | Detailed symptom description | Yes |
| symptom_onset | When symptoms began | Yes |
| vital_signs | Blood pressure, HR, temp, O2, RR | Yes |
| medical_history | Relevant medical history | No |
| age | Patient age | Yes |
| gender | Patient gender | No |
| patient_id | Patient identifier | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for triage assessment and clinical reasoning
- code_interpreter - Python-based urgency score calculation
- transform - Template-based report formatting
- file_write - Save triage report to file
- notify - Send triage notification

## Category

healthcare