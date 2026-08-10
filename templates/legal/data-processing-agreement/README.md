# DPA (Data Processing Agreement) Generator

## Description
A GDPR-compliant Data Processing Agreement (DPA) generator that creates complete DPAs under Article 28. Includes generation of Standard Contractual Clauses (SCCs) for international transfers, comprehensive technical and organizational security measures (TOMs), and compliance validation.

## Usage Example
```yaml
workflow: legal/data-processing-agreement
params:
  controller_name: "HealthData Corp"
  processor_name: "CloudAnalytics Ltd"
  processing_purposes: ["data analytics", "storage", "backup"]
  data_categories: ["health records", "demographic data", "contact information"]
  data_subjects: ["patients", "healthcare providers"]
  duration: "Duration of the service agreement"
  subprocessors: ["AWS (EU region)", "Snowflake"]
  cross_border: true
  governing_law: "GDPR / Irish Law"
  output_path: "output/dpa.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| controller_name | string | Yes | - | Name of the data controller |
| processor_name | string | Yes | - | Name of the data processor |
| processing_purposes | array | Yes | - | Purposes of data processing |
| data_categories | array | Yes | - | Categories of personal data |
| data_subjects | array | Yes | - | Categories of data subjects |
| duration | string | No | Duration of the service agreement | Duration of processing |
| subprocessors | array | No | [] | List of authorized sub-processors |
| cross_border | boolean | No | false | Whether international transfers |
| governing_law | string | No | GDPR / EU Member State Law | Governing law |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/data_processing_agreement.md | Output file path |

## Nodes Used
- **agent** (generate_dpa): Generates the complete DPA under GDPR Article 28
- **agent** (generate_sccs): Generates SCCs for international transfers
- **agent** (generate_security_measures): Generates TOMs annex
- **agent** (validate_dpa): Validates DPA for GDPR compliance
- **file_write** (save_dpa): Saves the complete DPA document

## Category
legal