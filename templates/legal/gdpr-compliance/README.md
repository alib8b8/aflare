# GDPR Compliance Checklist & Assessment

## Description
A comprehensive GDPR compliance assessment workflow tailored to your organization's data processing profile. Generates detailed compliance checklists with article references, performs Data Protection Impact Assessments (DPIA), conducts gap analysis, and provides a quantified readiness score with prioritized remediation actions.

## Usage Example
```yaml
workflow: legal/gdpr-compliance
params:
  organization_name: "TechCorp Inc"
  data_types: ["name", "email", "IP address", "location data", "payment info"]
  processing_purposes: ["service delivery", "marketing", "analytics"]
  data_subject_categories: ["customers", "employees", "website visitors"]
  cross_border: true
  dpo_required: true
  output_path: "output/gdpr_report.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| organization_name | string | Yes | - | Name of the organization being assessed |
| data_types | array | Yes | - | Types of personal data processed |
| processing_purposes | array | Yes | - | Purposes of data processing |
| data_subject_categories | array | No | [customers, employees] | Categories of data subjects |
| cross_border | boolean | No | false | Whether data is transferred internationally |
| dpo_required | boolean | No | false | Whether a DPO is required |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/gdpr_compliance_report.md | Output file path |

## Nodes Used
- **agent** (generate_checklist): Generates tailored GDPR compliance checklist
- **agent** (risk_assessment): Performs DPIA and risk evaluation
- **agent** (gap_analysis): Identifies gaps and creates remediation plan
- **code_interpreter** (calculate_readiness): Calculates quantitative readiness score
- **file_write** (save_report): Saves comprehensive compliance report

## Category
legal