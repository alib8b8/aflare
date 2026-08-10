# Customs Clearance

Customs documentation generation and clearance workflow with HS code classification.

## Description

This workflow classifies commodities by HS codes, estimates duty rates and landed costs (including freight and insurance per Incoterms), checks for free trade agreement preferences, identifies restricted/licensed items, generates AI-powered customs documentation (commercial invoice, packing list, certificate of origin), and submits to a customs management system.

## Usage

```yaml
params:
  shipment_data: '{"shipment_id":"SHIP-001","origin_country":"CN","destination_country":"US","items":[{"hs_code":"8471.30.0100","description":"Laptop computers","quantity":10,"value":800,"country_of_origin":"CN"}]}'
  customs_api_endpoint: "https://api.customs.example.com/v1/submit"
  incoterms: "FOB"
  document_types: '["commercial_invoice","packing_list","certificate_of_origin","bill_of_lading"]'
  output_file: "/tmp/customs_documents.json"
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| shipment_data | string | yes | - | JSON with shipment details and HS codes |
| customs_api_endpoint | string | yes | - | API endpoint for customs submission |
| incoterms | string | no | FOB | Incoterms for the shipment |
| document_types | string | no | see above | JSON array of required documents |
| output_file | string | no | /tmp/customs_documents.json | Output file |

## Nodes Used

- **code_interpreter** - Classifies commodities and computes duties
- **agent** - Generates customs documentation
- **http_request** - Submits documents to customs system
- **file_write** - Saves customs documentation to output file

## Category

supply-chain