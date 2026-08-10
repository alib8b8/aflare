# IP Portfolio Management & Analysis

## Description
A comprehensive intellectual property portfolio management workflow. It categorizes IP assets by type (patents, trademarks, copyrights, trade secrets), performs gap analysis to identify protection weaknesses, estimates portfolio valuation, and generates strategic recommendations for commercialization and enforcement.

## Usage Example
```yaml
workflow: legal/intellectual-property
params:
  portfolio_name: "TechCore IP Portfolio"
  assets:
    - {type: "patent", title: "Neural Network Optimization", status: "granted", expiry: "2038-05-15"}
    - {type: "trademark", title: "TechCore", status: "registered", classes: ["9", "42"]}
    - {type: "trade_secret", title: "Training Data Pipeline", status: "active"}
  analysis_type: "comprehensive"
  industry: "technology"
  jurisdiction: "US"
  output_path: "output/ip_report.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| portfolio_name | string | Yes | - | Name of the IP portfolio |
| assets | array | Yes | - | List of IP assets with details |
| analysis_type | string | No | comprehensive | Analysis type (valuation, gap-analysis, risk-assessment, comprehensive) |
| industry | string | No | technology | Industry sector for competitive analysis |
| jurisdiction | string | No | US | Primary jurisdiction for IP protection |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/ip_portfolio_report.md | Output file path |

## Nodes Used
- **agent** (categorize_assets): Categorizes and classifies all IP assets by type
- **agent** (gap_analysis): Identifies gaps in IP protection coverage
- **code_interpreter** (valuation_estimate): Estimates portfolio valuation
- **agent** (strategy_recommendations): Generates strategic IP recommendations
- **file_write** (save_report): Saves the comprehensive report

## Category
legal