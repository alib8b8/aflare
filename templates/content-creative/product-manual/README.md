# Product Manual Generator

> Product user manual and documentation writer

## Description

This workflow template generates comprehensive product user manuals with getting started guides, feature documentation, troubleshooting, maintenance instructions, and appendices. Suitable for hardware products, software applications, and SaaS platforms.

## Usage

```bash
aflare install creative/product-manual
aflare run product-manual/workflow.yaml
```

## Parameters

| Parameter | Description | Required |
|-----------|-------------|----------|
| product_name | Product name | Yes |
| product_type | Product type (hardware, software, SaaS, etc.) | Yes |
| user_level | Target user skill level (beginner, intermediate, advanced) | Yes |
| features | Key features list | Yes |
| platform | Platform/OS if applicable | No |
| doc_format | Documentation format preference | No |
| brand_voice | Brand voice and tone | Yes |
| compliance | Compliance requirements (FCC, CE, etc.) | No |

## Nodes Used

- agent - AI agent node for intelligent processing
- transform - Data transformation and extraction
- template_render - Template rendering with Go templates
- file_write - Write output to files
- notify - Send notifications

## Category

content-creative