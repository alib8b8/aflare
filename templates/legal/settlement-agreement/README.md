# Settlement Agreement Generator

## Description
A settlement agreement and release generator for dispute resolution. Analyzes the dispute context and litigation risks, drafts comprehensive settlement agreements with tailored release provisions, and reviews for legal enforceability to ensure the agreement withstands challenge.

## Usage Example
```yaml
workflow: legal/settlement-agreement
params:
  plaintiff: "Jane Doe"
  defendant: "RetailCorp Inc"
  case_reference: "Case No. 2025-CV-00452"
  dispute_description: "Personal injury claim arising from slip and fall incident at defendant's store on March 15, 2025."
  settlement_amount: "$75,000 payable within 30 days of execution"
  settlement_terms:
    - "Full release of all claims arising from the incident"
    - "Confidentiality of settlement terms"
    - "Non-disparagement mutual"
    - "No admission of liability"
    - "Each party bears own costs"
  release_scope: "all claims"
  confidentiality: true
  governing_law: "State of California"
  output_path: "output/settlement_agreement.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| plaintiff | string | Yes | - | Name of the plaintiff/claimant |
| defendant | string | Yes | - | Name of the defendant |
| case_reference | string | No | - | Case or claim reference number |
| dispute_description | string | Yes | - | Brief description of the dispute |
| settlement_amount | string | Yes | - | Settlement amount and payment terms |
| settlement_terms | array | Yes | - | Key settlement terms |
| release_scope | string | No | all claims | Scope of release |
| confidentiality | boolean | No | true | Whether terms are confidential |
| governing_law | string | No | State of Delaware | Governing law |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/settlement_agreement.md | Output file path |

## Nodes Used
- **agent** (analyze_dispute): Analyzes the dispute and settlement context
- **agent** (generate_agreement): Generates the settlement agreement and release
- **agent** (review_enforceability): Reviews the agreement for enforceability
- **file_write** (save_agreement): Saves the settlement agreement

## Category
legal