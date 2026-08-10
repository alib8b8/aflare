# Clinical Trial Matcher

> Match patients to relevant clinical trials using ClinicalTrials.gov data

## Description

This workflow template matches patients to recruiting clinical trials based on their condition, demographics, genetic markers, and treatment history. It queries ClinicalTrials.gov, uses AI to score eligibility matches, and produces a ranked list of suitable trials with contact information.

## Usage

```bash
aflare install healthcare/clinical-trial-matcher
aflare run clinical-trial-matcher/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| condition | Medical condition/diagnosis | Yes |
| age | Patient age | Yes |
| gender | Patient gender | No |
| stage | Disease stage or severity | No |
| prior_treatments | Previous treatments received | No |
| genetic_markers | Relevant genetic markers | No |
| location | Patient city/state/zip | Yes |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for criteria extraction and trial matching
- http_request - ClinicalTrials.gov API search
- code_interpreter - Match score ranking and categorization
- transform - Report formatting with trial details
- file_write - Save matched trials report

## Category

healthcare