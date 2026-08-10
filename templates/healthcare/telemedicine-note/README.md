# Telemedicine Visit Note Generator

> Generate structured telemedicine visit documentation with quality review

## Description

This workflow template produces comprehensive SOAP notes for telemedicine encounters. It generates structured documentation with virtual exam limitations, performs quality review scoring, suggests appropriate billing levels, and ensures all telemedicine-specific requirements (consent, location, emergency instructions) are included.

## Usage

```bash
aflare install healthcare/telemedicine-note
aflare run telemedicine-note/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| patient_name | Patient full name | Yes |
| age | Patient age | Yes |
| gender | Patient gender | No |
| visit_date | Date of telemedicine visit | Yes |
| visit_type | new patient, follow-up, urgent, etc. | Yes |
| chief_complaint | Primary reason for visit | Yes |
| symptoms | Patient-reported symptoms | Yes |
| platform | Telemedicine platform used | Yes |
| connection_quality | Video/audio quality notes | No |
| patient_location | Patient's physical location | Yes |
| provider_name | Provider name | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for SOAP note generation and quality review
- code_interpreter - Quality score calculation and billing level suggestion
- file_write - Save telemedicine visit note

## Category

healthcare