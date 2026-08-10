# Physiotherapy Exercise Plan Generator

> Generate personalized physiotherapy exercise plans with progression tracking

## Description

This workflow template creates individualized physiotherapy exercise programs based on diagnosis, injury phase, pain levels, and functional assessments. It calculates appropriate sets, reps, and frequency by phase (acute/subacute/chronic), generates detailed exercise instructions with form guidance, and provides progression milestones.

## Usage

```bash
aflare install healthcare/physiotherapy-plan
aflare run physiotherapy-plan/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| diagnosis | Medical diagnosis | Yes |
| body_area | Affected body area | Yes |
| injury_date | Date of injury or surgery | Yes |
| pain_level | Pain level 0-10 | Yes |
| range_of_motion | ROM assessment findings | No |
| strength_assessment | Strength assessment results | No |
| functional_limitations | Current functional limitations | No |
| patient_goals | Patient's rehabilitation goals | Yes |
| age | Patient age | No |
| activity_level | Pre-injury activity level | No |
| prior_therapy | Previous physiotherapy | No |
| contraindications | Exercise contraindications | No |
| therapist_name | Treating physiotherapist | No |
| output_path | Output file path | No |

## Nodes Used

- agent - AI agent for treatment planning and exercise instructions
- code_interpreter - Phase determination and exercise parameter calculation
- file_write - Save physiotherapy plan

## Category

healthcare