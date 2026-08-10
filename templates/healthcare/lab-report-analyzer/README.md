# Lab Report Analyzer

> Analyze and explain lab test results with clinical interpretation

## Description

This workflow template provides comprehensive lab report analysis, interpreting test results against reference ranges, calculating abnormality statistics, identifying trends against prior values, and generating patient-friendly explanations. It supports all common lab panels including CBC, CMP, lipid panel, thyroid, and more.

## Usage

```bash
aflare install healthcare/lab-report-analyzer
aflare run lab-report-analyzer/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| lab_panel | Type of lab panel (CBC, CMP, etc.) | Yes |
| test_results | JSON array of test results with values | Yes |
| age | Patient age | Yes |
| gender | Patient gender | No |
| fasting | Whether patient was fasting | No |
| lab_date | Date of lab draw | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for clinical interpretation and patient explanation
- code_interpreter - Abnormality statistics and trend calculation
- file_write - Save lab report analysis

## Category

healthcare