# Order Fulfillment Automation

An automated order fulfillment workflow that orchestrates payment verification, inventory checking, shipping method selection, and fulfillment execution.

## Description

This workflow template streamlines the entire order fulfillment process with intelligent decision-making:
- **Payment Verification**: Confirms payment status before processing
- **Inventory Check**: Validates stock availability for all line items
- **Fulfillment Evaluation**: AI-powered assessment of fulfillment readiness
- **Auto-Fulfillment**: Optional automatic execution when all checks pass
- **Partial Fulfillment**: Handles split shipments for backordered items

## Usage Example

```yaml
params:
  order_id: "ord_55555"
  auto_fulfill: true
  shipping_method: "express"
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| order_id | string | Yes | - | Order to fulfill |
| auto_fulfill | boolean | No | false | Auto-fulfill if all checks pass |
| shipping_method | string | No | standard | Preferred shipping method |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for evaluation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch order details, verify payment, check inventory, execute fulfillment
- **agent**: AI-powered fulfillment evaluation and decision-making
- **file_write**: Save fulfillment report

## Category

ecommerce