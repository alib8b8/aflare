# Procurement Workflow

End-to-end purchase order creation and procurement automation with approval logic.

## Description

This workflow validates requisition data, computes totals, determines approval requirements based on configurable thresholds, generates purchase order details with AI, submits to an ERP system via API, and sends automated notifications to procurement stakeholders.

## Usage

```yaml
params:
  requisition_data: '{"id":"REQ-001","items":[{"sku":"MAT-100","quantity":100,"unit_price":25.00}],"currency":"USD"}'
  approval_threshold: 5000
  erp_endpoint: "https://erp.example.com/api"
  notify_email: "procurement@example.com"
  output_file: "/tmp/purchase_order.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| requisition_data | string | yes | - | JSON with requisition details |
| approval_threshold | number | no | 5000 | Amount threshold requiring approval |
| erp_endpoint | string | yes | - | ERP system API endpoint |
| notify_email | string | no | "" | Email for procurement notifications |
| output_file | string | no | /tmp/purchase_order.json | Output file |

## Nodes Used

- **code_interpreter** - Validates requisition and computes totals
- **agent** - Generates PO details, payment terms, and delivery schedule
- **http_request** - Submits purchase order to ERP system
- **notify** - Sends procurement notification email
- **file_write** - Saves purchase order to output file

## Category

supply-chain