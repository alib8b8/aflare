# Resume Screener

AI-powered resume screening and ranking workflow for high-volume recruitment.

## Description

Automatically parse, filter, score, and rank candidate resumes against a job description. Uses AI to extract structured data from resumes, apply filtering criteria, score candidates on multiple dimensions, and generate a ranked report with recommendations.

## Usage Example

```yaml
params:
  job_description: "Senior Software Engineer with 5+ years in Python..."
  resumes:
    - name: "Jane Doe"
      text: "Experienced software engineer..."
    - name: "John Smith"
      text: "Full-stack developer..."
  top_n: 5
  required_skills: ["Python", "AWS", "Kubernetes"]
  min_experience_years: 3
  output_file: "screening_results.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| job_description | string | Yes | - | Full job description text |
| resumes | array | Yes | - | List of resume objects with candidate name and text |
| top_n | integer | No | 10 | Number of top candidates to return |
| required_skills | array | No | [] | Mandatory skills for filtering |
| min_experience_years | integer | No | 0 | Minimum years of experience |
| output_file | string | No | resume_screening_report.json | Output file path |

## Nodes Used

- **agent** (×2): Parse and extract resume data; score and rank candidates against job description
- **transform**: Filter candidates by required skills and experience
- **code_interpreter**: Generate summary statistics and score distribution
- **file_write**: Save final ranked report as JSON

## Category

HR > Recruitment