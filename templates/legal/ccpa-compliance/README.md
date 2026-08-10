# CCPA Compliance Workflow

## Description
A California Consumer Privacy Act (CCPA) compliance workflow that determines whether the CCPA applies to your organization, maps all consumer rights requirements, generates required notices (notice at collection, privacy policy, opt-out), and calculates a compliance score with actionable guidance.

## Usage Example
```yaml
workflow: legal/ccpa-compliance
params:
  organization_name: "DataCo LLC"
  annual_revenue: 50000000
  consumer_data_volume: 100000
  data_sales: true
  data_categories: ["identifiers", "commercial info", "internet activity", "geolocation"]
  output_path: "output/ccpa_report.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| organization_name | string | Yes | - | Name of the organization |
| annual_revenue | number | No | - | Annual gross revenue ($) |
| consumer_data_volume | integer | No | - | Number of consumers' data processed annually |
| data_sales | boolean | No | false | Whether organization sells personal info |
| data_categories | array | Yes | - | Categories of personal information collected |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/ccpa_compliance_report.md | Output file path |

## Nodes Used
- **agent** (applicability_check): Determines if CCPA applies to the organization
- **agent** (rights_requirements): Maps all CCPA consumer rights and requirements
- **agent** (notice_requirements): Generates required CCPA notice content
- **code_interpreter** (compliance_score): Calculates quantitative compliance score
- **file_write** (save_report): Saves the CCPA compliance report

## Category
legal