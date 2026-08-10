# Quality Inspection

Quality control inspection workflow with AQL (Acceptable Quality Level) sampling methodology.

## Description

This workflow computes statistically valid sample sizes and acceptance/rejection numbers based on AQL tables (ISO 2859). It generates a detailed AI-powered inspection checklist covering visual inspection, dimensional measurements, functional testing, and packaging verification, then submits the plan to a quality management system.

## Usage

```yaml
params:
  lot_data: '{"lot_size":500,"product_type":"standard","criteria":["visual","dimensional","functional"],"specs":{"weight":"100g","tolerance":"±2g"}}'
  aql_level: "II"
  aql_limit: 2.5
  inspection_api_endpoint: "https://api.example.com/qms/inspections"
  output_file: "/tmp/inspection_report.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| lot_data | string | yes | - | JSON with lot size, product specs, and criteria |
| aql_level | string | no | II | AQL inspection level |
| aql_limit | number | no | 2.5 | AQL acceptance limit percentage |
| inspection_api_endpoint | string | yes | - | API endpoint for submitting results |
| output_file | string | no | /tmp/inspection_report.json | Output file |

## Nodes Used

- **code_interpreter** - Computes AQL sample sizes and acceptance numbers
- **agent** - Generates detailed inspection checklist
- **http_request** - Submits plan to quality management system
- **file_write** - Saves inspection plan to output file

## Category

supply-chain