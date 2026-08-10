# E-commerce Fraud Detection

An e-commerce fraud detection workflow that analyzes multiple risk signals to compute risk scores and automate fraud decisions.

## Description

This workflow template provides comprehensive fraud detection:
- **Multi-Signal Analysis**: AVS, CVV, IP geolocation, email domain, shipping-billing matching
- **Risk Scoring**: Weighted risk factor calculation with configurable thresholds
- **Automated Decisions**: Approve, review, or reject based on risk score
- **Velocity Checks**: Detects rapid-fire ordering patterns
- **Value Anomaly Detection**: Flags unusually high-value orders
- **AI Analysis Report**: Detailed explanation of risk factors and recommendations

## Usage Example

```yaml
params:
  order_id: "ord_99999"
  risk_threshold: 0.70
  auto_reject: false
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| order_id | string | Yes | - | Order to evaluate for fraud |
| risk_threshold | number | No | 0.70 | Risk score threshold for flagging |
| auto_reject | boolean | No | false | Auto-reject orders above threshold |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for fraud report generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch order details, fraud signals, and execute fraud actions
- **code_interpreter**: Python-based multi-factor risk scoring
- **agent**: AI-powered fraud analysis report generation
- **file_write**: Save fraud detection report

## Category

ecommerce