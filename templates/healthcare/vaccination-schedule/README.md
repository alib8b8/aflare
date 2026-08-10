# Vaccination Schedule Planner

> Plan vaccination schedules with catch-up, travel, and occupational vaccines

## Description

This workflow template creates personalized vaccination schedules based on CDC/WHO guidelines. It considers patient age, previous vaccinations, medical conditions, travel plans, occupation, pregnancy status, and immunocompromised status. It identifies due, overdue, and upcoming vaccines, calculates dates, and provides patient education.

## Usage

```bash
aflare install healthcare/vaccination-schedule
aflare run vaccination-schedule/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| age | Patient age | Yes |
| age_unit | years, months, or weeks | Yes |
| previous_vaccinations | Previously received vaccines | Yes |
| medical_conditions | Relevant medical conditions | No |
| travel_plans | Upcoming travel destinations | No |
| occupation | Patient occupation | No |
| pregnancy_status | Pregnancy status | No |
| immunocompromised | Is patient immunocompromised | No |
| country | Country for guideline reference | Yes |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for vaccine recommendations and patient education
- code_interpreter - Schedule calculation and date math
- file_write - Save vaccination schedule
- notify - Send schedule summary

## Category

healthcare