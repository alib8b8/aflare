# Patient Education Material Generator

> Generate patient education materials in plain language with readability scoring

## Description

This workflow template creates evidence-based patient education materials at appropriate reading levels. It researches medical topics, transforms clinical content into plain language, calculates readability metrics (Flesch-Kincaid grade level), and generates materials with key points, doctor questions, and references.

## Usage

```bash
aflare install healthcare/patient-education
aflare run patient-education/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| topic | Education topic title | Yes |
| condition | Medical condition | Yes |
| treatment | Treatment or procedure | No |
| target_audience | patient, caregiver, pediatric, geriatric | Yes |
| reading_level | Target reading grade level | Yes |
| language | Output language | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for clinical research and patient-friendly writing
- code_interpreter - Readability scoring and metrics calculation
- file_write - Save patient education material

## Category

healthcare