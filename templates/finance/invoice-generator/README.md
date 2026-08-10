# Invoice Generator

> Automated invoice creation from transaction data with validation and professional formatting

## Description

This workflow template automates the creation of professional invoices from transaction data. It fetches customer details, generates line items, calculates taxes and totals, validates all amounts, and produces formatted invoices ready for distribution.

## Usage

```bash
aflare install finance/invoice-generator
aflare run invoice-generator/workflow.yaml \
  --params.transactions_file="/path/to/transactions.csv" \
  --params.company_name="Acme Corp" \
  --params.company_address="123 Main St, City, 00000" \
  --params.tax_id="TAX-123456" \
  --params.payment_terms="Net 30" \
  --params.currency="USD"
```

## Parameters

| Parameter | Required | Default | Description |
|-----------|----------|---------|-------------|
| transactions_file | Yes | - | Path to transactions data file |
| company_name | Yes | - | Company name for invoice header |
| company_address | Yes | - | Company address |
| tax_id | No | - | Tax identification number |
| payment_terms | No | Net 30 | Payment terms |
| due_date | No | +30 days | Invoice due date |
| currency | No | USD | Invoice currency |
| invoice_prefix | No | INV- | Invoice number prefix |
| customer_api_url | No | - | API URL for customer data |

## Nodes Used

- file_read - Read transaction data
- http_request - Fetch customer information
- agent - AI agent for invoice generation
- code_interpreter - Python-based calculation validation
- template_render - Render final invoice format
- file_write - Write output invoices
- notify - Send completion notification

## Category

finance