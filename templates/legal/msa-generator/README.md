# Master Service Agreement Generator

## Description
A comprehensive Master Service Agreement (MSA) generator that creates industry-specific commercial contracts. The workflow researches industry standards, generates complete MSAs with all standard legal clauses, and produces a negotiation playbook with fallback positions and talking points.

## Usage Example
```yaml
workflow: legal/msa-generator
params:
  provider_name: "CloudServe Technologies Inc"
  client_name: "Enterprise Solutions LLC"
  services: ["cloud hosting", "managed services", "technical support", "SLA monitoring"]
  industry: "technology"
  payment_terms: "Net 30"
  liability_cap: "Fees paid in the last 12 months"
  term_length: "36 months with auto-renewal"
  governing_law: "State of Delaware"
  output_path: "output/msa.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| provider_name | string | Yes | - | Name of the service provider |
| client_name | string | Yes | - | Name of the client |
| services | array | Yes | - | List of services covered |
| industry | string | No | technology | Industry sector |
| payment_terms | string | No | Net 30 | Payment terms |
| liability_cap | string | No | Fees paid in the last 12 months | Liability cap description |
| term_length | string | No | 12 months with auto-renewal | Agreement term |
| governing_law | string | No | State of Delaware | Governing law |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/master_service_agreement.md | Output file path |

## Nodes Used
- **agent** (research_industry_standards): Researches industry-specific MSA standards
- **agent** (generate_msa): Generates the complete Master Service Agreement
- **agent** (negotiate_playbook): Generates negotiation playbook
- **file_write** (save_msa): Saves the MSA and playbook

## Category
legal