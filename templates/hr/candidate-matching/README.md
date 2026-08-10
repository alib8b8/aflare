# Candidate Matching

Multi-dimensional job-candidate matching and scoring workflow.

## Description

Compute candidate fit across five dimensions (skills, experience, education, location, salary) using weighted scoring, then generate actionable insights about the talent pool. Supports configurable weights and match thresholds.

## Usage Example

```yaml
params:
  job_requisition:
    title: "Senior Data Engineer"
    required_skills: ["Python", "SQL", "Spark", "Airflow"]
    preferred_skills: ["Kafka", "dbt", "Snowflake"]
    min_experience: 5
    preferred_experience: 8
    required_education: "bachelor"
    location: "San Francisco, CA"
    remote: true
    salary_range: [140000, 190000]
  candidate_pool:
    - id: "C001"
      name: "Alice Chen"
      skills: ["Python", "SQL", "Spark", "Airflow", "Kafka"]
      years_of_experience: 7
      education_level: "master"
      location: "San Francisco, CA"
      expected_salary: 165000
  match_threshold: 60
  weights:
    skills: 0.35
    experience: 0.25
    education: 0.15
    location: 0.10
    salary_fit: 0.15
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| job_requisition | object | Yes | - | Job details with requirements, skills, salary range |
| candidate_pool | array | Yes | - | Array of structured candidate profiles |
| match_threshold | number | No | 60 | Minimum match score to include |
| weights | object | No | see workflow | Weight distribution across dimensions |
| output_file | string | No | candidate_matching_report.json | Output file path |

## Nodes Used

- **agent**: Normalize candidate profiles; generate talent pool insights
- **code_interpreter**: Compute multi-dimensional match scores with weighted algorithm
- **file_write**: Save aggregated match report as JSON

## Category

HR > Recruitment