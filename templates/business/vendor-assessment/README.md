# Vendor Assessment

> Evaluate and score potential vendors/suppliers

## Description

This workflow template provides a ready-to-use solution for evaluate and score potential vendors/suppliers.

## Usage

```bash
aflare install business/vendor-assessment
aflare run vendor-assessment/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| vendor_name | Name of the vendor being assessed | Yes |
| industry | Industry sector of the vendor | Yes |
| product_service | Product or service provided by the vendor | Yes |
| annual_spend | Estimated annual spend | No |
| region | Geographic region of the vendor | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- researcher - Research and information gathering
- template_render - Template rendering with Go templates
- file_write - Write output to files

## Category

business