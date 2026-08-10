# Disease Outbreak & Epidemiology Tracker

> Track disease outbreaks and epidemiological trends with risk assessment

## Description

This workflow template performs epidemiological surveillance by querying disease data APIs, calculating key metrics (total cases, deaths, CFR, moving averages, trends), and generating public health reports with risk assessments and recommendations. It supports outbreak detection and trend analysis.

## Usage

```bash
aflare install healthcare/epidemiology-tracker
aflare run epidemiology-tracker/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| disease | Disease or condition to track | Yes |
| geographic_area | Country, state, or region | Yes |
| time_period | Surveillance time period | Yes |
| population | Target population description | No |
| surveillance_goals | Goals of surveillance | No |
| last_days | Number of days to look back | Yes |
| report_date | Date of report generation | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for surveillance planning and report generation
- http_request - Disease data API
- code_interpreter - Epidemiological metric calculation and trend detection
- file_write - Save epidemiology report

## Category

healthcare