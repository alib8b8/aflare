# Terms of Use Generator

## Description
A comprehensive Terms of Use / Terms of Service generator that creates legally sound documents for various service types. The workflow researches applicable legal requirements based on the service type, generates complete terms with all necessary clauses, and performs a legal review to ensure enforceability and compliance.

## Usage Example
```yaml
workflow: legal/terms-of-use
params:
  company_name: "CloudApp Inc"
  service_type: "saas"
  features: ["file storage", "team collaboration", "API access", "data export"]
  has_user_content: true
  has_payments: true
  jurisdiction: "United States"
  output_path: "output/terms.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| company_name | string | Yes | - | Name of the company |
| service_type | string | Yes | - | Type of service (web-app, mobile-app, saas, marketplace, social-media) |
| features | array | Yes | - | List of service features |
| has_user_content | boolean | No | false | Whether users can upload content |
| has_payments | boolean | No | false | Whether the service processes payments |
| jurisdiction | string | No | United States | Governing jurisdiction |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/terms_of_use.md | Output file path |

## Nodes Used
- **agent** (research_requirements): Researches legal requirements for the service type
- **agent** (generate_terms): Generates the complete Terms of Use document
- **agent** (review_terms): Reviews generated terms for legal soundness
- **file_write** (save_terms): Saves the final document

## Category
legal