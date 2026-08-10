# Mental Health Screening Questionnaire

> Administer mental health screening with validated assessment tools (PHQ-9, GAD-7, etc.)

## Description

This workflow template administers validated mental health screening tools (PHQ-9, GAD-7, PHQ-2, etc.), scores responses against clinical thresholds, identifies critical items requiring immediate attention, compares with previous scores, and generates patient-friendly guidance with crisis resources.

## Usage

```bash
aflare install healthcare/mental-health-screener
aflare run mental-health-screener/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| screening_tool | Assessment tool (PHQ-9, GAD-7, PHQ-2) | Yes |
| responses | Patient questionnaire responses | Yes |
| demographics | Patient demographic information | No |
| reason | Reason for screening | No |
| previous_scores | Prior screening scores | No |
| screening_date | Date of screening | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for screening administration and patient guidance
- code_interpreter - Score calculation and threshold comparison
- file_write - Save screening report with crisis resources

## Category

healthcare