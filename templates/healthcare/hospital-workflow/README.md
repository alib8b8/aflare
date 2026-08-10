# Hospital Department Workflow Automation

> Automate and optimize hospital department workflows with efficiency analysis

## Description

This workflow template analyzes hospital department workflows, identifies bottlenecks, calculates efficiency metrics, and generates optimized workflows with implementation plans. It covers process flow mapping, wait time analysis, resource utilization, staffing recommendations, and quantified improvement projections.

## Usage

```bash
aflare install healthcare/hospital-workflow
aflare run hospital-workflow/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| department | Hospital department name | Yes |
| current_workflow | Current workflow description | Yes |
| staff_count | Number of staff members | Yes |
| patient_volume | Daily patient volume | Yes |
| avg_wait_time | Average patient wait time (minutes) | Yes |
| peak_hours | Peak operating hours | No |
| resources | Available resources and equipment | No |
| kpis | Current KPI metrics | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for workflow analysis and optimization planning
- code_interpreter - Efficiency metrics and staffing calculations
- file_write - Save workflow optimization report

## Category

healthcare