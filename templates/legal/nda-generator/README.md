# NDA Generator

## Description
An automated Non-Disclosure Agreement (NDA) generator that creates customized confidentiality agreements. The workflow uses AI to draft complete NDA clauses based on provided parameters and then validates the generated document for legal compliance.

## Usage Example
```yaml
workflow: legal/nda-generator
params:
  disclosing_party: "Acme Corp"
  receiving_party: "Beta Inc"
  purpose: "Evaluation of potential business partnership"
  duration_months: 36
  governing_law: "State of California"
  mutual: true
  output_path: "output/mutual_nda.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| disclosing_party | string | Yes | - | Name of the disclosing party |
| receiving_party | string | Yes | - | Name of the receiving party |
| purpose | string | Yes | - | Purpose of the disclosure |
| duration_months | integer | No | 24 | Confidentiality duration in months |
| governing_law | string | No | State of Delaware | Governing law jurisdiction |
| mutual | boolean | No | false | Whether this is a mutual NDA |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/nda_agreement.md | Output file path |

## Nodes Used
- **agent** (generate_clauses): Generates complete NDA text with all standard clauses
- **agent** (validate_compliance): Validates the NDA for legal completeness and compliance
- **file_write** (save_nda): Saves the final NDA document with compliance notes

## Category
legal