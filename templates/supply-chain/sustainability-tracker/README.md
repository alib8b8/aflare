# Sustainability Tracker

Supply chain carbon footprint tracking and reporting aligned with the GHG Protocol.

## Description

This workflow fetches current emission factors from regulatory databases, computes Scope 1 (direct), Scope 2 (energy), and Scope 3 (value chain) emissions across transportation modes, warehousing, packaging, and employee commuting. It calculates carbon intensity metrics and generates an AI-powered reduction roadmap with SBTi alignment and carbon offset strategies.

## Usage

```yaml
params:
  supply_chain_data: '{"scope1":[{"type":"fuel_diesel","quantity":1000,"unit":"liters","description":"Fleet diesel"}],"scope2":[{"type":"electricity","quantity":50000,"unit":"kWh"}],"scope3":[{"type":"air_freight","quantity":200000,"unit":"ton-km"}],"revenue":50000000}'
  emission_factors_api: "https://api.epa.gov/emission-factors"
  reporting_period: "monthly"
  scope_boundaries: '["scope1","scope2","scope3"]'
  output_file: "/tmp/carbon_footprint.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| supply_chain_data | string | yes | - | JSON with supply chain activities |
| emission_factors_api | string | yes | - | API endpoint for emission factors |
| reporting_period | string | no | monthly | Reporting period granularity |
| scope_boundaries | string | no | all three | JSON array of GHG Protocol scopes |
| output_file | string | no | /tmp/carbon_footprint.json | Output file |

## Nodes Used

- **http_request** - Fetches emission factors from regulatory database
- **code_interpreter** - Computes carbon footprint across all scopes
- **agent** - Generates carbon reduction roadmap
- **file_write** - Saves carbon footprint report to output file

## Category

supply-chain