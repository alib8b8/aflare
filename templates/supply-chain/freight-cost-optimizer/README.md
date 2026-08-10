# Freight Cost Optimizer

Multi-carrier freight cost comparison and optimization with composite scoring.

## Description

This workflow fetches rate quotes from multiple carriers, computes composite scores weighting cost (40%), transit time (30%), and reliability (30%), identifies the best-value carrier, and generates AI-powered recommendations with cost-savings analysis, trade-off considerations, and sustainability insights.

## Usage

```yaml
params:
  shipment_details: '{"origin":"Los Angeles, CA","destination":"New York, NY","weight_kg":500,"freight_class":"class-70"}'
  carrier_rates_api: "https://api.freight.com/v1/quotes"
  service_levels: '["standard","expedited","overnight"]'
  max_cost_threshold: 0
  output_file: "/tmp/freight_optimization.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| shipment_details | string | yes | - | JSON with origin, destination, weight, dimensions, freight class |
| carrier_rates_api | string | yes | - | API endpoint for carrier rate quotes |
| service_levels | string | no | ["standard","expedited","overnight"] | JSON array of desired service levels |
| max_cost_threshold | number | no | 0 | Maximum acceptable cost (0 = no limit) |
| output_file | string | no | /tmp/freight_optimization.json | Output file |

## Nodes Used

- **http_request** - Fetches rate quotes from multiple carriers
- **code_interpreter** - Compares rates and computes composite scores
- **agent** - Generates carrier recommendation with trade-off analysis
- **file_write** - Saves optimization results to output file

## Category

supply-chain