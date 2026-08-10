# Post-Purchase Upsell Recommender

A post-purchase upsell recommendation engine that analyzes recent orders, customer profiles, and purchase history to generate optimized upsell offers.

## Description

This workflow template automates post-purchase upsell campaigns:
- **Timing Optimization**: Determines the best time to present upsell offers
- **Value-Based Filtering**: Ensures upsells are proportional to original purchase
- **Discount Calculation**: Smart discount rates based on relevance
- **Personalized Messaging**: AI-generated messages tailored to customer history
- **Multi-Type Support**: Post-purchase, delayed, subscription, and upgrade upsells

## Usage Example

```yaml
params:
  customer_id: "cust_12345"
  order_id: "ord_55555"
  upsell_type: "post_purchase"
  max_upsell_value: 1.5
  api_base: "https://api.example.com/v1"
  model: "gpt-4"
  timestamp: "2026-08-10T12:00:00Z"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| customer_id | string | Yes | - | Customer to generate upsell recommendations for |
| order_id | string | Yes | - | Recent order for context |
| upsell_type | string | No | post_purchase | Upsell timing (post_purchase, delayed, subscription, upgrade) |
| max_upsell_value | number | No | 1.5 | Maximum upsell as multiple of original purchase |
| api_base | string | Yes | - | Base URL for ecommerce API |
| model | string | No | gpt-4 | AI model for message generation |
| timestamp | string | No | - | Execution timestamp |

## Nodes Used

- **http_request**: Fetch order details, customer profile, upsell opportunities, and schedule campaign
- **code_interpreter**: Python-based offer optimization and timing calculation
- **agent**: AI-powered personalized upsell message generation
- **file_write**: Save upsell campaign report

## Category

ecommerce