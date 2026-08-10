# Radiology Report Generator

> Generate and analyze radiology reports with structured findings and patient summary

## Description

This workflow template creates structured radiology reports from imaging findings. It supports all modalities (X-ray, CT, MRI, ultrasound, etc.), generates detailed findings organized by anatomical structure, categorizes them by severity, produces a clinical impression with differential diagnoses, and creates a patient-friendly layperson summary.

## Usage

```bash
aflare install healthcare/radiology-report
aflare run radiology-report/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| modality | Imaging modality (X-ray, CT, MRI, US, etc.) | Yes |
| body_region | Anatomical region imaged | Yes |
| clinical_indication | Reason for the imaging study | Yes |
| findings | Imaging findings description | Yes |
| comparison | Prior comparison studies | No |
| technique | Imaging protocol/technique | No |
| exam_date | Date of examination | No |
| ordering_provider | Ordering physician name | No |
| radiologist | Reviewing radiologist name | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for findings analysis and patient summary
- code_interpreter - Finding categorization and severity scoring
- transform - Structured report formatting
- file_write - Save radiology report

## Category

healthcare