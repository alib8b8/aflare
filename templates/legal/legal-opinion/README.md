# Legal Opinion Draft Generator

## Description
A formal legal opinion draft generator that produces professional opinion letters. The workflow researches applicable law and precedents, analyzes facts against the legal framework, drafts formal opinion letters with proper citations, and calculates confidence scores for each legal conclusion.

## Usage Example
```yaml
workflow: legal/legal-opinion
params:
  matter: "Enforceability of Non-Compete Agreement in California"
  jurisdiction: "California"
  facts: "Employee signed a 2-year non-compete agreement upon joining as a software engineer. Employee left after 18 months to join a competitor in a different market segment."
  legal_questions:
    - "Is the non-compete agreement enforceable under California Business and Professions Code Section 16600?"
    - "Does the 'different market segment' constitute a valid exception?"
    - "What remedies are available to the employer?"
  opinion_type: "general"
  applicable_law: ["Cal. Bus. & Prof. Code § 16600", "Edwards v. Arthur Andersen LLP"]
  assumptions: ["Employee had access to trade secrets", "Employee was not a shareholder"]
  output_path: "output/legal_opinion.md"
```

## Parameters
| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| matter | string | Yes | - | Subject matter of the legal opinion |
| jurisdiction | string | Yes | - | Governing jurisdiction |
| facts | string | Yes | - | Statement of relevant facts |
| legal_questions | array | Yes | - | Legal questions to address |
| opinion_type | string | No | general | Type of opinion |
| applicable_law | array | No | [] | Specific laws to analyze |
| assumptions | array | No | [] | Key assumptions |
| model | string | No | gpt-4 | AI model to use |
| output_path | string | No | output/legal_opinion.md | Output file path |

## Nodes Used
- **agent** (research_law): Researches applicable law and precedents
- **agent** (analyze_facts): Analyzes facts against legal framework
- **agent** (draft_opinion): Drafts the formal legal opinion letter
- **code_interpreter** (confidence_scoring): Calculates confidence scores
- **file_write** (save_opinion): Saves the legal opinion document

## Category
legal