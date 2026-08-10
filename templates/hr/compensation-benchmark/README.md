# Compensation Benchmark

Salary benchmarking and market analysis workflow with compa-ratio calculations.

## Description

Benchmark positions against market data from multiple sources, compute compa-ratios and range penetration, identify salary gaps, and generate strategic adjustment recommendations with budget impact estimates.

## Usage Example

```yaml
params:
  positions:
    - title: "Senior Software Engineer"
      level: "L5"
      location: "San Francisco"
      current_salary: 185000
    - title: "Product Manager"
      level: "L4"
      location: "San Francisco"
      current_salary: 155000
    - title: "Data Scientist"
      level: "L4"
      location: "New York"
      current_salary: 160000
  industry: "technology"
  company_size: "mid"
  location: "San Francisco"
  market_data_source: "radford"
  target_percentile: 65
  currency: "USD"
  output_file: "comp_benchmark_report.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| positions | array | Yes | - | Positions with title, level, location, current salary |
| industry | string | Yes | - | Industry for benchmarking |
| location | string | Yes | - | Primary location |
| market_data_source | string | No | radford | radford, mercer, payscale, custom |
| company_size | string | No | mid | startup, small, mid, large, enterprise |
| target_percentile | integer | No | 50 | Target market percentile |
| currency | string | No | USD | Currency code |
| output_file | string | No | compensation_benchmark_report.json | Output file path |

## Nodes Used

- **http_request**: Fetch market data from compensation benchmarking API
- **code_interpreter**: Compute compa-ratios, range penetration, gaps, and summary statistics
- **agent**: Generate strategic recommendations and budget impact analysis
- **file_write**: Save benchmark report as JSON

## Category

HR > Compensation & Benefits