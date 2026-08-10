# A/B Test Plan Generator

> Design and analyze A/B tests for marketing campaigns

## Description

This workflow template generates comprehensive A/B testing plans with statistical rigor. It helps marketers design experiments, calculate sample sizes, and establish proper decision criteria for campaign optimization.

## Usage

```bash
aflare install marketing/ab-test-plan
aflare run ab-test-plan/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| goal | Campaign optimization goal | Yes |
| channel | Marketing channel to test | Yes |
| metric | Primary metric to measure | Yes |
| budget | Test budget allocation | Yes |
| duration | Expected test duration | Yes |
| baseline_rate | Current conversion rate (0-1) | Yes |
| mde | Minimum detectable effect | Yes |
| alpha | Significance level (default 0.05) | No |
| power | Statistical power (default 0.80) | No |
| daily_visitors | Daily traffic volume | No |
| variants | Number of test variants | No |

## Nodes Used

- agent - AI agent for hypothesis generation and test design
- code_interpreter - Python-based sample size and statistical calculations
- template_render - Template rendering with Go templates
- json_parse - Parse JSON responses
- file_write - Write output to files

## Category

marketing