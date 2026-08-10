# Anomaly Detector

> Detect anomalies in datasets using statistical and ML methods

## Description

This workflow template provides a ready-to-use solution for detect anomalies in datasets using statistical and ml methods.

## Usage

```bash
aflare install data-ai/anomaly-detector
aflare run anomaly-detector/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| dataset_path | Path to the CSV dataset file | Yes |
| dataset_name | Name/label for the dataset | No |

## Nodes Used

- execute - Execute Python scripts for statistical analysis
- agent - AI agent node for intelligent processing
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

data-ai