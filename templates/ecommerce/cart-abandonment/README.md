# Cart Abandonment Recovery

A cart abandonment recovery workflow that identifies abandoned carts, prioritizes high-value opportunities, generates personalized recovery messages, and executes multi-channel recovery campaigns.

## Description

This workflow template automates cart abandonment recovery:
- **Cart Detection**: Identifies abandoned carts based on configurable time thresholds
- **Smart Prioritization**: Scores carts by value, item count, and urgency
- **Incentive Optimization**: Calculates optimal incentive amounts per cart
- **Multi-Channel Recovery**: Supports email, push notifications, and SMS
- **Personalized Messaging**: AI-generated messages tailored to cart contents
- **Attempt Management**: Respects maximum recovery attempt limits

## Usage Example

```yaml
params:
  abandonment_threshold_minutes: 30
  max_recovery_attempts: 3
  incentive_budget: 5.00
  channels: ["email", "push"]
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| abandonment_threshold_minutes | integer | No | 30 | Minutes after cart activity to trigger recovery |
| max_recovery_attempts | integer | No | 3 | Maximum recovery contact attempts |
| incentive_budget | number | No | 5.00 | Maximum incentive value per cart |
| channels | array | No | [email, push] | Recovery channels to use |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for message generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch abandoned carts and execute recovery actions
- **code_interpreter**: Python-based cart prioritization and incentive calculation
- **agent**: AI-powered personalized recovery message generation
- **file_write**: Save recovery execution report

## Category

ecommerce