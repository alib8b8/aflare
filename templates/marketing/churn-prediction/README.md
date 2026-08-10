# Churn Prediction & Retention

> Predict customer churn and generate retention strategies

## Description

This workflow template predicts customer churn risk using a multi-factor scoring model in Python, then generates tiered retention strategies. It analyzes inactivity, support tickets, usage decline, NPS scores, and payment issues to identify at-risk customers and at-risk revenue.

## Usage

```bash
aflare install marketing/churn-prediction
aflare run churn-prediction/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| product | Product or service name | Yes |
| industry | Industry context | Yes |

## Nodes Used

- agent - AI agent for retention strategy generation
- http_request - Fetch customer data from CRM
- code_interpreter - Python-based churn risk scoring model
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing