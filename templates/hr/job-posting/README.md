# Job Posting Generator

AI-powered job posting and description generator with inclusive language and platform optimization.

## Description

Create compelling, inclusive job postings with AI-generated content, platform-specific variants for LinkedIn/Indeed/careers pages, SEO metadata, and direct ATS publication. Ensures bias-free language and clear qualification differentiation.

## Usage Example

```yaml
params:
  job_title: "Senior Platform Engineer"
  department: "Infrastructure"
  location: "Remote (US)"
  employment_type: "Full-time"
  seniority_level: "Senior"
  key_responsibilities:
    - "Design and build scalable cloud infrastructure"
    - "Lead migration from monolith to microservices"
    - "Mentor junior engineers on platform best practices"
    - "Own SLOs for core platform services"
  required_qualifications:
    - "5+ years in platform/DevOps engineering"
    - "Expert knowledge of Kubernetes and Terraform"
    - "Strong programming in Go or Python"
    - "Experience with AWS/GCP at scale"
  preferred_qualifications:
    - "Experience with service mesh (Istio/Linkerd)"
    - "Contributions to open-source infrastructure projects"
    - "Knowledge of SOC2/HIPAA compliance"
  compensation_range:
    salary_min: 180000
    salary_max: 230000
    equity: "0.05% - 0.1%"
    benefits: "Health, 401k matching, unlimited PTO, remote stipend"
  company_overview: "Acme is a Series C startup building the future of..."
  target_platforms: ["linkedin", "indeed", "company_careers", "remote_ok"]
  output_file: "job_posting_senior_platform_engineer.md"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| job_title | string | Yes | - | Job title |
| department | string | Yes | - | Department name |
| location | string | Yes | - | Job location |
| employment_type | string | Yes | - | Full-time, Part-time, Contract, Internship |
| seniority_level | string | Yes | - | Entry, Mid, Senior, Lead, Director, Executive |
| key_responsibilities | array | Yes | - | Key responsibilities |
| required_qualifications | array | Yes | - | Required qualifications |
| preferred_qualifications | array | No | [] | Preferred qualifications |
| compensation_range | object | No | {} | Salary range and benefits |
| company_overview | string | Yes | - | Company description |
| target_platforms | array | No | [linkedin, indeed, company_careers] | Platforms to optimize |
| output_file | string | No | job_posting_{title}.md | Output file path |

## Nodes Used

- **agent** (×2): Generate inclusive job posting content; adapt for platform-specific formats
- **code_interpreter**: Generate SEO metadata, schema markup, and keywords
- **http_request**: Publish job posting to ATS system
- **file_write**: Save job posting with platform variants as markdown

## Category

HR > Recruitment