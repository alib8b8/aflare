# Vendor Risk

Multi-dimensional vendor risk assessment and continuous monitoring system.

## Description

This workflow fetches vendor risk intelligence from an external API, computes four-dimensional risk scores (financial, compliance, operational, geopolitical) with configurable thresholds, classifies vendors into Critical/High/Medium/Low risk tiers, and generates AI-powered mitigation strategies with immediate and long-term action plans.

## Usage

```yaml
params:
  vendor_list: '[{"vendor_id":"V-001","vendor_name":"Acme Supply Co"}]'
  risk_data_api: "https://api.riskintel.com/v1/vendor-assessment"
  risk_thresholds: '{"financial": 70, "compliance": 80, "operational": 60, "geopolitical": 50}'
  monitoring_frequency_days: 30
  output_file: "/tmp/vendor_risk_assessment.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| vendor_list | string | yes | - | JSON array of vendor IDs and names |
| risk_data_api | string | yes | - | API endpoint for vendor risk data |
| risk_thresholds | string | no | see above | JSON risk score thresholds |
| monitoring_frequency_days | integer | no | 30 | Days between risk reassessments |
| output_file | string | no | /tmp/vendor_risk_assessment.json | Output file |

## Nodes Used

- **http_request** - Fetches vendor risk intelligence data
- **code_interpreter** - Computes multi-dimensional risk scores and classifications
- **agent** - Generates mitigation strategies and action plans
- **file_write** - Saves risk assessment to output file

## Category

supply-chain